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

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// adminExperienceQueries is the subset of store.Queries the experience CRUD
// needs, as an interface so the handler is testable with a fake.
type adminExperienceQueries interface {
	ListExperience(ctx context.Context) ([]store.Experience, error)
	GetExperience(ctx context.Context, id pgtype.UUID) (store.Experience, error)
	CreateExperience(ctx context.Context, arg store.CreateExperienceParams) (store.Experience, error)
	UpdateExperience(ctx context.Context, arg store.UpdateExperienceParams) (store.Experience, error)
	DeleteExperience(ctx context.Context, id pgtype.UUID) (int64, error)
	SetExperienceSortOrder(ctx context.Context, arg store.SetExperienceSortOrderParams) error
	NextExperienceSortOrder(ctx context.Context) (int32, error)
}

// AdminExperience groups the protected experience CRUD handlers mounted under
// /api/v1/admin/experience (SCRUM-68).
type AdminExperience struct {
	Q adminExperienceQueries
}

// NewAdminExperience wires the handler against the live sqlc queries.
func NewAdminExperience(q adminExperienceQueries) *AdminExperience {
	return &AdminExperience{Q: q}
}

type adminExperienceRequest struct {
	Company     string  `json:"company"`
	Role        string  `json:"role"`
	Location    string  `json:"location"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Description string  `json:"description"`
}

type adminExperienceDTO struct {
	ID          string  `json:"id"`
	Company     string  `json:"company"`
	Role        string  `json:"role"`
	Location    string  `json:"location"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Description string  `json:"description"`
	SortOrder   int32   `json:"sort_order"`
}

// List handles GET /api/v1/admin/experience — every entry in display order
// (highest sort_order first), which is also the drag-to-reorder order.
func (a *AdminExperience) List(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListExperience(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list admin experience", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, experienceDTOs(rows))
}

// Get handles GET /api/v1/admin/experience/{id}.
func (a *AdminExperience) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	row, err := a.Q.GetExperience(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "experience not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get experience", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminExperienceFromRow(row))
}

// Create handles POST /api/v1/admin/experience. The new entry lands at the top
// of the list (highest sort_order).
func (a *AdminExperience) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeExperienceRequest(w, r)
	if !ok {
		return
	}
	f, ok := validateExperienceRequest(w, req)
	if !ok {
		return
	}

	sortOrder, err := a.Q.NextExperienceSortOrder(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "next experience sort order", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	row, err := a.Q.CreateExperience(r.Context(), store.CreateExperienceParams{
		Company:     f.company,
		Role:        f.role,
		Location:    f.location,
		StartDate:   f.startDate,
		EndDate:     f.endDate,
		Description: f.description,
		SortOrder:   sortOrder,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "an entry for that company, role and start date already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "create experience", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusCreated, adminExperienceFromRow(row))
}

// Update handles PUT /api/v1/admin/experience/{id}.
func (a *AdminExperience) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	req, ok := decodeExperienceRequest(w, r)
	if !ok {
		return
	}
	f, ok := validateExperienceRequest(w, req)
	if !ok {
		return
	}

	row, err := a.Q.UpdateExperience(r.Context(), store.UpdateExperienceParams{
		ID:          id,
		Company:     f.company,
		Role:        f.role,
		Location:    f.location,
		StartDate:   f.startDate,
		EndDate:     f.endDate,
		Description: f.description,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "experience not found"})
			return
		}
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "an entry for that company, role and start date already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "update experience", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminExperienceFromRow(row))
}

// Delete handles DELETE /api/v1/admin/experience/{id}.
func (a *AdminExperience) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	n, err := a.Q.DeleteExperience(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete experience", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "experience not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reorder handles POST /api/v1/admin/experience/reorder. The body is the full
// ordered list of ids, top to bottom as displayed. The first id is given the
// highest sort_order so the saved order matches the list's sort_order-DESC
// display. Returns the re-listed entries. This is the "display_order updated
// automatically when drag-to-reorder is saved" acceptance criterion.
func (a *AdminExperience) Reorder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids is required"})
		return
	}

	n := len(req.IDs)
	for i, raw := range req.IDs {
		id, err := parseUUID(strings.TrimSpace(raw))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id in ids"})
			return
		}
		// First item (top of the list) gets the largest sort_order.
		if err := a.Q.SetExperienceSortOrder(r.Context(), store.SetExperienceSortOrderParams{
			ID:        id,
			SortOrder: int32(n - 1 - i),
		}); err != nil {
			slog.ErrorContext(r.Context(), "set experience sort order", slog.String("error", err.Error()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
	}

	rows, err := a.Q.ListExperience(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list experience after reorder", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, experienceDTOs(rows))
}

// ---- internals -----------------------------------------------------------

type experienceFields struct {
	company     string
	role        string
	location    string
	startDate   pgtype.Date
	endDate     pgtype.Date
	description string
}

func decodeExperienceRequest(w http.ResponseWriter, r *http.Request) (adminExperienceRequest, bool) {
	var req adminExperienceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return req, false
	}
	return req, true
}

func validateExperienceRequest(w http.ResponseWriter, req adminExperienceRequest) (experienceFields, bool) {
	var f experienceFields
	fieldErrors := make(map[string]string)

	f.company = strings.TrimSpace(req.Company)
	if f.company == "" {
		fieldErrors["company"] = "Company is required."
	}
	f.role = strings.TrimSpace(req.Role)
	if f.role == "" {
		fieldErrors["role"] = "Role is required."
	}
	f.location = strings.TrimSpace(req.Location)
	f.description = req.Description // markdown — keep whitespace

	start := strings.TrimSpace(req.StartDate)
	if start == "" {
		fieldErrors["start_date"] = "Start date is required."
	} else if d, err := parseISODate(start); err != nil {
		fieldErrors["start_date"] = "Start date must be YYYY-MM-DD."
	} else {
		f.startDate = d
	}

	// End date is optional — absent/blank means a current role.
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		if d, err := parseISODate(strings.TrimSpace(*req.EndDate)); err != nil {
			fieldErrors["end_date"] = "End date must be YYYY-MM-DD."
		} else {
			f.endDate = d
		}
	}

	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrors})
		return f, false
	}
	return f, true
}

func experienceDTOs(rows []store.Experience) []adminExperienceDTO {
	out := make([]adminExperienceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminExperienceFromRow(row))
	}
	return out
}

func adminExperienceFromRow(row store.Experience) adminExperienceDTO {
	return adminExperienceDTO{
		ID:          uuidString(row.ID),
		Company:     row.Company,
		Role:        row.Role,
		Location:    row.Location,
		StartDate:   isoDateOnly(row.StartDate),
		EndDate:     isoDatePtr(row.EndDate),
		Description: row.Description,
		SortOrder:   row.SortOrder,
	}
}
