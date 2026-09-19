package main

import (
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Game struct {
	player        *Player
	level         *Level
	camera        *Camera
	heartRate     *HeartRate
	hrDisplay     HRDisplay
	lastUpdate    time.Time
	blackoutAlpha float64
	dead          bool
	diedAt        time.Time // when the death threshold was crossed
	flatlined     bool      // HR blew past hrFlatline: the heart stopped
	playerLevel   int

	cmdVerb      verb
	cmdPending   bool      // the top log line is a not-yet-committed command preview
	helpOpen     bool      // the ? command reference is showing (pauses the game)
	fainted      bool      // the player was unconscious on the previous frame
	maskProgress int       // mask layers currently blocked (0..4)
	maskNext     time.Time // when the next block/clear step lands

	inputLog      []logEntry // recent player commands, for the info panel
	floorItems    []string   // items on the current floor (empty for now)
	backpackItems []string   // items in the backpack (empty for now)
}

func NewGame() *Game {
	g := &Game{}
	g.reset()
	return g
}

// reset returns the game to a fresh run: new depth-1 dungeon, player, heart
// rate, camera, log, and window title. It clears any previous death so a
// restart genuinely starts over rather than un-fading the old world.
func (g *Game) reset() {
	level := NewLevel(screenWorldW, screenWorldH)
	startX, startY := level.StartPosition()

	g.player = NewPlayer(startX, startY)
	g.level = level
	g.camera = NewCamera(screenWidth, screenHeight)
	g.camera.SetWorld(level.Width*tileSize, level.Height*tileSize)
	g.heartRate = NewHeartRate(baseHR)
	g.hrDisplay = HRDisplay{}
	g.lastUpdate = time.Now()
	g.playerLevel = 0 // XP will raise this and scale the HR thresholds

	g.blackoutAlpha = 0
	g.dead = false
	g.diedAt = time.Time{}
	g.flatlined = false
	g.helpOpen = false
	g.fainted = false
	g.maskProgress = 0
	g.maskNext = time.Time{}
	g.cmdVerb = verbNone
	g.cmdPending = false
	g.inputLog = nil
	g.floorItems = nil
	g.backpackItems = nil

	ebiten.SetWindowTitle(fmt.Sprintf("Dungeon Crawl — Depth %d", level.Depth))
}

// restart is what the R key on the death screen invokes: a brand-new game.
func (g *Game) restart() {
	g.reset()
}

// Cached just-pressed keys freed by inpututil — only the command keys are
// recognized; anything else resolves a pending verb as Invalid.
var justPressedBuf = make([]ebiten.Key, 0, 8)

func (g *Game) Update() error {
	now := time.Now()
	dt := now.Sub(g.lastUpdate)
	g.lastUpdate = now
	if dt > 250*time.Millisecond {
		dt = 250 * time.Millisecond
	}

	if g.dead {
		g.blackoutAlpha = fadeToward(g.blackoutAlpha, 1, dt)
		// An overload death keeps the square hammering at the pass-out
		// cadence, exactly like a faint; only a flatline stops the pulse.
		if !g.flatlined {
			g.heartRate.HammerPassOut(dt)
		}
		// Once the fade has landed and the fateful beat has passed, the end
		// screen's two keys go live: R starts a new run, ESC quits cleanly.
		if deathReady(g.blackoutAlpha, now.Sub(g.diedAt)) {
			switch {
			case inpututil.IsKeyJustPressed(ebiten.KeyR):
				g.restart()
			case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
				return errQuit
			}
		}
		return nil
	}

	// Help is a paused modal: while it is open nothing else ticks. Closing is
	// Space, Esc, or repeating ?. Any other key is swallowed.
	if g.helpOpen {
		if helpKeyJustPressed() ||
			inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
			inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.helpOpen = false
		}
		return nil
	}
	if helpKeyJustPressed() {
		// The help screen supersedes any command in progress; a pending
		// command is abandoned rather than left to finish behind the menu.
		if g.cmdPending {
			g.cancelCommand()
		}
		g.cmdVerb = verbNone
		g.helpOpen = true
		return nil
	}

	conscious := !g.heartRate.passedOut()
	// A fainting spell is a distinct event: it logs in the history, abandons
	// any command in progress, and wakes into a fresh line when HR recovers.
	wasFainted := g.fainted
	passed := !conscious
	g.faintUpdate(passed)
	// The quadrant mask blinks over the dungeon as the player drops and
	// retracts in reverse as they come to, each panel snapping at its own
	// deadline rather than fading.
	g.maskUpdate(passed, wasFainted, now)
	// No player input while blacked out.
	claimed := false
	if conscious {
		just := inpututil.AppendJustPressedKeys(justPressedBuf[:0])
		other := false
		for _, k := range just {
			switch k {
			case ebiten.KeyJ, ebiten.KeyLeftBracket, ebiten.KeyRightBracket,
				ebiten.KeySpace, ebiten.KeyEscape,
				ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
				ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight,
				ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
				ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight:
				continue
			default:
				other = true
			}
		}
		// A frame claimed by the command system executes only that command:
		// movement and turns are dropped so input is atomic (valid as a whole
		// or invalid, never partially applied).
		claimed = g.handleCmdInput(cmdInput{
			verbKey: inpututil.IsKeyJustPressed(ebiten.KeyJ),
			handL:   inpututil.IsKeyJustPressed(ebiten.KeyLeftBracket),
			handR:   inpututil.IsKeyJustPressed(ebiten.KeyRightBracket),
			cancel: inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
				inpututil.IsKeyJustPressed(ebiten.KeySpace),
			other: other,
		})
		// A claimed frame consumes the key presses that landed on it: a held
		// key must not resurrect a second action (e.g. j then e must not turn)
		// once its press was swallowed by the command.
		if claimed {
			for _, k := range just {
				switch k {
				case ebiten.KeyW, ebiten.KeyA, ebiten.KeyS, ebiten.KeyD,
					ebiten.KeyQ, ebiten.KeyE:
					g.player.swallow(k)
				}
			}
		} else {
			for _, cmd := range g.player.Update(g.level) {
				g.pushLog(cmd)
			}
			if g.player.Bumped() {
				g.heartRate.Bump(hrBumpSpike)
			}
		}
		if g.playerOnStairs() {
			g.descend()
		}
		g.level.UpdateVision(int(g.player.X)/tileSize, int(g.player.Y)/tileSize, int(g.player.Dir))
		g.camera.Follow(g.player.X, g.player.Y)
	}

	// Movement raises HR, and a held key climbs toward the threshold
	// asymptote; resting recovers it. A blacked-out player is not moving, and
	// a frame claimed by a command is atomic — it does not count as motion.
	g.heartRate.Update(dt, conscious && !claimed && g.player.moving())

	// Monsters act regardless of the player's consciousness: a hit while
	// blacked out can push HR past the death threshold.
	ptx, pty := int(g.player.X)/tileSize, int(g.player.Y)/tileSize
	for _, m := range g.level.Monsters {
		if spike := m.Update(g.level, ptx, pty, dt); spike > 0 {
			g.heartRate.Bump(spike)
		}
	}

	if g.heartRate.BPM() >= g.heartRate.Death() {
		g.dead = true
		g.diedAt = now
		// Two deaths: an overload freeze in place, or a flatline where the
		// heart stops outright. Either way HR stops updating from this frame
		// on, since the dead branch below never reaches its Update. The
		// overload band (death..hrFlatline) keeps the bar masked as ??? like
		// a faint: only a devastating hit past the cap reveals the 0.
		g.flatlined = g.heartRate.BPM() > hrFlatline
		g.heartRate.Die(g.flatlined)
	}

	target := 0.0
	if g.dead {
		// Death keeps the smooth black fade; a passing-out spell uses the
		// stepping quadrant mask instead, so the two are visually distinct.
		target = 1.0
	}
	g.blackoutAlpha = fadeToward(g.blackoutAlpha, target, dt)

	g.level.Update()
	return nil
}

