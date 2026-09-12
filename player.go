package main

import (
	"image/color"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	tileSize       = 16
	playerSize     = 14
	moveRepeatWait = 150 * time.Millisecond
)

// targetOutline frames the tile a swing would reach; muted and slightly
// transparent so it reads as part of the floor rather than UI.
var targetOutline = color.RGBA{205, 200, 120, 190}

// Facing directions in screen coordinates (y grows downward).
type Direction int

const (
	DirNorth Direction = iota
	DirEast
	DirSouth
	DirWest
)

var dirVectors = [4][2]int{
	DirNorth: {0, -1},
	DirEast:  {1, 0},
	DirSouth: {0, 1},
	DirWest:  {-1, 0},
}

var facingAngles = [4]float64{
	DirNorth: -math.Pi / 2,
	DirEast:  0,
	DirSouth: math.Pi / 2,
	DirWest:  math.Pi,
}

type Player struct {
	X, Y  float64
	Dir   Direction
	HP    int
	MaxHP int
	Color [4]float64

	Weapon   Weapon
	lastMove time.Time
	bumped   bool
	// consumed holds movement/turn keys whose press event was swallowed by a
	// command frame: they stay blocked for movement until released again, so
	// a held key can't bleed a second action out of a settled command.
	consumed map[ebiten.Key]bool
}

func NewPlayer(x, y float64) *Player {
	return &Player{
		X:      x,
		Y:      y,
		Dir:    DirNorth,
		HP:     100,
		MaxHP:  100,
		Weapon: bareHands(),
		Color:  [4]float64{0.9, 0.9, 0.3, 1},
	}
}

// swallow marks a key as consumed by the command system; it no longer drives
// movement until it is released and pressed again.
func (p *Player) swallow(k ebiten.Key) {
	if p.consumed == nil {
		p.consumed = make(map[ebiten.Key]bool)
	}
	p.consumed[k] = true
}

// held reports whether a movement/turn key is down and not consumed.
func (p *Player) held(k ebiten.Key) bool {
	return ebiten.IsKeyPressed(k) && !p.consumed[k]
}

// moving reports whether a directional movement key is currently held.
func (p *Player) moving() bool {
	return p.held(ebiten.KeyW) ||
		p.held(ebiten.KeyS) ||
		p.held(ebiten.KeyD) ||
		p.held(ebiten.KeyA)
}

// Bumped reports whether the last Update attempt ran into an obstacle.
func (p *Player) Bumped() bool { return p.bumped }

// SwingResult is the outcome of a swing: the HR stress it cost, what (if
// anything) stood in the target tile and whether it was hit, and the monster
// killed by fatal damage.
type SwingResult struct {
	Cost   float64
	Target string // monster type name, "" when the swing found nothing
	Hit    bool
	Killed *Monster
}

// TargetTile is the tile a swing reaches: one tile straight ahead of the
// player's facing, regardless of which hand is raised.
func (p *Player) TargetTile() (int, int) {
	fv := dirVectors[p.Dir]
	return int(p.X)/tileSize + fv[0], int(p.Y)/tileSize + fv[1]
}

// Attack swings the equipped weapon at the tile the player faces with the
// given hand. An empty swing (air or wall) costs half the stress of a hit; a
// miss also costs half the stress of a hit.
func (p *Player) Attack(l *Level, side handSide) SwingResult {
	tx, ty := p.TargetTile()

	m := l.MonsterAt(tx, ty)
	if m == nil {
		// Swinging at air is exertion just like a whiffed swing: half cost.
		return SwingResult{Cost: p.Weapon.HRCost / 2}
	}

	res := SwingResult{Target: m.stats.Name}
	res.Cost = p.Weapon.HRCost
	if rand.Float64() >= p.Weapon.ToHit {
		res.Cost /= 2 // miss: half the stress of a hit
		return res
	}

	m.HP -= p.Weapon.Damage
	res.Hit = true
	if m.HP <= 0 {
		res.Killed = m
	}
	return res
}

// Update processes a frame of player input, returning the discrete commands
// performed (turns, moves, and wall bumps) as styled history lines so the
// game can log them on the info panel.
func (p *Player) Update(level *Level) []logEntry {
	for k := range p.consumed {
		if !ebiten.IsKeyPressed(k) {
			delete(p.consumed, k)
		}
	}

	p.bumped = false
	var actions []logEntry
	fwd := p.held(ebiten.KeyW)
	back := p.held(ebiten.KeyS)
	stepR := p.held(ebiten.KeyD)
	stepL := p.held(ebiten.KeyA)
	turnR := p.held(ebiten.KeyE)
	turnL := p.held(ebiten.KeyQ)

	if !fwd && !back && !stepR && !stepL && !turnR && !turnL {
		return actions
	}

	justPressed := inpututil.IsKeyJustPressed(ebiten.KeyW) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) ||
		inpututil.IsKeyJustPressed(ebiten.KeyA) ||
		inpututil.IsKeyJustPressed(ebiten.KeyD) ||
		inpututil.IsKeyJustPressed(ebiten.KeyE) ||
		inpututil.IsKeyJustPressed(ebiten.KeyQ)

	now := time.Now()
	if !justPressed && now.Sub(p.lastMove) < moveRepeatWait {
		return actions
	}

	if turnR {
		p.Dir = (p.Dir + 1) % 4
		actions = append(actions, logNormal("Turn Right"))
	}
	if turnL {
		p.Dir = (p.Dir + 3) % 4
		actions = append(actions, logNormal("Turn Left"))
	}

	fv := dirVectors[p.Dir]
	if fwd {
		actions = append(actions, p.stepAttempt(level, fv[0], fv[1], "Move Forward"))
	}
	if back {
		actions = append(actions, p.stepAttempt(level, -fv[0], -fv[1], "Move Back"))
	}
	if stepR {
		rv := dirVectors[(p.Dir+1)%4]
		actions = append(actions, p.stepAttempt(level, rv[0], rv[1], "Step Right"))
	}
	if stepL {
		lv := dirVectors[(p.Dir+3)%4]
		actions = append(actions, p.stepAttempt(level, lv[0], lv[1], "Step Left"))
	}

	p.lastMove = now
	return actions
}

