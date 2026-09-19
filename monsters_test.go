package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedMonstersParses(t *testing.T) {
	reg, order, err := loadMonsterSet(strings.NewReader(string(embeddedMonsters)), "embedded")
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != monsterIDSpider || order[1] != "snake" {
		t.Fatalf("embedded order = %v, want [spider snake]", order)
	}
	if _, ok := reg[monsterIDSpider]; !ok {
		t.Fatal("embedded data is missing the spider")
	}
	if _, ok := reg["snake"]; !ok {
		t.Fatal("embedded data is missing the snake")
	}
}

// Spider stats are the tuning baseline everything is balanced against, so
// pin them against drift the same way weapons_test pins bare hands.
func TestSpiderStatsFromDataFile(t *testing.T) {
	st := spiderStats()
	if st.Name != "Spider" ||
		st.Pace != 1200*time.Millisecond ||
		st.Spike != 12 ||
		st.HitChance != 0.5 ||
		st.OpenDoors ||
		st.MaxHP != 5 ||
		st.Shape != "spider" ||
		st.Color != [3]uint8{235, 120, 110} ||
		st.Invisible ||
		st.MinLevel != 1 ||
		st.MaxLevel != 0 ||
		st.SpawnCount != 5 {
		t.Fatalf("spider stats = %+v", st)
	}
}

// The snake is the first added type; its tuning pins the numbers chosen when
// it was introduced (faster, deadlier, and far sturdier than the spider).
func TestSnakeStatsFromDataFile(t *testing.T) {
	st, ok := monsterByID("snake")
	if !ok {
		t.Fatal("snake missing from the registry")
	}
	if st.Name != "Snake" ||
		st.Pace != 1000*time.Millisecond ||
		st.Spike != 24 ||
		st.HitChance != 0.6 ||
		st.OpenDoors ||
		st.MaxHP != 15 ||
		st.Shape != "snake" ||
		st.Color != [3]uint8{120, 180, 90} ||
		st.Invisible ||
		st.MinLevel != 1 ||
		st.MaxLevel != 0 ||
		st.SpawnCount != 2 {
		t.Fatalf("snake stats = %+v", st)
	}
}

func TestDurationUnmarshal(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("1200ms")); err != nil {
		t.Fatal(err)
	}
	if d.Std() != 1200*time.Millisecond {
		t.Fatalf("got %v, want 1200ms", d.Std())
	}
	if err := d.UnmarshalText([]byte("nope")); err == nil {
		t.Fatal("bad duration accepted")
	}
}

