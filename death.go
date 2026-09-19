package main

import (
	"image/color"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// The death screen is a centered block that appears once the slow fade to
// black is finished. There are two deaths, and they must read differently:
//
//   - Overload (HR frozen between death and hrFlatline) keeps the doubt of a
//     passing-out spell that never lifts: a single quiet bone-gray line over
//     the black dungeon, no alarm. Only the restart/quit prompt betrays that
//     the run is over.
//
//   - Flatline (HR blew past hrFlatline and crashed to 0) is unmistakable: a
//     big red YOU DIED, a red EKG stripe across the screen, and the caption
//     naming the stopped heart.
//
// The glyphs are the standard 7x13 bitmap face scaled up, which reads as
// chunky tombstone lettering that matches the grid-art dungeon.

var (
	deathRed  = color.RGBA{230, 40, 28, 255}
	deathBone = color.RGBA{214, 210, 218, 255}
)

const (
	deathTitleScale = 4 // the flatline "YOU DIED"
	deathTextScale  = 2 // captions, the doubt line, and the prompt

	deathTitleGap  = 18 // px between the title and the caption
	deathPromptGap = 8  // px between the caption and the prompt

	deathFlatH = 8 // px, the flatline stripe
	deathFlatY = 3 // the stripe sits at 3/4 of the dungeon height
)

// buildDeathText renders one line of text through the 7x13 face onto a wide
// little canvas, then scales that canvas up by `scale` with nearest-neighbor
// interpolation into its final image.
func buildDeathText(s string, scale int, c color.RGBA) *ebiten.Image {
	face := basicfont.Face7x13
	ascent := face.Metrics().Ascent.Ceil()
	w := font.MeasureString(face, s).Ceil()
	h := face.Metrics().Height.Ceil()

	base := ebiten.NewImage(w, h)
	text.Draw(base, s, face, 0, ascent, c)

	scaled := ebiten.NewImage(w*scale, h*scale)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(scale), float64(scale))
	scaled.DrawImage(base, op)
	return scaled
}

// The end-screen lines are fixed for the game's life, so they are drawn once
// and reused across frames and restarts.
var (
	deathOnce sync.Once
	deathTxt  struct {
		finalTitle   *ebiten.Image // YOU DIED (flatline only)
		finalCaption *ebiten.Image // YOUR HEART STOPPED (flatline only)
		doubtTitle   *ebiten.Image // YOU DON'T WAKE UP (overload only)
		prompt       *ebiten.Image // R TO TRY AGAIN   ESC TO QUIT
		flatline     *ebiten.Image // the red stripe
	}
)

func ensureDeathImages() {
	deathOnce.Do(func() {
		deathTxt.finalTitle = buildDeathText("YOU DIED", deathTitleScale, deathRed)
		deathTxt.finalCaption = buildDeathText("YOUR HEART STOPPED", deathTextScale, deathBone)
		deathTxt.doubtTitle = buildDeathText("YOU DON'T WAKE UP", deathTextScale, deathBone)
		deathTxt.prompt = buildDeathText("R TO TRY AGAIN   ESC TO QUIT", deathTextScale,
			color.RGBA{140, 142, 150, 255})

		deathTxt.flatline = ebiten.NewImage(screenWidth, deathFlatH)
		deathTxt.flatline.Fill(deathRed)
	})
}

// DrawDeath renders the death block centered in the dungeon area, choosing
// the overload or flatline presentation. It is only called after deathReady
// passes, so the text lands on final black instead of struggling against the
// fade.
func DrawDeath(screen *ebiten.Image, flatline bool) {
	ensureDeathImages()

	cx := screenWidth / 2
	title, caption := deathTxt.doubtTitle, (*ebiten.Image)(nil)
	if flatline {
		title, caption = deathTxt.finalTitle, deathTxt.finalCaption
	}

	titleH := title.Bounds().Dy()
	captionH := 0
	if caption != nil {
		captionH = caption.Bounds().Dy()
	}
	titleGap := deathTitleGap
	if caption == nil {
		titleGap = 0
	}
	total := titleH + titleGap + captionH + deathPromptGap + deathTxt.prompt.Bounds().Dy()
	top := (screenHeight - total) / 2

	if flatline {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, float64(screenHeight*deathFlatY/4))
		screen.DrawImage(deathTxt.flatline, op)
	}

	drawCentered(screen, title, cx, top)
	y := top + titleH + titleGap
	if caption != nil {
		drawCentered(screen, caption, cx, y)
		y += captionH
	}
	drawCentered(screen, deathTxt.prompt, cx, y+deathPromptGap)
}

func drawCentered(screen, img *ebiten.Image, cx, top int) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(cx-img.Bounds().Dx()/2), float64(top))
	screen.DrawImage(img, op)
}

// deathFadeDone is the alpha the blackout must reach before the end screen is
// shown; deathScreenDelay is the fateful beat of black in between, so the
// player watches the freeze (or the crash to 0) on the status bar for a full
// three seconds before the words appear.
const (
	deathFadeDone    = 0.99
	deathScreenDelay = 3 * time.Second
)

// deathReady reports whether the fade to black has finished and the fateful
// beat has passed, making both the restart and quit keys live.
func deathReady(alpha float64, elapsed time.Duration) bool {
	return alpha >= deathFadeDone && elapsed >= deathScreenDelay
}
