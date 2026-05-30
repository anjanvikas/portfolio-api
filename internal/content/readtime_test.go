package content

import (
	"strings"
	"testing"
)

func TestReadingTimeMins(t *testing.T) {
	cases := []struct {
		name  string
		words int
		want  int32
	}{
		{"empty", 0, 1},
		{"one word", 1, 1},
		{"exactly 200", 200, 1},
		{"201 words rounds up", 201, 2},
		{"400 words", 400, 2},
		{"450 words rounds up", 450, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := strings.TrimSpace(strings.Repeat("word ", c.words))
			if got := ReadingTimeMins(body); got != c.want {
				t.Errorf("ReadingTimeMins(%d words) = %d, want %d", c.words, got, c.want)
			}
		})
	}
}
