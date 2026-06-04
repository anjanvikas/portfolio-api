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

// adminProjectQueries is the slice of store.Queries the admin project CRUD
// needs, declared as an interface so the handler is unit-testable with a fake
// (the pattern every handler in this package follows). Tag replacement is the
// same clear-then-link approach as the blog editor: non-transactional, which is
// acceptable for a solo-author admin tool with rare writes.
type adminProjectQueries interface {
	ListAdminProjects(ctx context.Context) ([]store.ListAdminProjectsRow, error)
	GetAdminProject(ctx context.Context, id pgtype.UUID) (store.GetAdminProjectRow, error)
	CreateProject(ctx context.Context, arg store.CreateProjectParams) (pgtype.UUID, error)
	UpdateProject(ctx context.Context, arg store.UpdateProjectParams) (pgtype.UUID, error)
	PublishProject(ctx context.Context, id pgtype.UUID) (pgtype.UUID, error)
	DeleteProject(ctx context.Context, id pgtype.UUID) (int64, error)
	ClearProjectTags(ctx context.Context, projectID pgtype.UUID) error
	LinkProjectTag(ctx context.Context, arg store.LinkProjectTagParams) error
	UpsertTag(ctx context.Context, arg store.UpsertTagParams) (store.Tag, error)
	NextProjectSortOrder(ctx context.Context) (int32, error)
}

// AdminProjects groups the protected project CRUD handlers mounted under
// /api/v1/admin/projects (SCRUM-68). Every route sits behind RequireAdmin.
type AdminProjects struct {
	Q adminProjectQueries
}

// NewAdminProjects wires the handler against the live sqlc queries.
func NewAdminProjects(q adminProjectQueries) *AdminProjects {
	return &AdminProjects{Q: q}
}

// ---- request / response DTOs ---------------------------------------------

type adminProjectRequest struct {
	Title        string   `json:"title"`
	Slug         string   `json:"slug"`
	Tagline      string   `json:"tagline"`
	Summary      string   `json:"summary"`
	BodyOverview string   `json:"body_overview"`
	BodyWhyBuilt string   `json:"body_why_built"`
	BodyLearning string   `json:"body_learning"`
	CoverURL     string   `json:"cover_url"`
	RepoURL      string   `json:"repo_url"`
	LiveURL      string   `json:"live_url"`
	Featured     bool     `json:"featured"`
	Tags         []string `json:"tags"`
}

type adminProjectListItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
	Featured    bool    `json:"featured"`
	PublishedAt *string `json:"published_at"`
	SortOrder   int32   `json:"sort_order"`
}

type adminProjectDTO struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Slug         string   `json:"slug"`
	Tagline      string   `json:"tagline"`
	Summary      string   `json:"summary"`
	BodyOverview string   `json:"body_overview"`
	BodyWhyBuilt string   `json:"body_why_built"`
	BodyLearning string   `json:"body_learning"`
	CoverURL     string   `json:"cover_url"`
	RepoURL      string   `json:"repo_url"`
	LiveURL      string   `json:"live_url"`
	Featured     bool     `json:"featured"`
	Tags         []string `json:"tags"`
	Status       string   `json:"status"`
	PublishedAt  *string  `json:"published_at"`
}

// ---- list ----------------------------------------------------------------

