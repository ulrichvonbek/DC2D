package main

import (
	"fmt"
	"image/color"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const (
	// The status bar is a strip two dungeon squares tall, drawn below the
	// dungeon itself, which keeps its original window size.
	statusBarHeight = tileSize * 2

	baseHR = 80 // resting heart rate in beats per minute

	// Heart-rate thresholds scale with player level; these are the level-0
	// baselines. passOut rises 1 BPM per level, death 0.5 BPM per level, and
	// the movement ceiling stays 25 BPM above passOut so sprinting can
	// always black the player out. Cap at hrMaxLevel keeps death above
	// passOut (gap shrinks 0.5/level, bottoming out at 10).
	hrPassOut   = 180
	hrDeath     = 200
	hrAsymptote = hrPassOut + 25 // movement ramp ceiling
	hrWake      = 150            // vision returns below this
	hrMaxLevel  = 20

	// Approach rates per second: HR eases toward its target with
	// 1-exp(-rate*dt). Rates are guessy and easy to tune.
	moveClimbRate    = 0.18
	restRecoveryRate = 0.12

	// How fast the blackout overlay fades in and out, per second.
	blackoutFadeRate = 4.0

	hrSquareSmall = 16 // px, the small pulse state
	hrSquarePulse = 2  // px, how much wider the other state is

	// Numeric HR display next to the heart icon.
	hrTextRefresh = 500 * time.Millisecond // refresh rate of the number
	hrTextGap     = 6                      // px gap between the icon and the number

	hrBumpSpike = 6 // extra BPM per wall bump
)

var (
	hrColor     = color.RGBA{200, 40, 40, 255}
	barBG       = color.RGBA{64, 68, 88, 255}    // lighter than the dungeon floor, so the bar reads as a UI chrome rather than a world element
	barOutline  = color.RGBA{122, 128, 160, 255} // 1px border separating bar from dungeon
	hrTextColor = color.RGBA{220, 220, 220, 255}
	handLabel   = color.RGBA{120, 124, 148, 255} // placeholder color for an empty hand
)

const (
	barMargin = 12 // px inset for the left/right hand labels
)

// HRDisplay remembers the last rendered BPM so it can be redrawn in place;
// the number itself refreshes at hrTextRefresh.
type HRDisplay struct {
	text string
	at   time.Time
}

// Text returns the current BPM rounded to a whole number, recomputed at most
// once per second.
func (d *HRDisplay) Text(hr *HeartRate, now time.Time) string {
	if d.text == "" || now.Sub(d.at) >= hrTextRefresh {
		d.text = fmt.Sprintf("%d", int(math.Round(hr.bpm)))
		d.at = now
	}
	return d.text
}

// HRThresholds are the pass-out, death, and ramp-ceiling values for a player
// level.
type HRThresholds struct {
	passOut   float64
	death     float64
	asymptote float64
}

// hrThresholds returns the thresholds for a player level, clamped to
// hrMaxLevel. passOut = 180+L, death = 200+0.5L, asymptote = passOut+25, so
// at max level passOut is 200 and death 210 (gap 10, never inverted).
func hrThresholds(level int) HRThresholds {
	if level < 0 {
		level = 0
	}
	if level > hrMaxLevel {
		level = hrMaxLevel
	}
	passOut := hrPassOut + level
	return HRThresholds{
		passOut:   float64(passOut),
		death:     float64(hrDeath) + 0.5*float64(level),
		asymptote: float64(hrAsymptote + level),
	}
}

// HeartRate drives both the pulse animation and the physical state of the
// player. BPM rises exponentially toward the threshold asymptote while
// moving and eases back toward baseHR while resting.
type HeartRate struct {
	bpm     float64
	beats   float64 // fractional beats since the level started
	fainted bool
	th      HRThresholds
}

func NewHeartRate(bpm int) *HeartRate {
	return &HeartRate{bpm: float64(bpm), th: hrThresholds(0)}
}

func (h *HeartRate) BPM() float64 { return h.bpm }

// Death returns the current death threshold in BPM.
func (h *HeartRate) Death() float64 { return h.th.death }

// SetThresholds updates the pass-out, death, and ceiling values for a new
// player level.
func (h *HeartRate) SetThresholds(th HRThresholds) {
	h.th = th
}

// Bump adds an instantaneous spike, used when the player runs into a wall.
func (h *HeartRate) Bump(n float64) {
	h.bpm += n
}

// Update eases the BPM toward its target for this frame. moving=true pushes
// it toward the threshold asymptote; otherwise it recovers toward baseHR.
// The beat clock is integrated from dt so each beat advances exactly one
// flip, regardless of how BPM is ramping.
func (h *HeartRate) Update(dt time.Duration, moving bool) {
	target, rate := float64(baseHR), restRecoveryRate
	if moving {
		target, rate = h.th.asymptote, moveClimbRate
	}
	s := dt.Seconds()
	h.bpm += (target - h.bpm) * (1 - math.Exp(-rate*s))
	h.beats += h.bpm * s / 60
}

// passedOut reports whether the player is unconscious, applying hysteresis:
// pass out at the pass-out threshold, wake only once BPM falls to hrWake or
// below.
func (h *HeartRate) passedOut() bool {
	if h.fainted {
		if h.bpm <= hrWake {
			h.fainted = false
		}
	} else if h.bpm >= h.th.passOut {
		h.fainted = true
	}
	return h.fainted
}

// big reports whether the pulse square currently renders one pixel wider.
// Each integrated beat flips it, so the two sizes always alternate cleanly
// and the flip rate matches the current BPM.
func (h *HeartRate) big() bool {
	return int64(h.beats)%2 == 0
}

func (h *HeartRate) size() int {
	if h.big() {
		return hrSquareSmall + hrSquarePulse
	}
	return hrSquareSmall
}

var (
	barImgOnce   sync.Once
	barImage     *ebiten.Image
	hrSquares    [2]*ebiten.Image // index 0 = small, 1 = big
	blackImgOnce sync.Once
	blackImg     *ebiten.Image
)

// statusImages builds the bar background and the two pulse square sizes
// once; blackOverlay builds the dungeon-area fade layer. The overlay is
// sized to the dungeon viewport only, so the status bar below it stays
// visible during a blackout.
func statusImages() (*ebiten.Image, [2]*ebiten.Image) {
	barImgOnce.Do(func() {
		barImage = ebiten.NewImage(screenWidth, statusBarHeight)
		barImage.Fill(barOutline)
		inner := ebiten.NewImage(screenWidth-2, statusBarHeight-2)
		inner.Fill(barBG)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(1, 1)
		barImage.DrawImage(inner, op)
		for i := 1; i >= 0; i-- {
			s := hrSquareSmall + hrSquarePulse*i
			img := ebiten.NewImage(s, s)
			img.Fill(hrColor)
			hrSquares[i] = img
		}
	})
	return barImage, hrSquares
}

func blackOverlay() *ebiten.Image {
	blackImgOnce.Do(func() {
		blackImg = ebiten.NewImage(screenWidth, screenHeight)
		blackImg.Fill(color.RGBA{0, 0, 0, 255})
	})
	return blackImg
}

var (
	maskCellOnce  sync.Once
	maskCell      *ebiten.Image // one black dungeon square
	maskComposite *ebiten.Image // viewport-sized mask, rebuilt as layers grow
	maskBuiltFor  int
)

// maskCellImage returns the single-tile black square every covered dungeon
// square of the blackout mask is made of.
func maskCellImage() *ebiten.Image {
	maskCellOnce.Do(func() {
		maskCell = ebiten.NewImage(tileSize, tileSize)
		maskCell.Fill(color.RGBA{0, 0, 0, 255})
	})
	return maskCell
}

// maskView builds (or reuses, if the layer count is unchanged) the composite
// blackout mask: one black tile for every dungeon square whose maskRank is at
// or below `layer`, so the tiled dither layers darken the dungeon dot by dot.
// It is rebuilt only when the mask steps, so a fully blocked frame costs a
// single image draw.
func maskView(layer int) *ebiten.Image {
	if maskComposite != nil && maskBuiltFor == layer {
		return maskComposite
	}
	maskComposite = ebiten.NewImage(screenWidth, screenHeight)
	cell := maskCellImage()
	for ty := 0; ty < screenHeight/tileSize; ty++ {
		for tx := 0; tx < screenWidth/tileSize; tx++ {
			if maskRank(tx, ty) <= layer {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(tx*tileSize), float64(ty*tileSize))
				maskComposite.DrawImage(cell, op)
			}
		}
	}
	maskBuiltFor = layer
	return maskComposite
}

