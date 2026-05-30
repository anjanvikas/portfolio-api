package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

type fakeExperienceQ struct {
	rows []store.Experience
	err  error
}

func (f *fakeExperienceQ) ListExperience(ctx context.Context) ([]store.Experience, error) {
	return f.rows, f.err
}

func pgDate(y int, m time.Month, d int) pgtype.Date {
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

func TestExperienceList_PresentAndPastRoles(t *testing.T) {
	q := &fakeExperienceQ{
		rows: []store.Experience{
			{
				Company:     "Mealmind",
				Role:        "Founding engineer",
				Location:    "Remote",
				StartDate:   pgDate(2024, time.June, 1),
				EndDate:     pgtype.Date{}, // null = current
				Description: "Built the recipe pipeline.",
			},
			{
				Company:     "Acme Corp",
				Role:        "Software engineer",
				Location:    "Bengaluru, IN",
				StartDate:   pgDate(2022, time.July, 1),
				EndDate:     pgDate(2024, time.May, 31),
				Description: "Owned the billing rewrite.",
			},
		},
	}
	h := NewExperience(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/experience", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}

	var got []experienceDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d want 2", len(got))
	}
	// Current role: end_date must be JSON null.
	if got[0].EndDate != nil {
		t.Errorf("current role end_date: got %v want nil", *got[0].EndDate)
	}
	if got[0].StartDate != "2024-06-01" {
		t.Errorf("start_date: got %q want 2024-06-01", got[0].StartDate)
	}
	// Past role: end_date present as ISO date.
	if got[1].EndDate == nil || *got[1].EndDate != "2024-05-31" {
		t.Errorf("past role end_date: got %v want 2024-05-31", got[1].EndDate)
	}
}

func TestExperienceList_Empty(t *testing.T) {
	q := &fakeExperienceQ{rows: []store.Experience{}}
	h := NewExperience(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/experience", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	// Empty result must serialize as [] not null.
	if body := rr.Body.String(); body != "[]\n" && body != "[]" {
		t.Errorf("empty body: got %q want []", body)
	}
}

func TestExperienceList_QueryError(t *testing.T) {
	q := &fakeExperienceQ{err: errors.New("boom")}
	h := NewExperience(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/experience", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500", rr.Code)
	}
}

type fakeTestimonialQ struct {
	rows []store.Testimonial
	err  error
}

func (f *fakeTestimonialQ) ListTestimonials(ctx context.Context) ([]store.Testimonial, error) {
	return f.rows, f.err
}

func TestTestimonialsList_Fields(t *testing.T) {
	q := &fakeTestimonialQ{
		rows: []store.Testimonial{
			{
				AuthorName:    "Priya Sharma",
				AuthorRole:    "Eng manager",
				AuthorCompany: "Acme Corp",
				Quote:         "Shipped ahead of schedule.",
			},
		},
	}
	h := NewTestimonials(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/testimonials", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}

	var got []testimonialDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d want 1", len(got))
	}
	c := got[0]
	if c.AuthorName != "Priya Sharma" || c.AuthorRole != "Eng manager" ||
		c.AuthorCompany != "Acme Corp" || c.Quote == "" {
		t.Errorf("unexpected testimonial DTO: %+v", c)
	}
}

func TestTestimonialsList_Empty(t *testing.T) {
	q := &fakeTestimonialQ{rows: []store.Testimonial{}}
	h := NewTestimonials(q)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/testimonials", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	if body := rr.Body.String(); body != "[]\n" && body != "[]" {
		t.Errorf("empty body: got %q want []", body)
	}
}
