package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// Default and ceiling for the ?limit query param on the blog list. The blog
// index loads a page of cards; the ceiling keeps an unbounded ?limit from
// scanning the whole table.
const (
	defaultPostLimit = 12
	maxPostLimit     = 50
)

// postQueries is the subset of store.Queries used by the public posts handler.
// An interface keeps the handler unit-testable with a fake.
type postQueries interface {
	ListPublishedPostCards(ctx context.Context, rowLimit int32) ([]store.ListPublishedPostCardsRow, error)
	GetPublishedPostBySlug(ctx context.Context, slug string) (store.GetPublishedPostBySlugRow, error)
	ListPublishedPostsBySeriesSlug(ctx context.Context, slug string) ([]store.ListPublishedPostsBySeriesSlugRow, error)
}

// Posts groups the public read-only blog-post handlers behind GET /api/v1/posts
// and GET /api/v1/posts/{slug}.
type Posts struct {
	Q postQueries
}

// NewPosts wires the handler against the live sqlc queries.
func NewPosts(q postQueries) *Posts {
	return &Posts{Q: q}
}

// seriesRef is the compact series descriptor embedded on a card or post when
// the post belongs to a series. Null on standalone posts.
type seriesRef struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Order int32  `json:"order"`
}

type postCardDTO struct {
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Excerpt         string     `json:"excerpt"`
	CoverURL        string     `json:"cover_url"`
	PublishedAt     string     `json:"published_at"`
	ReadingTimeMins int32      `json:"reading_time_mins"`
	Tags            []string   `json:"tags"`
	Series          *seriesRef `json:"series"`
}

// List handles GET /api/v1/posts. Returns published post cards newest-first,
// each with tags, reading time, and (when set) a compact series ref. Supports
// ?limit=N (default 12, max 50).
func (p *Posts) List(w http.ResponseWriter, r *http.Request) {
	limit := parsePostLimit(r.URL.Query().Get("limit"))

	rows, err := p.Q.ListPublishedPostCards(r.Context(), limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "list post cards", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	out := make([]postCardDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, postCardDTO{
			Slug:    row.Slug,
			Title:   row.Title,
			Excerpt: row.Excerpt,
			// cover_key carries the post's cover_url (set in the admin editor);
			// empty when unset, and the UI falls back to a chartreuse slab.
			CoverURL:        row.CoverKey,
			PublishedAt:     isoDate(row.PublishedAt),
			ReadingTimeMins: row.ReadingTimeMins,
			Tags:            row.Tags,
			Series:          cardSeriesRef(row.SeriesName, row.SeriesSlug, row.SeriesOrder),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// seriesNavDTO is one sibling post in the prev/next series navigation.
type seriesNavDTO struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	SeriesOrder int32  `json:"series_order"`
}

type postDetailDTO struct {
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Excerpt         string     `json:"excerpt"`
	Body            string     `json:"body"`
	CoverURL        string     `json:"cover_url"`
	PublishedAt     string     `json:"published_at"`
	ReadingTimeMins int32      `json:"reading_time_mins"`
	Tags            []string   `json:"tags"`
	Series          *seriesRef `json:"series"`
	// SeriesPartCount is the total number of published posts in the series
	// (the "of Y" in "Part X of Y"); 0 for a standalone post.
	SeriesPartCount int           `json:"series_part_count"`
	Prev            *seriesNavDTO `json:"prev"`
	Next            *seriesNavDTO `json:"next"`
}

// Detail handles GET /api/v1/posts/{slug}. Returns the full post body plus tag,
// series, and prev/next sibling info. 404 when the slug is unknown/unpublished.
func (p *Posts) Detail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	row, err := p.Q.GetPublishedPostBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get post by slug", slog.String("slug", slug), slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	dto := postDetailDTO{
		Slug:            row.Slug,
		Title:           row.Title,
		Excerpt:         row.Excerpt,
		Body:            row.Body,
		CoverURL:        row.CoverKey,
		PublishedAt:     isoDate(row.PublishedAt),
		ReadingTimeMins: row.ReadingTimeMins,
		Tags:            row.Tags,
		Series:          cardSeriesRef(row.SeriesName, row.SeriesSlug, row.SeriesOrder),
	}

	// Resolve prev/next siblings only for posts in a series. The published
	// posts come back ordered by series_order, so the current post's neighbours
	// in that slice are its prev/next.
	if dto.Series != nil {
		siblings, err := p.Q.ListPublishedPostsBySeriesSlug(r.Context(), row.SeriesSlug.String)
		if err != nil {
			slog.ErrorContext(r.Context(), "list series siblings", slog.String("slug", slug), slog.String("error", err.Error()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		dto.SeriesPartCount = len(siblings)
		for i, sib := range siblings {
			if sib.Slug != row.Slug {
				continue
			}
			if i > 0 {
				dto.Prev = navFrom(siblings[i-1])
			}
			if i < len(siblings)-1 {
				dto.Next = navFrom(siblings[i+1])
			}
			break
		}
	}

	writeJSON(w, http.StatusOK, dto)
}

// cardSeriesRef builds a *seriesRef from the nullable series columns, returning
// nil for a standalone post (no series).
func cardSeriesRef(name, slug pgtype.Text, order pgtype.Int4) *seriesRef {
	if !slug.Valid {
		return nil
	}
	return &seriesRef{Name: name.String, Slug: slug.String, Order: order.Int32}
}

func navFrom(p store.ListPublishedPostsBySeriesSlugRow) *seriesNavDTO {
	return &seriesNavDTO{Title: p.Title, Slug: p.Slug, SeriesOrder: p.SeriesOrder.Int32}
}

// parsePostLimit clamps ?limit into [1, maxPostLimit], defaulting when absent
// or unparseable.
func parsePostLimit(raw string) int32 {
	if raw == "" {
		return defaultPostLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultPostLimit
	}
	if n > maxPostLimit {
		return maxPostLimit
	}
	return int32(n)
}
