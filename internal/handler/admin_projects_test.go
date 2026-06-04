package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

type fakeAdminProjectQ struct {
	listRows []store.ListAdminProjectsRow
	getRow   store.GetAdminProjectRow

	createErr  error
	updateErr  error
	publishErr error
	deleteRows int64

	createParams store.CreateProjectParams
	updateParams store.UpdateProjectParams
	clearedTags  []pgtype.UUID
	upsertedTags []string
}

func (f *fakeAdminProjectQ) ListAdminProjects(context.Context) ([]store.ListAdminProjectsRow, error) {
	return f.listRows, nil
}
func (f *fakeAdminProjectQ) GetAdminProject(context.Context, pgtype.UUID) (store.GetAdminProjectRow, error) {
	return f.getRow, nil
}
func (f *fakeAdminProjectQ) CreateProject(_ context.Context, arg store.CreateProjectParams) (pgtype.UUID, error) {
	f.createParams = arg
	if f.createErr != nil {
		return pgtype.UUID{}, f.createErr
	}
	return f.getRow.ID, nil
}
func (f *fakeAdminProjectQ) UpdateProject(_ context.Context, arg store.UpdateProjectParams) (pgtype.UUID, error) {
	f.updateParams = arg
	if f.updateErr != nil {
		return pgtype.UUID{}, f.updateErr
	}
	return arg.ID, nil
}
func (f *fakeAdminProjectQ) PublishProject(_ context.Context, id pgtype.UUID) (pgtype.UUID, error) {
	if f.publishErr != nil {
		return pgtype.UUID{}, f.publishErr
	}
	return id, nil
}
func (f *fakeAdminProjectQ) DeleteProject(context.Context, pgtype.UUID) (int64, error) {
	return f.deleteRows, nil
}
func (f *fakeAdminProjectQ) ClearProjectTags(_ context.Context, id pgtype.UUID) error {
	f.clearedTags = append(f.clearedTags, id)
	return nil
}
func (f *fakeAdminProjectQ) LinkProjectTag(context.Context, store.LinkProjectTagParams) error {
	return nil
}
func (f *fakeAdminProjectQ) UpsertTag(_ context.Context, arg store.UpsertTagParams) (store.Tag, error) {
	f.upsertedTags = append(f.upsertedTags, arg.Name)
	return store.Tag{ID: pgtype.UUID{Valid: true}, Slug: arg.Slug, Name: arg.Name}, nil
}
func (f *fakeAdminProjectQ) NextProjectSortOrder(context.Context) (int32, error) { return 7, nil }

func validProjectRow() store.GetAdminProjectRow {
	id, _ := parseUUID(validUUID)
	return store.GetAdminProjectRow{ID: id, Title: "T", Slug: "t", Tags: []string{}}
}

func TestAdminProjectCreate_DerivesSlugAndSyncsTags(t *testing.T) {
	q := &fakeAdminProjectQ{getRow: validProjectRow()}
	h := NewAdminProjects(q)
	payload := `{"title":"My Cool App","repo_url":"https://github.com/x/y","tags":["Go","go","Postgres"]}`
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/projects", strings.NewReader(payload)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rr.Code, rr.Body.String())
	}
	if q.createParams.Slug != "my-cool-app" {
		t.Errorf("slug: got %q want my-cool-app", q.createParams.Slug)
	}
	if q.createParams.SortOrder != 7 {
		t.Errorf("sort order: got %d want 7 (NextProjectSortOrder)", q.createParams.SortOrder)
	}
	if !q.createParams.RepoUrl.Valid || q.createParams.RepoUrl.String == "" {
		t.Errorf("repo url should be a non-null text, got %+v", q.createParams.RepoUrl)
	}
	if len(q.upsertedTags) != 2 { // Go == go dedup
		t.Errorf("expected 2 unique tags, got %v", q.upsertedTags)
	}
	if len(q.clearedTags) != 1 {
		t.Errorf("expected tags cleared once, got %d", len(q.clearedTags))
	}
}

func TestAdminProjectCreate_MissingTitle(t *testing.T) {
	q := &fakeAdminProjectQ{getRow: validProjectRow()}
	rr := httptest.NewRecorder()
	NewAdminProjects(q).Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/projects", strings.NewReader(`{"summary":"x"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Errors["title"] == "" {
		t.Errorf("expected title error, got %+v", resp.Errors)
	}
}

func TestAdminProjectCreate_SlugConflict(t *testing.T) {
	q := &fakeAdminProjectQ{getRow: validProjectRow(), createErr: &pgconn.PgError{Code: "23505"}}
	rr := httptest.NewRecorder()
	NewAdminProjects(q).Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/projects", strings.NewReader(`{"title":"Dup"}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d want 409; body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminProjectUpdate_UnknownID(t *testing.T) {
	q := &fakeAdminProjectQ{getRow: validProjectRow(), updateErr: pgx.ErrNoRows}
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"title":"X"}`)), validUUID)
	NewAdminProjects(q).Update(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestAdminProjectList_StatusAndFeatured(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakeAdminProjectQ{listRows: []store.ListAdminProjectsRow{
		{ID: id, Title: "Draft", Slug: "draft"},
		{ID: id, Title: "Live", Slug: "live", Featured: true, PublishedAt: pgtype.Timestamptz{Valid: true}},
	}}
	rr := httptest.NewRecorder()
	NewAdminProjects(q).List(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	var got []adminProjectListItem
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if len(got) != 2 || got[0].Status != "draft" || got[1].Status != "published" || !got[1].Featured {
		t.Errorf("unexpected list mapping: %+v", got)
	}
}

func TestAdminProjectDelete(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withID(httptest.NewRequest(http.MethodDelete, "/x", nil), validUUID)
		NewAdminProjects(&fakeAdminProjectQ{deleteRows: 1}).Delete(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status: got %d want 204", rr.Code)
		}
	})
	t.Run("missing", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withID(httptest.NewRequest(http.MethodDelete, "/x", nil), validUUID)
		NewAdminProjects(&fakeAdminProjectQ{deleteRows: 0}).Delete(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status: got %d want 404", rr.Code)
		}
	})
}
