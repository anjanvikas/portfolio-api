package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// profileQueries is the subset of store.Queries used by the public profile
// handlers. Defining it as an interface keeps the handler unit-testable with
// a fake.
type profileQueries interface {
	GetProfile(ctx context.Context) (store.Profile, error)
	ListSocialLinks(ctx context.Context) ([]store.SocialLink, error)
}

// resumePresigner mints a temporary download URL for an R2 object. Satisfied by
// *service.R2Presigner; left nil when R2 is not configured (dev).
type resumePresigner interface {
	PresignGetObject(key string, expiry time.Duration) (string, error)
}

// resumeLinkTTL is how long a presigned resume URL stays valid. Short — the
// browser follows the redirect immediately.
const resumeLinkTTL = 5 * time.Minute

// Profile groups the public read-only profile handlers used by the homepage.
//
// Presigner + ResumeKey are optional: when set (R2 configured), the resume
// download is served via a presigned R2 URL; otherwise the handler falls back
// to the resume_url stored on the profile row.
type Profile struct {
	Q          profileQueries
	Presigner  resumePresigner
	ResumeKey  string
	Normalizer urlNormalizer
}

// NewProfile wires the handler against the live sqlc queries.
func NewProfile(q profileQueries) *Profile {
	return &Profile{Q: q}
}

type socialLinkDTO struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type profileDTO struct {
	Name        string          `json:"name"`
	Headline    string          `json:"headline"`
	Bio         string          `json:"bio"`
	Location    string          `json:"location"`
	Email       string          `json:"email"`
	AvatarURL   string          `json:"avatar_url"`
	ResumeURL   string          `json:"resume_url"`
	SocialLinks []socialLinkDTO `json:"social_links"`
}

// Get handles GET /api/v1/profile. It returns the singleton profile row
// together with the ordered list of social links.
func (p *Profile) Get(w http.ResponseWriter, r *http.Request) {
	prof, err := p.Q.GetProfile(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
			return
		}
		slog.ErrorContext(r.Context(), "get profile", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	links, err := p.Q.ListSocialLinks(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list social links", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	dto := profileDTO{
		Name:        prof.Name,
		Headline:    prof.Headline,
		Bio:         nzBody(p.Normalizer, prof.Bio),
		Location:    prof.Location,
		Email:       prof.Email,
		AvatarURL:   nz(p.Normalizer, prof.AvatarUrl.String),
		ResumeURL:   nz(p.Normalizer, prof.ResumeUrl.String),
		SocialLinks: make([]socialLinkDTO, 0, len(links)),
	}
	for _, l := range links {
		dto.SocialLinks = append(dto.SocialLinks, socialLinkDTO{Name: l.Name, URL: l.Url})
	}
	writeJSON(w, http.StatusOK, dto)
}

// Resume handles GET /api/v1/profile/resume. When R2 is configured it mints a
// short-lived presigned URL for the resume PDF and 302-redirects to it;
// otherwise it falls back to the resume_url stored on the profile row. Returns
// 404 if neither is available.
func (p *Profile) Resume(w http.ResponseWriter, r *http.Request) {
	if p.Presigner != nil && p.ResumeKey != "" {
		url, err := p.Presigner.PresignGetObject(p.ResumeKey, resumeLinkTTL)
		if err != nil {
			slog.ErrorContext(r.Context(), "presign resume", slog.String("error", err.Error()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	prof, err := p.Q.GetProfile(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		slog.ErrorContext(r.Context(), "get profile", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if !prof.ResumeUrl.Valid || prof.ResumeUrl.String == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, nz(p.Normalizer, prof.ResumeUrl.String), http.StatusFound)
}
