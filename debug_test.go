package main

import "testing"

// Reproduces the doorway layout the corner-cutting x-ray bug showed up in:
//
//	========
//	->......
//	========
//
// The wall corner beside the player must not let light leak onto the wall
// behind it, either through pass 1 (cone) or pass 2 (surface reveal).
func TestNoCornerCutThroughWall(t *testing.T) {
	l := newOpenArea()

	// Door jamb beside the player: a wall column at x=0..1, y=1.
	l.Tiles[l.idx(0, 1)] = TileWall
	l.Tiles[l.idx(1, 1)] = TileWall
	// Second wall layer behind the jamb, previously x-rayed.
	l.Tiles[l.idx(2, 1)] = TileWall
	l.Tiles[l.idx(3, 1)] = TileWall

	l.UpdateVision(0, 2, int(DirEast))

	// Control: the corridor straight ahead is visible.
	if !l.visible[l.idx(2, 2)] {
		t.Fatal("setup broken: corridor ahead should be visible")
	}

	// The corner tile itself must not be cone-lit when its far side is a
	// wall (player is effectively peeking around it).
	if l.visible[l.idx(1, 1)] {
		t.Error("corner wall must not be cone-lit through the jamb")
	}
	// Wall behind the corner must not be lit (the original x-ray).
	if l.visible[l.idx(2, 1)] {
		t.Error("x-ray: wall behind corner must not be visible")
	}
	if l.visible[l.idx(3, 1)] {
		t.Error("x-ray: wall two tiles behind corner must not be visible")
	}
}
