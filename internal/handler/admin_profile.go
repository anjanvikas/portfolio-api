package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// adminProfileQueries is the subset of store.Queries the profile editor needs.
type adminProfileQueries interface {
	GetProfile(ctx context.Context) (store.Profile, error)
	UpdateProfile(ctx context.Context, arg store.UpdateProfileParams) (store.Profile, error)
}

// AdminProfile serves the protected profile editor (GET + PUT) mounted under
// /api/v1/admin/profile (SCRUM-68). The profile is a singleton row.
type AdminProfile struct {
	Q adminProfileQueries
}

// NewAdminProfile wires the handler against the live sqlc queries.
func NewAdminProfile(q adminProfileQueries) *AdminProfile {
	return &AdminProfile{Q: q}
}

type adminProfileRequest struct {
	Name      string `json:"name"`
	Headline  string `json:"headline"`
	Bio       string `json:"bio"`
	Location  string `json:"location"`
	Email     string `json:"email"`
	ResumeURL string `json:"resume_url"`
	AvatarURL string `json:"avatar_url"`
}

type adminProfileDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Headline  string `json:"headline"`
	Bio       string `json:"bio"`
	Location  string `json:"location"`
	Email     string `json:"email"`
	ResumeURL string `json:"resume_url"`
	AvatarURL string `json:"avatar_url"`
}

// Get handles GET /api/v1/admin/profile.
func (a *AdminProfile) Get(w http.ResponseWriter, r *http.Request) {
	row, err := a.Q.GetProfile(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get profile", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminProfileFromRow(row))
}

// Update handles PUT /api/v1/admin/profile. Loads the singleton to resolve its
// id, then writes the edited fields.
func (a *AdminProfile) Update(w http.ResponseWriter, r *http.Request) {
	var req adminProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	fieldErrors := make(map[string]string)
	if name == "" {
		fieldErrors["name"] = "Name is required."
	}
	if email == "" {
		fieldErrors["email"] = "Email is required."
	} else if !strings.Contains(email, "@") {
		fieldErrors["email"] = "Enter a valid email address."
	}
	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrors})
		return
	}

	existing, err := a.Q.GetProfile(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
			return
		}
		slog.ErrorContext(r.Context(), "load profile for update", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	row, err := a.Q.UpdateProfile(r.Context(), store.UpdateProfileParams{
		ID:        existing.ID,
		Name:      name,
		Headline:  strings.TrimSpace(req.Headline),
		Bio:       req.Bio, // markdown — keep whitespace
		Location:  strings.TrimSpace(req.Location),
		Email:     email,
		ResumeUrl: nullText(strings.TrimSpace(req.ResumeURL)),
		AvatarUrl: nullText(strings.TrimSpace(req.AvatarURL)),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "that email is already in use"})
			return
		}
		slog.ErrorContext(r.Context(), "update profile", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminProfileFromRow(row))
}

func adminProfileFromRow(row store.Profile) adminProfileDTO {
	return adminProfileDTO{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		Headline:  row.Headline,
		Bio:       row.Bio,
		Location:  row.Location,
		Email:     row.Email,
		ResumeURL: row.ResumeUrl.String,
		AvatarURL: row.AvatarUrl.String,
	}
}
