package main

import "testing"

const startXY = 16

func newTestDungeon() *Level {
	// 5x5 tile all walls (Tile is zero-valued to TileWall), with floor cells:
	//   1,1  2,1  3,1
	//   1,2
	//   1,3
	l := &Level{Width: 5, Height: 5, Tiles: make([]Tile, 25)}
	for _, c := range [][2]int{{1, 1}, {2, 1}, {3, 1}, {1, 2}, {1, 3}} {
		l.Tiles[l.idx(c[0], c[1])] = TileFloor
	}
	return l
}

func TestTryMoveToFloor(t *testing.T) {
	l := newTestDungeon()
	p := NewPlayer(startXY, startXY)

	p.tryMove(l, 1, 0)
	if p.X != 32 || p.Y != 16 {
		t.Fatalf("expected move to 32,16, got %v,%v", p.X, p.Y)
	}

	p.tryMove(l, 1, 0)
	if p.X != 48 || p.Y != 16 {
		t.Fatalf("expected move to 48,16, got %v,%v", p.X, p.Y)
	}
}

func TestTryMoveBlockedByWall(t *testing.T) {
	l := newTestDungeon()
	p := NewPlayer(startXY, startXY)

	// (1,1) -> (0,1): (0,1) is a wall.
	p.tryMove(l, -1, 0)
	// (1,1) -> (1,2) is floor, (1,1) -> (2,1) is floor; (1,1) -> (0,1) invalid must not move.
	if p.X != 16 || p.Y != 16 {
		t.Fatalf("expected no movement, got %v,%v", p.X, p.Y)
	}
}

func TestTryMoveNoOpDirection(t *testing.T) {
	l := newTestDungeon()
	p := NewPlayer(startXY, startXY)

	p.tryMove(l, 0, 0)
	if p.X != 16 || p.Y != 16 {
		t.Fatalf("no-op move should not move, got %v,%v", p.X, p.Y)
	}
}

func TestTryMoveOutOfBounds(t *testing.T) {
	l := &Level{Width: 5, Height: 5, Tiles: make([]Tile, 25)}
	l.Tiles[l.idx(0, 0)] = TileFloor

	p := NewPlayer(0, 0)
	p.tryMove(l, -1, 0)
	p.tryMove(l, 0, -1)
	p.tryMove(l, -1, -1)

	if p.X != 0 || p.Y != 0 {
		t.Fatalf("expected no movement out of bounds, got %v,%v", p.X, p.Y)
	}
}

func TestTryMoveDiagonalAllowed(t *testing.T) {
	l := &Level{Width: 5, Height: 5, Tiles: make([]Tile, 25)}
	for _, c := range [][2]int{{1, 1}, {2, 1}, {1, 2}, {2, 2}} {
		l.Tiles[l.idx(c[0], c[1])] = TileFloor
	}

	p := NewPlayer(startXY, startXY)
	p.tryMove(l, 1, 1)

	if p.X != 32 || p.Y != 32 {
		t.Fatalf("expected diagonal move to 32,32, got %v,%v", p.X, p.Y)
	}
}

func TestTryMoveDiagonalBlockedByCorner(t *testing.T) {
	// Floor at (1,1) and (2,2) only: flanking cells (2,1) and (1,2) are walls,
	// so a diagonal should not slip through the corners.
	l := &Level{Width: 5, Height: 5, Tiles: make([]Tile, 25)}
	l.Tiles[l.idx(1, 1)] = TileFloor
	l.Tiles[l.idx(2, 2)] = TileFloor

	p := NewPlayer(startXY, startXY)
	p.tryMove(l, 1, 1)

	if p.X != 16 || p.Y != 16 {
		t.Fatalf("expected diagonal move blocked by wall corner, got %v,%v", p.X, p.Y)
	}
}

func TestTryMoveKeepsGridAlignment(t *testing.T) {
	l := NewLevel(64, 48)
	x, y := l.StartPosition()
	p := NewPlayer(x, y)

	moves := [][2]int{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}}
	for _, m := range moves {
		p.tryMove(l, m[0], m[1])
		if int(p.X)%tileSize != 0 || int(p.Y)%tileSize != 0 {
			t.Fatalf("player left grid after move %v: %v,%v", m, p.X, p.Y)
		}
	}
}
