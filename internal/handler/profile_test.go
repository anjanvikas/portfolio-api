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

type fakeProfileQ struct {
	prof    store.Profile
	profErr error
	links   []store.SocialLink
	linkErr error
}

func (f *fakeProfileQ) GetProfile(ctx context.Context) (store.Profile, error) {
	return f.prof, f.profErr
}

func (f *fakeProfileQ) ListSocialLinks(ctx context.Context) ([]store.SocialLink, error) {
	return f.links, f.linkErr
}

func validText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func TestProfileGet_Success(t *testing.T) {
	q := &fakeProfileQ{
		prof: store.Profile{
			Name:      "Anjan Vikas Reddy",
			Headline:  "Backend engineer",
			Bio:       "Builds things.",
			Location:  "Hyderabad, IN",
			Email:     "a@b.c",
			AvatarUrl: validText("https://cdn/avatar.jpg"),
			ResumeUrl: validText("https://cdn/cv.pdf"),
		},
		links: []store.SocialLink{
			{Name: "github", Url: "https://github.com/x", SortOrder: 0},
			{Name: "linkedin", Url: "https://linkedin.com/in/x", SortOrder: 1},
		},
	}
	h := NewProfile(q)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got profileDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Anjan Vikas Reddy" || got.Headline != "Backend engineer" {
		t.Fatalf("name/headline mismatch: %+v", got)
	}
	if got.AvatarURL != "https://cdn/avatar.jpg" || got.ResumeURL != "https://cdn/cv.pdf" {
		t.Fatalf("urls: %+v", got)
	}
	if len(got.SocialLinks) != 2 || got.SocialLinks[0].Name != "github" {
		t.Fatalf("links: %+v", got.SocialLinks)
	}
}

func TestProfileGet_EmptySocials(t *testing.T) {
	q := &fakeProfileQ{
		prof: store.Profile{Name: "x", Headline: "y", Email: "z"},
	}
	h := NewProfile(q)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	// Must be `[]`, never `null`, so the frontend can iterate safely.
	if !jsonHas(rr.Body.Bytes(), `"social_links":[]`) {
		t.Fatalf("social_links should serialize as []: %s", rr.Body.String())
	}
}

func TestProfileGet_NotFound(t *testing.T) {
	h := NewProfile(&fakeProfileQ{profErr: pgx.ErrNoRows})
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestProfileResume_Redirect(t *testing.T) {
	q := &fakeProfileQ{
		prof: store.Profile{ResumeUrl: validText("https://cdn/cv.pdf")},
	}
	h := NewProfile(q)
	rr := httptest.NewRecorder()
	h.Resume(rr, httptest.NewRequest(http.MethodGet, "/api/v1/profile/resume", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("status: got %d want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "https://cdn/cv.pdf" {
		t.Fatalf("Location: got %q", loc)
	}
}

func TestProfileResume_Missing(t *testing.T) {
	h := NewProfile(&fakeProfileQ{prof: store.Profile{}}) // ResumeUrl zero/invalid
	rr := httptest.NewRecorder()
	h.Resume(rr, httptest.NewRequest(http.MethodGet, "/api/v1/profile/resume", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func jsonHas(body []byte, needle string) bool {
	return contains(string(body), needle)
}

// avoid pulling strings for this single check
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
