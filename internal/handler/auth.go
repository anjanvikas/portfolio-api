package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/anjanvikas/portfolio-api/internal/auth"
)

// AuthDeps bundles everything the auth handlers need. JWTSecret signs tokens,
// AdminPasswordHash is the bcrypt hash of the configured admin passphrase,
// Limiter throttles failed logins per IP, and CookieSecure controls the
// Secure attribute on the logout cookie (off in dev, on in prod).
type AuthDeps struct {
	JWTSecret         string
	AdminPasswordHash string
	Limiter           *auth.LoginRateLimiter
	CookieSecure      bool
}

// Auth groups the login and logout HTTP handlers.
type Auth struct {
	Deps AuthDeps
	now  func() time.Time
}

// NewAuth returns an Auth handler set with time.Now as the clock.
func NewAuth(deps AuthDeps) *Auth {
	return &Auth{Deps: deps, now: time.Now}
}

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login handles POST /api/v1/admin/login. It accepts a JSON body
// {"password": "..."} and returns {"token": "..."} on success. On failure it
// returns 401 with a generic error so the client cannot tell whether the
// password or rate limit was the problem.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	if !a.Deps.Limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
		a.Deps.Limiter.RecordFailure(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if err := auth.VerifyPassword(a.Deps.AdminPasswordHash, req.Password); err != nil {
		a.Deps.Limiter.RecordFailure(ip)
		if !errors.Is(err, auth.ErrInvalidPassword) {
			slog.WarnContext(r.Context(), "password verify error", slog.String("error", err.Error()))
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := auth.IssueAdminToken(a.Deps.JWTSecret, a.now())
	if err != nil {
		slog.ErrorContext(r.Context(), "issue token", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	a.Deps.Limiter.Reset(ip)
	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

// Logout handles POST /api/v1/admin/logout. It clears the admin_token cookie
// server-side and returns 200. The endpoint is unauthenticated so an expired
// session can still log out cleanly.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.Deps.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// clientIP returns the host portion of r.RemoteAddr, or the raw value if it
// cannot be split. Reverse-proxy headers (X-Forwarded-For) are intentionally
// not honored — the API is meant to be reached directly or through a proxy
// that rewrites RemoteAddr.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
