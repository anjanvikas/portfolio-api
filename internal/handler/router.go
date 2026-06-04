package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anjanvikas/portfolio-api/internal/auth"
	mw "github.com/anjanvikas/portfolio-api/internal/middleware"
	"github.com/anjanvikas/portfolio-api/internal/service"
	"github.com/anjanvikas/portfolio-api/internal/store"
)

// Deps bundles everything the HTTP layer needs from the rest of the app.
type Deps struct {
	Pool               *pgxpool.Pool
	CORSAllowedOrigins []string
	JWTSecret          string
	AdminPasswordHash  string
	CookieSecure       bool
	Mailer             service.Mailer
	// Presigner is nil when R2 is not configured; the resume endpoint then falls
	// back to the stored resume_url and the asset upload endpoints return 503.
	// One instance serves both resume GET presigning and asset PUT presigning.
	Presigner     *service.R2Presigner
	ResumeKey     string
	DocxConverter service.DocxConverter
	// OGGenerator + SiteURL together enable per-post OG image generation
	// (SCRUM-69). Both are optional; missing either disables the eager publish
	// step and makes the /og-image endpoint a 503.
	OGGenerator service.OGImageGenerator
	SiteURL     string
}

// NewRouter builds the top-level Chi router with the global middleware stack
// and mounts the versioned API under /api/v1.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(mw.RequestID)
	r.Use(mw.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(mw.Security)

	queries := store.New(d.Pool)
	health := &Health{Pool: d.Pool}
	profileH := NewProfile(queries)
	adminAssetsH := NewAdminAssets(queries)
	// Wire R2 presigning only when configured; the nil check keeps the handler
	// out of the nil-interface trap (a typed-nil pointer is a non-nil interface).
	if d.Presigner != nil {
		profileH.Presigner = d.Presigner
		profileH.ResumeKey = d.ResumeKey
		profileH.Normalizer = d.Presigner
		adminAssetsH.Presigner = d.Presigner
	}
	// Wire the OG image pipeline (SCRUM-69) only when both the generator and
	// the R2 presigner are available. Either missing leaves the per-post OG
	// fields nil, which the handlers treat as "feature disabled".
	ogReady := d.OGGenerator != nil && d.Presigner != nil && d.SiteURL != ""
	contactH := NewContact(d.Mailer)
	projectsH := NewProjects(queries)
	postsH := NewPosts(queries)
	if d.Presigner != nil {
		projectsH.Normalizer = d.Presigner
		postsH.Normalizer = d.Presigner
	}
	if ogReady {
		postsH.OG = d.OGGenerator
		postsH.R2 = d.Presigner
		postsH.SiteURL = d.SiteURL
	}
	seriesH := NewSeries(queries)
	experienceH := NewExperience(queries)
	testimonialsH := NewTestimonials(queries)
	statsH := NewStats(queries)
	adminPostsH := NewAdminPosts(queries)
	if ogReady {
		adminPostsH.OG = d.OGGenerator
		adminPostsH.R2 = d.Presigner
		adminPostsH.SiteURL = d.SiteURL
	}
	adminProjectsH := NewAdminProjects(queries)
	adminExperienceH := NewAdminExperience(queries)
	adminTestimonialsH := NewAdminTestimonials(queries)
	adminProfileH := NewAdminProfile(queries)
	adminConvertH := NewAdminConvert(d.DocxConverter)
	authH := NewAuth(AuthDeps{
		JWTSecret:         d.JWTSecret,
		AdminPasswordHash: d.AdminPasswordHash,
		Limiter:           auth.NewLoginRateLimiter(5, 15*time.Minute),
		CookieSecure:      d.CookieSecure,
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Method(http.MethodGet, "/health", health)

		// Public read endpoints powering the marketing site.
		r.Get("/profile", profileH.Get)
		r.Get("/profile/resume", profileH.Resume)
		r.Get("/projects", projectsH.List)
		r.Get("/projects/{slug}", projectsH.Detail)
		r.Get("/posts", postsH.List)
		r.Get("/posts/{slug}", postsH.Detail)
		r.Get("/posts/{slug}/og-image", postsH.OGImage)
		r.Get("/series", seriesH.List)
		r.Get("/series/{slug}", seriesH.Detail)
		r.Get("/experience", experienceH.List)
		r.Get("/testimonials", testimonialsH.List)
		r.Post("/contact", contactH.Submit)

		// Public auth endpoints — login is rate-limited and logout must work
		// even with an expired session, so neither sits behind RequireAdmin.
		r.Post("/admin/login", authH.Login)
		r.Post("/admin/logout", authH.Logout)

		// Protected admin subrouter — every route mounted here requires a
		// valid admin JWT.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAdmin(d.JWTSecret))
			r.Route("/admin", func(r chi.Router) {
				// Dashboard counts for the admin overview (SCRUM-65).
				r.Get("/stats", statsH.Get)

				// Blog post CRUD (SCRUM-66).
				r.Get("/posts", adminPostsH.List)
				r.Post("/posts", adminPostsH.Create)
				r.Get("/posts/{id}", adminPostsH.Get)
				r.Put("/posts/{id}", adminPostsH.Update)
				r.Delete("/posts/{id}", adminPostsH.Delete)
				r.Post("/posts/{id}/publish", adminPostsH.Publish)

				// Selectors backing the editor's series + tag inputs.
				r.Get("/series", adminPostsH.ListSeries)
				r.Get("/tags", adminPostsH.ListTags)

				// Project CRUD (SCRUM-68).
				r.Get("/projects", adminProjectsH.List)
				r.Post("/projects", adminProjectsH.Create)
				r.Get("/projects/{id}", adminProjectsH.Get)
				r.Put("/projects/{id}", adminProjectsH.Update)
				r.Delete("/projects/{id}", adminProjectsH.Delete)
				r.Post("/projects/{id}/publish", adminProjectsH.Publish)

				// Experience CRUD + drag-to-reorder (SCRUM-68).
				r.Get("/experience", adminExperienceH.List)
				r.Post("/experience", adminExperienceH.Create)
				r.Post("/experience/reorder", adminExperienceH.Reorder)
				r.Get("/experience/{id}", adminExperienceH.Get)
				r.Put("/experience/{id}", adminExperienceH.Update)
				r.Delete("/experience/{id}", adminExperienceH.Delete)

				// Testimonial CRUD + visibility toggle (SCRUM-68).
				r.Get("/testimonials", adminTestimonialsH.List)
				r.Post("/testimonials", adminTestimonialsH.Create)
				r.Get("/testimonials/{id}", adminTestimonialsH.Get)
				r.Put("/testimonials/{id}", adminTestimonialsH.Update)
				r.Delete("/testimonials/{id}", adminTestimonialsH.Delete)
				r.Patch("/testimonials/{id}/visibility", adminTestimonialsH.SetVisibility)

				// Profile editor (SCRUM-68).
				r.Get("/profile", adminProfileH.Get)
				r.Put("/profile", adminProfileH.Update)

				// Asset upload pipeline (SCRUM-67): presign a direct
				// browser→R2 PUT, register the uploaded object, list the
				// registry, soft-delete a row.
				r.Post("/assets/presign", adminAssetsH.Presign)
				r.Post("/assets", adminAssetsH.Register)
				r.Get("/assets", adminAssetsH.List)
				r.Delete("/assets/{id}", adminAssetsH.Delete)

				// Docx → markdown conversion (file never stored).
				r.Post("/convert/docx", adminConvertH.Docx)
			})
		})
	})

	return r
}
