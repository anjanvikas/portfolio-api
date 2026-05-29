package handler

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// uuidString renders a pgtype.UUID as its canonical hyphenated string, or "" if
// the value is NULL. Used when a DB id is surfaced in a JSON DTO.
func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// isoDate formats a pgtype.Timestamptz as an ISO date (YYYY-MM-DD), or "" when
// NULL. The marketing site shows ISO dates everywhere (engineer audience).
func isoDate(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02")
}
