package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/content"
	"github.com/anjanvikas2001/portfolio-api/internal/service"
	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// ogUploader puts the generated PNG bytes into the object store from the server
// itself. R2Presigner satisfies it; tests can fake it. Kept narrow so the OG
// pipeline doesn't depend on the whole presigner API surface.
type ogUploader interface {
	PutObject(ctx context.Context, key, contentType string, body []byte) (string, error)
}

// adminPostQueries is the slice of store.Queries the admin blog CRUD needs,
// declared as an interface so the handler is unit-testable with a fake (the
// pattern every other handler in this package follows). Tag replacement is done
// as clear-then-link rather than in a transaction: this is a solo-author admin
// tool, the writes are rare, and keeping the interface DB-free keeps the tests
// honest. Transactional integrity is noted as a future improvement.
type adminPostQueries interface {
	ListAdminPosts(ctx context.Context) ([]store.ListAdminPostsRow, error)
	GetAdminPost(ctx context.Context, id pgtype.UUID) (store.GetAdminPostRow, error)
	CreateBlogPost(ctx context.Context, arg store.CreateBlogPostParams) (store.BlogPost, error)
	UpdateBlogPost(ctx context.Context, arg store.UpdateBlogPostParams) (store.BlogPost, error)
	PublishBlogPost(ctx context.Context, id pgtype.UUID) (store.BlogPost, error)
	DeleteBlogPost(ctx context.Context, id pgtype.UUID) (int64, error)
	ClearBlogPostTags(ctx context.Context, blogPostID pgtype.UUID) error
	LinkBlogPostTag(ctx context.Context, arg store.LinkBlogPostTagParams) error
	UpsertTag(ctx context.Context, arg store.UpsertTagParams) (store.Tag, error)
	ListAllSeries(ctx context.Context) ([]store.ListAllSeriesRow, error)
	ListTags(ctx context.Context) ([]store.Tag, error)
	GetProfile(ctx context.Context) (store.Profile, error)
	SetBlogPostOGImage(ctx context.Context, arg store.SetBlogPostOGImageParams) error
}

// AdminPosts groups the protected blog-post CRUD handlers mounted under
// /api/v1/admin/posts (SCRUM-66). Every route here sits behind RequireAdmin.
//
// OG, R2, and SiteURL are optional (SCRUM-69): when all three are set, the
// Publish handler eagerly renders a 1200x630 OG card and uploads it to R2 at
// `og/posts/<slug>.png`. A generation/upload failure is logged but does NOT
// fail the publish — the post still goes live, just without a saved OG image
// (the /og-image GET endpoint will regenerate lazily on first hit).
type AdminPosts struct {
	Q       adminPostQueries
	OG      service.OGImageGenerator
	R2      ogUploader
	SiteURL string
}

// NewAdminPosts wires the handler against the live sqlc queries.
func NewAdminPosts(q adminPostQueries) *AdminPosts {
	return &AdminPosts{Q: q}
}

// ---- request / response DTOs ---------------------------------------------

type adminPostRequest struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Excerpt     string   `json:"excerpt"`
	Body        string   `json:"body"`
	CoverURL    string   `json:"cover_url"`
	SeriesID    *string  `json:"series_id"`
	SeriesOrder *int32   `json:"series_order"`
	Tags        []string `json:"tags"`
}

// adminPostListItem is one row of the admin posts table.
type adminPostListItem struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	Status          string  `json:"status"`
	PublishedAt     *string `json:"published_at"`
	ReadingTimeMins int32   `json:"reading_time_mins"`
	Series          *string `json:"series"`
}

// adminPostDTO is the full post the editor loads and gets back after a save.
type adminPostDTO struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Excerpt         string   `json:"excerpt"`
	Body            string   `json:"body"`
	CoverURL        string   `json:"cover_url"`
	SeriesID        *string  `json:"series_id"`
	SeriesOrder     *int32   `json:"series_order"`
	Tags            []string `json:"tags"`
	Status          string   `json:"status"`
	PublishedAt     *string  `json:"published_at"`
	ReadingTimeMins int32    `json:"reading_time_mins"`
}

type adminSeriesDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ---- list ----------------------------------------------------------------