// stepAttempt produces the history line for one movement attempt: a
// successful step logs the plain command, while one blocked by a wall logs
// the label with a red !OUCH! (following the styled-outcome pattern of an
// attack) and marks the bump that spikes the heart rate.
func (p *Player) stepAttempt(level *Level, dx, dy int, label string) logEntry {
	if p.tryMove(level, dx, dy) {
		p.bumped = true
		return bumpEntry(label)
	}
	return logNormal(label)
}

// tryMove steps the player one tile toward (dx, dy), returning true if the
// move was blocked by an obstacle (a bump). Diagonal slips between two wall
// corners are blocked too.
func (p *Player) tryMove(level *Level, dx, dy int) bool {
	curTX := int(p.X) / tileSize
	curTY := int(p.Y) / tileSize

	newTX := curTX + dx
	newTY := curTY + dy

	if dx != 0 && dy != 0 &&
		(level.IsBlockedTile(curTX+dx, curTY) || level.IsBlockedTile(curTX, curTY+dy)) {
		return true
	}

	if level.IsBlockedTile(newTX, newTY) {
		return true
	}

	p.X = float64(newTX * tileSize)
	p.Y = float64(newTY * tileSize)
	return false
}

func (p *Player) Draw(screen *ebiten.Image, camera *Camera) {
	sx, sy := camera.WorldToScreen(p.X, p.Y)

	hw := float64(playerSize) / 2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-hw, -hw)
	op.GeoM.Rotate(facingAngles[p.Dir])
	op.GeoM.Translate(sx+hw, sy+hw)
	screen.DrawImage(playerSprite(), op)
}

// DrawAttackTarget frames the tile a swing would reach with a subtle outline,
// drawn under the player and monsters so the reachable square reads at a
// glance as part of the floor.
func (p *Player) DrawAttackTarget(screen *ebiten.Image, camera *Camera) {
	tx, ty := p.TargetTile()
	px, py := camera.WorldToScreen(float64(tx*tileSize), float64(ty*tileSize))

	bar := ebiten.NewImage(1, 1)
	bar.Fill(targetOutline)

	draw := func(w, h int, dx, dy float64) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(w), float64(h))
		op.GeoM.Translate(px+dx, py+dy)
		screen.DrawImage(bar, op)
	}
	draw(tileSize, 1, 0, 0)
	draw(tileSize, 1, 0, float64(tileSize-1))
	draw(1, tileSize, 0, 0)
	draw(1, tileSize, float64(tileSize-1), 0)
}

var (
	playerSpriteImg *ebiten.Image
	playerSpriteOne sync.Once
)

func playerSprite() *ebiten.Image {
	playerSpriteOne.Do(func() {
		src := ebiten.NewImage(1, 1)
		src.Fill(color.RGBA{255, 255, 255, 255})

		img := ebiten.NewImage(playerSize, playerSize)

		// 30/75/75 isosceles triangle pointing right (East), rotated into
		// place per facing direction. Tip at the right edge, narrow apex,
		// wider base so facing reads clearly.
		verts := []ebiten.Vertex{
			{SrcX: 0, SrcY: 0, ColorR: 0.9, ColorG: 0.9, ColorB: 0.3, ColorA: 1, DstX: 13, DstY: 7.5},
			{SrcX: 0, SrcY: 0, ColorR: 0.9, ColorG: 0.9, ColorB: 0.3, ColorA: 1, DstX: 1.4, DstY: 4.4},
			{SrcX: 0, SrcY: 0, ColorR: 0.9, ColorG: 0.9, ColorB: 0.3, ColorA: 1, DstX: 1.4, DstY: 10.6},
		}
		indices := []uint16{0, 1, 2}

		img.DrawTriangles(verts, indices, src, nil)
		playerSpriteImg = img
	})
	return playerSpriteImg
}

func colorFromFloats(rgba [4]float64) (c color.RGBA) {
	return color.RGBA{
		R: uint8(rgba[0] * 255),
		G: uint8(rgba[1] * 255),
		B: uint8(rgba[2] * 255),
		A: uint8(rgba[3] * 255),
	}
}
