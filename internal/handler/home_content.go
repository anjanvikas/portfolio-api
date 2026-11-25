package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

// Home page copy (SCRUM-116). A singleton home_content row holds the badge
// toggle as a real column and all other copy as one JSONB document. Defaults
// live in the frontend; empty strings / arrays mean "use the default".

type homeHeroDTO struct {
	Greeting string `json:"greeting"`
	Caption  string `json:"caption"`
	Note     string `json:"note"`
	Hint     string `json:"hint"`
}

type homeBeliefDTO struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type homeChecklistItemDTO struct {
	Label string `json:"label"`
	Sub   string `json:"sub"`
}

type homeLookingForDTO struct {
	Intro    string                 `json:"intro"`
	CTALabel string                 `json:"cta_label"`
	DoneNote string                 `json:"done_note"`
	Items    []homeChecklistItemDTO `json:"items"`
}

type homeStickerDTO struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Shape string `json:"shape"`
}

// homeContentDoc is the JSONB document shape (everything except open_to_roles).
type homeContentDoc struct {
	OpenToRolesLabel string            `json:"open_to_roles_label"`
	Hero             homeHeroDTO       `json:"hero"`
	Beliefs          []homeBeliefDTO   `json:"beliefs"`
	LookingFor       homeLookingForDTO `json:"looking_for"`
	Stickers         []homeStickerDTO  `json:"stickers"`
}

// homeContentDTO is the API shape shared by the public and admin endpoints.
type homeContentDTO struct {
	OpenToRoles bool `json:"open_to_roles"`
	homeContentDoc
}

// normalize replaces nil slices with empty ones so they serialise as [].
func (d *homeContentDTO) normalize() {
	if d.Beliefs == nil {
		d.Beliefs = []homeBeliefDTO{}
	}
	if d.LookingFor.Items == nil {
		d.LookingFor.Items = []homeChecklistItemDTO{}
	}
	if d.Stickers == nil {
		d.Stickers = []homeStickerDTO{}
	}
}

func zeroHomeDTO() homeContentDTO {
	d := homeContentDTO{OpenToRoles: true}
	d.normalize()
	return d
}

// homeDTOFromRow decodes the JSONB; a corrupt document yields the zero copy
// with the row's open_to_roles.
func homeDTOFromRow(ctx context.Context, row store.HomeContent) homeContentDTO {
	d := homeContentDTO{OpenToRoles: row.OpenToRoles}
	if len(row.Content) > 0 {
		if err := json.Unmarshal(row.Content, &d.homeContentDoc); err != nil {
			slog.WarnContext(ctx, "decode home_content jsonb", slog.String("error", err.Error()))
			d.homeContentDoc = homeContentDoc{}
		}
	}
	d.normalize()
	return d
}

type homeContentReader interface {
	GetHomeContent(ctx context.Context) (store.HomeContent, error)
}

// loadHomeDTO reads the singleton; a missing row is the zero DTO.
func loadHomeDTO(r *http.Request, q homeContentReader) (homeContentDTO, bool) {
	row, err := q.GetHomeContent(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zeroHomeDTO(), true
		}
		slog.ErrorContext(r.Context(), "get home content", slog.String("error", err.Error()))
		return homeContentDTO{}, false
	}
	return homeDTOFromRow(r.Context(), row), true
}

// HomeContent serves the public GET /api/v1/home.
type HomeContent struct {
	Q homeContentReader
}

func NewHomeContent(q homeContentReader) *HomeContent { return &HomeContent{Q: q} }

