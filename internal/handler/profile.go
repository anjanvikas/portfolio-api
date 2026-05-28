package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

// profileQueries is the subset of store.Queries used by the public profile
// handlers. Defining it as an interface keeps the handler unit-testable with
// a fake.
type profileQueries interface {
	GetProfile(ctx context.Context) (store.Profile, error)
	ListSocialLinks(ctx context.Context) ([]store.SocialLink, error)
}

// Profile groups the public read-only profile handlers used by the homepage.
type Profile struct {
	Q profileQueries
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
		Bio:         prof.Bio,
		Location:    prof.Location,
		Email:       prof.Email,
		AvatarURL:   prof.AvatarUrl.String,
		ResumeURL:   prof.ResumeUrl.String,
		SocialLinks: make([]socialLinkDTO, 0, len(links)),
	}
	for _, l := range links {
		dto.SocialLinks = append(dto.SocialLinks, socialLinkDTO{Name: l.Name, URL: l.Url})
	}
	writeJSON(w, http.StatusOK, dto)
}

// Resume handles GET /api/v1/profile/resume. It 302-redirects to the
// resume_url stored on the profile row. Returns 404 if no resume is set.
func (p *Profile) Resume(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, prof.ResumeUrl.String, http.StatusFound)
}
