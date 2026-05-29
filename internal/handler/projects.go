package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
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
			// cover_key is the R2 object key; until SCRUM-16 wires real asset
			// hosting the seed leaves it empty and the UI falls back to a
			// colored cover slab.
			CoverURL: row.CoverKey.String,
			Tags:     row.Tags,
		})
	}
	writeJSON(w, http.StatusOK, out)
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
