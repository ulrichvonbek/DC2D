package main

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// A lethal heart rate is what the snake experiment surfaced: a spike past the
// death threshold must flip the game into its end state and start the fade,
// not keep playing as if nothing happened.
func TestDeathFlagSetOnLethalHR(t *testing.T) {
	g := NewGame()
	g.heartRate.Bump(g.heartRate.Death() + 1 - baseHR) // just past the threshold

	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if !g.dead {
		t.Fatal("lethal HR did not set the dead state")
	}
	if g.flatlined {
		t.Fatal("a threshold-crossing hit was treated as a flatline")
	}
	if g.diedAt.IsZero() {
		t.Fatal("death transition did not record its time")
	}
	if g.blackoutAlpha <= 0 {
		t.Fatal("the fade did not start")
	}
}

// An overload death (death threshold..hrFlatline) freezes HR in place: it
// stays where the lethal hit landed and never recovers across later frames.
func TestOverloadFreezesHRInPlace(t *testing.T) {
	g := NewGame()
	g.level.Monsters = nil // keep enemy spikes from upsetting the exact BPM
	g.heartRate.Bump(130)  // 80 + 130 = 210, inside the overload band

	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if !g.dead {
		t.Fatal("overload did not set the dead state")
	}
	if g.flatlined {
		t.Fatal("an overload was treated as a flatline")
	}
	frozen := g.heartRate.BPM()
	if frozen < 200 || frozen > 220 {
		t.Fatalf("HR = %.2f, want it inside the overload band 200..220", frozen)
	}
	// The bar hides the frozen figure behind ??? so a barely-fatal hit reads
	// exactly like a faint; only a flatline reveals 0.
	disp := &HRDisplay{}
	if got := disp.Text(g.heartRate, time.Now()); got != "???" {
		t.Fatalf("overload bar shows %q, want %q", got, "???")
	}

	// Later frames must not touch the frozen HR: no recovery, no drift.
	before := g.heartRate.BPM()
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if after := g.heartRate.BPM(); after != before {
		t.Fatalf("HR moved after death: %.2f -> %.2f, want it frozen", before, after)
	}
}

// A flatline death (HR > hrFlatline) crashes the bar to 0: the heart stops.
func TestFlatlineZeroesHR(t *testing.T) {
	g := NewGame()
	g.level.Monsters = nil
	g.heartRate.Bump(141) // 80 + 141 = 221, past the flatline cap

	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if !g.dead {
		t.Fatal("flatline did not set the dead state")
	}
	if !g.flatlined {
		t.Fatal("a > hrFlatline hit was not marked as a flatline")
	}
	if got := g.heartRate.BPM(); got != 0 {
		t.Fatalf("HR = %.0f after flatline, want 0", got)
	}
	// Devastation is not hidden: the bar drops to a stark 0.
	disp := &HRDisplay{}
	if got := disp.Text(g.heartRate, time.Now()); got != "0" {
		t.Fatalf("flatline bar shows %q, want %q", got, "0")
	}
}

func TestDeathReadyGate(t *testing.T) {
	if deathScreenDelay != 3*time.Second {
		t.Fatalf("deathScreenDelay = %v, want 3s", deathScreenDelay)
	}
	if deathReady(deathFadeDone-0.01, deathScreenDelay) {
		t.Error("ready before the fade finished")
	}
	if deathReady(deathFadeDone, deathScreenDelay-time.Millisecond) {
		t.Error("ready before the fateful beat passed")
	}
	if !deathReady(1, deathScreenDelay) {
		t.Error("not ready with the full fade and delay elapsed")
	}
}

// Restart is R on the death screen: it must clear every death and modal trace
// and rebuild a depth-1 world with a resting heart.
func TestRestartReturnsToStart(t *testing.T) {
	g := NewGame()
	g.dead = true
	g.diedAt = time.Now()
	g.flatlined = true
	g.blackoutAlpha = 0.7
	g.cmdPending = true
	g.helpOpen = true
	g.pushLog(logNormal("Attack Left"))
	g.heartRate.Bump(50)

	g.restart()

	if g.dead || g.flatlined || g.blackoutAlpha != 0 || !g.diedAt.IsZero() {
		t.Fatalf("death state survived restart: dead=%v flat=%v alpha=%v",
			g.dead, g.flatlined, g.blackoutAlpha)
	}
	if g.cmdPending || g.helpOpen {
		t.Fatal("a modal state survived restart")
	}
	if len(g.inputLog) != 0 {
		t.Fatal("the input log survived restart")
	}
	if got := g.heartRate.BPM(); got != float64(baseHR) {
		t.Fatalf("HR = %.0f after restart, want %d", got, baseHR)
	}
	if g.level.Depth != 1 {
		t.Fatalf("level depth = %d after restart, want 1", g.level.Depth)
	}
}

// The death lettering is the 7x13 bitmap face scaled up; the sizing math must
// hold or the block will sit off-center.
func TestDeathTextScaling(t *testing.T) {
	const scale = 4
	img := buildDeathText("YOU DIED", scale, deathRed)
	face := basicfont.Face7x13
	wantW := scale * font.MeasureString(face, "YOU DIED").Ceil()
	wantH := scale * face.Metrics().Height.Ceil()
	if gotW, gotH := img.Bounds().Dx(), img.Bounds().Dy(); gotW != wantW || gotH != wantH {
		t.Fatalf("scaled text = %dx%d, want %dx%d", gotW, gotH, wantW, wantH)
	}
}

// Both presentations must lay out without panicking on the rendered screen.
func TestDrawDeathRenderBoth(t *testing.T) {
	screen := ebiten.NewImage(screenWidth, screenHeight)
	for _, flatline := range []bool{false, true} {
		DrawDeath(screen, flatline)
	}
}
