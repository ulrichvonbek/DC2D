package main

import (
	"image/color"
	"math"
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
	X, Y   float64
	Dir    Direction
	HP     int
	MaxHP  int
	Attack int
	Color  [4]float64

	lastMove time.Time
	bumped   bool
}

func NewPlayer(x, y float64) *Player {
	return &Player{
		X:      x,
		Y:      y,
		Dir:    DirNorth,
		HP:     100,
		MaxHP:  100,
		Attack: 5,
		Color:  [4]float64{0.9, 0.9, 0.3, 1},
	}
}

// moving reports whether a directional movement key is currently held.
func (p *Player) moving() bool {
	return ebiten.IsKeyPressed(ebiten.KeyW) ||
		ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyD) ||
		ebiten.IsKeyPressed(ebiten.KeyA)
}

// Bumped reports whether the last Update attempt ran into an obstacle.
func (p *Player) Bumped() bool { return p.bumped }

func (p *Player) Update(level *Level) {
	p.bumped = false
	fwd := ebiten.IsKeyPressed(ebiten.KeyW)
	back := ebiten.IsKeyPressed(ebiten.KeyS)
	stepR := ebiten.IsKeyPressed(ebiten.KeyD)
	stepL := ebiten.IsKeyPressed(ebiten.KeyA)
	turnR := ebiten.IsKeyPressed(ebiten.KeyE)
	turnL := ebiten.IsKeyPressed(ebiten.KeyQ)

	if !fwd && !back && !stepR && !stepL && !turnR && !turnL {
		return
	}

	justPressed := inpututil.IsKeyJustPressed(ebiten.KeyW) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) ||
		inpututil.IsKeyJustPressed(ebiten.KeyA) ||
		inpututil.IsKeyJustPressed(ebiten.KeyD) ||
		inpututil.IsKeyJustPressed(ebiten.KeyE) ||
		inpututil.IsKeyJustPressed(ebiten.KeyQ)

	now := time.Now()
	if !justPressed && now.Sub(p.lastMove) < moveRepeatWait {
		return
	}

	if turnR {
		p.Dir = (p.Dir + 1) % 4
	}
	if turnL {
		p.Dir = (p.Dir + 3) % 4
	}

	fv := dirVectors[p.Dir]
	if fwd {
		if p.tryMove(level, fv[0], fv[1]) {
			p.bumped = true
		}
	}
	if back {
		if p.tryMove(level, -fv[0], -fv[1]) {
			p.bumped = true
		}
	}
	if stepR {
		rv := dirVectors[(p.Dir+1)%4]
		if p.tryMove(level, rv[0], rv[1]) {
			p.bumped = true
		}
	}
	if stepL {
		lv := dirVectors[(p.Dir+3)%4]
		if p.tryMove(level, lv[0], lv[1]) {
			p.bumped = true
		}
	}

	p.lastMove = now
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
