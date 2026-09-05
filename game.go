package main

import (
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
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
	playerLevel   int
}

func NewGame() *Game {
	level := NewLevel(screenWorldW, screenWorldH)
	startX, startY := level.StartPosition()
	player := NewPlayer(startX, startY)

	camera := NewCamera(screenWidth, screenHeight)
	camera.SetWorld(level.Width*tileSize, level.Height*tileSize)
	ebiten.SetWindowTitle(fmt.Sprintf("Dungeon Crawl — Depth %d", level.Depth))

	return &Game{
		player:      player,
		level:       level,
		camera:      camera,
		heartRate:   NewHeartRate(baseHR),
		lastUpdate:  time.Now(),
		playerLevel: 0, // XP will raise this and scale the HR thresholds
	}
}

func (g *Game) Update() error {
	now := time.Now()
	dt := now.Sub(g.lastUpdate)
	g.lastUpdate = now
	if dt > 250*time.Millisecond {
		dt = 250 * time.Millisecond
	}

	if g.dead {
		g.blackoutAlpha = fadeToward(g.blackoutAlpha, 1, dt)
		return nil
	}

	conscious := !g.heartRate.passedOut()
	// No player input while blacked out.
	if conscious {
		g.player.Update(g.level)
		if g.player.Bumped() {
			g.heartRate.Bump(hrBumpSpike)
		}
		if g.playerOnStairs() {
			g.descend()
		}
		g.level.UpdateVision(int(g.player.X)/tileSize, int(g.player.Y)/tileSize, int(g.player.Dir))
		g.camera.Follow(g.player.X, g.player.Y)
	}

	// Movement raises HR, and a held key climbs toward the threshold
	// asymptote; resting recovers it. A blacked-out player is not moving.
	g.heartRate.Update(dt, conscious && g.player.moving())

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
	}

	target := 0.0
	if g.dead || g.heartRate.passedOut() {
		target = 1.0
	}
	g.blackoutAlpha = fadeToward(g.blackoutAlpha, target, dt)

	g.level.Update()
	return nil
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

func (g *Game) Draw(screen *ebiten.Image) {
	g.level.Draw(screen, g.camera)
	g.player.Draw(screen, g.camera)
	g.level.DrawMonsters(screen, g.camera)
	DrawStatusBar(screen, g.heartRate, &g.hrDisplay, time.Now())
	if g.blackoutAlpha > 0 {
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.ScaleAlpha(float32(g.blackoutAlpha))
		screen.DrawImage(blackOverlay(), op)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight + statusBarHeight
}
