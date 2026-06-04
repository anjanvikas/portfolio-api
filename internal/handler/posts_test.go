package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

type fakePostQ struct {
	cards    []store.ListPublishedPostCardsRow
	cardsErr error
	lastLim  int32

	detail    store.GetPublishedPostBySlugRow
	detailErr error
	lastSlug  string

	siblings       []store.ListPublishedPostsBySeriesSlugRow
	siblingsErr    error
	siblingsCalled bool
	lastSeriesSlug string

	ogTarget    store.GetPublishedPostOGTargetRow
	ogTargetErr error
	setOGCalls  []store.SetBlogPostOGImageParams
}

func (f *fakePostQ) ListPublishedPostCards(ctx context.Context, rowLimit int32) ([]store.ListPublishedPostCardsRow, error) {
	f.lastLim = rowLimit
	return f.cards, f.cardsErr
}

func (f *fakePostQ) GetPublishedPostBySlug(ctx context.Context, slug string) (store.GetPublishedPostBySlugRow, error) {
	f.lastSlug = slug
	return f.detail, f.detailErr
}

func (f *fakePostQ) ListPublishedPostsBySeriesSlug(ctx context.Context, slug string) ([]store.ListPublishedPostsBySeriesSlugRow, error) {
	f.siblingsCalled = true
	f.lastSeriesSlug = slug
	return f.siblings, f.siblingsErr
}

func (f *fakePostQ) GetPublishedPostOGTarget(_ context.Context, _ string) (store.GetPublishedPostOGTargetRow, error) {
	return f.ogTarget, f.ogTargetErr
}
func (f *fakePostQ) GetProfile(context.Context) (store.Profile, error) {
	return store.Profile{Name: "Test Author"}, nil
}
func (f *fakePostQ) SetBlogPostOGImage(_ context.Context, arg store.SetBlogPostOGImageParams) error {
	f.setOGCalls = append(f.setOGCalls, arg)
	return nil
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func TestPostsList_DefaultsAndClamp(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  int32
	}{
		{"no params", "", defaultPostLimit},
		{"zero falls back", "?limit=0", defaultPostLimit},
		{"garbage falls back", "?limit=abc", defaultPostLimit},
		{"over max clamps", "?limit=999", maxPostLimit},
		{"custom honored", "?limit=5", 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakePostQ{cards: []store.ListPublishedPostCardsRow{}}
			h := NewPosts(q)
			rr := httptest.NewRecorder()
			h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/posts"+tc.query, nil))
			if rr.Code != http.StatusOK {
				t.Fatalf("status: got %d want 200", rr.Code)
			}
			if q.lastLim != tc.want {
				t.Errorf("limit: got %d want %d", q.lastLim, tc.want)
			}
			if body := rr.Body.String(); body != "[]\n" && body != "[]" {
				t.Errorf("empty body: got %q want []", body)
			}
		})
	}
}

func TestPostsList_SeriesRefAndReadingTime(t *testing.T) {
	q := &fakePostQ{
		cards: []store.ListPublishedPostCardsRow{
			{Slug: "p2", Title: "Part 2", ReadingTimeMins: 9, Tags: []string{"go"},
				SeriesName: text("Building CarPilot"), SeriesSlug: text("building-carpilot"), SeriesOrder: order(2)},
			{Slug: "standalone", Title: "Standalone", ReadingTimeMins: 4, Tags: []string{}},
		},
	}
	h := NewPosts(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil))

	var got []postCardDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0].Series == nil || got[0].Series.Slug != "building-carpilot" || got[0].Series.Order != 2 {
		t.Errorf("series ref: got %+v", got[0].Series)
	}
	if got[0].ReadingTimeMins != 9 {
		t.Errorf("reading time: got %d want 9", got[0].ReadingTimeMins)
	}
	if got[1].Series != nil {
		t.Errorf("standalone series: got %+v want nil", got[1].Series)
	}
}

