package main

import (
	"testing"

	"pgregory.net/rapid"
)

var moveDirs = rapid.SampledFrom([][2]int{
	{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
})

func walkableTile(l *Level, tx, ty int) bool {
	if !l.inBounds(tx, ty) {
		return false
	}
	return l.Tiles[l.idx(tx, ty)] != TileWall
}

func findStairs(l *Level) (int, int, bool) {
	for i, t := range l.Tiles {
		if t == TileStairs {
			return i % l.Width, i / l.Width, true
		}
	}
	return 0, 0, false
}

func floorReachable(l *Level, fromX, fromY, toX, toY int) bool {
	if !walkableTile(l, fromX, fromY) || !walkableTile(l, toX, toY) {
		return false
	}

	visited := make([]bool, len(l.Tiles))
	visited[l.idx(fromX, fromY)] = true
	queue := [][2]int{{fromX, fromY}}

	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur[0] == toX && cur[1] == toY {
			return true
		}

		for _, d := range dirs {
			nx, ny := cur[0]+d[0], cur[1]+d[1]
			if !l.inBounds(nx, ny) || visited[l.idx(nx, ny)] || !walkableTile(l, nx, ny) {
				continue
			}
			visited[l.idx(nx, ny)] = true
			queue = append(queue, [2]int{nx, ny})
		}
	}
	return false
}

// The stairs must always be reachable from the spawn point through walkable tiles.
func TestPropertyStairsReachableFromSpawn(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(12, 80).Draw(t, "width")
		h := rapid.IntRange(12, 60).Draw(t, "height")

		level := NewLevel(w, h)

		sx, sy := level.StartPosition()
		stx, sty, ok := findStairs(level)
		if !ok {
			t.Fatalf("no stairs tile on level")
		}

		startTile := [2]int{int(sx) / tileSize, int(sy) / tileSize}
		if !floorReachable(level, startTile[0], startTile[1], stx, sty) {
			t.Fatalf("stairs at %d,%d unreachable from spawn %v (map %dx%d)",
				stx, sty, startTile, w, h)
		}
	})
}

// After any sequence of legal moves, the player must be standing on a walkable tile.
func TestPropertyPlayerNeverEndsInWall(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(12, 80).Draw(t, "width")
		h := rapid.IntRange(12, 60).Draw(t, "height")

		level := NewLevel(w, h)
		sx, sy := level.StartPosition()
		p := NewPlayer(sx, sy)

		moves := rapid.SliceOfN(moveDirs, 0, 300).Draw(t, "moves")
		for _, m := range moves {
			p.tryMove(level, m[0], m[1])

			tx := int(p.X) / tileSize
			ty := int(p.Y) / tileSize
			if !level.inBounds(tx, ty) || level.Tiles[level.idx(tx, ty)] == TileWall {
				t.Fatalf("player ended inside a wall at %d,%d after move %d,%d (map %dx%d)",
					tx, ty, m[0], m[1], w, h)
			}
		}
	})
}

// The fog of war must only ever grow: once a tile is explored, no sequence of
// moves and turns can hide it again. Also, the currently visible set must
// always be a subset of the explored set.
func TestPropertyExplorationIsMonotonic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(12, 64).Draw(t, "width")
		h := rapid.IntRange(12, 48).Draw(t, "height")

		level := NewLevel(w, h)
		sx, sy := level.StartPosition()
		p := NewPlayer(sx, sy)

		level.UpdateVision(int(p.X)/tileSize, int(p.Y)/tileSize, int(p.Dir))

		steps := rapid.IntRange(1, 100).Draw(t, "steps")
		for i := 0; i < steps; i++ {
			m := moveDirs.Draw(t, "move")
			p.tryMove(level, m[0], m[1])
			if rapid.Bool().Draw(t, "turn") {
				p.Dir = Direction(rapid.IntRange(0, 3).Draw(t, "newdir"))
			}

			before := append([]bool(nil), level.explored...)
			level.UpdateVision(int(p.X)/tileSize, int(p.Y)/tileSize, int(p.Dir))

			for idx, e := range level.explored {
				if before[idx] && !e {
					t.Fatalf("explored shrank at tile %d,%d on step %d (map %dx%d)",
						idx%w, idx/w, i, w, h)
				}
				if level.visible[idx] && !e {
					t.Fatalf("visible but not explored at tile %d,%d on step %d (map %dx%d)",
						idx%w, idx/w, i, w, h)
				}
			}
		}
	})
}

