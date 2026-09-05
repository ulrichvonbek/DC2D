package main

import (
	"math"
	"testing"

	"pgregory.net/rapid"
)

func TestTileInCone(t *testing.T) {
	fx, fy := 1, 0 // facing East
	cases := []struct {
		dx, dy int
		want   bool
	}{
		{0, 0, true},   // own tile always visible
		{1, 0, true},   // straight ahead
		{3, 1, true},   // ~18° off-axis
		{2, 3, true},   // ~56°: inside 60° half-angle
		{2, 2, true},   // 45°: on the boundary
		{1, 2, false},  // ~63°: outside the cone
		{0, 1, false},  // 90° to the side
		{-1, 0, false}, // directly behind
	}
	for _, c := range cases {
		if got := tileInCone(fx, fy, c.dx, c.dy); got != c.want {
			t.Errorf("tileInCone(%d,%d) = %v, want %v", c.dx, c.dy, got, c.want)
		}
	}
}

func TestSightClear(t *testing.T) {
	l := &Level{Width: 10, Height: 10, Tiles: make([]Tile, 100)}
	for x := 0; x < 10; x++ {
		l.Tiles[l.idx(x, 5)] = TileFloor
	}
	l.Tiles[l.idx(5, 5)] = TileWall

	if l.sightClear(2, 5, 8, 5) {
		t.Error("expected wall at (5,5) to block line of sight")
	}
	if !l.sightClear(2, 5, 4, 5) {
		t.Error("expected clear line of sight to (4,5)")
	}
}

func TestUpdateVisionMarksExplored(t *testing.T) {
	l := newOpenArea()
	l.UpdateVision(12, 12, int(DirEast))

	tx, ty := 12, 12
	if !l.visible[l.idx(tx, ty)] {
		t.Error("player's own tile should be visible")
	}
	if !l.explored[l.idx(tx, ty)] {
		t.Error("player's own tile should be explored")
	}

	tx, ty = 15, 12
	if !l.visible[l.idx(tx, ty)] {
		t.Error("tile straight ahead within range should be visible")
	}

	tx, ty = 12, 15
	if l.visible[l.idx(tx, ty)] {
		t.Error("tile directly to the side should be outside the cone")
	}
	if l.explored[l.idx(tx, ty)] {
		t.Error("tile directly to the side should not be explored")
	}
}

func TestVisionRevealsAdjacentWalls(t *testing.T) {
	l := newOpenArea()
	// Wall one tile south of the player: orthogonal neighbor of the visible
	// player tile, so it gets explored even though it's outside the cone.
	l.Tiles[l.idx(12, 13)] = TileWall

	l.UpdateVision(12, 12, int(DirEast))

	if !l.explored[l.idx(12, 13)] {
		t.Error("wall adjacent to a visible tile should be explored")
	}
	if l.visible[l.idx(12, 13)] {
		t.Error("off-cone wall should not be in the visible set")
	}
}

func TestVisionRevealsNearWallsBehind(t *testing.T) {
	l := newOpenArea()
	// Walls behind and beside the player: inside the near-wall radius but
	// outside the forward cone, so they must still be revealed.
	l.Tiles[l.idx(12, 11)] = TileWall // north (perpendicular to facing East)
	l.Tiles[l.idx(11, 11)] = TileWall // behind-left diagonal

	l.UpdateVision(12, 12, int(DirEast))

	if !l.explored[l.idx(12, 11)] {
		t.Error("wall beside the player should be explored")
	}
	if !l.explored[l.idx(11, 11)] {
		t.Error("wall behind the player within near radius should be explored")
	}
	for _, c := range [][2]int{{12, 11}, {11, 11}} {
		if l.visible[l.idx(c[0], c[1])] {
			t.Error("near-revealed wall should not be in the visible set")
		}
	}
}

