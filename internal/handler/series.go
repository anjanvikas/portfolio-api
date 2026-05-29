package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// seriesQueries is the subset of store.Queries used by the public series
// handler. An interface keeps the handler unit-testable with a fake.
type seriesQueries interface {
	ListSeriesWithCounts(ctx context.Context) ([]store.ListSeriesWithCountsRow, error)
	GetBlogSeriesBySlug(ctx context.Context, slug string) (store.BlogSeries, error)
	ListPublishedPostsBySeriesSlug(ctx context.Context, slug string) ([]store.ListPublishedPostsBySeriesSlugRow, error)
}

// Series groups the public read-only blog-series handlers that power series
// landing pages and the post renderer's prev/next navigation.
type Series struct {
	Q seriesQueries
}

// NewSeries wires the handler against the live sqlc queries.
func NewSeries(q seriesQueries) *Series {
	return &Series{Q: q}
}

// seriesSummaryDTO is one entry in the GET /api/v1/series list. The schema
// column is `name`; the AC names the field `title`, so the JSON key is `title`.
type seriesSummaryDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	PostCount int64  `json:"post_count"`
}

// List handles GET /api/v1/series. Returns every series that has at least one
// published post, each with its published-post count. Empty (all-draft) series
// are excluded by the query's inner join.
func (s *Series) List(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Q.ListSeriesWithCounts(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list series", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	out := make([]seriesSummaryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, seriesSummaryDTO{
			ID:        uuidString(row.ID),
			Title:     row.Name,
			Slug:      row.Slug,
			PostCount: row.PostCount,
		})
	}
	// Wrapped in an object per the AC: {"series": [...]}.
	writeJSON(w, http.StatusOK, map[string]any{"series": out})
}

type seriesPostDTO struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	SeriesOrder int32  `json:"series_order"`
	PublishedAt string `json:"published_at"`
}

type seriesDetailDTO struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	PostCount   int             `json:"post_count"`
	Posts       []seriesPostDTO `json:"posts"`
}

// Detail handles GET /api/v1/series/{slug}. Returns the series meta plus its
// full ordered list of published posts. 404 when the slug is unknown.
func (s *Series) Detail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	series, err := s.Q.GetBlogSeriesBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "series not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get series by slug", slog.String("slug", slug), slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	rows, err := s.Q.ListPublishedPostsBySeriesSlug(r.Context(), slug)
	if err != nil {
		slog.ErrorContext(r.Context(), "list series posts", slog.String("slug", slug), slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	posts := make([]seriesPostDTO, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, seriesPostDTO{
			Title:       row.Title,
			Slug:        row.Slug,
			SeriesOrder: row.SeriesOrder.Int32,
			PublishedAt: isoDate(row.PublishedAt),
		})
	}

	writeJSON(w, http.StatusOK, seriesDetailDTO{
		ID:          uuidString(series.ID),
		Title:       series.Name,
		Slug:        series.Slug,
		Description: series.Description,
		PostCount:   len(posts),
		Posts:       posts,
	})
}
