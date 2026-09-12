package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// The help modal is laid out as a two-column top section (verbs and the
// hand prepositions they can be completed with) and a bottom examples
// section showing how the pieces compose. planned terms are not yet bound in
// code and render struck through.
const (
	helpMargin = 60
	helpColGap = 24
)

type helpTerm struct {
	keys    string
	phrase  string
	planned bool
}

var helpVerbs = []helpTerm{
	{"W", "Move forward", false},
	{"S", "Move back", false},
	{"A", "Step left", false},
	{"D", "Step right", false},
	{"Q", "Turn left", false},
	{"E", "Turn right", false},
	{"j", "Attack", false},
	{"g", "Get item", true},
	{"h", "Drop from hand", true},
	{"n", "Stow in backpack", true},
	{"p", "Pull item from backpack", true},
	{"u", "Use an item", true},
	{"=", "Swap hands", true},
}

var helpModifiers = []helpTerm{
	{"[", "Left hand", false},
	{"]", "Right hand", false},
}

var helpExamples = []helpTerm{
	{"j [", "Attack with left hand", false},
	{"j ]", "Attack with right hand", false},
	{"Space / Esc", "Discard a pending command", false},
	{"?", "Show or hide this help", false},
}

// helpKeyJustPressed reports whether the ? key (Shift + /) fired this frame.
func helpKeyJustPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySlash) && ebiten.IsKeyPressed(ebiten.KeyShift)
}

// helpTopRows is the taller of the two top columns.
func helpTopRows() int {
	if len(helpModifiers) > len(helpVerbs) {
		return len(helpModifiers)
	}
	return len(helpVerbs)
}

// helpRows is the full help body: title, column headers, the top columns,
// a sep, the examples header, then the examples.
func helpRows() int {
	return 2 + helpTopRows() + 2 + len(helpExamples)
}

// helpPanelHeight is the bordered help box height in px.
func helpPanelHeight() int {
	return helpRows()*infoLineHeight + 2*infoSidePad
}

// helpKeyColumnWidth is the widest binding text of a column plus a gap.
func helpKeyColumnWidth(face font.Face, terms []helpTerm) int {
	w := 0
	for _, e := range terms {
		if m := font.MeasureString(face, e.keys).Ceil(); m > w {
			w = m
		}
	}
	return w + 16
}

// drawStrikethrough renders a phrase with a horizontal bar through the middle
// of the glyphs, the stand-in for not-yet-implemented.
func drawStrikethrough(screen *ebiten.Image, s string, face font.Face, x, baseline int, c color.Color) {
	text.Draw(screen, s, face, x, baseline, c)

	ascent := face.Metrics().Ascent.Ceil()
	w := font.MeasureString(face, s).Ceil()

	bar := ebiten.NewImage(1, 1)
	bar.Fill(c)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), 1)
	op.GeoM.Translate(float64(x), float64(baseline-ascent/2))
	screen.DrawImage(bar, op)
}

// drawHelpColumn renders one aligned key/phrase column starting at the top of
// a given row.
func drawHelpColumn(screen *ebiten.Image, face font.Face, terms []helpTerm, x, rowY, keyW int) {
	ascent := face.Metrics().Ascent.Ceil()
	for i, e := range terms {
		base := rowY + i*infoLineHeight + ascent
		text.Draw(screen, e.keys, face, x, base, infoLatest)
		px := x + keyW
		if e.planned {
			drawStrikethrough(screen, e.phrase, face, px, base, infoText)
		} else {
			text.Draw(screen, e.phrase, face, px, base, infoText)
		}
	}
}

// DrawHelp renders the command reference as a centered modal over the dungeon.
func DrawHelp(screen *ebiten.Image) {
	h := helpPanelHeight()
	x := helpMargin
	w := screenWidth - 2*helpMargin
	y := (screenHeight - h) / 2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(borderedPanel(w, h), op)

	face := basicfont.Face7x13
	ascent := face.Metrics().Ascent.Ceil()

	groupW := (w - 2*infoColPad - helpColGap) / 2
	verbX := x + infoColPad
	modX := verbX + groupW + helpColGap

	rowY := func(row int) int { return y + infoSidePad + row*infoLineHeight }

	text.Draw(screen, "COMMANDS", face, verbX, rowY(0)+ascent, infoHeader)
	text.Draw(screen, "VERB", face, verbX, rowY(1)+ascent, infoHeader)
	text.Draw(screen, "MODIFIER", face, modX, rowY(1)+ascent, infoHeader)

	drawHelpColumn(screen, face, helpVerbs, verbX, rowY(2), helpKeyColumnWidth(face, helpVerbs))
	drawHelpColumn(screen, face, helpModifiers, modX, rowY(2), helpKeyColumnWidth(face, helpModifiers))

	// Separator between the top columns and the examples section.
	sepRow := 2 + helpTopRows()
	sepY := rowY(sepRow) + infoLineHeight/2
	sep := ebiten.NewImage(1, 1)
	sep.Fill(infoSep)
	sepOp := &ebiten.DrawImageOptions{}
	sepOp.GeoM.Scale(float64(w-2*infoColPad), 1)
	sepOp.GeoM.Translate(float64(verbX), float64(sepY))
	screen.DrawImage(sep, sepOp)

	exRow := sepRow + 1
	text.Draw(screen, "EXAMPLES", face, verbX, rowY(exRow)+ascent, infoHeader)
	drawHelpColumn(screen, face, helpExamples, verbX, rowY(exRow+1), helpKeyColumnWidth(face, helpExamples))
}