// faintLog turns a faint-state edge into its history-line event: passing out
// logs a red !PASSING OUT! (the same bright-to-dim red as a whiffed swing),
// waking logs a neutral COMING TO. A steady state logs nothing.
func faintLog(wasFainted, fainted bool) *logEntry {
	switch {
	case fainted && !wasFainted:
		e := logStyled("!!PASSING OUT!!", cmdMiss)
		return &e
	case wasFainted && !fainted:
		e := logNormal("COMING TO")
		return &e
	}
	return nil
}

// faintUpdate tracks the pass-out state across frames, logging the edge
// events. Passing out also abandons any command in progress: the player
// cannot finish typing an attack while unconscious.
func (g *Game) faintUpdate(fainted bool) {
	if entry := faintLog(g.fainted, fainted); entry != nil {
		g.fainted = fainted
		if fainted {
			if g.cmdPending {
				g.cancelCommand()
			}
			g.cmdVerb = verbNone
		}
		g.pushLog(*entry)
	}
}

// maskStep paces the blackout mask's four layers.
const maskStep = 250 * time.Millisecond

// maskRank maps a dungeon-tile position to the layer (1..4) that covers it
// as the player passes out. It directly encodes the tiled dither masks:
// every row of each mask only alternates by column parity, so the tiling
// becomes a per-tile rank. Even/even tiles fall in layer 1, odd/odd in 2,
// even-row/odd-col in 3 (a full block every other row now), and the
// odd-row/even-col stragglers — the only squares still transparent before
// the final mask — in 4.
func maskRank(tx, ty int) int {
	evenRow, evenCol := ty%2 == 0, tx%2 == 0
	switch {
	case evenRow && evenCol:
		return 1
	case !evenRow && !evenCol:
		return 2
	case evenRow:
		return 3
	}
	return 4
}

