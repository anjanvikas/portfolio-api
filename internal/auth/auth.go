// Package auth handles password verification and JWT issue/verify for the
// single-admin portfolio API. There is no user table — the admin's bcrypt hash
// lives in ADMIN_PASSWORD and the signing key in JWT_SECRET.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin = "admin"
	// AdminSubject is the constant sub claim used for the single admin.
	AdminSubject = "admin"

	tokenLifetime = 7 * 24 * time.Hour
)

// ErrInvalidPassword and ErrInvalidToken are returned by VerifyPassword and
// VerifyToken respectively. Callers compare against these — the underlying
// reason is deliberately not surfaced to clients.
var (
	ErrInvalidPassword = errors.New("auth: invalid password")
	ErrInvalidToken    = errors.New("auth: invalid token")
)

// Claims is the JWT body for an admin session.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// VerifyPassword compares a plaintext password against a bcrypt hash. Bcrypt
// runs in constant time relative to the hash, so this is safe against timing
// attacks without an explicit subtle.ConstantTimeCompare.
func VerifyPassword(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return ErrInvalidPassword
	}
	return nil
}

// IssueAdminToken returns an HS256-signed JWT for the admin role with a 7 day
// expiry. now is taken as a parameter so tests can use a fixed clock.
func IssueAdminToken(secret string, now time.Time) (string, error) {
	claims := Claims{
		Role: RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   AdminSubject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// VerifyAdminToken parses and validates raw, requiring an HS256 signature,
// an unexpired exp, and role == "admin". Returns the parsed Claims on success.
func VerifyAdminToken(secret, raw string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.Role != RoleAdmin {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
