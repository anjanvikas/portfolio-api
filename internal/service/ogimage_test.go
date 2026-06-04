package service

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestGGGenerator_RenderPost(t *testing.T) {
	g, err := NewGGGenerator()
	if err != nil {
		t.Fatalf("NewGGGenerator: %v", err)
	}

	pngBytes, err := g.RenderPost(
		"How I shipped a brutalist portfolio in 30 days",
		"Anjan Vikas Reddy",
		"anjanvikasreddy.dev",
	)
	if err != nil {
		t.Fatalf("RenderPost: %v", err)
	}

	if !bytes.HasPrefix(pngBytes, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("output is not a PNG (first bytes: %x)", pngBytes[:8])
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if img.Bounds().Dx() != OGWidth || img.Bounds().Dy() != OGHeight {
		t.Fatalf("dimensions: got %dx%d want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), OGWidth, OGHeight)
	}
}

func TestGGGenerator_RenderHomepage(t *testing.T) {
	g, err := NewGGGenerator()
	if err != nil {
		t.Fatalf("NewGGGenerator: %v", err)
	}
	pngBytes, err := g.RenderHomepage("Anjan Vikas Reddy", "engineer, builder, writer", "anjanvikasreddy.dev")
	if err != nil {
		t.Fatalf("RenderHomepage: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != OGWidth {
		t.Fatalf("width: got %d want %d", img.Bounds().Dx(), OGWidth)
	}
}

func TestGGGenerator_FitTitleShrinksOnLongTitle(t *testing.T) {
	g, err := NewGGGenerator()
	if err != nil {
		t.Fatalf("NewGGGenerator: %v", err)
	}
	long := "An exhaustively long title that absolutely will not fit on a single line at the default font size and needs to wrap several times to be visible"
	_, sz := g.fitTitle(long, 1056, 360)
	if sz >= 88 {
		t.Errorf("fitTitle should have shrunk; got %.0f", sz)
	}
}
