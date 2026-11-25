package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func putHome(q *fakeHomeQ, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	NewAdminHomeContent(q).Update(rr, httptest.NewRequest(http.MethodPut, "/admin/home", strings.NewReader(body)))
	return rr
}

func TestAdminHomeUpdate_Happy(t *testing.T) {
	q := &fakeHomeQ{}
	rr := putHome(q, `{"open_to_roles":false,"open_to_roles_label":"  Hire me  ",
		"hero":{"greeting":" hey ","caption":"","note":"","hint":""},
		"beliefs":[{"title":" A ","body":"a"},{"title":"B","body":"b"},{"title":"C","body":"c"}],
		"looking_for":{"intro":"","cta_label":"","done_note":"","items":[]},
		"stickers":[{"name":" Go ","color":"sky","shape":"pill"}]}`)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if q.params.OpenToRoles {
		t.Error("open_to_roles should be false")
	}
	var doc homeContentDoc
	if err := json.Unmarshal(q.params.Content, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenToRolesLabel != "Hire me" || doc.Hero.Greeting != "hey" || doc.Beliefs[0].Title != "A" || doc.Stickers[0].Name != "Go" {
		t.Errorf("not trimmed: %+v", doc)
	}
	var got homeContentDTO
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.OpenToRoles || got.OpenToRolesLabel != "Hire me" || len(got.Beliefs) != 3 {
		t.Errorf("response: %s", rr.Body.String())
	}
}

func TestAdminHomeUpdate_NullArraysStoredEmpty(t *testing.T) {
	q := &fakeHomeQ{}
	rr := putHome(q, `{"open_to_roles":true,"stickers":null,"beliefs":null}`)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	s := string(q.params.Content)
	if !strings.Contains(s, `"stickers":[]`) || !strings.Contains(s, `"beliefs":[]`) || strings.Contains(s, "null") {
		t.Errorf("stored: %s", s)
	}
}

func TestAdminHomeUpdate_Validation(t *testing.T) {
	belief := `{"title":"T","body":"b"}`
	three := `[` + belief + `,` + belief + `,` + belief + `]`
	var many []string
	for i := 0; i < 17; i++ {
		many = append(many, `{"name":"S`+strings.Repeat("x", i)+`","color":"sky","shape":"pill"}`)
	}
	cases := []struct {
		name, body, key string
		status          int
	}{
		{"two beliefs", `{"open_to_roles":true,"beliefs":[` + belief + `,` + belief + `]}`, "beliefs", 400},
		{"empty belief title", `{"open_to_roles":true,"beliefs":[{"title":" ","body":"b"},` + belief + `,` + belief + `]}`, "beliefs[0].title", 400},
		{"bad color", `{"open_to_roles":true,"stickers":[{"name":"Go","color":"red","shape":"pill"}]}`, "stickers[0].color", 400},
		{"bad shape", `{"open_to_roles":true,"stickers":[{"name":"Go","color":"sky","shape":"x"}]}`, "stickers[0].shape", 400},
		{"duplicate names", `{"open_to_roles":true,"stickers":[{"name":"Go","color":"sky","shape":"pill"},{"name":"go","color":"pink","shape":"pill"}]}`, "stickers[1].name", 400},
		{"17 stickers", `{"open_to_roles":true,"stickers":[` + strings.Join(many, ",") + `]}`, "stickers", 400},
		{"long note", `{"open_to_roles":true,"hero":{"note":"` + strings.Repeat("a", 121) + `"}}`, "hero.note", 400},
		{"two items", `{"open_to_roles":true,"looking_for":{"items":[{"label":"a","sub":""},{"label":"b","sub":""}]}}`, "looking_for.items", 400},
		{"runes not bytes ok", `{"open_to_roles":true,"hero":{"note":"` + strings.Repeat("é", 120) + `"}}`, "", 200},
		{"three beliefs ok", `{"open_to_roles":true,"beliefs":` + three + `}`, "", 200},
		{"unknown key", `{"open_to_roles":true,"bogus":1}`, "", 400},
		{"malformed", `{`, "", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeHomeQ{}
			rr := putHome(q, tc.body)
			if rr.Code != tc.status {
				t.Fatalf("status %d want %d; body %s", rr.Code, tc.status, rr.Body.String())
			}
			if tc.status == 200 {
				return
			}
			if q.called {
				t.Error("upsert must not be called on invalid input")
			}
			if tc.key == "" {
				if !strings.Contains(rr.Body.String(), "invalid request body") {
					t.Errorf("body %s", rr.Body.String())
				}
				return
			}
			var resp struct {
				Errors map[string]string `json:"errors"`
			}
			_ = json.Unmarshal(rr.Body.Bytes(), &resp)
			if resp.Errors[tc.key] == "" {
				t.Errorf("missing error %q in %s", tc.key, rr.Body.String())
			}
		})
	}
}

func TestAdminHomeGet_NoRowsZeroDTO(t *testing.T) {
	rr := httptest.NewRecorder()
	NewAdminHomeContent(&fakeHomeQ{err: errNoRows()}).Get(rr, httptest.NewRequest(http.MethodGet, "/admin/home", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"beliefs":[]`) {
		t.Errorf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func errNoRows() error { return pgx.ErrNoRows }
