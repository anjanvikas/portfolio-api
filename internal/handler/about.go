package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// The /about page is powered by two independent read endpoints — the work
// history and the testimonials strip. They share this file because they back a
// single page, but each keeps its own narrow query interface so the handlers
// stay unit-testable with a fake (no DB needed).

// ---------------------------------------------------------------------------
// Experience timeline — GET /api/v1/experience
// ---------------------------------------------------------------------------

type experienceQueries interface {
	ListExperience(ctx context.Context) ([]store.Experience, error)
}

// Experience serves the public work-history timeline.
type Experience struct {
	Q experienceQueries
}

// NewExperience wires the handler against the live sqlc queries.
func NewExperience(q experienceQueries) *Experience {
	return &Experience{Q: q}
}

type experienceDTO struct {
	ID       string `json:"id"`
	Company  string `json:"company"`
	Role     string `json:"role"`
	Location string `json:"location"`
	// ISO date (YYYY-MM-DD).
	StartDate string `json:"start_date"`
	// ISO date or null. null means the role is current; the UI renders
	// "Present" for the end date.
	EndDate *string `json:"end_date"`
	// Markdown — rendered client-side.
	Description string `json:"description"`
}

// List handles GET /api/v1/experience. Entries come back in the order the
// timeline should display them (by sort_order, newest start first). An entry
// with a null end_date is a current role.
func (e *Experience) List(w http.ResponseWriter, r *http.Request) {
	rows, err := e.Q.ListExperience(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list experience", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	out := make([]experienceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, experienceDTO{
			ID:          uuidString(row.ID),
			Company:     row.Company,
			Role:        row.Role,
			Location:    row.Location,
			StartDate:   isoDateOnly(row.StartDate),
			EndDate:     isoDatePtr(row.EndDate),
			Description: row.Description,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Testimonials strip — GET /api/v1/testimonials
// ---------------------------------------------------------------------------

type testimonialQueries interface {
	ListVisibleTestimonials(ctx context.Context) ([]store.Testimonial, error)
}

// Testimonials serves the public testimonials strip.
type Testimonials struct {
	Q testimonialQueries
}

// NewTestimonials wires the handler against the live sqlc queries.
func NewTestimonials(q testimonialQueries) *Testimonials {
	return &Testimonials{Q: q}
}

type testimonialDTO struct {
	ID            string `json:"id"`
	AuthorName    string `json:"author_name"`
	AuthorRole    string `json:"author_role"`
	AuthorCompany string `json:"author_company"`
	Quote         string `json:"quote"`
}

// List handles GET /api/v1/testimonials, ordered by sort_order. Only rows
// flagged visible are returned (the admin toggle controls this, SCRUM-68).
func (t *Testimonials) List(w http.ResponseWriter, r *http.Request) {
	rows, err := t.Q.ListVisibleTestimonials(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list testimonials", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	out := make([]testimonialDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, testimonialDTO{
			ID:            uuidString(row.ID),
			AuthorName:    row.AuthorName,
			AuthorRole:    row.AuthorRole,
			AuthorCompany: row.AuthorCompany,
			Quote:         row.Quote,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
