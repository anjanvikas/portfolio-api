package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/anjanvikas/portfolio-api/internal/service"
)

// contactMailer is the slice of service.Mailer the handler needs, declared as
// an interface so the handler can be unit-tested with a fake.
type contactMailer interface {
	SendContact(ctx context.Context, msg service.ContactMessage) error
}

// Contact handles the public contact form submission.
type Contact struct {
	Mailer contactMailer
}

// NewContact wires the handler against a mailer.
func NewContact(m contactMailer) *Contact {
	return &Contact{Mailer: m}
}

type contactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// emailPattern is a deliberately loose sanity check: one @, a dot in the
// domain, no whitespace. Full RFC 5322 validation is pointless here — the real
// proof an address works is the reply landing.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// maxContactMessageBytes caps the message body so a single submission can't
// post an unbounded payload.
const maxContactMessageBytes = 5000

// Submit handles POST /api/v1/contact. It validates {name, email, message},
// emails the message to the site owner, and returns 200 on success. Validation
// failures return 400 with a per-field `errors` map the form renders inline.
func (c *Contact) Submit(w http.ResponseWriter, r *http.Request) {
	var req contactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	message := strings.TrimSpace(req.Message)

	fieldErrors := make(map[string]string)
	if name == "" {
		fieldErrors["name"] = "Name is required."
	}
	switch {
	case email == "":
		fieldErrors["email"] = "Email is required."
	case !emailPattern.MatchString(email):
		fieldErrors["email"] = "Enter a valid email address."
	}
	switch {
	case message == "":
		fieldErrors["message"] = "Message is required."
	case len(message) > maxContactMessageBytes:
		fieldErrors["message"] = "Message is too long."
	}
	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": fieldErrors})
		return
	}

	if err := c.Mailer.SendContact(r.Context(), service.ContactMessage{
		Name:    name,
		Email:   email,
		Message: message,
	}); err != nil {
		slog.ErrorContext(r.Context(), "send contact email", slog.String("error", err.Error()))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not send message, please try again later"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Message sent!"})
}
