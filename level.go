package main

import (
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
)

const tileEdge = tileSize

type Tile int

const (
	TileWall Tile = iota
	TileFloor
	TileStairs
	TileDoor
	TileSecretDoor
)

type room struct{ x, y, w, h int }

type Level struct {
	Width, Height int
	Depth         int
	Tiles         []Tile

	explored []bool
	visible  []bool

	rooms     []room
	firstRoom struct{ x, y, w, h int }

	Monsters []*Monster
}

func NewLevel(width, height int) *Level {
	return NewLevelAt(width, height, 1)
}

func NewLevelAt(width, height, depth int) *Level {
	l := &Level{
		Width:    width,
		Height:   height,
		Depth:    depth,
		Tiles:    make([]Tile, width*height),
		explored: make([]bool, width*height),
		visible:  make([]bool, width*height),
	}
	l.generate()
	return l
}

func (l *Level) idx(x, y int) int {
	return y*l.Width + x
}

func (l *Level) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < l.Width && y < l.Height
}

func (l *Level) generate() {
	// Start with everything as walls.
	for i := range l.Tiles {
		l.Tiles[i] = TileWall
	}

	// Carve a set of random rooms connected by horizontal and vertical corridors.
	const (
		roomMinW = 5
		roomMaxW = 10
		roomMinH = 4
		roomMaxH = 7
		maxRooms = 20
	)

	var rooms []room

	for i := 0; i < maxRooms; i++ {
		maxW := l.Width - 3
		if maxW > roomMaxW {
			maxW = roomMaxW
		}
		maxH := l.Height - 3
		if maxH > roomMaxH {
			maxH = roomMaxH
		}
		if maxW < roomMinW || maxH < roomMinH {
			break
		}

		w := rand.Intn(maxW-roomMinW+1) + roomMinW
		h := rand.Intn(maxH-roomMinH+1) + roomMinH
		x := rand.Intn(l.Width-w-2) + 1
		y := rand.Intn(l.Height-h-2) + 1

		newRoom := room{x, y, w, h}
		overlaps := false
		for _, r := range rooms {
			if newRoom.x < r.x+r.w+2 && newRoom.x+newRoom.w+2 > r.x &&
				newRoom.y < r.y+r.h+2 && newRoom.y+newRoom.h+2 > r.y {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		l.carveRoom(newRoom)

		if len(rooms) > 0 {
			prev := rooms[len(rooms)-1]

			pcx := prev.x + prev.w/2
			pcy := prev.y + prev.h/2
			ccx := newRoom.x + newRoom.w/2
			ccy := newRoom.y + newRoom.h/2

			if rand.Intn(2) == 0 {
				l.carveHCorridor(pcx, ccx, pcy)
				l.carveVCorridor(pcy, ccy, ccx)
			} else {
				l.carveVCorridor(pcy, ccy, pcx)
				l.carveHCorridor(pcx, ccx, ccy)
			}
		}

		rooms = append(rooms, newRoom)
	}

	// Place stairs in the last room.
	if len(rooms) > 0 {
		last := rooms[len(rooms)-1]
		stairsX := last.x + last.w/2
		stairsY := last.y + last.h/2
		l.Tiles[l.idx(stairsX, stairsY)] = TileStairs
	}

	l.rooms = rooms
	l.placeDoors()

	l.firstRoom = rooms[0]
	l.Monsters = l.spawnMonsters(
		l.firstRoom.x+l.firstRoom.w/2,
		l.firstRoom.y+l.firstRoom.h/2,
		l.Depth)
}

// placeDoors converts qualifying tiles (one-tile passages through rock)
// into doors. Tiles on a room's perimeter — its entrances — are a special
// case: each entrance becomes an open corridor 60% of the time, a normal
// door 30%, or a hidden door 10%. Everywhere else (mid-hallway passages)
// keeps the flat per-tile probability.
func (l *Level) placeDoors() {
	const (
		doorProb   = 0.05 // chance per qualifying hallway tile
		secretProb = 0.12 // share of placed hallway doors that are secret
	)
	const (
		entranceOpenShare   = 0.6 // leave an entrance as open corridor
		entranceDoorShare   = 0.3 // normal door
		entranceSecretShare = 0.1 // hidden door
	)

	type point struct{ x, y int }

	// Room entrances are the qualifying tiles sitting on a room's perimeter:
	// the wall rows and columns the corridors carve through to get in.
	entrances := make(map[point]bool, len(l.rooms)*4)
	for _, r := range l.rooms {
		for i := r.x; i < r.x+r.w; i++ {
			if l.doorShapeOK(i, r.y-1) {
				entrances[point{i, r.y - 1}] = true
			}
			if l.doorShapeOK(i, r.y+r.h) {
				entrances[point{i, r.y + r.h}] = true
			}
		}
		for j := r.y; j < r.y+r.h; j++ {
			if l.doorShapeOK(r.x-1, j) {
				entrances[point{r.x - 1, j}] = true
			}
			if l.doorShapeOK(r.x+r.w, j) {
				entrances[point{r.x + r.w, j}] = true
			}
		}
	}

	var candidates []point
	for ty := 1; ty < l.Height-1; ty++ {
		for tx := 1; tx < l.Width-1; tx++ {
			i := l.idx(tx, ty)
			if l.Tiles[i] != TileFloor {
				continue
			}
			if !l.doorShapeOK(tx, ty) {
				continue
			}
			candidates = append(candidates, point{tx, ty})
		}
	}

	// Random order so declustering does not bias which entrance wins.
	rand.Shuffle(len(candidates), func(a, b int) {
		candidates[a], candidates[b] = candidates[b], candidates[a]
	})

	for _, c := range candidates {
		if l.hasDoorNeighbor(c.x, c.y) {
			continue
		}
		i := l.idx(c.x, c.y)

		if entrances[c] {
			switch {
			case rand.Float64() < entranceSecretShare:
				l.Tiles[i] = TileSecretDoor
			case rand.Float64() < entranceDoorShare/(entranceDoorShare+entranceOpenShare):
				l.Tiles[i] = TileDoor
			}
			continue
		}

		if rand.Float64() >= doorProb {
			continue
		}
		if rand.Float64() < secretProb {
			l.Tiles[i] = TileSecretDoor
		} else {
			l.Tiles[i] = TileDoor
		}
	}
}

// doorShapeOK reports whether a floor tile is a legal door position: a
// one-tile passage through rock, with exactly two open cardinal neighbours
// opposite each other and two wall neighbours opposite each other:
//
//	.W.
//	FDF
//	.W.
//
// Corners, junctions, and room interiors (where the open neighbours are
// perpendicular) never qualify.
func (l *Level) doorShapeOK(tx, ty int) bool {
	var opens, walls [2]int // direction offsets, encoded so opposites sum to 0
	oc, wc := 0, 0
	for _, n := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := tx+n[0], ty+n[1]
		if !l.inBounds(nx, ny) {
			return false
		}
		dir := n[0]*4 + n[1]
		if l.Tiles[l.idx(nx, ny)] == TileWall {
			if wc == 2 {
				return false
			}
			walls[wc] = dir
			wc++
		} else {
			if oc == 2 {
				return false
			}
			opens[oc] = dir
			oc++
		}
	}
	return oc == 2 && wc == 2 && opens[0]+opens[1] == 0 && walls[0]+walls[1] == 0
}

func (l *Level) hasDoorNeighbor(tx, ty int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := tx+dx, ty+dy
			if !l.inBounds(nx, ny) {
				continue
			}
			t := l.Tiles[l.idx(nx, ny)]
			if t == TileDoor || t == TileSecretDoor {
				return true
			}
		}
	}
	return false
}

