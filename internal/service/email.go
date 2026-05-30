// Package service holds outbound integrations the HTTP handlers depend on —
// transactional email (Resend) and R2 object presigning. Each integration is
// expressed as a small interface so handlers stay unit-testable with fakes.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ContactMessage is one inbound message from the public contact form.
type ContactMessage struct {
	Name    string
	Email   string
	Message string
}

// Mailer delivers a contact message to the site owner.
type Mailer interface {
	SendContact(ctx context.Context, msg ContactMessage) error
}

// ResendMailer sends contact messages through the Resend REST API
// (https://resend.com). It speaks plain HTTP+JSON so it needs no SDK.
type ResendMailer struct {
	apiKey string
	from   string
	to     string
	client *http.Client
}

// NewResendMailer wires a mailer that sends from `from` to `to`. Both must be
// addresses Resend will accept (the `from` domain has to be verified, or use
// Resend's onboarding@resend.dev sender while testing).
func NewResendMailer(apiKey, from, to string) *ResendMailer {
	return &ResendMailer{
		apiKey: apiKey,
		from:   from,
		to:     to,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

const resendEndpoint = "https://api.resend.com/emails"

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	ReplyTo string   `json:"reply_to,omitempty"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

// SendContact emails the message body to the configured recipient. The sender's
// address is set as Reply-To so a reply goes straight back to them.
func (m *ResendMailer) SendContact(ctx context.Context, msg ContactMessage) error {
	payload := resendRequest{
		From:    m.from,
		To:      []string{m.to},
		ReplyTo: msg.Email,
		Subject: fmt.Sprintf("Portfolio contact from %s", msg.Name),
		Text: fmt.Sprintf("Name: %s\nEmail: %s\n\n%s\n",
			msg.Name, msg.Email, msg.Message),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal resend payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusMultipleChoices {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("resend returned %d: %s", res.StatusCode, bytes.TrimSpace(snippet))
	}
	return nil
}

// LogMailer is the development fallback. It logs the message instead of sending
// an email, so the contact form works end-to-end locally without Resend
// credentials. main.go selects it when RESEND_API_KEY / CONTACT_TO_EMAIL are
// unset.
type LogMailer struct{}

// SendContact records the message at info level and reports success.
func (LogMailer) SendContact(ctx context.Context, msg ContactMessage) error {
	slog.InfoContext(ctx, "contact message received (LogMailer — no email sent)",
		slog.String("name", msg.Name),
		slog.String("email", msg.Email),
		slog.String("message", msg.Message),
	)
	return nil
}
