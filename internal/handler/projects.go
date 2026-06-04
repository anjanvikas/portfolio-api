package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// Default and ceiling for the ?limit query param. The homepage strip asks for
// 3; the ceiling keeps an unbounded ?limit from scanning the whole table.
const (
	defaultProjectLimit = 3
	maxProjectLimit     = 24
)

// projectQueries is the subset of store.Queries used by the public projects
// handler. An interface keeps the handler unit-testable with a fake.
type projectQueries interface {
	ListProjectCards(ctx context.Context, arg store.ListProjectCardsParams) ([]store.ListProjectCardsRow, error)
	GetProjectBySlug(ctx context.Context, slug string) (store.GetProjectBySlugRow, error)
}

// Projects groups the public read-only project handlers used by the homepage
// featured strip and the projects index.
type Projects struct {
	Q projectQueries
}

// NewProjects wires the handler against the live sqlc queries.
func NewProjects(q projectQueries) *Projects {
	return &Projects{Q: q}
}

type projectCardDTO struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	CoverURL string   `json:"cover_url"`
	Tags     []string `json:"tags"`
}

// List handles GET /api/v1/projects. It supports two query params:
//   - featured=true  → restrict to projects flagged for the homepage strip
//   - limit=N        → cap the result count (default 3, max 24)
//
// Results are ordered by sort_order then most-recently published.
func (p *Projects) List(w http.ResponseWriter, r *http.Request) {
	featured := r.URL.Query().Get("featured") == "true"
	limit := parseLimit(r.URL.Query().Get("limit"))

	rows, err := p.Q.ListProjectCards(r.Context(), store.ListProjectCardsParams{
		FeaturedOnly: featured,
		RowLimit:     limit,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "list project cards", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	out := make([]projectCardDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectCardDTO{
			Slug:    row.Slug,
			Title:   row.Title,
			Summary: row.Summary,
			// cover_key is the project's cover_url (empty when unset); the UI
			// falls back to a colored cover slab.
			CoverURL: row.CoverKey,
			Tags:     row.Tags,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type projectDetailDTO struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Tagline      string   `json:"tagline"`
	Summary      string   `json:"summary"`
	BodyOverview string   `json:"body_overview"`
	BodyWhyBuilt string   `json:"body_why_built"`
	BodyLearning string   `json:"body_learning"`
	CoverURL     string   `json:"cover_url"`
	RepoURL      string   `json:"repo_url"`
	LiveURL      string   `json:"live_url"`
	PublishedAt  string   `json:"published_at"`
	Tags         []string `json:"tags"`
}

// Detail handles GET /api/v1/projects/{slug}. It returns the full project —
// all three markdown body sections, repo/live links, and tags — for the
// project detail page. Returns 404 when the slug is unknown or unpublished.
func (p *Projects) Detail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	row, err := p.Q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get project by slug", slog.String("slug", slug), slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	dto := projectDetailDTO{
		Slug:         row.Slug,
		Title:        row.Title,
		Tagline:      row.Tagline,
		Summary:      row.Summary,
		BodyOverview: row.BodyOverview,
		BodyWhyBuilt: row.BodyWhyBuilt,
		BodyLearning: row.BodyLearning,
		// cover_key is the project's cover_url (empty when unset); the UI
		// falls back to a colored cover slab.
		CoverURL: row.CoverKey,
		RepoURL:  row.RepoUrl.String,
		LiveURL:  row.LiveUrl.String,
		Tags:     row.Tags,
	}
	if row.PublishedAt.Valid {
		dto.PublishedAt = row.PublishedAt.Time.Format("2006-01-02")
	}
	writeJSON(w, http.StatusOK, dto)
}

// parseLimit clamps the ?limit param into [1, maxProjectLimit], falling back to
// the default when absent or unparseable.
func parseLimit(raw string) int32 {
	if raw == "" {
		return defaultProjectLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultProjectLimit
	}
	if n > maxProjectLimit {
		return maxProjectLimit
	}
	return int32(n)
}