func (l *Level) StartPosition() (float64, float64) {
	r := l.firstRoom
	x := float64(r.x + r.w/2)
	y := float64(r.y + r.h/2)
	return x * tileEdge, y * tileEdge
}

func (l *Level) carveRoom(r struct{ x, y, w, h int }) {
	for yy := r.y; yy < r.y+r.h; yy++ {
		for xx := r.x; xx < r.x+r.w; xx++ {
			l.Tiles[l.idx(xx, yy)] = TileFloor
		}
	}
}

func (l *Level) carveHCorridor(x1, x2, y int) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		if l.inBounds(x, y) {
			l.Tiles[l.idx(x, y)] = TileFloor
		}
	}
}

func (l *Level) carveVCorridor(y1, y2, x int) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		if l.inBounds(x, y) {
			l.Tiles[l.idx(x, y)] = TileFloor
		}
	}
}

func (l *Level) IsBlocked(x, y float64) bool {
	return l.IsBlockedTile(int(x)/tileEdge, int(y)/tileEdge)
}

func (l *Level) IsBlockedTile(tx, ty int) bool {
	if !l.inBounds(tx, ty) {
		return true
	}
	return l.Tiles[l.idx(tx, ty)] == TileWall
}

// MonsterAt returns the monster on a tile, or nil. Monsters never share a
// tile, so at most one can be there.
func (l *Level) MonsterAt(tx, ty int) *Monster {
	for _, m := range l.Monsters {
		if m.TX == tx && m.TY == ty {
			return m
		}
	}
	return nil
}

// RemoveMonster removes a monster from the level.
func (l *Level) RemoveMonster(dead *Monster) {
	live := l.Monsters[:0]
	for _, m := range l.Monsters {
		if m != dead {
			live = append(live, m)
		}
	}
	l.Monsters = live
}

// monsterCanEnter reports whether a monster of a type with the given door
// flag may occupy the tile. Floor and stairs are open to all; door tiles
// only to door openers; walls never.
func (l *Level) monsterCanEnter(tx, ty int, openDoors bool) bool {
	if !l.inBounds(tx, ty) {
		return false
	}
	switch l.Tiles[l.idx(tx, ty)] {
	case TileFloor, TileStairs:
		return true
	case TileDoor, TileSecretDoor:
		return openDoors
	default:
		return false
	}
}