// An omitted shape defaults to the only silhouette code defines today, and a
// monster may opt into invisibility before any light source exists.
func TestShapeDefaultsAndInvisibleParses(t *testing.T) {
	y := "monsters:\n" +
		"  - id: ghost\n" +
		"    name: Ghost\n" +
		"    pace: 10ms\n" +
		"    spike: 0\n" +
		"    hit_chance: 0.5\n" +
		"    max_hp: 1\n" +
		"    invisible: true\n" +
		"    min_level: 1\n"
	reg, order, err := loadMonsterSet(strings.NewReader(y), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || order[0] != "ghost" {
		t.Fatalf("order = %v", order)
	}
	st := reg["ghost"]
	if st.Shape != "spider" {
		t.Fatalf("default shape = %q, want spider", st.Shape)
	}
	if !st.Invisible {
		t.Fatal("invisible: true was not parsed")
	}
}

func TestLoadMonsterSetValidation(t *testing.T) {
	valid := "  - id: a\n    pace: 10ms\n    spike: 1\n    hit_chance: 0.5\n    max_hp: 1\n    min_level: 1\n"
	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"empty set", "monsters: []\n", "no monsters"},
		{"missing id", "monsters:\n  - name: X\n    pace: 10ms\n    max_hp: 1\n", "missing an id"},
		{"duplicate id",
			"monsters:\n" + valid + valid, "duplicate"},
		{"bad pace", "monsters:\n  - id: a\n    pace: 0ms\n    max_hp: 1\n", "pace"},
		{"unparseable pace", "monsters:\n  - id: a\n    pace: quickly\n    max_hp: 1\n", "invalid duration"},
		{"bad max_hp", "monsters:\n  - id: a\n    pace: 10ms\n    max_hp: 0\n", "max_hp"},
		{"bad hit_chance high", "monsters:\n  - id: a\n    pace: 10ms\n    hit_chance: 1.5\n    max_hp: 1\n", "hit_chance"},
		{"bad hit_chance low", "monsters:\n  - id: a\n    pace: 10ms\n    hit_chance: -0.1\n    max_hp: 1\n", "hit_chance"},
		{"bad spike", "monsters:\n  - id: a\n    pace: 10ms\n    spike: -1\n    max_hp: 1\n", "spike"},
		{"unknown shape", "monsters:\n  - id: a\n    pace: 10ms\n    shape: batman\n    max_hp: 1\n", "unknown shape"},
		{"bad spawn_count",
			"monsters:\n  - id: a\n    pace: 10ms\n    max_hp: 1\n    min_level: 1\n    spawn_count: -1\n",
			"spawn_count"},
		{"bad min_level",
			"monsters:\n  - id: a\n    pace: 10ms\n    max_hp: 1\n    min_level: 0\n",
			"min_level"},
		{"max_level below min_level",
			"monsters:\n  - id: a\n    pace: 10ms\n    max_hp: 1\n    min_level: 3\n    max_level: 2\n",
			"below min_level"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := loadMonsterSet(strings.NewReader(tc.yaml), "test")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestShown(t *testing.T) {
	visible := MonsterStats{}
	if !shown(visible, true) {
		t.Error("visible monster not shown while in sight")
	}
	if shown(visible, false) {
		t.Error("visible monster shown out of sight")
	}

	invisible := MonsterStats{Invisible: true}
	if shown(invisible, true) {
		t.Error("invisible monster shown while in sight")
	}
	if shown(invisible, false) {
		t.Error("invisible monster shown out of sight")
	}
}

// spawnPool feeds level generation, so pin the baseline depth-1 pool and the
// order the types appear in (data-file order, spider heavyweights first until
// the snake joins them at depth 1 too).
func TestSpawnPoolDepth1(t *testing.T) {
	pool := spawnPool(1)
	if len(pool) != 7 {
		t.Fatalf("depth 1 pool has %d monsters, want 7 (5 spiders + 2 snakes)", len(pool))
	}
	for i, st := range pool {
		want := "Spider"
		if i >= 5 {
			want = "Snake"
		}
		if st.Name != want {
			t.Fatalf("pool[%d] = %s, want %s", i, st.Name, want)
		}
	}
}

// Depth windows gate which types join the pool: a type with max_level set
// fades out past that depth, while max_level 0 means no upper bound.
func TestSpawnPoolHonorsDepthWindow(t *testing.T) {
	saveMonsterRegistry(t)

	path := filepath.Join(t.TempDir(), "monsters.yaml")
	override := "monsters:\n" +
		"  - id: spider\n" +
		"    name: Spider\n" +
		"    pace: 10ms\n" +
		"    spike: 1\n" +
		"    hit_chance: 0.5\n" +
		"    max_hp: 1\n" +
		"    shape: spider\n" +
		"    color: [1, 1, 1]\n" +
		"    min_level: 1\n" +
		"    max_level: 1\n" +
		"    spawn_count: 2\n" +
		"  - id: snake\n" +
		"    name: Snake\n" +
		"    pace: 10ms\n" +
		"    spike: 1\n" +
		"    hit_chance: 0.5\n" +
		"    max_hp: 1\n" +
		"    shape: snake\n" +
		"    color: [1, 1, 1]\n" +
		"    min_level: 2\n" +
		"    max_level: 0\n" +
		"    spawn_count: 2\n"
	if err := os.WriteFile(path, []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, _, _ := applyMonsterOverrideAt(path)
	if !loaded {
		t.Fatal("override did not load")
	}

	for i := 1; i <= 2; i++ {
		pool := spawnPool(i)
		if len(pool) != 2 {
			t.Fatalf("depth %d pool has %d monsters, want 2", i, len(pool))
		}
		want := "Spider"
		if i >= 2 {
			want = "Snake"
		}
		for _, st := range pool {
			if st.Name != want {
				t.Fatalf("depth %d pool = %s, want all %s", i, st.Name, want)
			}
		}
	}

	if pool := spawnPool(3); len(pool) != 2 || pool[0].Name != "Snake" {
		t.Fatalf("depth 3 pool = %+v, want 2 snakes (max_level 0 is open-ended)", pool)
	}
}

// saveMonsterRegistry snapshots the global registry so a test that swaps it
// can restore the embedded baseline when it finishes.
func saveMonsterRegistry(t *testing.T) {
	t.Helper()
	ensureMonsters()
	monsterMu.RLock()
	savedReg, savedOrder := monsterReg, monsterOrder
	monsterMu.RUnlock()
	t.Cleanup(func() {
		monsterMu.Lock()
		monsterReg, monsterOrder = savedReg, savedOrder
		monsterMu.Unlock()
	})
}

func TestApplyMonsterOverrideFromDisk(t *testing.T) {
	saveMonsterRegistry(t)

	path := filepath.Join(t.TempDir(), "monsters.yaml")
	override := "monsters:\n" +
		"  - id: spider\n" +
		"    name: Fiend\n" +
		"    pace: 900ms\n" +
		"    spike: 9\n" +
		"    hit_chance: 0.25\n" +
		"    open_doors: true\n" +
		"    max_hp: 99\n" +
		"    shape: spider\n" +
		"    color: [1, 2, 3]\n" +
		"    invisible: false\n" +
		"    min_level: 1\n" +
		"    max_level: 0\n" +
		"    spawn_count: 3\n"
	if err := os.WriteFile(path, []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, n, src := applyMonsterOverrideAt(path)
	if !loaded || n != 1 || src != path {
		t.Fatalf("loaded=%v n=%d src=%q", loaded, n, src)
	}

	st, ok := monsterByID("spider")
	if !ok {
		t.Fatal("spider missing after override")
	}
	want := MonsterStats{
		Name:       "Fiend",
		Pace:       900 * time.Millisecond,
		Spike:      9,
		HitChance:  0.25,
		OpenDoors:  true,
		MaxHP:      99,
		Shape:      "spider",
		Color:      [3]uint8{1, 2, 3},
		MinLevel:   1,
		MaxLevel:   0,
		SpawnCount: 3,
	}
	if st != want {
		t.Fatalf("override stats = %+v, want %+v", st, want)
	}
}

func TestApplyMonsterOverrideMissingKeepsEmbedded(t *testing.T) {
	saveMonsterRegistry(t)

	loaded, n, src := applyMonsterOverrideAt(filepath.Join(t.TempDir(), "nope.yaml"))
	if loaded || n != 0 || src != "" {
		t.Fatalf("loaded=%v n=%d src=%q", loaded, n, src)
	}

	st, ok := monsterByID("spider")
	if !ok || st.MaxHP != 5 {
		t.Fatalf("embedded spider disturbed by missing override: %+v", st)
	}
}

func TestApplyMonsterOverrideMalformedKeepsEmbedded(t *testing.T) {
	saveMonsterRegistry(t)

	path := filepath.Join(t.TempDir(), "monsters.yaml")
	malformed := "monsters:\n  - id: spider\n    pace: 0ms\n"
	if err := os.WriteFile(path, []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, _, _ := applyMonsterOverrideAt(path)
	if loaded {
		t.Fatal("malformed override was applied")
	}

	st, ok := monsterByID("spider")
	if !ok || st.MaxHP != 5 {
		t.Fatalf("embedded spider disturbed by malformed override: %+v", st)
	}
}
