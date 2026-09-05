package main

import (
	"image/color"
	"math/rand"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	spiderSpawnCount = 5   // spiders placed per level for now
	seekDuration     = 4.0 // s, how long a monster pursues a lost scent
	monsterSize      = 12  // px sprite size
)

// MonsterStats holds the per-type knobs. Pace is the single rhythm that
// governs the monster: on every beat it attacks if the player is adjacent,
// otherwise it moves a tile. Spike is the HR bump on a landed hit; HitChance
// is the chance to land it; OpenDoors says whether the type can pass through
// door tiles.
type MonsterStats struct {
	Pace      time.Duration
	Spike     float64
	HitChance float64
	OpenDoors bool
}

func spiderStats() MonsterStats {
	return MonsterStats{
		Pace:      1200 * time.Millisecond,
		Spike:     5,
		HitChance: 0.5,
		OpenDoors: false,
	}
}

type monsterState int

const (
	monsterWander monsterState = iota
	monsterChase
	monsterSeek
)

// Monster is an oldschool dungeon dweller: it wanders randomly unless it has
// line of sight on the player (whose light makes them visible at any
// distance), in which case it chases. Once it loses sight it moves to the
// spot the player was last seen, then gives up and wanders again.
type Monster struct {
	TX, TY int
	stats  MonsterStats
	state  monsterState

	lastSeenX, lastSeenY int
	lastSeenValid        bool
	seekT                float64 // s remaining in the seek state

	paceT float64 // s until the next beat (attack or step)
}

func NewMonster(tx, ty int, stats MonsterStats) *Monster {
	return &Monster{
		TX:    tx,
		TY:    ty,
		stats: stats,
		paceT: rand.Float64() * stats.Pace.Seconds(),
	}
}

// Update advances the monster by dt. px/py are the player's tile
// coordinates. The returned value is the HR spike dealt to the player this
// frame (0 if the monster did not land a hit).
func (m *Monster) Update(l *Level, px, py int, dt time.Duration) float64 {
	s := dt.Seconds()

	// Reverse LOS: the player's light makes them visible in a straight line
	// at any distance, blocked only by opaque tiles (walls and doors).
	if l.sightClear(m.TX, m.TY, px, py) {
		m.lastSeenX, m.lastSeenY = px, py
		m.lastSeenValid = true
		m.state = monsterChase
	} else if m.state == monsterChase {
		m.state = monsterSeek
		m.seekT = seekDuration
	}

	if m.state == monsterSeek {
		m.seekT -= s
	}

	// One beat: attack when adjacent, otherwise move.
	spike := 0.0
	m.paceT -= s
	if m.paceT <= 0 {
		m.paceT += m.stats.Pace.Seconds()
		if m.adjacent(px, py) {
			if rand.Float64() < m.stats.HitChance {
				spike = m.stats.Spike
			}
		} else {
			m.move(l, px, py)
		}
	}

	return spike
}

func (m *Monster) move(l *Level, px, py int) {
	switch m.state {
	case monsterChase:
		m.stepToward(l, px, py)
	case monsterSeek:
		m.stepToward(l, m.lastSeenX, m.lastSeenY)
		if m.TX == m.lastSeenX && m.TY == m.lastSeenY {
			m.state = monsterWander
		} else if m.seekT <= 0 {
			m.state = monsterWander
		}
	default:
		m.wander(l)
	}
}

// stepToward makes a single greedy, 4-directional step toward a tile. If the
// preferred axis is blocked it tries the other; no pathfinding.
func (m *Monster) stepToward(l *Level, tx, ty int) {
	dx, dy := 0, 0
	if tx > m.TX {
		dx = 1
	} else if tx < m.TX {
		dx = -1
	}
	if ty > m.TY {
		dy = 1
	} else if ty < m.TY {
		dy = -1
	}

	if absInt(dx) >= absInt(dy) {
		if m.tryStep(l, dx, 0) {
			return
		}
		m.tryStep(l, 0, dy)
		return
	}
	if m.tryStep(l, 0, dy) {
		return
	}
	m.tryStep(l, dx, 0)
}

// wander takes one random step, retrying up to each direction once.
func (m *Monster) wander(l *Level) {
	for _, i := range rand.Perm(4) {
		d := dirVectors[i]
		if m.tryStep(l, d[0], d[1]) {
			return
		}
	}
}

func (m *Monster) tryStep(l *Level, dx, dy int) bool {
	nx, ny := m.TX+dx, m.TY+dy
	if !l.monsterCanEnter(nx, ny, m.stats.OpenDoors) {
		return false
	}
	m.TX, m.TY = nx, ny
	return true
}

func (m *Monster) adjacent(px, py int) bool {
	dx := absInt(m.TX - px)
	dy := absInt(m.TY - py)
	return dx <= 1 && dy <= 1 && dx+dy > 0
}

func (m *Monster) Draw(screen *ebiten.Image, camera *Camera) {
	hw := float64(monsterSize) / 2
	wx := float64(m.TX)*tileSize + float64(tileSize)/2
	wy := float64(m.TY)*tileSize + float64(tileSize)/2
	sx, sy := camera.WorldToScreen(wx, wy)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-hw, -hw)
	op.GeoM.Translate(sx+hw, sy+hw)
	screen.DrawImage(spiderSprite(), op)
}

var (
	spiderSpriteImg *ebiten.Image
	spiderSpriteOne sync.Once
)

func spiderSprite() *ebiten.Image {
	spiderSpriteOne.Do(func() {
		img := ebiten.NewImage(monsterSize, monsterSize)
		c := color.RGBA{235, 120, 110, 255}

		fill := func(x, y, w, h int) {
			part := ebiten.NewImage(w, h)
			part.Fill(c)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			img.DrawImage(part, op)
		}

		fill(4, 4, 4, 4) // body
		// eight short legs at the corners
		fill(0, 0, 2, 3)
		fill(10, 0, 2, 3)
		fill(0, 9, 2, 3)
		fill(10, 9, 2, 3)
		fill(2, 0, 2, 4)
		fill(8, 0, 2, 4)
		fill(2, 8, 2, 4)
		fill(8, 8, 2, 4)

		spiderSpriteImg = img
	})
	return spiderSpriteImg
}
