package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

type fakeAdminExpQ struct {
	listRows  []store.Experience
	getErr    error
	updateErr error
	deleteN   int64

	createParams store.CreateExperienceParams
	sortCalls    []store.SetExperienceSortOrderParams
}

func (f *fakeAdminExpQ) ListExperience(context.Context) ([]store.Experience, error) {
	return f.listRows, nil
}
func (f *fakeAdminExpQ) GetExperience(_ context.Context, id pgtype.UUID) (store.Experience, error) {
	return store.Experience{ID: id}, f.getErr
}
func (f *fakeAdminExpQ) CreateExperience(_ context.Context, arg store.CreateExperienceParams) (store.Experience, error) {
	f.createParams = arg
	return store.Experience{Company: arg.Company, Role: arg.Role, StartDate: arg.StartDate, EndDate: arg.EndDate, SortOrder: arg.SortOrder}, nil
}
func (f *fakeAdminExpQ) UpdateExperience(_ context.Context, arg store.UpdateExperienceParams) (store.Experience, error) {
	if f.updateErr != nil {
		return store.Experience{}, f.updateErr
	}
	return store.Experience{ID: arg.ID, Company: arg.Company}, nil
}
func (f *fakeAdminExpQ) DeleteExperience(context.Context, pgtype.UUID) (int64, error) {
	return f.deleteN, nil
}
func (f *fakeAdminExpQ) SetExperienceSortOrder(_ context.Context, arg store.SetExperienceSortOrderParams) error {
	f.sortCalls = append(f.sortCalls, arg)
	return nil
}
func (f *fakeAdminExpQ) NextExperienceSortOrder(context.Context) (int32, error) { return 3, nil }

func TestAdminExpCreate_ParsesDatesAndCurrentRole(t *testing.T) {
	q := &fakeAdminExpQ{}
	h := NewAdminExperience(q)
	// No end_date → current role (null end date).
	payload := `{"company":"Acme","role":"SWE","start_date":"2024-01-15"}`
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(payload)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rr.Code, rr.Body.String())
	}
	if !q.createParams.StartDate.Valid {
		t.Errorf("start date should be parsed valid")
	}
	if q.createParams.EndDate.Valid {
		t.Errorf("absent end_date should be null (current role)")
	}
	if q.createParams.SortOrder != 3 {
		t.Errorf("sort order: got %d want 3", q.createParams.SortOrder)
	}
}

func TestAdminExpCreate_MissingFields(t *testing.T) {
	rr := httptest.NewRecorder()
	NewAdminExperience(&fakeAdminExpQ{}).Create(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"company":"Acme"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Errors["role"] == "" || resp.Errors["start_date"] == "" {
		t.Errorf("expected role + start_date errors, got %+v", resp.Errors)
	}
}

func TestAdminExpCreate_BadDate(t *testing.T) {
	rr := httptest.NewRecorder()
	NewAdminExperience(&fakeAdminExpQ{}).Create(rr, httptest.NewRequest(http.MethodPost, "/x",
		strings.NewReader(`{"company":"Acme","role":"SWE","start_date":"15-01-2024"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminExpUpdate_UnknownID(t *testing.T) {
	q := &fakeAdminExpQ{updateErr: pgx.ErrNoRows}
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"company":"A","role":"R","start_date":"2024-01-01"}`)), validUUID)
	NewAdminExperience(q).Update(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestAdminExpReorder_AssignsDescendingSortOrder(t *testing.T) {
	id1, id2, id3 := validUUID, "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"
	q := &fakeAdminExpQ{}
	h := NewAdminExperience(q)
	payload := `{"ids":["` + id1 + `","` + id2 + `","` + id3 + `"]}`
	rr := httptest.NewRecorder()
	h.Reorder(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(payload)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if len(q.sortCalls) != 3 {
		t.Fatalf("expected 3 sort updates, got %d", len(q.sortCalls))
	}
	// First id (top of list) gets the highest sort_order.
	if q.sortCalls[0].SortOrder != 2 || q.sortCalls[2].SortOrder != 0 {
		t.Errorf("expected descending sort orders 2,1,0; got %d,%d,%d",
			q.sortCalls[0].SortOrder, q.sortCalls[1].SortOrder, q.sortCalls[2].SortOrder)
	}
}

func TestAdminExpReorder_BadID(t *testing.T) {
	rr := httptest.NewRecorder()
	NewAdminExperience(&fakeAdminExpQ{}).Reorder(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"ids":["nope"]}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
}

func TestAdminExpReorder_Empty(t *testing.T) {
	rr := httptest.NewRecorder()
	NewAdminExperience(&fakeAdminExpQ{}).Reorder(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"ids":[]}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
}
