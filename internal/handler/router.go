package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anjanvikas2001/portfolio-api/internal/auth"
	mw "github.com/anjanvikas2001/portfolio-api/internal/middleware"
	"github.com/anjanvikas2001/portfolio-api/internal/service"
	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// Deps bundles everything the HTTP layer needs from the rest of the app.
type Deps struct {
	Pool               *pgxpool.Pool
	CORSAllowedOrigins []string
	JWTSecret          string
	AdminPasswordHash  string
	CookieSecure       bool
	Mailer             service.Mailer
	// ResumePresigner is nil when R2 is not configured; the resume endpoint
	// then falls back to the stored resume_url.
	ResumePresigner *service.R2Presigner
	ResumeKey       string
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
	// Wire R2 presigning only when configured; the nil check keeps the handler
	// out of the nil-interface trap (a typed-nil pointer is a non-nil interface).
	if d.ResumePresigner != nil {
		profileH.Presigner = d.ResumePresigner
		profileH.ResumeKey = d.ResumeKey
	}
	contactH := NewContact(d.Mailer)
	projectsH := NewProjects(queries)
	postsH := NewPosts(queries)
	seriesH := NewSeries(queries)
	experienceH := NewExperience(queries)
	testimonialsH := NewTestimonials(queries)
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
				// Resource routes land here in later stories.
			})
		})
	})

	return r
}
