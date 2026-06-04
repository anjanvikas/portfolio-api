package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

type fakeStatsQ struct {
	row store.GetAdminStatsRow
	err error
}

func (f *fakeStatsQ) GetAdminStats(ctx context.Context) (store.GetAdminStatsRow, error) {
	return f.row, f.err
}

func TestStatsGet_Counts(t *testing.T) {
	q := &fakeStatsQ{row: store.GetAdminStatsRow{
		TotalPosts:     7,
		PublishedPosts: 5,
		DraftPosts:     2,
		TotalProjects:  3,
	}}
	h := NewStats(q)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}

	var got statsDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalPosts != 7 || got.PublishedPosts != 5 || got.DraftPosts != 2 || got.TotalProjects != 3 {
		t.Errorf("unexpected stats DTO: %+v", got)
	}
}

func TestStatsGet_QueryError(t *testing.T) {
	q := &fakeStatsQ{err: errors.New("boom")}
	h := NewStats(q)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500", rr.Code)
	}
}
