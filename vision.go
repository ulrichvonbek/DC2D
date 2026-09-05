package main

import "math"

const (
	visionRadius    = 6
	halfVisionAngle = math.Pi / 3 // 60° each side of facing → 120° total
	nearWallRadius  = 1           // walls within this Chebyshev distance are always revealed
)

var wallNeighborOffsets = [][2]int{
	{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
}

// opaqueToVision reports whether a tile blocks line of sight. Walls, doors,
// and secret doors all block vision; unlike walls, doors are still passable.
func opaqueToVision(t Tile) bool {
	return t == TileWall || t == TileDoor || t == TileSecretDoor
}

// visionBlocked reports whether the tile at (tx, ty) blocks line of sight,
// treating out-of-bounds as blocking.
func (l *Level) visionBlocked(tx, ty int) bool {
	if !l.inBounds(tx, ty) {
		return true
	}
	return opaqueToVision(l.Tiles[l.idx(tx, ty)])
}

// tileInCone reports whether the offset (dx, dy) from the player lies within
// the forward vision cone centered on the facing vector (fx, fy).
func tileInCone(fx, fy, dx, dy int) bool {
	if dx == 0 && dy == 0 {
		return true
	}
	n := math.Hypot(float64(dx), float64(dy))
	dot := (float64(fx*dx) + float64(fy*dy)) / n
	return dot >= math.Cos(halfVisionAngle)
}

// sightClear reports whether the tile (x1, y1) is reachable from (x0, y0)
// without crossing a wall tile. A Bresenham line walk that would cut a wall
// corner is treated as blocked.
func (l *Level) sightClear(x0, y0, x1, y1 int) bool {
	dx := absInt(x1 - x0)
	dy := absInt(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}

	err := dx - dy
	x, y := x0, y0
	for {
		if x == x1 && y == y1 {
			return true
		}
		if (x != x0 || y != y0) && opaqueToVision(l.Tiles[l.idx(x, y)]) {
			return false
		}

		e2 := 2 * err
		nx, ny := x, y
		if e2 > -dy {
			err -= dy
			nx += sx
		}
		if e2 < dx {
			err += dx
			ny += sy
		}

		// A diagonal step must not slip through a wall corner: block it if
		// either orthogonal neighbour is opaque (or out of bounds).
		if nx != x && ny != y && (l.visionBlocked(x+sx, y) || l.visionBlocked(x, y+sy)) {
			return false
		}

		x, y = nx, ny
	}
}

// UpdateVision recomputes the set of tiles the player can currently see and
// accumulates them into the explored map. dir is one of DirNorth..DirWest.
func (l *Level) UpdateVision(px, py, dir int) {
	for i := range l.visible {
		l.visible[i] = false
	}

	fx, fy := dirVectors[dir][0], dirVectors[dir][1]
	for dy := -visionRadius; dy <= visionRadius; dy++ {
		for dx := -visionRadius; dx <= visionRadius; dx++ {
			tx, ty := px+dx, py+dy
			if !l.inBounds(tx, ty) {
				continue
			}
			if math.Hypot(float64(dx), float64(dy)) > float64(visionRadius) {
				continue
			}
			if !tileInCone(fx, fy, dx, dy) {
				continue
			}
			if !l.sightClear(px, py, tx, ty) {
				continue
			}

			i := l.idx(tx, ty)
			l.visible[i] = true
			l.explored[i] = true
		}
	}

	// Reveal opaque surfaces next to visible walkable tiles only: a lit wall
	// must not propagate into the rock behind it (that would x-ray through
	// walls). A visible door also reveals its frame, so the far side stays
	// hidden until you step through. Revealed tiles are remembered as explored,
	// not visible.
	for dy := -visionRadius; dy <= visionRadius; dy++ {
		for dx := -visionRadius; dx <= visionRadius; dx++ {
			tx, ty := px+dx, py+dy
			if !l.inBounds(tx, ty) {
				continue
			}
			i := l.idx(tx, ty)
			if !l.visible[i] || l.Tiles[i] == TileWall {
				continue
			}
			l.revealWallNeighbors(tx, ty)
		}
	}

	// Always reveal the walls you can brush against: a small all-around
	// radius regardless of facing, so doorways and tunnel walls beside or
	// behind you are never a black void. Doors count as walls here.
	for dy := -nearWallRadius; dy <= nearWallRadius; dy++ {
		for dx := -nearWallRadius; dx <= nearWallRadius; dx++ {
			if !l.inBounds(px+dx, py+dy) {
				continue
			}
			i := l.idx(px+dx, py+dy)
			if opaqueToVision(l.Tiles[i]) {
				l.explored[i] = true
			}
		}
	}
}

func (l *Level) revealWallNeighbors(tx, ty int) {
	for _, n := range wallNeighborOffsets {
		nx, ny := tx+n[0], ty+n[1]
		if !l.inBounds(nx, ny) {
			continue
		}
		ni := l.idx(nx, ny)
		if opaqueToVision(l.Tiles[ni]) {
			l.explored[ni] = true
		}
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
