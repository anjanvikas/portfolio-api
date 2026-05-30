package service

import (
	"net/url"
	"testing"
	"time"
)

func fixedPresigner() *R2Presigner {
	p := NewR2Presigner(
		"AKIDEXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"portfolio-assets",
		"https://acct123.r2.cloudflarestorage.com",
	)
	p.now = func() time.Time { return time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC) }
	return p
}

func TestPresignGetObject_Structure(t *testing.T) {
	p := fixedPresigner()
	raw, err := p.PresignGetObject("resume.pdf", 5*time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("result is not a valid URL: %v", err)
	}
	if u.Host != "acct123.r2.cloudflarestorage.com" {
		t.Fatalf("host: got %q", u.Host)
	}
	if u.Path != "/portfolio-assets/resume.pdf" {
		t.Fatalf("path: got %q", u.Path)
	}

	q := u.Query()
	wantParams := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Date":          "20260530T120000Z",
		"X-Amz-Expires":       "300",
		"X-Amz-SignedHeaders": "host",
	}
	for k, want := range wantParams {
		if got := q.Get(k); got != want {
			t.Fatalf("query %s: got %q want %q", k, got, want)
		}
	}
	if cred := q.Get("X-Amz-Credential"); cred != "AKIDEXAMPLE/20260530/auto/s3/aws4_request" {
		t.Fatalf("credential: got %q", cred)
	}
	if sig := q.Get("X-Amz-Signature"); len(sig) != 64 {
		t.Fatalf("signature should be 64 hex chars, got %d: %q", len(sig), sig)
	}
}

func TestPresignGetObject_Deterministic(t *testing.T) {
	a, _ := fixedPresigner().PresignGetObject("resume.pdf", 5*time.Minute)
	b, _ := fixedPresigner().PresignGetObject("resume.pdf", 5*time.Minute)
	if a != b {
		t.Fatalf("same inputs should produce identical URLs\n a=%s\n b=%s", a, b)
	}
}

func TestPresignGetObject_KeyChangesSignature(t *testing.T) {
	a, _ := fixedPresigner().PresignGetObject("resume.pdf", 5*time.Minute)
	b, _ := fixedPresigner().PresignGetObject("cv-2026.pdf", 5*time.Minute)

	sigA := mustQuery(t, a).Get("X-Amz-Signature")
	sigB := mustQuery(t, b).Get("X-Amz-Signature")
	if sigA == sigB {
		t.Fatal("different object keys should yield different signatures")
	}
}

func TestPresignGetObject_EscapesKey(t *testing.T) {
	p := fixedPresigner()
	raw, err := p.PresignGetObject("docs/resume final.pdf", 5*time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	u := mustParse(t, raw)
	// Slash separating segments is preserved; the space is percent-encoded.
	if u.EscapedPath() != "/portfolio-assets/docs/resume%20final.pdf" {
		t.Fatalf("escaped path: got %q", u.EscapedPath())
	}
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	return mustParse(t, raw).Query()
}
