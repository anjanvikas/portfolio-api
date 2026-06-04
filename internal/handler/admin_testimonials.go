package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// adminTestimonialQueries is the subset of store.Queries the testimonial CRUD
// needs, as an interface so the handler is testable with a fake.
type adminTestimonialQueries interface {
	ListTestimonials(ctx context.Context) ([]store.Testimonial, error)
	GetTestimonial(ctx context.Context, id pgtype.UUID) (store.Testimonial, error)
	CreateTestimonial(ctx context.Context, arg store.CreateTestimonialParams) (store.Testimonial, error)
	UpdateTestimonial(ctx context.Context, arg store.UpdateTestimonialParams) (store.Testimonial, error)
	SetTestimonialVisibility(ctx context.Context, arg store.SetTestimonialVisibilityParams) (store.Testimonial, error)
	DeleteTestimonial(ctx context.Context, id pgtype.UUID) (int64, error)
	NextTestimonialSortOrder(ctx context.Context) (int32, error)
}

// AdminTestimonials groups the protected testimonial CRUD handlers mounted under
// /api/v1/admin/testimonials (SCRUM-68).
type AdminTestimonials struct {
	Q adminTestimonialQueries
}

// NewAdminTestimonials wires the handler against the live sqlc queries.
func NewAdminTestimonials(q adminTestimonialQueries) *AdminTestimonials {
	return &AdminTestimonials{Q: q}
}

type adminTestimonialRequest struct {
	AuthorName    string `json:"author_name"`
	AuthorRole    string `json:"author_role"`
	AuthorCompany string `json:"author_company"`
	Quote         string `json:"quote"`
	Visible       bool   `json:"visible"`
}

type adminTestimonialDTO struct {
	ID            string `json:"id"`
	AuthorName    string `json:"author_name"`
	AuthorRole    string `json:"author_role"`
	AuthorCompany string `json:"author_company"`
	Quote         string `json:"quote"`
	Visible       bool   `json:"visible"`
	SortOrder     int32  `json:"sort_order"`
}

// List handles GET /api/v1/admin/testimonials — every testimonial regardless of
// visibility, in display order.
func (a *AdminTestimonials) List(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListTestimonials(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list admin testimonials", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	out := make([]adminTestimonialDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminTestimonialFromRow(row))
	}
	writeJSON(w, http.StatusOK, out)
}

// Get handles GET /api/v1/admin/testimonials/{id}.
func (a *AdminTestimonials) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	row, err := a.Q.GetTestimonial(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "testimonial not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get testimonial", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminTestimonialFromRow(row))
}

// Create handles POST /api/v1/admin/testimonials.
func (a *AdminTestimonials) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeTestimonialRequest(w, r)
	if !ok {
		return
	}
	f, ok := validateTestimonialRequest(w, req)
	if !ok {
		return
	}

	sortOrder, err := a.Q.NextTestimonialSortOrder(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "next testimonial sort order", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	row, err := a.Q.CreateTestimonial(r.Context(), store.CreateTestimonialParams{
		AuthorName:    f.authorName,
		AuthorRole:    f.authorRole,
		AuthorCompany: f.authorCompany,
		Quote:         f.quote,
		Visible:       req.Visible,
		SortOrder:     sortOrder,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a testimonial with that author and quote already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "create testimonial", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusCreated, adminTestimonialFromRow(row))
}

// Update handles PUT /api/v1/admin/testimonials/{id}.
func (a *AdminTestimonials) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	req, ok := decodeTestimonialRequest(w, r)
	if !ok {
		return
	}
	f, ok := validateTestimonialRequest(w, req)
	if !ok {
		return
	}

	row, err := a.Q.UpdateTestimonial(r.Context(), store.UpdateTestimonialParams{
		ID:            id,
		AuthorName:    f.authorName,
		AuthorRole:    f.authorRole,
		AuthorCompany: f.authorCompany,
		Quote:         f.quote,
		Visible:       req.Visible,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "testimonial not found"})
			return
		}
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a testimonial with that author and quote already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "update testimonial", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminTestimonialFromRow(row))
}

// SetVisibility handles PATCH /api/v1/admin/testimonials/{id}/visibility — the
// list's quick visibility toggle. Body {visible: bool}.
func (a *AdminTestimonials) SetVisibility(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	var req struct {
		Visible bool `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	row, err := a.Q.SetTestimonialVisibility(r.Context(), store.SetTestimonialVisibilityParams{ID: id, Visible: req.Visible})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "testimonial not found"})
			return
		}
		slog.ErrorContext(r.Context(), "set testimonial visibility", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminTestimonialFromRow(row))
}

// Delete handles DELETE /api/v1/admin/testimonials/{id}.
func (a *AdminTestimonials) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	n, err := a.Q.DeleteTestimonial(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete testimonial", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "testimonial not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- internals -----------------------------------------------------------

type testimonialFields struct {
	authorName    string
	authorRole    string
	authorCompany string
	quote         string
}

func decodeTestimonialRequest(w http.ResponseWriter, r *http.Request) (adminTestimonialRequest, bool) {
	var req adminTestimonialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return req, false
	}
	return req, true
}

func validateTestimonialRequest(w http.ResponseWriter, req adminTestimonialRequest) (testimonialFields, bool) {
	var f testimonialFields
	fieldErrors := make(map[string]string)

	f.authorName = strings.TrimSpace(req.AuthorName)
	if f.authorName == "" {
		fieldErrors["author_name"] = "Author name is required."
	}
	f.quote = strings.TrimSpace(req.Quote)
	if f.quote == "" {
		fieldErrors["quote"] = "Quote is required."
	}
	f.authorRole = strings.TrimSpace(req.AuthorRole)
	f.authorCompany = strings.TrimSpace(req.AuthorCompany)

	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrors})
		return f, false
	}
	return f, true
}

func adminTestimonialFromRow(row store.Testimonial) adminTestimonialDTO {
	return adminTestimonialDTO{
		ID:            uuidString(row.ID),
		AuthorName:    row.AuthorName,
		AuthorRole:    row.AuthorRole,
		AuthorCompany: row.AuthorCompany,
		Quote:         row.Quote,
		Visible:       row.Visible,
		SortOrder:     row.SortOrder,
	}
}
