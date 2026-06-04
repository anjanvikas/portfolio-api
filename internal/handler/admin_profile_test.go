package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

type fakeAdminProfileQ struct {
	profile      store.Profile
	updateParams store.UpdateProfileParams
}

func (f *fakeAdminProfileQ) GetProfile(context.Context) (store.Profile, error) {
	return f.profile, nil
}
func (f *fakeAdminProfileQ) UpdateProfile(_ context.Context, arg store.UpdateProfileParams) (store.Profile, error) {
	f.updateParams = arg
	return store.Profile{
		ID: arg.ID, Name: arg.Name, Headline: arg.Headline, Bio: arg.Bio,
		Location: arg.Location, Email: arg.Email, ResumeUrl: arg.ResumeUrl, AvatarUrl: arg.AvatarUrl,
	}, nil
}

func profileFixture() store.Profile {
	id, _ := parseUUID(validUUID)
	return store.Profile{ID: id, Name: "Old", Email: "old@example.com"}
}

func TestAdminProfileGet(t *testing.T) {
	q := &fakeAdminProfileQ{profile: profileFixture()}
	rr := httptest.NewRecorder()
	NewAdminProfile(q).Get(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	var got adminProfileDTO
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if got.ID == "" || got.Email != "old@example.com" {
		t.Errorf("unexpected profile DTO: %+v", got)
	}
}

func TestAdminProfileUpdate_UsesSingletonID(t *testing.T) {
	q := &fakeAdminProfileQ{profile: profileFixture()}
	h := NewAdminProfile(q)
	payload := `{"name":"New Name","email":"new@example.com","bio":"hi","avatar_url":"https://r2/x.png","resume_url":""}`
	rr := httptest.NewRecorder()
	h.Update(rr, httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(payload)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	// The update must target the singleton's existing id, not a client value.
	if q.updateParams.ID != profileFixture().ID {
		t.Errorf("update should use the loaded singleton id")
	}
	if q.updateParams.Name != "New Name" || q.updateParams.Email != "new@example.com" {
		t.Errorf("unexpected update params: %+v", q.updateParams)
	}
	// Avatar set → non-null text; empty resume → null text.
	if !q.updateParams.AvatarUrl.Valid {
		t.Errorf("avatar url should be non-null")
	}
	if q.updateParams.ResumeUrl.Valid {
		t.Errorf("empty resume url should be null, got %+v", q.updateParams.ResumeUrl)
	}
}

func TestAdminProfileUpdate_Validation(t *testing.T) {
	q := &fakeAdminProfileQ{profile: profileFixture()}
	rr := httptest.NewRecorder()
	NewAdminProfile(q).Update(rr, httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"name":"","email":"notanemail"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Errors["name"] == "" || resp.Errors["email"] == "" {
		t.Errorf("expected name + email errors, got %+v", resp.Errors)
	}
}
