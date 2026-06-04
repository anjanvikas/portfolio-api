package service

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed fonts/SpaceGrotesk-Bold.ttf
var spaceGroteskBoldTTF []byte

//go:embed fonts/JetBrainsMono-Bold.ttf
var jetbrainsMonoBoldTTF []byte

// OGImageGenerator renders a 1200x630 OG card. Returns PNG bytes.
type OGImageGenerator interface {
	RenderPost(title, authorName, siteURL string) ([]byte, error)
	RenderHomepage(name, headline, siteURL string) ([]byte, error)
}

// GGGenerator renders OG cards with fogleman/gg + embedded fonts. The brutalist
// card mirrors the marketing palette: paper background, 12px ink border, top
// chartreuse band with site URL + "BLOG" tag, big post title, byline footer.
type GGGenerator struct {
	display *truetype.Font
	mono    *truetype.Font
}

// OGWidth + OGHeight are the dimensions LinkedIn / Twitter / Slack expect.
const (
	OGWidth  = 1200
	OGHeight = 630
)

var (
	ogInk      = color.RGBA{0x0E, 0x0E, 0x10, 0xFF}
	ogPaper    = color.RGBA{0xFA, 0xFA, 0xF5, 0xFF}
	ogPaper2   = color.RGBA{0xF0, 0xEF, 0xE6, 0xFF}
	ogAccent   = color.RGBA{0xC6, 0xFF, 0x3D, 0xFF}
	ogAccent2  = color.RGBA{0xFF, 0x5C, 0x8A, 0xFF}
	ogMuted    = color.RGBA{0x6B, 0x6B, 0x6B, 0xFF}
)

func NewGGGenerator() (*GGGenerator, error) {
	display, err := truetype.Parse(spaceGroteskBoldTTF)
	if err != nil {
		return nil, fmt.Errorf("parse display font: %w", err)
	}
	mono, err := truetype.Parse(jetbrainsMonoBoldTTF)
	if err != nil {
		return nil, fmt.Errorf("parse mono font: %w", err)
	}
	return &GGGenerator{display: display, mono: mono}, nil
}

