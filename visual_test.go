package main

import "testing"

// TestSpiderSpriteSitsDeadCenterInAttackFrame pins the invariant behind the
// attack-target outline: a monster standing on the tile that outline frames
// must render with the sprite dead in the middle of that tile. This exercises
// the exact anchor monster.Draw feeds the centering idiom, so it would fail
// if the sprite were anchored to the tile center instead (which pushed it a
// half-sprite toward the lower-right, the bug that broke the outline's look).
func TestSpiderSpriteSitsDeadCenterInAttackFrame(t *testing.T) {
	const tx, ty = 10, 5
	cam := NewCamera(screenWidth, screenHeight)

	ox, oy := monsterSpriteOrigin(tx, ty, cam)
	centerX := ox + float64(monsterSize)/2
	centerY := oy + float64(monsterSize)/2
	wantCX := float64(tx*tileSize) + float64(tileSize)/2
	wantCY := float64(ty*tileSize) + float64(tileSize)/2
	if centerX != wantCX || centerY != wantCY {
		t.Fatalf("drawn center (%v,%v) != tile center (%v,%v)", centerX, centerY, wantCX, wantCY)
	}

	// And with even margins on every side of the tile.
	left := ox - float64(tx*tileSize)
	top := oy - float64(ty*tileSize)
	right := float64(tx*tileSize+tileSize) - (ox + float64(monsterSize))
	bottom := float64(ty*tileSize+tileSize) - (oy + float64(monsterSize))
	if left != top || left != right || left != bottom {
		t.Fatalf("lopsided margins l=%v t=%v r=%v b=%v", left, top, right, bottom)
	}
	if left < 0 {
		t.Fatal("sprite must not poke outside the tile")
	}
}
