package main

import "testing"

func TestNewLevelInvariants(t *testing.T) {
	l := NewLevel(64, 48)

	floors, walls, stairs := 0, 0, 0
	for _, tl := range l.Tiles {
		switch tl {
		case TileFloor:
			floors++
		case TileWall:
			walls++
		case TileStairs:
			stairs++
		}
	}

	if floors == 0 {
		t.Error("expected at least one floor tile")
	}
	if walls == 0 {
		t.Error("expected at least one wall tile")
	}
	if stairs != 1 {
		t.Errorf("expected exactly 1 stairs tile, got %d", stairs)
	}
}

func TestStartPositionOnFloor(t *testing.T) {
	for i := 0; i < 50; i++ {
		l := NewLevel(64, 48)
		x, y := l.StartPosition()

		if int(x)%tileSize != 0 || int(y)%tileSize != 0 {
			t.Fatalf("start position %v,%v is not tile aligned", x, y)
		}

		tx := int(x) / tileSize
		ty := int(y) / tileSize
		if !l.inBounds(tx, ty) {
			t.Fatalf("start position out of bounds: %d,%d", tx, ty)
		}
		if l.Tiles[l.idx(tx, ty)] != TileFloor {
			t.Fatalf("start position %d,%d is not on floor", tx, ty)
		}
	}
}

func TestIsBlockedTileOutOfBounds(t *testing.T) {
	l := NewLevel(10, 10)

	for _, c := range [][2]int{{-1, 0}, {0, -1}, {10, 0}, {0, 10}, {-5, -5}, {11, 11}} {
		if !l.IsBlockedTile(c[0], c[1]) {
			t.Errorf("expected tile %d,%d to be blocked (out of bounds)", c[0], c[1])
		}
	}
}

func TestCarveRoom(t *testing.T) {
	l := &Level{Width: 10, Height: 10, Tiles: make([]Tile, 100)}

	l.carveRoom(struct{ x, y, w, h int }{2, 2, 4, 3})

	for yy := 2; yy < 5; yy++ {
		for xx := 2; xx < 6; xx++ {
			if l.Tiles[l.idx(xx, yy)] != TileFloor {
				t.Fatalf("expected floor inside room at %d,%d", xx, yy)
			}
		}
	}

	if l.Tiles[l.idx(1, 2)] != TileWall {
		t.Error("cell outside room should remain wall")
	}
}

func TestCorridorCarving(t *testing.T) {
	l := &Level{Width: 10, Height: 10, Tiles: make([]Tile, 100)}

	l.carveHCorridor(2, 6, 5)
	for x := 2; x <= 6; x++ {
		if l.Tiles[l.idx(x, 5)] != TileFloor {
			t.Fatalf("expected floor in horizontal corridor at %d,5", x)
		}
	}

	l.carveVCorridor(1, 4, 8)
	for y := 1; y <= 4; y++ {
		if l.Tiles[l.idx(8, y)] != TileFloor {
			t.Fatalf("expected floor in vertical corridor at 8,%d", y)
		}
	}
}

func TestDoorShapeOK(t *testing.T) {
	shape := func(walls, floors [][2]int) *Level {
		l := &Level{Width: 5, Height: 5, Tiles: make([]Tile, 25)}
		for i := range l.Tiles {
			l.Tiles[i] = TileWall
		}
		for _, w := range walls {
			l.Tiles[l.idx(w[0], w[1])] = TileWall
		}
		for _, f := range floors {
			l.Tiles[l.idx(f[0], f[1])] = TileFloor
		}
		return l
	}

	cases := []struct {
		name  string
		walls [][2]int
		flo   [][2]int
		want  bool
	}{
		// The user's doorway: walls north/south, floors east/west.
		{"horizontal passage", [][2]int{{2, 1}, {2, 3}}, [][2]int{{1, 2}, {3, 2}}, true},
		// Vertical passage: walls east/west, floors north/south.
		{"vertical passage", [][2]int{{1, 2}, {3, 2}}, [][2]int{{2, 1}, {2, 3}}, true},
		// Corridor corner: floors perpendicular, walls perpendicular.
		{"corner", [][2]int{{1, 2}, {2, 1}, {1, 1}}, [][2]int{{3, 2}, {2, 3}}, false},
		// Room edge where both perpendicular sides are open.
		{"three open", [][2]int{{2, 1}}, [][2]int{{1, 2}, {3, 2}, {2, 3}}, false},
		// Room interior: fully open.
		{"open area", nil, [][2]int{{1, 2}, {2, 1}, {3, 2}, {2, 3}}, false},
		// Three walls, one floor.
		{"dead end", [][2]int{{1, 2}, {2, 1}, {3, 2}}, [][2]int{{2, 3}}, false},
	}
	for _, c := range cases {
		l := shape(c.walls, c.flo)
		if got := l.doorShapeOK(2, 2); got != c.want {
			t.Errorf("%s: doorShapeOK(2,2) = %v, want %v", c.name, got, c.want)
		}
	}
}
