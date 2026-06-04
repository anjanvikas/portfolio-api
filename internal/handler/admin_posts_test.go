package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/content"
	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// fakeAdminPostQ is a configurable fake implementing adminPostQueries and
// recording the writes the handler makes, so tests can assert on the saved
// reading time and the tag-sync sequence without a database.
type fakeAdminPostQ struct {
	listRows []store.ListAdminPostsRow
	getRow   store.GetAdminPostRow
	getErr   error

	createErr  error
	updateErr  error
	publishErr error
	deleteRows int64
	deleteErr  error

	createParams store.CreateBlogPostParams
	updateParams store.UpdateBlogPostParams
	clearedTags  []pgtype.UUID
	linkedTags   []string
	upsertedTags []string
	setOGCalls   []store.SetBlogPostOGImageParams
}

func (f *fakeAdminPostQ) ListAdminPosts(context.Context) ([]store.ListAdminPostsRow, error) {
	return f.listRows, nil
}
func (f *fakeAdminPostQ) GetAdminPost(context.Context, pgtype.UUID) (store.GetAdminPostRow, error) {
	return f.getRow, f.getErr
}
func (f *fakeAdminPostQ) CreateBlogPost(_ context.Context, arg store.CreateBlogPostParams) (store.BlogPost, error) {
	f.createParams = arg
	if f.createErr != nil {
		return store.BlogPost{}, f.createErr
	}
	return store.BlogPost{ID: f.getRow.ID}, nil
}
func (f *fakeAdminPostQ) UpdateBlogPost(_ context.Context, arg store.UpdateBlogPostParams) (store.BlogPost, error) {
	f.updateParams = arg
	if f.updateErr != nil {
		return store.BlogPost{}, f.updateErr
	}
	return store.BlogPost{ID: arg.ID}, nil
}
func (f *fakeAdminPostQ) PublishBlogPost(_ context.Context, id pgtype.UUID) (store.BlogPost, error) {
	if f.publishErr != nil {
		return store.BlogPost{}, f.publishErr
	}
	// Mirror the loaded row so OG generation sees a real title + slug.
	return store.BlogPost{ID: id, Slug: f.getRow.Slug, Title: f.getRow.Title}, nil
}
func (f *fakeAdminPostQ) DeleteBlogPost(context.Context, pgtype.UUID) (int64, error) {
	return f.deleteRows, f.deleteErr
}
func (f *fakeAdminPostQ) ClearBlogPostTags(_ context.Context, id pgtype.UUID) error {
	f.clearedTags = append(f.clearedTags, id)
	return nil
}
func (f *fakeAdminPostQ) LinkBlogPostTag(_ context.Context, arg store.LinkBlogPostTagParams) error {
	return nil
}
func (f *fakeAdminPostQ) UpsertTag(_ context.Context, arg store.UpsertTagParams) (store.Tag, error) {
	f.upsertedTags = append(f.upsertedTags, arg.Name)
	f.linkedTags = append(f.linkedTags, arg.Slug)
	return store.Tag{ID: pgtype.UUID{Valid: true}, Slug: arg.Slug, Name: arg.Name}, nil
}
func (f *fakeAdminPostQ) ListAllSeries(context.Context) ([]store.ListAllSeriesRow, error) {
	return nil, nil
}
func (f *fakeAdminPostQ) ListTags(context.Context) ([]store.Tag, error) { return nil, nil }
func (f *fakeAdminPostQ) GetProfile(context.Context) (store.Profile, error) {
	return store.Profile{Name: "Test Author"}, nil
}
func (f *fakeAdminPostQ) SetBlogPostOGImage(_ context.Context, arg store.SetBlogPostOGImageParams) error {
	f.setOGCalls = append(f.setOGCalls, arg)
	return nil
}

// withID injects a chi route param so handlers reading {id} work in tests.
func withID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

const validUUID = "11111111-1111-1111-1111-111111111111"

func validRow() store.GetAdminPostRow {
	id, _ := parseUUID(validUUID)
	return store.GetAdminPostRow{ID: id, Title: "T", Slug: "t", ReadingTimeMins: 1}
}

func TestAdminCreate_SavesReadingTimeAndTags(t *testing.T) {
	body := strings.Repeat("word ", 450) // 450 words → ceil(450/200) = 3 min
	q := &fakeAdminPostQ{getRow: validRow()}
	h := NewAdminPosts(q)

	payload := `{"title":"Hello World","body":"` + strings.TrimSpace(body) + `","tags":["Go","go","  PgVector  "]}`
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts", strings.NewReader(payload)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rr.Code, rr.Body.String())
	}
	if want := content.ReadingTimeMins(strings.TrimSpace(body)); q.createParams.ReadingTimeMins != want {
		t.Errorf("reading time: got %d want %d", q.createParams.ReadingTimeMins, want)
	}
	if q.createParams.ReadingTimeMins != 3 {
		t.Errorf("expected 3 min for 450 words, got %d", q.createParams.ReadingTimeMins)
	}
	// Slug auto-derived from the title.
	if q.createParams.Slug != "hello-world" {
		t.Errorf("slug: got %q want hello-world", q.createParams.Slug)
	}
	// Draft on create.
	if q.createParams.PublishedAt.Valid {
		t.Errorf("new post should be a draft (published_at NULL)")
	}
	// Tags de-duplicated by slug (Go == go), whitespace trimmed.
	if len(q.upsertedTags) != 2 {
		t.Errorf("expected 2 unique tags, got %v", q.upsertedTags)
	}
	if len(q.clearedTags) != 1 {
		t.Errorf("expected tags cleared once before relink, got %d", len(q.clearedTags))
	}
}

