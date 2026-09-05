package main

import (
	"testing"
	"time"
)

func TestHeartRateTogglesAtPrescribedBPM(t *testing.T) {
	hr := NewHeartRate(baseHR)

	// Beats are integrated from dt, so a full period advances exactly one
	// beat, flipping the size cleanly at a rate of baseHR beats per minute.
	period := time.Minute / time.Duration(baseHR) // 750 ms at 80 bpm

	if s := hr.size(); s != hrSquareSmall+hrSquarePulse {
		t.Fatalf("square should start wide at beat 0 (got %d)", s)
	}

	// Half a beat: still wide.
	hr.Update(period/2, false)
	if s := hr.size(); s != hrSquareSmall+hrSquarePulse {
		t.Fatalf("square shrank too early: got %d", s)
	}

	// One full beat: narrow.
	hr.Update(period/2, false)
	if s := hr.size(); s != hrSquareSmall {
		t.Fatalf("expected narrow after one beat, got %d", s)
	}

	// Two beats: wide again.
	hr.Update(period, false)
	if s := hr.size(); s != hrSquareSmall+hrSquarePulse {
		t.Fatalf("expected wide after two beats, got %d", s)
	}
}

func TestHeartRatePulsesBetweenTwoSizes(t *testing.T) {
	hr := NewHeartRate(baseHR)

	period := time.Minute / time.Duration(baseHR)
	last := hr.size()
	for i := 0; i < 10; i++ {
		hr.Update(period, false) // exactly one beat at 80 bpm
		s := hr.size()
		if s != hrSquareSmall && s != hrSquareSmall+hrSquarePulse {
			t.Fatalf("square size %d out of range", s)
		}
		if s == last {
			t.Fatalf("expected the size to flip each beat, both %d", s)
		}
		last = s
	}
}

// The pulse must never stall: the gap between size flips has to track the
// current BPM even while it is ramping up and recovering. Drives the full
// pass-out / recovery cycle like the game does.
func TestHeartRatePulseNeverStallsWhileRamping(t *testing.T) {
	hr := NewHeartRate(baseHR)
	step := 16 * time.Millisecond
	conscious := true
	prevSize := hr.size()
	var lastFlip time.Duration
	var elapsed time.Duration

	for i := 0; i < 4000; i++ { // 64 simulated seconds
		elapsed += step
		hr.Update(step, conscious)
		conscious = !hr.passedOut()

		s := hr.size()
		if s == prevSize {
			continue
		}
		gap := elapsed - lastFlip
		// One flip per beat, so a gap of at most two beats is healthy.
		maxGap := time.Duration(2 * float64(time.Minute) / hr.bpm)
		if gap > maxGap {
			t.Fatalf("pulse stalled: %.0f ms gap at %.1f BPM (max %.0f ms)",
				float64(gap)/float64(time.Millisecond), hr.bpm, float64(maxGap)/float64(time.Millisecond))
		}
		lastFlip = elapsed
		prevSize = s
	}
}

func TestHeartRateAscentApproachesAsymptote(t *testing.T) {
	hr := NewHeartRate(baseHR)
	step := 100 * time.Millisecond

	prev := hr.bpm
	for i := 0; i < 200; i++ { // 20 simulated seconds of movement
		hr.Update(step, true)
		if hr.bpm <= prev {
			t.Fatalf("bpm did not rise during movement at step %d", i)
		}
		if hr.bpm >= hrAsymptote {
			t.Fatalf("bpm %.1f must approach %d from below", hr.bpm, hrAsymptote)
		}
		prev = hr.bpm
	}
	if hr.bpm < hrDeath {
		t.Fatalf("sustained raw movement should climb past the death threshold (got %.1f)", hr.bpm)
	}
}

func TestHeartRateRecoveryApproachesRest(t *testing.T) {
	hr := NewHeartRate(190)
	step := 250 * time.Millisecond

	for i := 0; i < 400; i++ { // 100 simulated seconds of rest
		prev := hr.bpm
		hr.Update(step, false)
		if hr.bpm >= prev {
			t.Fatalf("bpm must fall while resting at step %d", i)
		}
	}
	if hr.bpm > baseHR+1 {
		t.Fatalf("expected to approach rest, got %.1f", hr.bpm)
	}
}