// List handles GET /api/v1/admin/posts. Returns every post (drafts + published)
// for the admin table, drafts first.
func (a *AdminPosts) List(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListAdminPosts(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list admin posts", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	out := make([]adminPostListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminPostListItem{
			ID:              uuidString(row.ID),
			Title:           row.Title,
			Slug:            row.Slug,
			Status:          statusOf(row.PublishedAt),
			PublishedAt:     timestampPtr(row.PublishedAt),
			ReadingTimeMins: row.ReadingTimeMins,
			Series:          textPtr(row.SeriesName),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- get one -------------------------------------------------------------

// Get handles GET /api/v1/admin/posts/{id}. Loads a single post into the editor.
func (a *AdminPosts) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	row, err := a.Q.GetAdminPost(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get admin post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminPostFromRow(row))
}

// ---- create --------------------------------------------------------------

// Create handles POST /api/v1/admin/posts. New posts are always drafts
// (published_at NULL); use the publish endpoint to go live.
func (a *AdminPosts) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodePostRequest(w, r)
	if !ok {
		return
	}
	fields, ok := validatePostRequest(w, req)
	if !ok {
		return
	}

	created, err := a.Q.CreateBlogPost(r.Context(), store.CreateBlogPostParams{
		Slug:            fields.slug,
		Title:           fields.title,
		Excerpt:         fields.excerpt,
		Body:            fields.body,
		CoverUrl:        fields.coverURL,
		SeriesID:        fields.seriesID,
		SeriesOrder:     fields.seriesOrder,
		PublishedAt:     pgtype.Timestamptz{}, // draft
		ReadingTimeMins: fields.readingTime,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a post with that slug already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "create blog post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := a.syncTags(r.Context(), created.ID, fields.tags); err != nil {
		slog.ErrorContext(r.Context(), "sync tags", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	a.respondWithPost(w, r, created.ID, http.StatusCreated)
}

// ---- update --------------------------------------------------------------

// Update handles PUT /api/v1/admin/posts/{id}. Saves the editable fields and
// recomputes reading time; leaves the publish state untouched ("Save Draft"
// never unpublishes a live post).
func (a *AdminPosts) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	req, ok := decodePostRequest(w, r)
	if !ok {
		return
	}
	fields, ok := validatePostRequest(w, req)
	if !ok {
		return
	}

	_, err := a.Q.UpdateBlogPost(r.Context(), store.UpdateBlogPostParams{
		ID:              id,
		Slug:            fields.slug,
		Title:           fields.title,
		Excerpt:         fields.excerpt,
		Body:            fields.body,
		CoverUrl:        fields.coverURL,
		SeriesID:        fields.seriesID,
		SeriesOrder:     fields.seriesOrder,
		ReadingTimeMins: fields.readingTime,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a post with that slug already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "update blog post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := a.syncTags(r.Context(), id, fields.tags); err != nil {
		slog.ErrorContext(r.Context(), "sync tags", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	a.respondWithPost(w, r, id, http.StatusOK)
}

// ---- publish -------------------------------------------------------------

// Publish handles POST /api/v1/admin/posts/{id}/publish. Sets published_at to
// now() the first time; re-publishing preserves the original date. After a
// successful publish, eagerly generates + uploads the OG image (best-effort —
// see GeneratePostOG).
func (a *AdminPosts) Publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	post, err := a.Q.PublishBlogPost(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		slog.ErrorContext(r.Context(), "publish blog post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	a.generatePostOG(r.Context(), post.ID, post.Slug, post.Title)
	a.respondWithPost(w, r, id, http.StatusOK)
}

// generatePostOG renders the OG card, uploads it to R2 at og/posts/<slug>.png,
// and persists the public URL on the post. Best-effort: any failure is logged
// and swallowed so a publish never fails because of OG generation. No-op when
// the OG pipeline isn't fully wired (R2 not configured, generator nil, etc).
func (a *AdminPosts) generatePostOG(ctx context.Context, id pgtype.UUID, slug, title string) {
	if a.OG == nil || a.R2 == nil || a.SiteURL == "" {
		return
	}
	prof, err := a.Q.GetProfile(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "og: load profile", slog.String("error", err.Error()))
		return
	}
	png, err := a.OG.RenderPost(title, prof.Name, a.SiteURL)
	if err != nil {
		slog.ErrorContext(ctx, "og: render", slog.String("error", err.Error()), slog.String("slug", slug))
		return
	}
	key := "og/posts/" + slug + ".png"
	publicURL, err := a.R2.PutObject(ctx, key, "image/png", png)
	if err != nil {
		slog.ErrorContext(ctx, "og: upload", slog.String("error", err.Error()), slog.String("slug", slug))
		return
	}
	if err := a.Q.SetBlogPostOGImage(ctx, store.SetBlogPostOGImageParams{ID: id, OgImageUrl: publicURL}); err != nil {
		slog.ErrorContext(ctx, "og: persist url", slog.String("error", err.Error()), slog.String("slug", slug))
		return
	}
	slog.InfoContext(ctx, "og image generated", slog.String("slug", slug), slog.String("url", publicURL))
}

// ---- delete --------------------------------------------------------------

// Delete handles DELETE /api/v1/admin/posts/{id}. Hard delete (tag links
// cascade). 404 when the id is unknown, 204 on success.
func (a *AdminPosts) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	n, err := a.Q.DeleteBlogPost(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete blog post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- selectors (series + tags) -------------------------------------------

// ListSeries handles GET /api/v1/admin/series — every series for the editor's
// series selector (including all-draft ones).
func (a *AdminPosts) ListSeries(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListAllSeries(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list series", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	out := make([]adminSeriesDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminSeriesDTO{ID: uuidString(row.ID), Slug: row.Slug, Name: row.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// ListTags handles GET /api/v1/admin/tags — all tag names for the autocomplete.
func (a *AdminPosts) ListTags(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListTags(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list tags", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Name)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- internals -----------------------------------------------------------

// postFields holds a validated, DB-ready request.
type postFields struct {
	title       string
	slug        string
	excerpt     string
	body        string
	coverURL    string
	seriesID    pgtype.UUID
	seriesOrder pgtype.Int4
	tags        []string
	readingTime int32
}

func decodePostRequest(w http.ResponseWriter, r *http.Request) (adminPostRequest, bool) {
	var req adminPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return req, false
	}
	return req, true
}

// validatePostRequest trims, validates, and normalises a request into DB-ready
// fields. Returns 400 with a per-field `errors` map on failure (mirrors the
// contact form's shape so the frontend renders inline errors the same way).
func validatePostRequest(w http.ResponseWriter, req adminPostRequest) (postFields, bool) {
	var f postFields
	fieldErrors := make(map[string]string)

	f.title = strings.TrimSpace(req.Title)
	if f.title == "" {
		fieldErrors["title"] = "Title is required."
	}

	// Slug: fall back to the slugified title when the field is blank.
	f.slug = slugify(strings.TrimSpace(req.Slug))
	if f.slug == "" {
		f.slug = slugify(f.title)
	}
	if f.slug == "" {
		fieldErrors["slug"] = "Slug is required (add a title or a slug)."
	}

	f.excerpt = strings.TrimSpace(req.Excerpt)
	f.body = req.Body // body keeps its whitespace (markdown is significant)
	f.coverURL = strings.TrimSpace(req.CoverURL)
	f.readingTime = content.ReadingTimeMins(f.body)
	f.tags = normaliseTags(req.Tags)

	// Series + order travel together (DB CHECK enforces both-or-neither).
	if req.SeriesID != nil && strings.TrimSpace(*req.SeriesID) != "" {
		sid, err := parseUUID(strings.TrimSpace(*req.SeriesID))
		if err != nil {
			fieldErrors["series_id"] = "Invalid series."
		} else {
			f.seriesID = sid
			order := int32(1)
			if req.SeriesOrder != nil && *req.SeriesOrder > 0 {
				order = *req.SeriesOrder
			}
			f.seriesOrder = pgtype.Int4{Int32: order, Valid: true}
		}
	}

	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrors})
		return f, false
	}
	return f, true
}

// syncTags replaces a post's tag set with the submitted names: each name is
// upserted into the shared tag table (keyed on its slug) and re-linked.
func (a *AdminPosts) syncTags(ctx context.Context, postID pgtype.UUID, names []string) error {
	if err := a.Q.ClearBlogPostTags(ctx, postID); err != nil {
		return err
	}
	for _, name := range names {
		tag, err := a.Q.UpsertTag(ctx, store.UpsertTagParams{Slug: slugify(name), Name: name})
		if err != nil {
			return err
		}
		if err := a.Q.LinkBlogPostTag(ctx, store.LinkBlogPostTagParams{BlogPostID: postID, TagID: tag.ID}); err != nil {
			return err
		}
	}
	return nil
}

// respondWithPost re-reads the post and writes it as the response body, so the
// editor always gets the canonical persisted state (resolved series + tags).
func (a *AdminPosts) respondWithPost(w http.ResponseWriter, r *http.Request, id pgtype.UUID, status int) {
	row, err := a.Q.GetAdminPost(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "reload admin post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, status, adminPostFromRow(row))
}

func adminPostFromRow(row store.GetAdminPostRow) adminPostDTO {
	dto := adminPostDTO{
		ID:              uuidString(row.ID),
		Title:           row.Title,
		Slug:            row.Slug,
		Excerpt:         row.Excerpt,
		Body:            row.Body,
		CoverURL:        row.CoverUrl,
		Tags:            row.Tags,
		Status:          statusOf(row.PublishedAt),
		PublishedAt:     timestampPtr(row.PublishedAt),
		ReadingTimeMins: row.ReadingTimeMins,
	}
	if row.SeriesID.Valid {
		sid := uuidString(row.SeriesID)
		dto.SeriesID = &sid
	}
	if row.SeriesOrder.Valid {
		order := row.SeriesOrder.Int32
		dto.SeriesOrder = &order
	}
	return dto
}

// ---- small shared helpers ------------------------------------------------

var slugNonWord = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases, replaces any run of non-alphanumerics with a single
// hyphen, and trims leading/trailing hyphens. Mirrors the frontend slugify so
// an auto-derived slug matches what the editor previews.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugNonWord.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// normaliseTags trims, drops blanks, and de-duplicates (case-insensitively by
// slug) while preserving first-seen order.
func normaliseTags(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		key := slugify(name)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

// statusOf maps the nullable published_at to the status badge value.
func statusOf(t pgtype.Timestamptz) string {
	if t.Valid {
		return "published"
	}
	return "draft"
}

// timestampPtr formats a published_at as an ISO date, or nil (JSON null) when
// the post is still a draft.
func timestampPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format("2006-01-02")
	return &s
}

// textPtr returns the string value of a nullable text column, or nil when NULL.
func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// parsePathUUID reads the {id} path param as a UUID, writing a 400 and
// returning ok=false when it is malformed.
func parsePathUUID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return id, false
	}
	return id, true
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — used to map a duplicate slug to a 409.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
