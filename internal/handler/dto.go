package handler

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// nullText wraps a string as a nullable text column: an empty string becomes
// SQL NULL. Reading it back via pgtype.Text.String yields "" again, so the
// round-trip is transparent. Used for optional URL fields (repo/live/resume/
// avatar).
func nullText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// parseISODate parses a YYYY-MM-DD string into a pgtype.Date. Used for the
// experience start/end date request fields.
func parseISODate(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// uuidString renders a pgtype.UUID as its canonical hyphenated string, or "" if
// the value is NULL. Used when a DB id is surfaced in a JSON DTO.
func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// parseUUID parses a canonical UUID string into a valid pgtype.UUID, used for
// path params and request-body ids. Returns an error for malformed input.
func parseUUID(s string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// isoDate formats a pgtype.Timestamptz as an ISO date (YYYY-MM-DD), or "" when
// NULL. The marketing site shows ISO dates everywhere (engineer audience).
func isoDate(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02")
}

// isoDateOnly formats a pgtype.Date as an ISO date (YYYY-MM-DD), or "" when
// NULL. Used for non-nullable date columns surfaced in a DTO.
func isoDateOnly(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// isoDatePtr formats a pgtype.Date as an ISO date (YYYY-MM-DD), returning nil
// when NULL so the field serialises as JSON `null`. Used for the experience
// timeline's end_date, where null means the role is current ("Present").
func isoDatePtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}
