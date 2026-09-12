package main

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// The info panel is a three-column strip below the status bar: the
// player-input log, items lying on the current floor, and the backpack. Row
// height matches the 7x13 font.
const (
	infoHeaderRow   = 1
	infoContentRows = 8
	infoLineHeight  = 13 // px, basicfont.Face7x13
	infoSidePad     = 6  // px above/below the text block
	infoColPad      = 12 // px inside each column edge
	infoPanelHeight = (infoHeaderRow+infoContentRows)*infoLineHeight + infoSidePad*2
)

var (
	infoBG      = color.RGBA{34, 36, 46, 255}
	infoOutline = color.RGBA{78, 82, 104, 255}
	infoSep     = color.RGBA{54, 56, 72, 255}
	infoHeader  = color.RGBA{128, 132, 152, 255}
	infoText    = color.RGBA{140, 142, 150, 255} // older input history, neutral gray
	infoLatest  = color.RGBA{240, 244, 255, 255} // the most recent input command
	infoDim     = color.RGBA{88, 92, 110, 255}
	// Outcome runs are tinted by their result: bright when the line is
	// newest, dimming as it scrolls into history.
	infoHitBright  = color.RGBA{120, 220, 130, 255}
	infoHitDim     = color.RGBA{72, 128, 80, 255}
	infoMissBright = color.RGBA{252, 120, 108, 255}
	infoMissDim    = color.RGBA{172, 86, 80, 255}
)

// cmdStyle tags an input-log run with its semantic color.
type cmdStyle int

const (
	cmdNormal cmdStyle = iota // neutral history color
	cmdHit                    // damaging swings: green
	cmdMiss                   // whiffed swings: red
)

// logSeg is one colored run of text within an input-log line.
type logSeg struct {
	text  string
	style cmdStyle
}

// logEntry is one line of the input log, possibly split into colored runs
// (e.g. "Attack Left - " neutral + "Hit Spider!" green).
type logEntry struct {
	segments []logSeg
}

// logNormal builds a single-run neutral entry.
func logNormal(text string) logEntry {
	return logEntry{segments: []logSeg{{text: text, style: cmdNormal}}}
}

// logStyled builds a single-run entry with an explicit style (e.g. the red
// bright-dim warning pair for passing out).
func logStyled(text string, style cmdStyle) logEntry {
	return logEntry{segments: []logSeg{{text: text, style: style}}}
}

// bumpEntry builds a wall-bump line: the movement attempt plus a red !OUCH!,
// mirroring the styled outcome runs of an attack ("Move Forward !OUCH!").
func bumpEntry(label string) logEntry {
	return logEntry{segments: []logSeg{
		{text: label + " ", style: cmdNormal},
		{text: "!OUCH!", style: cmdMiss},
	}}
}

// textColor picks the color for a run based on its style and whether its line
// is the newest (bright) or has receded into history (dim).
func textColor(s cmdStyle, latest bool) color.Color {
	switch s {
	case cmdHit:
		if latest {
			return infoHitBright
		}
		return infoHitDim
	case cmdMiss:
		if latest {
			return infoMissBright
		}
		return infoMissDim
	}
	if latest {
		return infoLatest
	}
	return infoText
}

var (
	infoImgOnce sync.Once
	infoImg     *ebiten.Image
)

// borderedPanel builds a bordered block with the info-panel palette (outline
// frame, dark background), sized w x h.
func borderedPanel(w, h int) *ebiten.Image {
	img := ebiten.NewImage(w, h)
	img.Fill(infoOutline)
	inner := ebiten.NewImage(w-2, h-2)
	inner.Fill(infoBG)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(1, 1)
	img.DrawImage(inner, op)
	return img
}

// infoPanelImg builds the panel background once: a bordered block with
// vertical separators between the three columns.
func infoPanelImg() *ebiten.Image {
	infoImgOnce.Do(func() {
		infoImg = borderedPanel(screenWidth, infoPanelHeight)

		sep := ebiten.NewImage(1, infoPanelHeight)
		sep.Fill(infoSep)
		for _, x := range []int{screenWidth / 3, 2 * screenWidth / 3} {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), 0)
			infoImg.DrawImage(sep, op)
		}
	})
	return infoImg
}

// DrawInfoPanel renders the three columns below the status bar. A column
// with no entries shows a dim dash.
func DrawInfoPanel(screen *ebiten.Image, input []logEntry, floor, backpack []string) {
	top := screenHeight + statusBarHeight
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, float64(top))
	screen.DrawImage(infoPanelImg(), op)

	face := basicfont.Face7x13
	ascent := face.Metrics().Ascent.Ceil()
	headerY := top + infoSidePad + ascent
	firstRowY := top + infoSidePad + infoLineHeight + ascent

	colW := screenWidth / 3
	headers := []string{"INPUT", "FLOOR", "BACKPACK"}
	sets := [][]logEntry{input, logSlice(floor), logSlice(backpack)}
	for i, set := range sets {
		x := i*colW + infoColPad
		text.Draw(screen, headers[i], face, x, headerY, infoHeader)
		// The input log scrolls newest-first; its top line is highlighted.
		drawInfoColumn(screen, face, set, x, firstRowY)
	}
}

func logSlice(rows []string) []logEntry {
	out := make([]logEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, logNormal(r))
	}
	return out
}

func drawInfoColumn(screen *ebiten.Image, face font.Face, entries []logEntry, x, firstBaseline int) {
	if len(entries) == 0 {
		text.Draw(screen, "\u2014", face, x, firstBaseline, infoDim)
		return
	}
	for i := 0; i < infoContentRows && i < len(entries); i++ {
		cx := x
		for _, seg := range entries[i].segments {
			text.Draw(screen, seg.text, face, cx, firstBaseline+i*infoLineHeight, textColor(seg.style, i == 0))
			cx += font.MeasureString(face, seg.text).Ceil()
		}
	}
}
