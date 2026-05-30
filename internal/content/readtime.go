// Package content holds small, dependency-free helpers for derived post data
// that both the HTTP handlers and the seed script need to compute identically.
package content

import "strings"

// ReadingTimeMins estimates a markdown body's reading time at ~200 words per
// minute, floored at 1 minute. It mirrors the formula the SQL read queries used
// before reading_time_mins became a stored column (SCRUM-66), so a re-seed and
// an admin save produce the same number. strings.Fields splits on any run of
// whitespace and drops empties, so a blank body counts as 0 words → 1 min.
func ReadingTimeMins(body string) int32 {
	words := len(strings.Fields(body))
	mins := words / 200
	if words%200 != 0 {
		mins++ // ceil: a partial 200-word block still costs a minute
	}
	if mins < 1 {
		return 1
	}
	return int32(mins)
}
