package service

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"net/url"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// Static (non-variable) OFL fonts; see fonts/OFL-*.txt. golang/freetype cannot
// select a variable-font instance, so these are pre-instanced TTFs.
//
//go:embed fonts/Outfit-ExtraBold.ttf
var outfitExtraBoldTTF []byte

//go:embed fonts/Caveat-Bold.ttf
var caveatBoldTTF []byte

//go:embed fonts/JetBrainsMono-Bold.ttf
var jetbrainsMonoBoldTTF []byte

// OGImageGenerator renders a 1200x630 OG card. Returns PNG bytes.
type OGImageGenerator interface {
	RenderPost(title, authorName, siteURL string) ([]byte, error)
	RenderHomepage(name, headline, siteURL string) ([]byte, error)
}

// GGGenerator renders OG cards with fogleman/gg + embedded fonts. The card is
// the notebook look: a dotted paper desk, a ruled sheet with a red margin line
// and two washi-tape strips, Outfit title, Caveat handwriting, mono meta line.
type GGGenerator struct {
	display *truetype.Font // Outfit ExtraBold
	hand    *truetype.Font // Caveat Bold
	mono    *truetype.Font // JetBrains Mono Bold
}

// OGWidth + OGHeight are the dimensions LinkedIn / Twitter / Slack expect.
const (
	OGWidth  = 1200
	OGHeight = 630
)

// Notebook palette (docs/design-tokens.md). Text pairs on sheet: ink 15:1,
// pencil 7.0:1; ink on highlight 13.4:1. Highlighters/tape are backgrounds only.
var (
	ogPaper     = color.RGBA{0xFB, 0xF7, 0xEE, 0xFF} // paper (desk)
	ogPaperDot  = color.RGBA{0xD9, 0xCF, 0xBD, 0xFF} // paper-dot
	ogSheet     = color.RGBA{0xFF, 0xFD, 0xF8, 0xFF} // sheet
	ogRuleBlue  = color.RGBA{0xDC, 0xE8, 0xF5, 0xFF} // rule-blue
	ogMarginRed = color.RGBA{0xF0, 0xA5, 0xA5, 0xFF} // margin-red
	ogInk       = color.RGBA{0x1F, 0x1B, 0x16, 0xFF} // ink
	ogPencil    = color.RGBA{0x5E, 0x57, 0x4D, 0xFF} // pencil
	ogHighlight = color.RGBA{0xFF, 0xE2, 0x7A, 0xFF} // highlight-yellow (swipe)
	ogTape      = color.NRGBA{0xFF, 0xE2, 0x7A, 0xD9} // washi tape, translucent yellow
	ogShadow    = color.NRGBA{0x1F, 0x1B, 0x16, 0x26} // shadow-paper, ink at ~15%
)

// Sheet geometry shared by both cards.
const (
	ogSheetInset  = 48.0
	ogSheetRadius = 16.0
	ogRuleGap     = 36.0
	ogTextX       = 200.0 // right of the margin line
	ogTextMaxW    = 900.0
)

func NewGGGenerator() (*GGGenerator, error) {
	display, err := truetype.Parse(outfitExtraBoldTTF)
	if err != nil {
		return nil, fmt.Errorf("parse display font: %w", err)
	}
	hand, err := truetype.Parse(caveatBoldTTF)
	if err != nil {
		return nil, fmt.Errorf("parse hand font: %w", err)
	}
	mono, err := truetype.Parse(jetbrainsMonoBoldTTF)
	if err != nil {
		return nil, fmt.Errorf("parse mono font: %w", err)
	}
	return &GGGenerator{display: display, hand: hand, mono: mono}, nil
}

func (g *GGGenerator) face(f *truetype.Font, size float64) font.Face {
	return truetype.NewFace(f, &truetype.Options{Size: size, DPI: 72, Hinting: font.HintingNone})
}

// drawNotebook paints the desk, dot grid, shadow, ruled sheet, margin line and
// tape strips. Text is drawn by the callers on top.
func (g *GGGenerator) drawNotebook(dc *gg.Context) {
	dc.SetColor(ogPaper)
	dc.Clear()

	// Dot grid on the desk, 26px pitch.
	dc.SetColor(ogPaperDot)
	for y := 13.0; y < OGHeight; y += 26 {
		for x := 13.0; x < OGWidth; x += 26 {
			dc.DrawCircle(x, y, 2)
			dc.Fill()
		}
	}

	sx, sy := ogSheetInset, ogSheetInset
	sw, sh := OGWidth-2*ogSheetInset, OGHeight-2*ogSheetInset

	// Offset paper shadow, then the sheet.
	dc.SetColor(ogShadow)
	dc.DrawRoundedRectangle(sx+8, sy+10, sw, sh, ogSheetRadius)
	dc.Fill()
	dc.SetColor(ogSheet)
	dc.DrawRoundedRectangle(sx, sy, sw, sh, ogSheetRadius)
	dc.Fill()

	// Blue rules every 36px, clipped to the sheet interior.
	dc.SetColor(ogRuleBlue)
	dc.SetLineWidth(2)
	for y := sy + 84; y < sy+sh-12; y += ogRuleGap {
		dc.DrawLine(sx+12, y, sx+sw-12, y)
		dc.Stroke()
	}

	// Red margin line.
	dc.SetColor(ogMarginRed)
	dc.SetLineWidth(3)
	dc.DrawLine(sx+110, sy+6, sx+110, sy+sh-6)
	dc.Stroke()

	// Two washi-tape strips at small angles (max 3 degrees).
	g.tape(dc, sx+90, sy+2, 150, 40, -3)
	g.tape(dc, sx+sw-110, sy+2, 150, 40, 2.5)
}