// Recomputing vision for the same position and facing must produce identical
// results every time — no dependence on time, order, or prior state.
func TestPropertyVisionIsDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(12, 80).Draw(t, "width")
		h := rapid.IntRange(12, 60).Draw(t, "height")

		level := NewLevel(w, h)
		px := rapid.IntRange(0, w-1).Draw(t, "px")
		py := rapid.IntRange(0, h-1).Draw(t, "py")
		dir := rapid.IntRange(0, 3).Draw(t, "dir")

		level.UpdateVision(px, py, dir)
		vis1 := append([]bool(nil), level.visible...)
		exp1 := append([]bool(nil), level.explored...)

		level.UpdateVision(px, py, dir)

		if !slicesEqual(level.visible, vis1) {
			t.Fatalf("visible set changed on recompute for (%d,%d) dir %d", px, py, dir)
		}
		if !slicesEqual(level.explored, exp1) {
			t.Fatalf("explored set changed on recompute for (%d,%d) dir %d", px, py, dir)
		}
	})
}

func slicesEqual(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Every generated door and secret door must sit in a one-tile passage:
// two opposite open neighbours and two opposite wall neighbours (corridor
// or room-edge gap, never a room interior), be passable, and block sight.
func TestPropertyDoorsInValidPlaces(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(12, 80).Draw(t, "width")
		h := rapid.IntRange(12, 60).Draw(t, "height")
		l := NewLevel(w, h)

		for i, tile := range l.Tiles {
			if tile != TileDoor && tile != TileSecretDoor {
				continue
			}
			tx, ty := i%l.Width, i/l.Width

			// 1. Placement: doorway shape, checked independently of the
			//    generator's own predicate.
			if !doorNeighborsOK(l, tx, ty) {
				t.Fatalf("door %d,%d is not a one-tile passage shape", tx, ty)
			}

			// 2. Passable: movement must allow stepping onto a door.
			if l.IsBlockedTile(tx, ty) {
				t.Fatalf("door %d,%d should be passable for movement", tx, ty)
			}

			// 3. Opacity: sight never crosses a door.
			assertDoorOpaque(t, l, tx, ty)
		}
	})
}

func doorNeighborsOK(l *Level, tx, ty int) bool {
	var oc, wc int
	var openSum, wallSum int
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := tx+d[0], ty+d[1]
		if !l.inBounds(nx, ny) {
			return false
		}
		dir := d[0]*4 + d[1]
		if l.Tiles[l.idx(nx, ny)] == TileWall {
			wc++
			wallSum += dir
		} else {
			oc++
			openSum += dir
		}
	}
	return oc == 2 && wc == 2 && openSum == 0 && wallSum == 0
}

func assertDoorOpaque(t *rapid.T, l *Level, tx, ty int) {
	offsets := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for _, d := range offsets {
		// Look from an open tile on one side of the door at the open tile
		// on the opposite side: the door sits directly between them.
		ax, ay := tx+d[0], ty+d[1]
		if !l.inBounds(ax, ay) {
			continue
		}
		if opaqueToVision(l.Tiles[l.idx(ax, ay)]) {
			continue // must look from an open tile
		}
		cx, cy := tx-d[0], ty-d[1]
		if !l.inBounds(cx, cy) {
			continue
		}
		if opaqueToVision(l.Tiles[l.idx(cx, cy)]) {
			continue // the far tile must be open to test opacity
		}
		if !l.sightClear(ax, ay, tx, ty) {
			t.Fatalf("door %d,%d must be visible from adjacent tile %d,%d", tx, ty, ax, ay)
		}
		if l.sightClear(ax, ay, cx, cy) {
			t.Fatalf("x-ray: line of sight travels through door %d,%d on the way to %d,%d", tx, ty, cx, cy)
		}
	}
}