func TestPassOutWakeHysteresis(t *testing.T) {
	hr := NewHeartRate(baseHR)

	hr.bpm = hrPassOut - 1
	if hr.passedOut() {
		t.Error("should be conscious at 179 BPM")
	}
	hr.bpm = hrPassOut
	if !hr.passedOut() {
		t.Error("should pass out at 180 BPM")
	}
	hr.bpm = hrWake + 1
	if !hr.passedOut() {
		t.Error("should stay passed out above the wake threshold")
	}
	hr.bpm = hrWake
	if hr.passedOut() {
		t.Error("should wake once BPM reaches 150")
	}
}

// Holding movement makes the player pass out before HR could ever reach the
// death threshold, because input is dropped while unconscious.
func TestMovementAloneCannotReachDeath(t *testing.T) {
	hr := NewHeartRate(baseHR)
	step := 100 * time.Millisecond

	conscious := true
	faintedOnce := false
	for i := 0; i < 3600; i++ { // 6 simulated minutes of holding a key
		hr.Update(step, conscious) // blacked-out, so no movement input
		conscious = !hr.passedOut()
		if hr.bpm >= hrDeath {
			t.Fatalf("movement drove BPM to %.1f, crossing death at %d", hr.bpm, hrDeath)
		}
		if !conscious {
			faintedOnce = true
		}
	}
	if !faintedOnce {
		t.Fatal("sustained movement should have caused a blackout")
	}
}

func TestHRDisplayRefreshesOncePerSecond(t *testing.T) {
	hr := NewHeartRate(baseHR)
	disp := &HRDisplay{}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if got := disp.Text(hr, t0); got != "80" {
		t.Fatalf("initial text = %q, want %q", got, "80")
	}

	// BPM changed, but stay inside the refresh window: still the old number.
	hr.bpm = 84.4
	if got := disp.Text(hr, t0.Add(400*time.Millisecond)); got != "80" {
		t.Fatalf("text refreshed too early: got %q, want %q", got, "80")
	}

	// Half a second later: the number updates, rounded.
	if got := disp.Text(hr, t0.Add(500*time.Millisecond)); got != "84" {
		t.Fatalf("text did not refresh at 500ms: got %q, want %q", got, "84")
	}

	// Rounding to the nearest whole number.
	hr.bpm = 179.5
	if got := disp.Text(hr, t0.Add(2*time.Second)); got != "180" {
		t.Fatalf("expected round-half-up, got %q", got)
	}
}

func TestHeartRateBumpSpikes(t *testing.T) {
	hr := NewHeartRate(120)
	hr.Bump(hrBumpSpike)
	if hr.bpm != 120+hrBumpSpike {
		t.Fatalf("bump did not add %d BPM: got %.1f", hrBumpSpike, hr.bpm)
	}
}

func TestWallBumpingCannotReachDeath(t *testing.T) {
	hr := NewHeartRate(baseHR)
	step := 100 * time.Millisecond
	repeat := 150 * time.Millisecond

	conscious := true
	fainted := false
	var elapsed time.Duration
	for i := 0; elapsed < 2*time.Minute; i++ {
		elapsed += step
		if conscious {
			// Movement ramp against the wall, plus a bump every key repeat.
			hr.Update(step, true)
			if time.Duration(i)*step%repeat == 0 {
				hr.Bump(hrBumpSpike)
			}
		} else {
			hr.Update(step, false)
		}
		conscious = !hr.passedOut()
		if hr.bpm >= hrDeath {
			t.Fatalf("holding a move key into a wall reached %.1f BPM (death at %d)", hr.bpm, hrDeath)
		}
		if !conscious {
			fainted = true
		}
	}
	if !fainted {
		t.Fatal("wall bumping should black the player out")
	}
}

