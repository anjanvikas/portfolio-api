package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

type fakeProjectQ struct {
	rows    []store.ListProjectCardsRow
	err     error
	lastArg store.ListProjectCardsParams
}

func (f *fakeProjectQ) ListProjectCards(ctx context.Context, arg store.ListProjectCardsParams) ([]store.ListProjectCardsRow, error) {
	f.lastArg = arg
	return f.rows, f.err
}

func TestProjectsList_FeaturedAndLimit(t *testing.T) {
	q := &fakeProjectQ{
		rows: []store.ListProjectCardsRow{
			{Slug: "mealmind", Title: "Mealmind", Summary: "Recipe engine", Tags: []string{"Go", "Design systems"}},
		},
	}
	h := NewProjects(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/projects?featured=true&limit=3", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !q.lastArg.FeaturedOnly {
		t.Errorf("FeaturedOnly: got false want true")
	}
	if q.lastArg.RowLimit != 3 {
		t.Errorf("RowLimit: got %d want 3", q.lastArg.RowLimit)
	}

	var got []projectCardDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "mealmind" {
		t.Fatalf("unexpected body: %+v", got)
	}
	if len(got[0].Tags) != 2 {
		t.Errorf("tags: got %v", got[0].Tags)
	}
}

func TestProjectsList_DefaultsAndClamp(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantLimit int32
		wantFeat  bool
	}{
		{"no params", "", defaultProjectLimit, false},
		{"featured false", "?featured=false", defaultProjectLimit, false},
		{"limit zero falls back", "?limit=0", defaultProjectLimit, false},
		{"limit garbage falls back", "?limit=abc", defaultProjectLimit, false},
		{"limit over max clamps", "?limit=999", maxProjectLimit, false},
		{"custom limit honored", "?limit=5", 5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeProjectQ{rows: []store.ListProjectCardsRow{}}
			h := NewProjects(q)
			rr := httptest.NewRecorder()
			h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/projects"+tc.query, nil))

			if rr.Code != http.StatusOK {
				t.Fatalf("status: got %d want 200", rr.Code)
			}
			if q.lastArg.RowLimit != tc.wantLimit {
				t.Errorf("RowLimit: got %d want %d", q.lastArg.RowLimit, tc.wantLimit)
			}
			if q.lastArg.FeaturedOnly != tc.wantFeat {
				t.Errorf("FeaturedOnly: got %v want %v", q.lastArg.FeaturedOnly, tc.wantFeat)
			}
			// Empty result must serialize as [] not null.
			if body := rr.Body.String(); body != "[]\n" && body != "[]" {
				t.Errorf("empty body: got %q want []", body)
			}
		})
	}
}