func (h *HomeContent) Get(w http.ResponseWriter, r *http.Request) {
	dto, ok := loadHomeDTO(r, h.Q)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

var (
	homeStickerColors = map[string]bool{"yellow": true, "sky": true, "pink": true, "mint": true, "sheet": true}
	homeStickerShapes = map[string]bool{"rounded": true, "pill": true}
)

// validateHomeContent trims every string, coerces nil arrays to [] and returns
// field errors keyed by path (empty when valid).
func validateHomeContent(d *homeContentDTO) map[string]string {
	errs := map[string]string{}
	d.normalize()

	maxLen := func(path, v string, max int) {
		if runeLen(v) > max {
			errs[path] = fmt.Sprintf("Must be at most %d characters.", max)
		}
	}
	rng := func(path, v string, min, max int) {
		n := runeLen(v)
		if n < min {
			errs[path] = "Required."
		} else if n > max {
			errs[path] = fmt.Sprintf("Must be at most %d characters.", max)
		}
	}

	d.OpenToRolesLabel = strings.TrimSpace(d.OpenToRolesLabel)
	maxLen("open_to_roles_label", d.OpenToRolesLabel, 40)

	h := &d.Hero
	h.Greeting = strings.TrimSpace(h.Greeting)
	h.Caption = strings.TrimSpace(h.Caption)
	h.Note = strings.TrimSpace(h.Note)
	h.Hint = strings.TrimSpace(h.Hint)
	maxLen("hero.greeting", h.Greeting, 120)
	maxLen("hero.caption", h.Caption, 120)
	maxLen("hero.note", h.Note, 120)
	maxLen("hero.hint", h.Hint, 120)

	if n := len(d.Beliefs); n != 0 && n != 3 {
		errs["beliefs"] = "Provide exactly 3 beliefs, or none to use the defaults."
	}
	for i := range d.Beliefs {
		b := &d.Beliefs[i]
		b.Title = strings.TrimSpace(b.Title)
		b.Body = strings.TrimSpace(b.Body)
		rng(fmt.Sprintf("beliefs[%d].title", i), b.Title, 1, 60)
		rng(fmt.Sprintf("beliefs[%d].body", i), b.Body, 1, 280)
	}

	lf := &d.LookingFor
	lf.Intro = strings.TrimSpace(lf.Intro)
	lf.CTALabel = strings.TrimSpace(lf.CTALabel)
	lf.DoneNote = strings.TrimSpace(lf.DoneNote)
	maxLen("looking_for.intro", lf.Intro, 120)
	maxLen("looking_for.cta_label", lf.CTALabel, 40)
	maxLen("looking_for.done_note", lf.DoneNote, 40)
	if n := len(lf.Items); n != 0 && n != 3 {
		errs["looking_for.items"] = "Provide exactly 3 items, or none to use the defaults."
	}
	for i := range lf.Items {
		it := &lf.Items[i]
		it.Label = strings.TrimSpace(it.Label)
		it.Sub = strings.TrimSpace(it.Sub)
		rng(fmt.Sprintf("looking_for.items[%d].label", i), it.Label, 1, 60)
		maxLen(fmt.Sprintf("looking_for.items[%d].sub", i), it.Sub, 120)
	}

	if len(d.Stickers) > 16 {
		errs["stickers"] = "At most 16 stickers."
	}
	seen := map[string]bool{}
	for i := range d.Stickers {
		s := &d.Stickers[i]
		s.Name = strings.TrimSpace(s.Name)
		s.Color = strings.TrimSpace(s.Color)
		s.Shape = strings.TrimSpace(s.Shape)
		p := fmt.Sprintf("stickers[%d]", i)
		rng(p+".name", s.Name, 1, 24)
		if s.Name != "" {
			key := strings.ToLower(s.Name)
			if seen[key] {
				if _, has := errs[p+".name"]; !has {
					errs[p+".name"] = "Sticker names must be unique."
				}
			}
			seen[key] = true
		}
		if !homeStickerColors[s.Color] {
			errs[p+".color"] = "Must be one of yellow, sky, pink, mint, sheet."
		}
		if !homeStickerShapes[s.Shape] {
			errs[p+".shape"] = "Must be one of rounded, pill."
		}
	}
	return errs
}