func TestAdminCreate_MissingTitle(t *testing.T) {
	q := &fakeAdminPostQ{getRow: validRow()}
	h := NewAdminPosts(q)
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts", strings.NewReader(`{"body":"x"}`)))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Errors["title"] == "" {
		t.Errorf("expected a title field error, got %+v", resp.Errors)
	}
}

func TestAdminCreate_SlugConflict(t *testing.T) {
	q := &fakeAdminPostQ{getRow: validRow(), createErr: &pgconn.PgError{Code: "23505"}}
	h := NewAdminPosts(q)
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts", strings.NewReader(`{"title":"Dup"}`)))

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d want 409; body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminUpdate_UnknownID(t *testing.T) {
	q := &fakeAdminPostQ{getRow: validRow(), updateErr: pgx.ErrNoRows}
	h := NewAdminPosts(q)
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPut, "/api/v1/admin/posts/"+validUUID, strings.NewReader(`{"title":"X"}`)), validUUID)
	h.Update(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestAdminUpdate_BadID(t *testing.T) {
	q := &fakeAdminPostQ{getRow: validRow()}
	h := NewAdminPosts(q)
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPut, "/api/v1/admin/posts/nope", strings.NewReader(`{"title":"X"}`)), "not-a-uuid")
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
}

func TestAdminDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		q := &fakeAdminPostQ{deleteRows: 1}
		rr := httptest.NewRecorder()
		req := withID(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/posts/"+validUUID, nil), validUUID)
		NewAdminPosts(q).Delete(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status: got %d want 204", rr.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		q := &fakeAdminPostQ{deleteRows: 0}
		rr := httptest.NewRecorder()
		req := withID(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/posts/"+validUUID, nil), validUUID)
		NewAdminPosts(q).Delete(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status: got %d want 404", rr.Code)
		}
	})
}

func TestAdminPublish_UnknownID(t *testing.T) {
	q := &fakeAdminPostQ{publishErr: pgx.ErrNoRows}
	h := NewAdminPosts(q)
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts/"+validUUID+"/publish", nil), validUUID)
	h.Publish(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

// fakeOGGen + fakeOGUploader stand in for the SCRUM-69 OG pipeline so the
// publish test can verify the eager-generation step writes through to the DB.
type fakeOGGen struct{ lastTitle, lastAuthor, lastSite string }

func (g *fakeOGGen) RenderPost(title, author, site string) ([]byte, error) {
	g.lastTitle, g.lastAuthor, g.lastSite = title, author, site
	return []byte("fake-png-bytes"), nil
}
func (g *fakeOGGen) RenderHomepage(string, string, string) ([]byte, error) {
	return []byte("fake-home-png"), nil
}

type fakeOGR2 struct{ lastKey, lastCT string; bodyLen int }

func (r *fakeOGR2) PutObject(_ context.Context, key, ct string, body []byte) (string, error) {
	r.lastKey, r.lastCT, r.bodyLen = key, ct, len(body)
	return "https://example.test/" + key, nil
}

func TestAdminPublish_GeneratesAndSavesOGImage(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakeAdminPostQ{
		getRow: store.GetAdminPostRow{ID: id, Title: "Hello world", Slug: "hello", PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
	}
	og := &fakeOGGen{}
	r2 := &fakeOGR2{}
	h := NewAdminPosts(q)
	h.OG = og
	h.R2 = r2
	h.SiteURL = "https://anjanvikasreddy.dev"

	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts/"+validUUID+"/publish", nil), validUUID)
	h.Publish(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if og.lastTitle != "Hello world" || og.lastAuthor != "Test Author" || og.lastSite != "https://anjanvikasreddy.dev" {
		t.Errorf("OG render args wrong: %+v", og)
	}
	if r2.lastKey != "og/posts/hello.png" || r2.lastCT != "image/png" || r2.bodyLen == 0 {
		t.Errorf("R2 put wrong: %+v", r2)
	}
	if len(q.setOGCalls) != 1 || q.setOGCalls[0].OgImageUrl != "https://example.test/og/posts/hello.png" {
		t.Errorf("OG URL not persisted: %+v", q.setOGCalls)
	}
}

func TestAdminPublish_OGFailureDoesNotFailPublish(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakeAdminPostQ{
		getRow: store.GetAdminPostRow{ID: id, Title: "Hello", Slug: "hello", PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
	}
	og := &failingOGGen{}
	h := NewAdminPosts(q)
	h.OG = og
	h.R2 = &fakeOGR2{}
	h.SiteURL = "https://anjanvikasreddy.dev"

	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts/"+validUUID+"/publish", nil), validUUID)
	h.Publish(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("publish must succeed even when OG render fails; got %d", rr.Code)
	}
	if len(q.setOGCalls) != 0 {
		t.Errorf("must not persist OG URL on render failure: %+v", q.setOGCalls)
	}
}

type failingOGGen struct{}

func (failingOGGen) RenderPost(string, string, string) ([]byte, error) {
	return nil, errFake
}
func (failingOGGen) RenderHomepage(string, string, string) ([]byte, error) {
	return nil, errFake
}

var errFake = errors.New("boom")

func TestAdminList_StatusMapping(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakeAdminPostQ{listRows: []store.ListAdminPostsRow{
		{ID: id, Title: "Draft", Slug: "draft"},
		{ID: id, Title: "Live", Slug: "live", PublishedAt: pgtype.Timestamptz{Valid: true}},
	}}
	h := NewAdminPosts(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/admin/posts", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	var got []adminPostListItem
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || got[0].Status != "draft" || got[1].Status != "published" {
		t.Errorf("unexpected status mapping: %+v", got)
	}
	if got[0].PublishedAt != nil {
		t.Errorf("draft published_at should be null")
	}
}
