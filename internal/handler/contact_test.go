package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anjanvikas/portfolio-api/internal/service"
)

type fakeMailer struct {
	got     service.ContactMessage
	called  bool
	sendErr error
}

func (f *fakeMailer) SendContact(_ context.Context, msg service.ContactMessage) error {
	f.called = true
	f.got = msg
	return f.sendErr
}

func postContact(t *testing.T, h *Contact, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contact", strings.NewReader(body))
	h.Submit(rr, req)
	return rr
}

func TestContactSubmit_Success(t *testing.T) {
	m := &fakeMailer{}
	rr := postContact(t, NewContact(m),
		`{"name":"Recruiter","email":"r@firm.com","message":"Loved your work."}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !m.called {
		t.Fatal("mailer was not called")
	}
	if m.got.Name != "Recruiter" || m.got.Email != "r@firm.com" || m.got.Message != "Loved your work." {
		t.Fatalf("forwarded message mismatch: %+v", m.got)
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["message"] != "Message sent!" {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestContactSubmit_TrimsWhitespace(t *testing.T) {
	m := &fakeMailer{}
	postContact(t, NewContact(m),
		`{"name":"  Jane  ","email":"  jane@x.io ","message":"  hi  "}`)
	if m.got.Name != "Jane" || m.got.Email != "jane@x.io" || m.got.Message != "hi" {
		t.Fatalf("expected trimmed fields, got %+v", m.got)
	}
}

func TestContactSubmit_ValidationErrors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{"empty name", `{"name":"","email":"a@b.co","message":"hello"}`, "name"},
		{"empty email", `{"name":"A","email":"","message":"hello"}`, "email"},
		{"bad email", `{"name":"A","email":"not-an-email","message":"hello"}`, "email"},
		{"empty message", `{"name":"A","email":"a@b.co","message":"   "}`, "message"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &fakeMailer{}
			rr := postContact(t, NewContact(m), tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400", rr.Code)
			}
			if m.called {
				t.Fatal("mailer must not be called on validation failure")
			}
			var resp struct {
				Errors map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, ok := resp.Errors[tc.wantField]; !ok {
				t.Fatalf("expected field error for %q, got %+v", tc.wantField, resp.Errors)
			}
		})
	}
}

func TestContactSubmit_MailerError(t *testing.T) {
	m := &fakeMailer{sendErr: errors.New("resend down")}
	rr := postContact(t, NewContact(m),
		`{"name":"A","email":"a@b.co","message":"hello"}`)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d want 502", rr.Code)
	}
}

func TestContactSubmit_BadJSON(t *testing.T) {
	m := &fakeMailer{}
	rr := postContact(t, NewContact(m), `{not json`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rr.Code)
	}
	if m.called {
		t.Fatal("mailer must not be called on bad JSON")
	}
}
