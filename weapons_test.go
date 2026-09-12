package main

import (
	"testing"
	"time"
)

// combatTestPlayer builds a player standing on tile (10,5), facing east,
// inside a test corridor.
func combatTestPlayer(t *testing.T) *Player {
	t.Helper()
	p := NewPlayer(160, 80) // tile 10,5
	p.Dir = DirEast
	return p
}

func TestBareHandsProfile(t *testing.T) {
	w := bareHands()
	if w.Damage <= 0 {
		t.Fatalf("bare hands must deal some damage, got %d", w.Damage)
	}
	if w.ToHit <= 0 || w.ToHit >= 1 {
		t.Fatalf("bare hands hit chance %.2f out of (0,1)", w.ToHit)
	}
	if w.HRCost <= 0 {
		t.Fatalf("a swing must cost HR, got %.1f", w.HRCost)
	}
}

func TestPlayerAttackHitCostsFullAndDamages(t *testing.T) {
	l := testCorridor()
	p := combatTestPlayer(t)
	w := bareHands()
	w.ToHit = 1 // force a hit
	p.Weapon = w

	m := NewMonster(11, 5, spiderStats())
	before := m.HP
	l.Monsters = []*Monster{m}

	res := p.Attack(l, handLeft)
	if res.Cost != w.HRCost {
		t.Fatalf("hit cost = %.1f, want full %.1f", res.Cost, w.HRCost)
	}
	if !res.Hit {
		t.Fatal("expected a hit")
	}
	if res.Target != "Spider" {
		t.Fatalf("target = %q, want %q", res.Target, "Spider")
	}
	if res.Killed != nil {
		t.Fatalf("a spider at %d HP shouldn't die in one bare-handed swing", before)
	}
	if m.HP != before-w.Damage {
		t.Fatalf("HP after hit = %d, want %d", m.HP, before-w.Damage)
	}
}

func TestPlayerAttackMissCostsHalf(t *testing.T) {
	l := testCorridor()
	p := combatTestPlayer(t)
	w := bareHands()
	w.ToHit = 0 // force a miss
	p.Weapon = w

	m := NewMonster(11, 5, spiderStats())
	before := m.HP
	l.Monsters = []*Monster{m}

	res := p.Attack(l, handRight)
	if res.Cost != w.HRCost/2 {
		t.Fatalf("miss cost = %.1f, want half %.1f", res.Cost, w.HRCost/2)
	}
	if res.Hit {
		t.Fatal("expected a miss")
	}
	if res.Target != "Spider" {
		t.Fatalf("target = %q, want %q", res.Target, "Spider")
	}
	if res.Killed != nil {
		t.Fatal("a miss must not kill")
	}
	if m.HP != before {
		t.Fatal("a miss must not damage")
	}
}

func TestPlayerAttackEmptyTileCostsLikeAMiss(t *testing.T) {
	l := testCorridor()
	p := combatTestPlayer(t)
	w := bareHands()
	p.Weapon = w

	res := p.Attack(l, handLeft)
	if res.Cost != w.HRCost/2 {
		t.Fatalf("a swing at air should cost half (like a miss), got %.1f, want %.1f", res.Cost, w.HRCost/2)
	}
	if res.Hit || res.Target != "" || res.Killed != nil {
		t.Fatalf("a swing at air should hit nothing, got %+v", res)
	}
}

func TestPlayerAttackKillRemovesMonster(t *testing.T) {
	l := testCorridor()
	p := combatTestPlayer(t)
	w := bareHands()
	w.ToHit = 1
	p.Weapon = w

	stats := spiderStats()
	stats.MaxHP = 1 // dies in one swing
	m := NewMonster(11, 5, stats)
	l.Monsters = []*Monster{m}

	res := p.Attack(l, handLeft)
	if !res.Hit || res.Killed == nil {
		t.Fatalf("expected a killing swing, got %+v", res)
	}
	if res.Cost != w.HRCost {
		t.Fatalf("a killing swing should cost full HR, got %.1f", res.Cost)
	}

	l.RemoveMonster(res.Killed)
	if l.MonsterAt(m.TX, m.TY) != nil {
		t.Fatal("monster still present after removal")
	}
}

func TestPlayerTargetTileTracksFacing(t *testing.T) {
	p := NewPlayer(160, 80) // tile 10,5
	cases := []struct {
		dir    Direction
		tx, ty int
	}{
		{DirNorth, 10, 4},
		{DirEast, 11, 5},
		{DirSouth, 10, 6},
		{DirWest, 9, 5},
	}
	for _, c := range cases {
		p.Dir = c.dir
		tx, ty := p.TargetTile()
		if tx != c.tx || ty != c.ty {
			t.Fatalf("facing %v reach = (%d,%d), want (%d,%d)", c.dir, tx, ty, c.tx, c.ty)
		}
	}
}

func TestPlayerAttackHitsTheTargetTile(t *testing.T) {
	l := testCorridor()
	p := combatTestPlayer(t)
	w := bareHands()
	w.ToHit = 1
	p.Weapon = w

	stats := spiderStats()
	stats.MaxHP = 999
	m := NewMonster(11, 5, stats) // the east-facing player's TargetTile
	l.Monsters = []*Monster{m}

	res := p.Attack(l, handLeft)
	tx, ty := p.TargetTile()
	if !res.Hit || m.TX != tx || m.TY != ty {
		t.Fatalf("swing reached (%d,%d) but hit a monster elsewhere: %+v", tx, ty, res)
	}
}

func TestMonsterBitesWhenSharingThePlayersTile(t *testing.T) {
	l := testCorridor()
	stats := spiderStats()
	stats.HitChance = 1
	m := NewMonster(10, 5, stats) // same tile as the player
	l.Monsters = []*Monster{m}

	p := combatTestPlayer(t)
	if got := m.Update(l, int(p.X/tileSize), int(p.Y/tileSize), 1200*time.Millisecond); got == 0 {
		t.Fatal("a monster sharing the player's tile should bite")
	}
}