func TestPostDetail_SeriesPrevNext(t *testing.T) {
	q := &fakePostQ{
		detail: store.GetPublishedPostBySlugRow{
			Slug: "p2", Title: "Part 2", Body: "## h\n\nbody", ReadingTimeMins: 9,
			SeriesName: text("Building CarPilot"), SeriesSlug: text("building-carpilot"), SeriesOrder: order(2),
			Tags: []string{"go"},
		},
		siblings: []store.ListPublishedPostsBySeriesSlugRow{
			{Title: "Part 1", Slug: "p1", SeriesOrder: order(1)},
			{Title: "Part 2", Slug: "p2", SeriesOrder: order(2)},
			{Title: "Part 3", Slug: "p3", SeriesOrder: order(3)},
		},
	}
	h := NewPosts(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/p2", nil), "p2")
	h.Detail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !q.siblingsCalled || q.lastSeriesSlug != "building-carpilot" {
		t.Errorf("expected siblings query for series slug; called=%v slug=%q", q.siblingsCalled, q.lastSeriesSlug)
	}
	var got postDetailDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.SeriesPartCount != 3 {
		t.Errorf("part count: got %d want 3", got.SeriesPartCount)
	}
	if got.Prev == nil || got.Prev.Slug != "p1" {
		t.Errorf("prev: got %+v want p1", got.Prev)
	}
	if got.Next == nil || got.Next.Slug != "p3" {
		t.Errorf("next: got %+v want p3", got.Next)
	}
}

func TestPostDetail_StandaloneNoSiblingQuery(t *testing.T) {
	q := &fakePostQ{
		detail: store.GetPublishedPostBySlugRow{Slug: "solo", Title: "Solo", Body: "x", Tags: []string{}},
	}
	h := NewPosts(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/solo", nil), "solo")
	h.Detail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	if q.siblingsCalled {
		t.Errorf("standalone post should not query series siblings")
	}
	var got postDetailDTO
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if got.Series != nil || got.Prev != nil || got.Next != nil {
		t.Errorf("standalone nav should be null: %+v", got)
	}
}

func TestPostDetail_NotFound(t *testing.T) {
	q := &fakePostQ{detailErr: pgx.ErrNoRows}
	h := NewPosts(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/ghost", nil), "ghost")
	h.Detail(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestPostsOGImage_RedirectsToSavedURL(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakePostQ{ogTarget: store.GetPublishedPostOGTargetRow{
		ID: id, Slug: "hello", Title: "Hello", OgImageUrl: "https://example.test/og/posts/hello.png",
	}}
	h := NewPosts(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/hello/og-image", nil), "hello")
	h.OGImage(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status: got %d want 302", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "https://example.test/og/posts/hello.png" {
		t.Errorf("Location: got %q want saved URL", got)
	}
	if len(q.setOGCalls) != 0 {
		t.Errorf("must not regenerate when URL already set")
	}
}

func TestPostsOGImage_LazyRegenerateWhenURLEmpty(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakePostQ{ogTarget: store.GetPublishedPostOGTargetRow{
		ID: id, Slug: "hello", Title: "Hello world", OgImageUrl: "",
	}}
	og := &fakeOGGen{}
	r2 := &fakeOGR2{}
	h := NewPosts(q)
	h.OG = og
	h.R2 = r2
	h.SiteURL = "https://anjanvikasreddy.dev"

	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/hello/og-image", nil), "hello")
	h.OGImage(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status: got %d want 302; body=%s", rr.Code, rr.Body.String())
	}
	if r2.lastKey != "og/posts/hello.png" {
		t.Errorf("R2 key wrong: %q", r2.lastKey)
	}
	if len(q.setOGCalls) != 1 || q.setOGCalls[0].OgImageUrl == "" {
		t.Errorf("OG URL not persisted: %+v", q.setOGCalls)
	}
}

func TestPostsOGImage_NotFound(t *testing.T) {
	q := &fakePostQ{ogTargetErr: pgx.ErrNoRows}
	h := NewPosts(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/ghost/og-image", nil), "ghost")
	h.OGImage(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestPostsOGImage_503WhenPipelineNotWired(t *testing.T) {
	id, _ := parseUUID(validUUID)
	q := &fakePostQ{ogTarget: store.GetPublishedPostOGTargetRow{ID: id, Slug: "hello", Title: "Hello", OgImageUrl: ""}}
	h := NewPosts(q) // no OG/R2/SiteURL
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/posts/hello/og-image", nil), "hello")
	h.OGImage(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", rr.Code)
	}
}