// monsterMayOccupy reports whether a monster m may step onto (tx, ty): the
// tile must be passable and unoccupied by the player or any other monster.
// Stronger monsters may later swap places with weaker ones instead of being
// blocked by this check.
func (l *Level) monsterMayOccupy(tx, ty int, m *Monster, px, py int) bool {
	if !l.monsterCanEnter(tx, ty, m.stats.OpenDoors) {
		return false
	}
	if tx == px && ty == py {
		return false
	}
	for _, o := range l.Monsters {
		if o != m && o.TX == tx && o.TY == ty {
			return false
		}
	}
	return true
}

// spawnPool builds the MonsterStats list to place on a depth: every type whose
// depth window covers it contributes spawn_count entries, in data-file order.
// Spawning reads this pool so monster rosters are entirely data-driven.
func spawnPool(depth int) []MonsterStats {
	ensureMonsters()
	monsterMu.RLock()
	reg, order := monsterReg, monsterOrder
	monsterMu.RUnlock()

	var pool []MonsterStats
	for _, id := range order {
		st := reg[id]
		if depth < st.MinLevel {
			continue
		}
		if st.MaxLevel != 0 && depth > st.MaxLevel {
			continue
		}
		for i := 0; i < st.SpawnCount; i++ {
			pool = append(pool, st)
		}
	}
	return pool
}

// spawnMonsters places one monster per spawnPool slot on random floor tiles at
// least 10 Manhattan tiles from the player's start, so the first room is calm.
func (l *Level) spawnMonsters(startX, startY, depth int) []*Monster {
	var tiles [][2]int
	for ty := 0; ty < l.Height; ty++ {
		for tx := 0; tx < l.Width; tx++ {
			if l.Tiles[l.idx(tx, ty)] != TileFloor {
				continue
			}
			if absInt(tx-startX)+absInt(ty-startY) < 10 {
				continue
			}
			tiles = append(tiles, [2]int{tx, ty})
		}
	}

	pool := spawnPool(depth)

	rand.Shuffle(len(tiles), func(a, b int) {
		tiles[a], tiles[b] = tiles[b], tiles[a]
	})

	monsters := make([]*Monster, 0, len(pool))
	for i := 0; i < len(pool) && i < len(tiles); i++ {
		monsters = append(monsters, NewMonster(tiles[i][0], tiles[i][1], pool[i]))
	}
	return monsters
}

func (l *Level) Update() {}

// DrawMonsters renders each monster, but only where it is currently lit by
// the player's vision — never from memory, and never if the monster's type is
// invisible.
func (l *Level) DrawMonsters(screen *ebiten.Image, camera *Camera) {
	for _, m := range l.Monsters {
		if !shown(m.stats, l.visible[l.idx(m.TX, m.TY)]) {
			continue
		}
		m.Draw(screen, camera)
	}
}

var (
	colorWall    = color.RGBA{80, 80, 90, 255}
	colorFloor   = color.RGBA{40, 40, 50, 255}
	colorStair   = color.RGBA{240, 220, 100, 255}
	colorDoor    = color.RGBA{139, 90, 43, 255}
	colorWallDim = color.RGBA{52, 52, 59, 255}
	colorFloorDm = color.RGBA{22, 22, 28, 255}
	colorStairDm = color.RGBA{110, 100, 48, 255}
	colorDoorDm  = color.RGBA{88, 58, 30, 255}
)

func (l *Level) Draw(screen *ebiten.Image, camera *Camera) {
	tilesWide := screenWidth/tileEdge + 2
	tilesHigh := screenHeight/tileEdge + 2

	startX := camera.ViewX() / tileEdge
	startY := camera.ViewY() / tileEdge

	img := ebiten.NewImage(tileEdge, tileEdge)

	for ty := startY; ty < startY+tilesHigh; ty++ {
		for tx := startX; tx < startX+tilesWide; tx++ {
			if !l.inBounds(tx, ty) {
				continue
			}

			i := l.idx(tx, ty)
			if !l.explored[i] {
				continue
			}

			var c color.Color
			switch {
			case l.visible[i]:
				switch l.Tiles[i] {
				case TileWall, TileSecretDoor:
					c = colorWall
				case TileFloor:
					c = colorFloor
				case TileStairs:
					c = colorStair
				case TileDoor:
					c = colorDoor
				}
			default:
				switch l.Tiles[i] {
				case TileWall, TileSecretDoor:
					c = colorWallDim
				case TileFloor:
					c = colorFloorDm
				case TileStairs:
					c = colorStairDm
				case TileDoor:
					c = colorDoorDm
				}
			}

			wx := float64(tx) * tileEdge
			wy := float64(ty) * tileEdge
			sx, sy := camera.WorldToScreen(wx, wy)

			img.Fill(c)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(sx, sy)
			screen.DrawImage(img, op)
		}
	}
}