func (g *GGGenerator) tape(dc *gg.Context, cx, cy, w, h, deg float64) {
	dc.Push()
	dc.RotateAbout(gg.Radians(deg), cx, cy)
	dc.SetColor(ogTape)
	dc.DrawRectangle(cx-w/2, cy-h/2, w, h)
	dc.Fill()
	dc.Pop()
}

// highlight draws a yellow highlighter swipe behind a text run whose vertical
// centre is cy and which starts at x with width w.
func (g *GGGenerator) highlight(dc *gg.Context, x, cy, w, h float64) {
	dc.Push()
	dc.RotateAbout(gg.Radians(-0.8), x, cy)
	dc.SetColor(ogHighlight)
	dc.DrawRoundedRectangle(x-10, cy-h/2, w+20, h, 6)
	dc.Fill()
	dc.Pop()
}

// siteHost reduces a site URL to its host for the meta line.
func siteHost(siteURL string) string {
	s := strings.TrimSpace(siteURL)
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		return u.Host
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	return strings.TrimRight(s, "/")
}

func encodeOG(dc *gg.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderPost draws the notebook card for a blog post. Title is wrapped to the
// text column; if it is still too tall the font shrinks down the ladder.
func (g *GGGenerator) RenderPost(title, authorName, siteURL string) ([]byte, error) {
	dc := gg.NewContext(OGWidth, OGHeight)
	g.drawNotebook(dc)

	// "From the notebook" label on a highlighter swipe.
	dc.SetFontFace(g.face(g.hand, 42))
	label := "From the notebook"
	lw, _ := dc.MeasureString(label)
	g.highlight(dc, ogTextX, 128, lw, 54)
	dc.SetColor(ogInk)
	dc.DrawString(label, ogTextX, 142)

	// Title in Outfit ExtraBold.
	wrapped, fontSize := g.fitTitle(title, ogTextMaxW, 290)
	dc.SetFontFace(g.face(g.display, fontSize))
	dc.SetColor(ogInk)
	dc.DrawStringWrapped(wrapped, ogTextX, 186, 0, 0, ogTextMaxW, ogTitleLineHeight, gg.AlignLeft)

	// Footer: handwritten byline left, mono meta right.
	dc.SetFontFace(g.face(g.hand, 44))
	dc.SetColor(ogPencil)
	dc.DrawString("by "+authorName, ogTextX, 548)
	dc.SetFontFace(g.face(g.mono, 24))
	dc.DrawStringAnchored(siteHost(siteURL)+" · POST", OGWidth-ogSheetInset-48, 540, 1, 0.5)

	return encodeOG(dc)
}

// RenderHomepage draws the static homepage card: name on a highlighter swipe,
// the headline in handwriting, and the site host.
func (g *GGGenerator) RenderHomepage(name, headline, siteURL string) ([]byte, error) {
	dc := gg.NewContext(OGWidth, OGHeight)
	g.drawNotebook(dc)

	// Largest name size that fits the text column.
	size := 112.0
	for ; size > 56; size -= 4 {
		dc.SetFontFace(g.face(g.display, size))
		if w, _ := dc.MeasureString(name); w <= ogTextMaxW {
			break
		}
	}
	dc.SetFontFace(g.face(g.display, size))
	nw, _ := dc.MeasureString(name)
	const nameCY = 272.0
	g.highlight(dc, ogTextX, nameCY+size*0.12, nw, size*0.62)
	dc.SetColor(ogInk)
	dc.DrawStringAnchored(name, ogTextX, nameCY, 0, 0.5)

	dc.SetFontFace(g.face(g.hand, 64))
	dc.SetColor(ogPencil)
	dc.DrawString(headline, ogTextX, 400)

	dc.SetFontFace(g.face(g.mono, 26))
	dc.DrawString(siteHost(siteURL), ogTextX, 548)

	return encodeOG(dc)
}

// ogTitleLineHeight is the line-height multiple for post titles.
const ogTitleLineHeight = 1.12

// ogTitleSizes is the title size ladder, tuned for Outfit ExtraBold.
var ogTitleSizes = []float64{80, 72, 64, 56, 48, 42}

// fitTitle picks the largest font size from a fixed ladder whose wrapped height
// (1.12 line-height) fits maxHeight. Returns the chosen string and font size.
// At the smallest size it still returns the wrapped string even if it overflows
// — the renderer will clip, but we prefer ugly over crashed.
func (g *GGGenerator) fitTitle(title string, maxW, maxH float64) (string, float64) {
	sizes := ogTitleSizes
	for _, sz := range sizes {
		face := g.face(g.display, sz)
		dc := gg.NewContext(OGWidth, OGHeight)
		dc.SetFontFace(face)
		lines := dc.WordWrap(title, maxW)
		lineH := sz * ogTitleLineHeight
		if float64(len(lines))*lineH <= maxH {
			return strings.Join(lines, "\n"), sz
		}
	}
	face := g.face(g.display, ogTitleSizes[len(ogTitleSizes)-1])
	dc := gg.NewContext(OGWidth, OGHeight)
	dc.SetFontFace(face)
	return strings.Join(dc.WordWrap(title, maxW), "\n"), ogTitleSizes[len(ogTitleSizes)-1]
}
