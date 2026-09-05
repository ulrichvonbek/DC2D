package main

import (
	"testing"
	"time"
)

// testCorridor builds a small level that is all walls except a single row of
// floor across its width, giving an unobstructed straight line for LOS.
func testCorridor() *Level {
	const w, h = 40, 15
	tiles := make([]Tile, w*h)
	for i := range tiles {
		tiles[i] = TileWall
	}
	l := &Level{Width: w, Height: h, Tiles: tiles}
	for x := 0; x < w; x++ {
		l.Tiles[l.idx(x, 5)] = TileFloor
	}
	return l
}

func TestSpiderIsWeakNonDoorOpener(t *testing.T) {
	s := spiderStats()
	if s.Spike != 12 {
		t.Fatalf("spider spike = %.0f, want 12", s.Spike)
	}
	if s.HitChance != 0.5 {
		t.Fatalf("spider hit chance = %.2f, want 0.5", s.HitChance)
	}
	if s.OpenDoors {
		t.Fatal("spiders should not open doors")
	}
}

func TestMonstersCannotShareASquare(t *testing.T) {
	l := testCorridor()
	a := NewMonster(0, 5, spiderStats())
	b := NewMonster(1, 5, spiderStats())
	l.Monsters = []*Monster{a, b}

	// b tries to step onto a's tile from one tile away.
	b.TX, b.TY = 0, 5 // force collision scenario
	if l.monsterMayOccupy(0, 5, b, 20, 5) {
		t.Fatal("a monster may occupy another monster's tile")
	}

	// A monster cannot step onto the player either.
	if l.monsterMayOccupy(2, 5, a, 2, 5) {
		t.Fatal("a monster may occupy the player's tile")
	}

	// An empty floor tile is fine.
	if !l.monsterMayOccupy(6, 5, a, 20, 5) {
		t.Fatal("a monster should move onto an empty floor tile")
	}
}

func TestMonsterDoesNotMoveOntoAnotherMonster(t *testing.T) {
	l := testCorridor()
	a := NewMonster(2, 5, spiderStats())
	b := NewMonster(3, 5, spiderStats())
	l.Monsters = []*Monster{a, b}

	a.state = monsterChase
	a.paceT = 0
	// b sits between a and the player at (20,5): the target step is blocked.
	a.Update(l, 20, 5, 10*time.Millisecond)
	if a.TX == b.TX && a.TY == b.TY {
		t.Fatalf("monster stepped onto an occupied tile (%d,%d)", a.TX, a.TY)
	}
}

func TestMonsterChasesAtAnyDistance(t *testing.T) {
	l := testCorridor()
	m := NewMonster(0, 5, spiderStats())
	m.Update(l, 39, 5, 10*time.Millisecond)
	if m.state != monsterChase {
		t.Fatalf("spider 39 tiles down a straight corridor did not chase (state %v)", m.state)
	}
}

func TestMonsterSeesThroughNothing(t *testing.T) {
	l := testCorridor()
	l.Tiles[l.idx(20, 5)] = TileDoor
	m := NewMonster(5, 5, spiderStats())
	m.Update(l, 35, 5, 10*time.Millisecond)
	if m.state == monsterChase {
		t.Fatal("spider must not see through a closed door")
	}

	// Player tucked off the corridor row, behind wall tiles.
	m2 := NewMonster(5, 5, spiderStats())
	m2.Update(l, 30, 7, 10*time.Millisecond)
	if m2.state == monsterChase {
		t.Fatal("spider must not see through a wall")
	}
}

func TestMonsterDoorPassage(t *testing.T) {
	l := testCorridor()
	l.Tiles[l.idx(6, 5)] = TileDoor

	if l.monsterCanEnter(6, 5, false) {
		t.Fatal("a door refuser stepped through a door")
	}
	if !l.monsterCanEnter(6, 5, true) {
		t.Fatal("a door opener was blocked by a door")
	}
	if l.monsterCanEnter(5, 10, true) {
		t.Fatal("a monster entered a wall tile")
	}
}

func TestMonsterSeeksLastSeenThenWanders(t *testing.T) {
	l := testCorridor()
	m := NewMonster(5, 5, spiderStats())
	m.state = monsterSeek
	m.lastSeenX, m.lastSeenY = 10, 5
	m.seekT = 100
	m.paceT = m.stats.Pace.Seconds()

	for i := 0; i < 5; i++ {
		m.Update(l, 0, 0, m.stats.Pace) // player is in a wall: no LOS
	}
	if m.TX != 10 || m.TY != 5 {
		t.Fatalf("seek ended at (%d,%d), want (10,5)", m.TX, m.TY)
	}
	if m.state != monsterWander {
		t.Fatalf("state after reaching last seen = %v, want wander", m.state)
	}
}

func TestMonsterAttackCadence(t *testing.T) {
	l := testCorridor()
	stats := spiderStats()
	stats.HitChance = 1
	stats.Pace = time.Second
	m := NewMonster(5, 5, stats)
	m.paceT = 0

	if spike := m.Update(l, 6, 5, 50*time.Millisecond); spike != stats.Spike {
		t.Fatalf("first swing spike = %.1f, want %.1f", spike, stats.Spike)
	}
	if spike := m.Update(l, 6, 5, 100*time.Millisecond); spike != 0 {
		t.Fatalf("swung before the cadence was up, spike %.1f", spike)
	}
	if spike := m.Update(l, 6, 5, 1100*time.Millisecond); spike != stats.Spike {
		t.Fatalf("swing after cadence spike = %.1f, want %.1f", spike, stats.Spike)
	}
}

func TestMonsterMissDealsNoSpike(t *testing.T) {
	l := testCorridor()
	stats := spiderStats()
	stats.HitChance = 0
	m := NewMonster(5, 5, stats)
	m.paceT = 0

	if spike := m.Update(l, 6, 5, 10*time.Millisecond); spike != 0 {
		t.Fatalf("a miss dealt spike %.1f", spike)
	}
}

func TestMonsterMovesInsteadOfAttackingWhenNotAdjacent(t *testing.T) {
	l := testCorridor()
	m := NewMonster(5, 5, spiderStats())
	m.paceT = 0

	// Player two tiles away along the corridor: the beat should move, not
	// strike.
	m.Update(l, 7, 5, 10*time.Millisecond)
	if m.TX != 6 {
		t.Fatalf("expected a step toward the player, monster at x=%d", m.TX)
	}
}
