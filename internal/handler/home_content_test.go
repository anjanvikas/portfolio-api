package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/anjanvikas/portfolio-api/internal/store"
)

type fakeHomeQ struct {
	row    store.HomeContent
	err    error
	params store.UpsertHomeContentParams
	called bool
}

func (f *fakeHomeQ) GetHomeContent(context.Context) (store.HomeContent, error) {
	return f.row, f.err
}

func (f *fakeHomeQ) UpsertHomeContent(_ context.Context, arg store.UpsertHomeContentParams) (store.HomeContent, error) {
	f.called = true
	f.params = arg
	return store.HomeContent{ID: 1, OpenToRoles: arg.OpenToRoles, Content: arg.Content}, nil
}

func getHome(t *testing.T, q *fakeHomeQ) (*httptest.ResponseRecorder, homeContentDTO) {
	t.Helper()
	rr := httptest.NewRecorder()
	NewHomeContent(q).Get(rr, httptest.NewRequest(http.MethodGet, "/home", nil))
	var dto homeContentDTO
	_ = json.Unmarshal(rr.Body.Bytes(), &dto)
	return rr, dto
}

func TestHomeContentGet_Populated(t *testing.T) {
	q := &fakeHomeQ{row: store.HomeContent{
		OpenToRoles: false,
		Content: []byte(`{"open_to_roles_label":"Hiring?","hero":{"greeting":"hi"},
			"beliefs":[{"title":"A","body":"b"}],"stickers":[{"name":"Go","color":"sky","shape":"pill"}]}`),
	}}
	rr, dto := getHome(t, q)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	if dto.OpenToRoles || dto.OpenToRolesLabel != "Hiring?" || dto.Hero.Greeting != "hi" ||
		len(dto.Beliefs) != 1 || dto.Stickers[0].Name != "Go" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.LookingFor.Items == nil {
		t.Error("items should be [] not nil")
	}
}

func TestHomeContentGet_NoRows(t *testing.T) {
	rr, dto := getHome(t, &fakeHomeQ{err: pgx.ErrNoRows})
	if rr.Code != 200 || !dto.OpenToRoles {
		t.Fatalf("status %d dto %+v", rr.Code, dto)
	}
	body := rr.Body.String()
	for _, want := range []string{`"beliefs":[]`, `"stickers":[]`, `"items":[]`, `"open_to_roles":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "null") {
		t.Errorf("body contains null: %s", body)
	}
}

func TestHomeContentGet_CorruptJSON(t *testing.T) {
	rr, dto := getHome(t, &fakeHomeQ{row: store.HomeContent{OpenToRoles: false, Content: []byte(`{not json`)}})
	if rr.Code != 200 || dto.OpenToRoles || len(dto.Beliefs) != 0 || strings.Contains(rr.Body.String(), "null") {
		t.Errorf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestHomeContentGet_DBError(t *testing.T) {
	rr, _ := getHome(t, &fakeHomeQ{err: errors.New("boom")})
	if rr.Code != 500 {
		t.Errorf("status %d want 500", rr.Code)
	}
}