func TestVisionNoXRayThroughWalls(t *testing.T) {
	l := newOpenArea()
	// A thick wall block ahead of the player. (10,10) is the lit face.
	for x := 10; x <= 14; x++ {
		for y := 9; y <= 13; y++ {
			l.Tiles[l.idx(x, y)] = TileWall
		}
	}

	l.UpdateVision(5, 10, int(DirEast))

	if !l.visible[l.idx(10, 10)] {
		t.Fatal("setup broken: block's face wall should be visible")
	}
	// (11,10) sits behind the lit face, adjacent only to other walls. It must
	// not be revealed by propagating from the lit wall.
	if l.explored[l.idx(11, 10)] {
		t.Error("x-ray: second-layer wall must not be revealed from a lit wall")
	}
}

func corridorWithDoor(t Tile) *Level {
	l := &Level{
		Width:    12,
		Height:   5,
		Tiles:    make([]Tile, 12*5),
		explored: make([]bool, 12*5),
		visible:  make([]bool, 12*5),
	}
	for i := range l.Tiles {
		l.Tiles[i] = TileWall
	}
	for x := 0; x < 12; x++ {
		l.Tiles[l.idx(x, 2)] = TileFloor
	}
	l.Tiles[l.idx(4, 2)] = t // the door at (4,2)
	return l
}

func TestDoorBlocksVision(t *testing.T) {
	l := corridorWithDoor(TileDoor)
	l.UpdateVision(2, 2, int(DirEast))

	if !l.visible[l.idx(4, 2)] {
		t.Error("the door tile itself should be visible")
	}
	if l.visible[l.idx(5, 2)] {
		t.Error("x-ray: tile behind the door must not be visible")
	}
	if l.visible[l.idx(6, 2)] {
		t.Error("x-ray: tile two steps behind the door must not be visible")
	}
	if l.explored[l.idx(5, 2)] {
		t.Error("tile behind the door should not be explored either")
	}
	// Doors are passable: moving into one must not be blocked.
	if l.IsBlockedTile(4, 2) {
		t.Error("door should be passable for movement")
	}
}

func TestSecretDoorBlocksVision(t *testing.T) {
	l := corridorWithDoor(TileSecretDoor)
	l.UpdateVision(2, 2, int(DirEast))

	if !l.visible[l.idx(4, 2)] {
		t.Error("the secret door tile itself should be visible")
	}
	if l.visible[l.idx(5, 2)] {
		t.Error("x-ray: tile behind the secret door must not be visible")
	}
	if l.IsBlockedTile(4, 2) {
		t.Error("secret door should be passable for movement")
	}
}

func TestDoorVisibleWhenStandingOnIt(t *testing.T) {
	l := corridorWithDoor(TileDoor)
	l.UpdateVision(4, 2, int(DirEast)) // player stands in the doorway

	if !l.visible[l.idx(4, 2)] {
		t.Error("the door under the player must stay visible")
	}
	if !l.visible[l.idx(5, 2)] {
		t.Error("standing in the doorway should let you peek through it")
	}
}

func newOpenArea() *Level {
	l := &Level{
		Width:    24,
		Height:   24,
		Tiles:    make([]Tile, 576),
		explored: make([]bool, 576),
		visible:  make([]bool, 576),
	}
	for i := range l.Tiles {
		l.Tiles[i] = TileFloor
	}
	return l
}

// Every tile marked visible must be within range, inside the cone, and have
// unobstructed line of sight.
func TestPropertyVisionRespectsBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		l := NewLevel(24, 24)
		px, py := 12, 12
		dir := rapid.IntRange(0, 3).Draw(t, "dir")

		l.UpdateVision(px, py, dir)
		fx, fy := dirVectors[dir][0], dirVectors[dir][1]

		for i, vis := range l.visible {
			if !vis {
				continue
			}
			tx, ty := i%l.Width, i/l.Width
			dx, dy := tx-px, ty-py
			if math.Hypot(float64(dx), float64(dy)) > float64(visionRadius) {
				t.Fatalf("visible tile %d,%d beyond range %d", tx, ty, visionRadius)
			}
			if !tileInCone(fx, fy, dx, dy) {
				t.Fatalf("visible tile %d,%d outside cone", tx, ty)
			}
			if !l.sightClear(px, py, tx, ty) {
				t.Fatalf("visible tile %d,%d has no line of sight", tx, ty)
			}
		}
	})
}
