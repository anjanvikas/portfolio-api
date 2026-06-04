package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/anjanvikas/portfolio-api/internal/auth"
	mw "github.com/anjanvikas/portfolio-api/internal/middleware"
)

const testPassword = "correct horse battery staple"
const testSecret = "test-secret-not-for-production"

func newAuth(t *testing.T) *Auth {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return NewAuth(AuthDeps{
		JWTSecret:         testSecret,
		AdminPasswordHash: string(hash),
		Limiter:           auth.NewLoginRateLimiter(5, 15*time.Minute),
	})
}

func postLogin(h *Auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.10:54321"
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	return rr
}

func TestLogin_Success(t *testing.T) {
	h := newAuth(t)
	rr := postLogin(h, `{"password":"`+testPassword+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp loginResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("empty token")
	}
	if _, err := auth.VerifyAdminToken(testSecret, resp.Token); err != nil {
		t.Fatalf("token does not verify: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := newAuth(t)
	rr := postLogin(h, `{"password":"nope"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "invalid credentials") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestLogin_RateLimit(t *testing.T) {
	h := newAuth(t)
	for i := 0; i < 5; i++ {
		if rr := postLogin(h, `{"password":"nope"}`); rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d want 401", i, rr.Code)
		}
	}
	rr := postLogin(h, `{"password":"nope"}`)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: got %d want 429", rr.Code)
	}
	// Even the correct password is rejected while throttled.
	rr = postLogin(h, `{"password":"`+testPassword+`"}`)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("throttled correct pw: got %d want 429", rr.Code)
	}
}

func TestRequireAdmin(t *testing.T) {
	token, err := auth.IssueAdminToken(testSecret, time.Now())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	protected := mw.RequireAdmin(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := mw.AdminIDFromContext(r.Context()); got != auth.AdminSubject {
			t.Errorf("admin id: got %q want %q", got, auth.AdminSubject)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong scheme", "Token " + token, http.StatusUnauthorized},
		{"bad token", "Bearer not-a-jwt", http.StatusUnauthorized},
		{"valid", "Bearer " + token, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rr := httptest.NewRecorder()
			protected.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status: got %d want %d", rr.Code, tc.want)
			}
		})
	}

}

func TestLogout(t *testing.T) {
	h := newAuth(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	cookie := rr.Result().Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "admin_token=") {
		t.Fatalf("missing admin_token cookie: %q", cookie)
	}
	if !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("expected Max-Age=0 (clear) cookie: %q", cookie)
	}
	if !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("expected HttpOnly: %q", cookie)
	}
	if !strings.Contains(cookie, "SameSite=Strict") {
		t.Fatalf("expected SameSite=Strict: %q", cookie)
	}
}