func TestWallBumpingBlackoutsSoonerThanRampAlone(t *testing.T) {
	step := 100 * time.Millisecond
	repeat := 150 * time.Millisecond

	sim := func(bumping bool) time.Duration {
		hr := NewHeartRate(baseHR)
		conscious := true
		var faintedAt time.Duration
		for i := 0; i < 1200; i++ {
			if !conscious {
				break
			}
			hr.Update(step, true)
			if bumping && time.Duration(i)*step%repeat == 0 {
				hr.Bump(hrBumpSpike)
			}
			conscious = !hr.passedOut()
			faintedAt = time.Duration(i) * step
		}
		if conscious {
			t.Fatalf("sim did not faint (bumping=%v)", bumping)
		}
		return faintedAt
	}

	plain, bumped := sim(false), sim(true)
	if bumped >= plain {
		t.Fatalf("expected bumps to faint sooner: plain %v, bumped %v", plain, bumped)
	}
}

func TestHRThresholdsScaling(t *testing.T) {
	th := hrThresholds(0)
	if th.passOut != 180 || th.death != 200 || th.asymptote != 205 {
		t.Fatalf("level 0 = %+v", th)
	}

	th = hrThresholds(5)
	if th.passOut != 185 || th.death != 202.5 || th.asymptote != 210 {
		t.Fatalf("level 5 = %+v", th)
	}

	th = hrThresholds(10)
	if th.passOut != 190 || th.death != 205 || th.asymptote != 215 {
		t.Fatalf("level 10 = %+v", th)
	}

	// Max level: passOut 200, death 210, ceiling always 25 above passOut.
	th = hrThresholds(hrMaxLevel)
	if th.passOut != 200 || th.death != 210 {
		t.Fatalf("max level = %+v", th)
	}
	if th.asymptote-th.passOut != 25 {
		t.Fatalf("ceiling margin = %.0f, want 25", th.asymptote-th.passOut)
	}

	// Clamped past the cap, and never below level 0.
	capped := hrThresholds(hrMaxLevel * 2)
	if capped != th {
		t.Fatalf("expected clamping past max: got %+v, want %+v", capped, th)
	}
	if neg := hrThresholds(-5); neg != hrThresholds(0) {
		t.Fatalf("expected negative level to clamp to 0: got %+v", neg)
	}
}

func TestRunningAlwaysFaintsAtEveryLevel(t *testing.T) {
	for level := 0; level <= hrMaxLevel; level++ {
		hr := NewHeartRate(baseHR)
		hr.SetThresholds(hrThresholds(level))
		step := 100 * time.Millisecond

		conscious := true
		fainted := false
		for i := 0; i < 3000; i++ { // up to 5 simulated minutes
			hr.Update(step, conscious) // input dropped while blacked out
			conscious = !hr.passedOut()
			if hr.bpm >= hr.Death() {
				t.Fatalf("level %d: movement reached %.1f BPM, crossing death %.1f",
					level, hr.bpm, hr.Death())
			}
			if !conscious {
				fainted = true
				break
			}
		}
		if !fainted {
			t.Fatalf("level %d: held movement never passed out", level)
		}
	}
}

func TestSetThresholdsChangessPassOutAndDeath(t *testing.T) {
	hr := NewHeartRate(baseHR)
	hr.SetThresholds(hrThresholds(hrMaxLevel))

	hr.bpm = 199
	if hr.passedOut() {
		t.Error("should be conscious at 199 BPM at max level")
	}
	hr.bpm = 200
	if !hr.passedOut() {
		t.Error("should pass out at 200 BPM at max level")
	}
	if hr.Death() != 210 {
		t.Fatalf("death = %.1f at max level, want 210", hr.Death())
	}

	// Game wiring: leveling updates the heart rate's thresholds too.
	g := NewGame()
	g.playerLevel = hrMaxLevel
	g.heartRate.SetThresholds(hrThresholds(g.playerLevel))
	if g.heartRate.Death() != 210 {
		t.Fatalf("game death = %.1f, want 210", g.heartRate.Death())
	}
}

func TestLayoutIncludesStatusBar(t *testing.T) {
	g := NewGame()
	w, h := g.Layout(0, 0)
	if w != screenWidth {
		t.Errorf("width = %d, want %d", w, screenWidth)
	}
	if h != screenHeight+statusBarHeight {
		t.Errorf("height = %d, want %d", h, screenHeight+statusBarHeight)
	}
}
