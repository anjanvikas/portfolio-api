package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

type fakeSeriesQ struct {
	list    []store.ListSeriesWithCountsRow
	listErr error

	series    store.BlogSeries
	seriesErr error

	posts    []store.ListPublishedPostsBySeriesSlugRow
	postsErr error

	lastSlug string
}

func (f *fakeSeriesQ) ListSeriesWithCounts(ctx context.Context) ([]store.ListSeriesWithCountsRow, error) {
	return f.list, f.listErr
}

func (f *fakeSeriesQ) GetBlogSeriesBySlug(ctx context.Context, slug string) (store.BlogSeries, error) {
	f.lastSlug = slug
	return f.series, f.seriesErr
}

func (f *fakeSeriesQ) ListPublishedPostsBySeriesSlug(ctx context.Context, slug string) ([]store.ListPublishedPostsBySeriesSlugRow, error) {
	return f.posts, f.postsErr
}

func order(n int32) pgtype.Int4 { return pgtype.Int4{Int32: n, Valid: true} }

func TestSeriesList_WrapsAndMapsTitle(t *testing.T) {
	q := &fakeSeriesQ{
		list: []store.ListSeriesWithCountsRow{
			{Slug: "building-carpilot", Name: "Building CarPilot", Description: "x", PostCount: 3},
		},
	}
	h := NewSeries(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/series", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Series []seriesSummaryDTO `json:"series"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Series) != 1 {
		t.Fatalf("len: got %d want 1", len(got.Series))
	}
	if got.Series[0].Title != "Building CarPilot" {
		t.Errorf("title (mapped from name): got %q", got.Series[0].Title)
	}
	if got.Series[0].PostCount != 3 {
		t.Errorf("post_count: got %d want 3", got.Series[0].PostCount)
	}
}

func TestSeriesList_EmptyIsArrayNotNull(t *testing.T) {
	q := &fakeSeriesQ{list: []store.ListSeriesWithCountsRow{}}
	h := NewSeries(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/series", nil))

	if body := rr.Body.String(); body != `{"series":[]}`+"\n" && body != `{"series":[]}` {
		t.Errorf("empty body: got %q want {\"series\":[]}", body)
	}
}

func TestSeriesDetail_Found(t *testing.T) {
	q := &fakeSeriesQ{
		series: store.BlogSeries{Slug: "building-carpilot", Name: "Building CarPilot", Description: "x"},
		posts: []store.ListPublishedPostsBySeriesSlugRow{
			{Title: "Part 1", Slug: "p1", SeriesOrder: order(1)},
			{Title: "Part 2", Slug: "p2", SeriesOrder: order(2)},
		},
	}
	h := NewSeries(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/series/building-carpilot", nil), "building-carpilot")
	h.Detail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if q.lastSlug != "building-carpilot" {
		t.Errorf("slug passed to query: got %q", q.lastSlug)
	}
	var got seriesDetailDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PostCount != 2 || len(got.Posts) != 2 {
		t.Fatalf("post_count/posts: got %d / %d want 2 / 2", got.PostCount, len(got.Posts))
	}
	if got.Posts[0].SeriesOrder != 1 || got.Posts[1].SeriesOrder != 2 {
		t.Errorf("order: got %d, %d", got.Posts[0].SeriesOrder, got.Posts[1].SeriesOrder)
	}
}

func TestSeriesDetail_NotFound(t *testing.T) {
	q := &fakeSeriesQ{seriesErr: pgx.ErrNoRows}
	h := NewSeries(q)
	rr := httptest.NewRecorder()
	req := withChiSlug(httptest.NewRequest(http.MethodGet, "/api/v1/series/ghost", nil), "ghost")
	h.Detail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404; body=%s", rr.Code, rr.Body.String())
	}
}