func DrawStatusBar(screen *ebiten.Image, hr *HeartRate, disp *HRDisplay, now time.Time) {
	bar, squares := statusImages()

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, screenHeight)
	screen.DrawImage(bar, op)

	// The pulsing square with its numeric BPM beside it, centered as one
	// group so the pair grows and shrinks around the middle of the bar.
	s := hr.size()
	txt := disp.Text(hr, now)
	face := basicfont.Face7x13
	ascent := face.Metrics().Ascent.Ceil()
	txtW := font.MeasureString(face, txt).Ceil()

	groupW := s + hrTextGap + txtW
	x := (screenWidth - groupW) / 2

	op = &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(screenHeight)+(float64(statusBarHeight)-float64(s))/2)
	screen.DrawImage(squares[(s-hrSquareSmall)/hrSquarePulse], op)

	baseline := screenHeight + statusBarHeight/2 + ascent/2

	// Left and right hand slots, empty for now: two "EMPTY" placeholders
	// showing the player holds nothing. These will age into held-item names.
	empty := "EMPTY"
	text.Draw(screen, empty, face, barMargin, baseline, handLabel)
	rightX := screenWidth - barMargin - font.MeasureString(face, empty).Ceil()
	text.Draw(screen, empty, face, rightX, baseline, handLabel)

	text.Draw(screen, txt, face, x+s+hrTextGap, baseline, hrTextColor)
}