func (g *GGGenerator) face(f *truetype.Font, size float64) font.Face {
	return truetype.NewFace(f, &truetype.Options{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// RenderPost draws a brutalist card for a blog post. Title is wrapped to fit
// the 1024px text column; if it's still too tall the font shrinks one step.
func (g *GGGenerator) RenderPost(title, authorName, siteURL string) ([]byte, error) {
	dc := gg.NewContext(OGWidth, OGHeight)

	// Paper background.
	dc.SetColor(ogPaper)
	dc.Clear()

	// 12px ink border on all four sides.
	const border = 12
	dc.SetColor(ogInk)
	dc.DrawRectangle(0, 0, OGWidth, border)
	dc.DrawRectangle(0, OGHeight-border, OGWidth, border)
	dc.DrawRectangle(0, 0, border, OGHeight)
	dc.DrawRectangle(OGWidth-border, 0, border, OGHeight)
	dc.Fill()

	// Top chartreuse band: site URL on the left, "POST" chip on the right.
	const bandH = 120
	dc.SetColor(ogAccent)
	dc.DrawRectangle(border, border, OGWidth-2*border, bandH)
	dc.Fill()
	// 3px ink rule under the band.
	dc.SetColor(ogInk)
	dc.DrawRectangle(border, border+bandH, OGWidth-2*border, 3)
	dc.Fill()

	dc.SetFontFace(g.face(g.mono, 28))
	dc.SetColor(ogInk)
	dc.DrawStringAnchored(strings.ToUpper(siteURL), 72, float64(border+bandH)/2+float64(border)/2, 0, 0.5)
	dc.DrawStringAnchored("// POST", OGWidth-72, float64(border+bandH)/2+float64(border)/2, 1, 0.5)

	// Title: Space Grotesk Bold. Try sizes 88 → 76 → 64 until wrap fits.
	titleX := 72.0
	titleY := float64(border + bandH + 80)
	titleMaxW := float64(OGWidth) - 144.0
	wrapped, fontSize := g.fitTitle(title, titleMaxW, 360.0)
	dc.SetFontFace(g.face(g.display, fontSize))
	dc.SetColor(ogInk)
	dc.DrawStringWrapped(wrapped, titleX, titleY, 0, 0, titleMaxW, 1.15, gg.AlignLeft)

	// Bottom: 3px ink rule + byline (left) + paper-2 chip with the byline accent.
	const footerH = 96
	footerTop := float64(OGHeight - border - footerH)
	dc.SetColor(ogInk)
	dc.DrawRectangle(border, footerTop-3, OGWidth-2*border, 3)
	dc.Fill()
	// paper-2 strip beneath the rule
	dc.SetColor(ogPaper2)
	dc.DrawRectangle(border, footerTop, OGWidth-2*border, footerH)
	dc.Fill()

	dc.SetFontFace(g.face(g.mono, 26))
	dc.SetColor(ogInk)
	dc.DrawStringAnchored("BY "+strings.ToUpper(authorName), 72, footerTop+float64(footerH)/2, 0, 0.5)

	// Small chartreuse square on the right of the footer (brutalist accent).
	dc.SetColor(ogAccent)
	const sq = 32
	sqX := float64(OGWidth) - 72 - sq
	sqY := footerTop + float64(footerH)/2 - sq/2
	dc.DrawRectangle(sqX, sqY, sq, sq)
	dc.Fill()
	dc.SetColor(ogInk)
	dc.SetLineWidth(3)
	dc.DrawRectangle(sqX, sqY, sq, sq)
	dc.Stroke()

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderHomepage draws the static homepage card: big name, headline, site URL.
// The pink accent block lives where the post version puts the byline chip.
func (g *GGGenerator) RenderHomepage(name, headline, siteURL string) ([]byte, error) {
	dc := gg.NewContext(OGWidth, OGHeight)

	dc.SetColor(ogPaper)
	dc.Clear()

	const border = 12
	dc.SetColor(ogInk)
	dc.DrawRectangle(0, 0, OGWidth, border)
	dc.DrawRectangle(0, OGHeight-border, OGWidth, border)
	dc.DrawRectangle(0, 0, border, OGHeight)
	dc.DrawRectangle(OGWidth-border, 0, border, OGHeight)
	dc.Fill()

	// Top chartreuse band — same as posts for visual consistency.
	const bandH = 120
	dc.SetColor(ogAccent)
	dc.DrawRectangle(border, border, OGWidth-2*border, bandH)
	dc.Fill()
	dc.SetColor(ogInk)
	dc.DrawRectangle(border, border+bandH, OGWidth-2*border, 3)
	dc.Fill()

	dc.SetFontFace(g.face(g.mono, 28))
	dc.SetColor(ogInk)
	dc.DrawStringAnchored(strings.ToUpper(siteURL), 72, float64(border+bandH)/2+float64(border)/2, 0, 0.5)
	dc.DrawStringAnchored("// PORTFOLIO", OGWidth-72, float64(border+bandH)/2+float64(border)/2, 1, 0.5)

	// Big name in display font, vertically centered in the area between the
	// chartreuse band and the bottom footer rule.
	const nameSize = 124
	midCenterY := float64(border+bandH+3+OGHeight-border-80) / 2.0
	dc.SetFontFace(g.face(g.display, nameSize))
	dc.SetColor(ogInk)
	dc.DrawStringAnchored(name+".", OGWidth/2, midCenterY-30, 0.5, 0.5)

	// Pink underline below the name; ay=0.5 sets the baseline a bit below
	// midCenterY, so we clear the descenders with a safe gap.
	underlineY := midCenterY + 70
	dc.SetColor(ogAccent2)
	dc.DrawRectangle(OGWidth/2-180, underlineY, 360, 8)
	dc.Fill()

	// Headline below the underline.
	dc.SetFontFace(g.face(g.display, 36))
	dc.SetColor(ogMuted)
	dc.DrawStringAnchored(headline, OGWidth/2, underlineY+42, 0.5, 0.5)

	// Footer rule + paper-2 strip, like the post card.
	const footerH = 80
	footerTop := float64(OGHeight - border - footerH)
	dc.SetColor(ogInk)
	dc.DrawRectangle(border, footerTop-3, OGWidth-2*border, 3)
	dc.Fill()
	dc.SetColor(ogPaper2)
	dc.DrawRectangle(border, footerTop, OGWidth-2*border, footerH)
	dc.Fill()

	dc.SetFontFace(g.face(g.mono, 24))
	dc.SetColor(ogInk)
	dc.DrawStringAnchored("ENGINEER · BUILDER · WRITER", OGWidth/2, footerTop+float64(footerH)/2, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// fitTitle picks the largest font size from a fixed ladder whose wrapped height
// (1.15 line-height) fits maxHeight. Returns the chosen string and font size.
// At the smallest size it still returns the wrapped string even if it overflows
// — the renderer will clip, but we prefer ugly over crashed.
func (g *GGGenerator) fitTitle(title string, maxW, maxH float64) (string, float64) {
	sizes := []float64{88, 80, 72, 64, 56}
	for _, sz := range sizes {
		face := g.face(g.display, sz)
		dc := gg.NewContext(OGWidth, OGHeight)
		dc.SetFontFace(face)
		lines := dc.WordWrap(title, maxW)
		lineH := sz * 1.15
		if float64(len(lines))*lineH <= maxH {
			return strings.Join(lines, "\n"), sz
		}
	}
	face := g.face(g.display, 56)
	dc := gg.NewContext(OGWidth, OGHeight)
	dc.SetFontFace(face)
	return strings.Join(dc.WordWrap(title, maxW), "\n"), 56
}