// maskUpdate drives the layered blackout mask. While passing out it steps
// through maskRank 1..4 at maskStep cadence, each layer adding its dither
// tiles until every dungeon square is dark; waking removes the layers in
// the mirror order at the same cadence.
func (g *Game) maskUpdate(passed, wasFainted bool, now time.Time) {
	switch {
	case passed && !wasFainted:
		g.maskProgress = 0
		g.maskNext = now.Add(maskStep)
	case !passed && wasFainted:
		g.maskNext = now.Add(maskStep)
	}
	if passed && g.maskProgress < 4 && !now.Before(g.maskNext) {
		g.maskProgress++
		g.maskNext = g.maskNext.Add(maskStep)
	} else if !passed && g.maskProgress > 0 && !now.Before(g.maskNext) {
		g.maskProgress--
		g.maskNext = g.maskNext.Add(maskStep)
	}
}

// fadeToward eases alpha toward target, snapping to zero when done.
func fadeToward(alpha, target float64, dt time.Duration) float64 {
	alpha += (target - alpha) * (1 - math.Exp(-blackoutFadeRate*dt.Seconds()))
	if alpha < 0.005 && target == 0 {
		return 0
	}
	return alpha
}

func (g *Game) playerOnStairs() bool {
	tx := int(g.player.X) / tileSize
	ty := int(g.player.Y) / tileSize
	return g.level.inBounds(tx, ty) && g.level.Tiles[g.level.idx(tx, ty)] == TileStairs
}

func (g *Game) descend() {
	g.level = NewLevelAt(screenWorldW, screenWorldH, g.level.Depth+1)
	x, y := g.level.StartPosition()
	g.player.X = x
	g.player.Y = y
	ebiten.SetWindowTitle(fmt.Sprintf("Dungeon Crawl — Depth %d", g.level.Depth))
}

// pushLog prepends a player command to the info-panel input log, keeping only
// the newest infoContentRows entries. While a command is pending its preview
// must stay on top, so new entries slot in beneath it without committing it.
func (g *Game) pushLog(entry logEntry) {
	var next []logEntry
	if g.cmdPending && len(g.inputLog) > 0 {
		rest := g.inputLog[1:]
		next = append([]logEntry{g.inputLog[0], entry}, rest...)
	} else {
		next = append([]logEntry{entry}, g.inputLog...)
	}
	if len(next) > infoContentRows {
		next = next[:infoContentRows]
	}
	g.inputLog = next
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.level.Draw(screen, g.camera)
	g.player.DrawAttackTarget(screen, g.camera)
	g.player.Draw(screen, g.camera)
	g.level.DrawMonsters(screen, g.camera)
	DrawStatusBar(screen, g.heartRate, &g.hrDisplay, time.Now())
	DrawInfoPanel(screen, g.inputLog, g.floorItems, g.backpackItems)
	if g.dead {
		if g.blackoutAlpha > 0 {
			op := &ebiten.DrawImageOptions{}
			op.ColorScale.ScaleAlpha(float32(g.blackoutAlpha))
			screen.DrawImage(blackOverlay(), op)
		}
		if deathReady(g.blackoutAlpha, time.Now().Sub(g.diedAt)) {
			DrawDeath(screen, g.flatlined)
		}
	} else if g.maskProgress > 0 {
		// A passing-out spell erodes the dungeon chunk by chunk; the info
		// panel and status bar below the dungeon stay visible throughout.
		op := &ebiten.DrawImageOptions{}
		screen.DrawImage(maskView(g.maskProgress), op)
	}
	if g.helpOpen {
		DrawHelp(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight + statusBarHeight + infoPanelHeight
}
