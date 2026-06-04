package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// Stats powers the admin dashboard overview — GET /api/v1/admin/stats. It sits
// behind RequireAdmin, so the JWT has already been verified by the time Get
// runs. Like the public handlers it keeps a narrow query interface so the
// dashboard counts are unit-testable with a fake (no DB needed).
type statsQueries interface {
	GetAdminStats(ctx context.Context) (store.GetAdminStatsRow, error)
}

// Stats serves the admin dashboard counts.
type Stats struct {
	Q statsQueries
}

// NewStats wires the handler against the live sqlc queries.
func NewStats(q statsQueries) *Stats {
	return &Stats{Q: q}
}

type statsDTO struct {
	TotalPosts     int64 `json:"total_posts"`
	PublishedPosts int64 `json:"published_posts"`
	DraftPosts     int64 `json:"draft_posts"`
	TotalProjects  int64 `json:"total_projects"`
}

// Get handles GET /api/v1/admin/stats and returns the dashboard counts. A post
// is "published" when published_at IS NOT NULL and a "draft" otherwise (there
// is no status column).
func (s *Stats) Get(w http.ResponseWriter, r *http.Request) {
	row, err := s.Q.GetAdminStats(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "admin stats", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, statsDTO{
		TotalPosts:     row.TotalPosts,
		PublishedPosts: row.PublishedPosts,
		DraftPosts:     row.DraftPosts,
		TotalProjects:  row.TotalProjects,
	})
}