// List handles GET /api/v1/admin/projects — every project (drafts + published)
// for the admin table, drafts first.
func (a *AdminProjects) List(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListAdminProjects(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list admin projects", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	out := make([]adminProjectListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminProjectListItem{
			ID:          uuidString(row.ID),
			Title:       row.Title,
			Slug:        row.Slug,
			Status:      statusOf(row.PublishedAt),
			Featured:    row.Featured,
			PublishedAt: timestampPtr(row.PublishedAt),
			SortOrder:   row.SortOrder,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- get one -------------------------------------------------------------

// Get handles GET /api/v1/admin/projects/{id}.
func (a *AdminProjects) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	row, err := a.Q.GetAdminProject(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get admin project", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, adminProjectFromRow(row))
}

// ---- create --------------------------------------------------------------

// Create handles POST /api/v1/admin/projects. New projects are drafts.
func (a *AdminProjects) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeProjectRequest(w, r)
	if !ok {
		return
	}
	f, ok := validateProjectRequest(w, req)
	if !ok {
		return
	}

	sortOrder, err := a.Q.NextProjectSortOrder(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "next project sort order", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	id, err := a.Q.CreateProject(r.Context(), store.CreateProjectParams{
		Slug:         f.slug,
		Title:        f.title,
		Tagline:      f.tagline,
		Summary:      f.summary,
		BodyOverview: f.bodyOverview,
		BodyWhyBuilt: f.bodyWhyBuilt,
		BodyLearning: f.bodyLearning,
		CoverUrl:     f.coverURL,
		RepoUrl:      nullText(f.repoURL),
		LiveUrl:      nullText(f.liveURL),
		Featured:     f.featured,
		SortOrder:    sortOrder,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a project with that slug already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "create project", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := a.syncTags(r.Context(), id, f.tags); err != nil {
		slog.ErrorContext(r.Context(), "sync project tags", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	a.respondWithProject(w, r, id, http.StatusCreated)
}

// ---- update --------------------------------------------------------------

// Update handles PUT /api/v1/admin/projects/{id}. Saves the editable fields;
// leaves the publish state untouched.
func (a *AdminProjects) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	req, ok := decodeProjectRequest(w, r)
	if !ok {
		return
	}
	f, ok := validateProjectRequest(w, req)
	if !ok {
		return
	}

	if _, err := a.Q.UpdateProject(r.Context(), store.UpdateProjectParams{
		ID:           id,
		Slug:         f.slug,
		Title:        f.title,
		Tagline:      f.tagline,
		Summary:      f.summary,
		BodyOverview: f.bodyOverview,
		BodyWhyBuilt: f.bodyWhyBuilt,
		BodyLearning: f.bodyLearning,
		CoverUrl:     f.coverURL,
		RepoUrl:      nullText(f.repoURL),
		LiveUrl:      nullText(f.liveURL),
		Featured:     f.featured,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a project with that slug already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "update project", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := a.syncTags(r.Context(), id, f.tags); err != nil {
		slog.ErrorContext(r.Context(), "sync project tags", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	a.respondWithProject(w, r, id, http.StatusOK)
}

// ---- publish -------------------------------------------------------------

// Publish handles POST /api/v1/admin/projects/{id}/publish.
func (a *AdminProjects) Publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	if _, err := a.Q.PublishProject(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		slog.ErrorContext(r.Context(), "publish project", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	a.respondWithProject(w, r, id, http.StatusOK)
}

// ---- delete --------------------------------------------------------------

// Delete handles DELETE /api/v1/admin/projects/{id}. Hard delete; tag links
// cascade.
func (a *AdminProjects) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r)
	if !ok {
		return
	}
	n, err := a.Q.DeleteProject(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete project", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- internals -----------------------------------------------------------

type projectFields struct {
	title        string
	slug         string
	tagline      string
	summary      string
	bodyOverview string
	bodyWhyBuilt string
	bodyLearning string
	coverURL     string
	repoURL      string
	liveURL      string
	featured     bool
	tags         []string
}

func decodeProjectRequest(w http.ResponseWriter, r *http.Request) (adminProjectRequest, bool) {
	var req adminProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return req, false
	}
	return req, true
}

// validateProjectRequest trims, validates, and normalises into DB-ready fields.
// Returns 400 with a per-field `errors` map on failure (the contact-form shape).
func validateProjectRequest(w http.ResponseWriter, req adminProjectRequest) (projectFields, bool) {
	var f projectFields
	fieldErrors := make(map[string]string)

	f.title = strings.TrimSpace(req.Title)
	if f.title == "" {
		fieldErrors["title"] = "Title is required."
	}

	f.slug = slugify(strings.TrimSpace(req.Slug))
	if f.slug == "" {
		f.slug = slugify(f.title)
	}
	if f.slug == "" {
		fieldErrors["slug"] = "Slug is required (add a title or a slug)."
	}

	f.tagline = strings.TrimSpace(req.Tagline)
	f.summary = strings.TrimSpace(req.Summary)
	// Body sections keep their whitespace (markdown is significant).
	f.bodyOverview = req.BodyOverview
	f.bodyWhyBuilt = req.BodyWhyBuilt
	f.bodyLearning = req.BodyLearning
	f.coverURL = strings.TrimSpace(req.CoverURL)
	f.repoURL = strings.TrimSpace(req.RepoURL)
	f.liveURL = strings.TrimSpace(req.LiveURL)
	f.featured = req.Featured
	f.tags = normaliseTags(req.Tags)

	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrors})
		return f, false
	}
	return f, true
}

// syncTags replaces a project's tag set with the submitted names (upsert by slug
// into the shared tag table, then re-link).
func (a *AdminProjects) syncTags(ctx context.Context, projectID pgtype.UUID, names []string) error {
	if err := a.Q.ClearProjectTags(ctx, projectID); err != nil {
		return err
	}
	for _, name := range names {
		tag, err := a.Q.UpsertTag(ctx, store.UpsertTagParams{Slug: slugify(name), Name: name})
		if err != nil {
			return err
		}
		if err := a.Q.LinkProjectTag(ctx, store.LinkProjectTagParams{ProjectID: projectID, TagID: tag.ID}); err != nil {
			return err
		}
	}
	return nil
}

// respondWithProject re-reads the project so the editor always gets the
// canonical persisted state (resolved tags + status).
func (a *AdminProjects) respondWithProject(w http.ResponseWriter, r *http.Request, id pgtype.UUID, status int) {
	row, err := a.Q.GetAdminProject(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "reload admin project", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, status, adminProjectFromRow(row))
}

func adminProjectFromRow(row store.GetAdminProjectRow) adminProjectDTO {
	return adminProjectDTO{
		ID:           uuidString(row.ID),
		Title:        row.Title,
		Slug:         row.Slug,
		Tagline:      row.Tagline,
		Summary:      row.Summary,
		BodyOverview: row.BodyOverview,
		BodyWhyBuilt: row.BodyWhyBuilt,
		BodyLearning: row.BodyLearning,
		CoverURL:     row.CoverUrl,
		RepoURL:      row.RepoUrl.String,
		LiveURL:      row.LiveUrl.String,
		Featured:     row.Featured,
		Tags:         row.Tags,
		Status:       statusOf(row.PublishedAt),
		PublishedAt:  timestampPtr(row.PublishedAt),
	}
}
