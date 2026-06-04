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

type fakeAdminTestimonialQ struct {
	listRows  []store.Testimonial
	updateErr error
	visErr    error
	deleteN   int64

	createParams store.CreateTestimonialParams
	visParams    store.SetTestimonialVisibilityParams
}

func (f *fakeAdminTestimonialQ) ListTestimonials(context.Context) ([]store.Testimonial, error) {
	return f.listRows, nil
}
func (f *fakeAdminTestimonialQ) GetTestimonial(_ context.Context, id pgtype.UUID) (store.Testimonial, error) {
	return store.Testimonial{ID: id}, nil
}
func (f *fakeAdminTestimonialQ) CreateTestimonial(_ context.Context, arg store.CreateTestimonialParams) (store.Testimonial, error) {
	f.createParams = arg
	return store.Testimonial{AuthorName: arg.AuthorName, Quote: arg.Quote, Visible: arg.Visible, SortOrder: arg.SortOrder}, nil
}
func (f *fakeAdminTestimonialQ) UpdateTestimonial(_ context.Context, arg store.UpdateTestimonialParams) (store.Testimonial, error) {
	if f.updateErr != nil {
		return store.Testimonial{}, f.updateErr
	}
	return store.Testimonial{ID: arg.ID, AuthorName: arg.AuthorName, Visible: arg.Visible}, nil
}
func (f *fakeAdminTestimonialQ) SetTestimonialVisibility(_ context.Context, arg store.SetTestimonialVisibilityParams) (store.Testimonial, error) {
	f.visParams = arg
	if f.visErr != nil {
		return store.Testimonial{}, f.visErr
	}
	return store.Testimonial{ID: arg.ID, Visible: arg.Visible}, nil
}
func (f *fakeAdminTestimonialQ) DeleteTestimonial(context.Context, pgtype.UUID) (int64, error) {
	return f.deleteN, nil
}
func (f *fakeAdminTestimonialQ) NextTestimonialSortOrder(context.Context) (int32, error) { return 5, nil }

func TestAdminTestimonialCreate_OK(t *testing.T) {
	q := &fakeAdminTestimonialQ{}
	h := NewAdminTestimonials(q)
	payload := `{"author_name":"Priya","quote":"Great work","visible":true}`
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(payload)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rr.Code, rr.Body.String())
	}
	if q.createParams.SortOrder != 5 {
		t.Errorf("sort order: got %d want 5", q.createParams.SortOrder)
	}
	if !q.createParams.Visible {
		t.Errorf("visible should be true")
	}
}

func TestAdminTestimonialCreate_MissingFields(t *testing.T) {
	rr := httptest.NewRecorder()
	NewAdminTestimonials(&fakeAdminTestimonialQ{}).Create(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"author_role":"x"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Errors["author_name"] == "" || resp.Errors["quote"] == "" {
		t.Errorf("expected author_name + quote errors, got %+v", resp.Errors)
	}
}

func TestAdminTestimonialSetVisibility(t *testing.T) {
	q := &fakeAdminTestimonialQ{}
	h := NewAdminTestimonials(q)
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{"visible":false}`)), validUUID)
	h.SetVisibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if q.visParams.Visible {
		t.Errorf("expected visible=false forwarded to query")
	}
	var got adminTestimonialDTO
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if got.Visible {
		t.Errorf("response should reflect visible=false")
	}
}

func TestAdminTestimonialSetVisibility_NotFound(t *testing.T) {
	q := &fakeAdminTestimonialQ{visErr: pgx.ErrNoRows}
	rr := httptest.NewRecorder()
	req := withID(httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{"visible":true}`)), validUUID)
	NewAdminTestimonials(q).SetVisibility(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestAdminTestimonialList(t *testing.T) {
	q := &fakeAdminTestimonialQ{listRows: []store.Testimonial{
		{AuthorName: "A", Quote: "q1", Visible: true},
		{AuthorName: "B", Quote: "q2", Visible: false},
	}}
	rr := httptest.NewRecorder()
	NewAdminTestimonials(q).List(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	var got []adminTestimonialDTO
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if len(got) != 2 || got[0].Visible != true || got[1].Visible != false {
		t.Errorf("admin list should include hidden rows with visible flag: %+v", got)
	}
}
