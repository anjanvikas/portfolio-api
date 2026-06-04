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

	"github.com/anjanvikas/portfolio-api/internal/service"
	"github.com/anjanvikas/portfolio-api/internal/store"
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
	GetPublishedPostOGTarget(ctx context.Context, slug string) (store.GetPublishedPostOGTargetRow, error)
	GetProfile(ctx context.Context) (store.Profile, error)
	SetBlogPostOGImage(ctx context.Context, arg store.SetBlogPostOGImageParams) error
}

// Posts groups the public read-only blog-post handlers behind GET /api/v1/posts
// and GET /api/v1/posts/{slug}.
//
// OG, R2, and SiteURL are optional (SCRUM-69): when all three are wired the
// /og-image endpoint will lazily regenerate when og_image_url is empty. With
// any of them unset, that endpoint 404s on an unset image.
type Posts struct {
	Q          postQueries
	OG         service.OGImageGenerator
	R2         ogUploader
	SiteURL    string
	Normalizer urlNormalizer
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
			CoverURL:        nz(p.Normalizer, row.CoverKey),
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
	OGImageURL      string     `json:"og_image_url"`
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
		Body:            nzBody(p.Normalizer, row.Body),
		CoverURL:        nz(p.Normalizer, row.CoverKey),
		OGImageURL:      nz(p.Normalizer, row.OgImageUrl),
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

// OGImage handles GET /api/v1/posts/{slug}/og-image. 302-redirects to the saved
// og_image_url on the post (eager-generated at publish time). When the URL is
// empty — either OG generation failed at publish or the column was cleared to
// force a regenerate — it lazily renders, uploads, and persists the image,
// then redirects. Published posts only; 404 otherwise. 503 when the OG/R2
// pipeline isn't configured and no image exists yet.
func (p *Posts) OGImage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	row, err := p.Q.GetPublishedPostOGTarget(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		slog.ErrorContext(r.Context(), "og: load post", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if row.OgImageUrl != "" {
		http.Redirect(w, r, nz(p.Normalizer, row.OgImageUrl), http.StatusFound)
		return
	}

	// Lazy regenerate path. Requires the full pipeline; otherwise the post just
	// has no OG image yet.
	if p.OG == nil || p.R2 == nil || p.SiteURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "og image not yet generated"})
		return
	}

	prof, err := p.Q.GetProfile(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "og: load profile", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	png, err := p.OG.RenderPost(row.Title, prof.Name, p.SiteURL)
	if err != nil {
		slog.ErrorContext(r.Context(), "og: render", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "og render failed"})
		return
	}
	key := "og/posts/" + row.Slug + ".png"
	publicURL, err := p.R2.PutObject(r.Context(), key, "image/png", png)
	if err != nil {
		slog.ErrorContext(r.Context(), "og: upload", slog.String("error", err.Error()))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "og upload failed"})
		return
	}
	if err := p.Q.SetBlogPostOGImage(r.Context(), store.SetBlogPostOGImageParams{ID: row.ID, OgImageUrl: publicURL}); err != nil {
		// Persist failed but the object is up — still serve the URL once.
		slog.ErrorContext(r.Context(), "og: persist url", slog.String("error", err.Error()))
	}
	http.Redirect(w, r, publicURL, http.StatusFound)
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
