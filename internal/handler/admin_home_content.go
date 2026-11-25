package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

type adminHomeContentQueries interface {
	GetHomeContent(ctx context.Context) (store.HomeContent, error)
	UpsertHomeContent(ctx context.Context, arg store.UpsertHomeContentParams) (store.HomeContent, error)
}

// AdminHomeContent serves GET + PUT /api/v1/admin/home (SCRUM-116).
type AdminHomeContent struct {
	Q adminHomeContentQueries
}

func NewAdminHomeContent(q adminHomeContentQueries) *AdminHomeContent {
	return &AdminHomeContent{Q: q}
}

// Get returns the raw stored copy (not merged with defaults).
func (a *AdminHomeContent) Get(w http.ResponseWriter, r *http.Request) {
	dto, ok := loadHomeDTO(r, a.Q)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

// Update replaces the whole document.
func (a *AdminHomeContent) Update(w http.ResponseWriter, r *http.Request) {
	var req homeContentDTO
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if errs := validateHomeContent(&req); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": errs})
		return
	}
	doc, err := json.Marshal(req.homeContentDoc)
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal home content", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	row, err := a.Q.UpsertHomeContent(r.Context(), store.UpsertHomeContentParams{
		OpenToRoles: req.OpenToRoles,
		Content:     doc,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "upsert home content", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, homeDTOFromRow(r.Context(), row))
}
