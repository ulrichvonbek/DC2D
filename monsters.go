package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// monsterIDSpider is the stable id of the default monster, used by spawning
// and tests.
const monsterIDSpider = "spider"

// monsters.yaml ships inside the binary so a run from any directory still has
// baseline monster data; applyMonsterOverride may layer a disk copy over it.
//
//go:embed monsters.yaml
var embeddedMonsters []byte

// Duration is a YAML-friendly time.Duration: it unmarshals strings like
// "1200ms" through time.ParseDuration.
type Duration time.Duration

// UnmarshalText lets yaml.v3 decode a scalar into a Duration.
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// Std converts back to a time.Duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }

// monsterDef is one entry of monsters.yaml.
type monsterDef struct {
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name"`
	Pace      Duration `yaml:"pace"`
	Spike     float64  `yaml:"spike"`
	HitChance float64  `yaml:"hit_chance"`
	OpenDoors bool     `yaml:"open_doors"`
	MaxHP     int      `yaml:"max_hp"`
	Shape     string   `yaml:"shape"`
	Color     [3]uint8 `yaml:"color"`
	Invisible bool     `yaml:"invisible"`
	MinLevel  int      `yaml:"min_level"`
	MaxLevel  int      `yaml:"max_level"`
	SpawnCnt  int      `yaml:"spawn_count"`
}

// monsterFile is the top-level shape of monsters.yaml.
type monsterFile struct {
	Monsters []monsterDef `yaml:"monsters"`
}

// monsterRegistry is the loaded monster table, keyed by id. It is filled from
// the embedded file on first use and may be swapped by the disk override.
var (
	monsterMu    sync.RWMutex
	monsterReg   map[string]MonsterStats
	monsterOrder []string
)

// ensureMonsters loads the embedded monster data once, lazily. A parse or
// validation failure is a development bug and panics at startup.
func ensureMonsters() {
	monsterMu.Lock()
	defer monsterMu.Unlock()
	if monsterReg != nil {
		return
	}
	reg, order, err := loadMonsterSet(bytes.NewReader(embeddedMonsters), "embedded monsters.yaml")
	if err != nil {
		panic(err)
	}
	monsterReg, monsterOrder = reg, order
}

// loadMonsterSet parses and validates a monster file from r, naming the
// source in any error. It returns the id-keyed table and the ids in file
// order.
func loadMonsterSet(r io.Reader, src string) (map[string]MonsterStats, []string, error) {
	var f monsterFile
	if err := yaml.NewDecoder(r).Decode(&f); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", src, err)
	}
	if len(f.Monsters) == 0 {
		return nil, nil, fmt.Errorf("%s: no monsters defined", src)
	}

	registry := make(map[string]MonsterStats, len(f.Monsters))
	order := make([]string, 0, len(f.Monsters))
	for i, d := range f.Monsters {
		if d.ID == "" {
			return nil, nil, fmt.Errorf("%s: entry %d is missing an id", src, i)
		}
		if _, dup := registry[d.ID]; dup {
			return nil, nil, fmt.Errorf("%s: duplicate monster id %q", src, d.ID)
		}
		shape := d.Shape
		if shape == "" {
			shape = shapeSpider
		}
		if !knownShape(shape) {
			return nil, nil, fmt.Errorf("%s: monster %q has unknown shape %q (known: %s)",
				src, d.ID, shape, knownShapes())
		}
		pace := d.Pace.Std()
		if pace <= 0 {
			return nil, nil, fmt.Errorf("%s: monster %q pace must be greater than 0", src, d.ID)
		}
		if d.MaxHP <= 0 {
			return nil, nil, fmt.Errorf("%s: monster %q max_hp must be greater than 0", src, d.ID)
		}
		if d.Spike < 0 {
			return nil, nil, fmt.Errorf("%s: monster %q spike must not be negative", src, d.ID)
		}
		if d.HitChance < 0 || d.HitChance > 1 {
			return nil, nil, fmt.Errorf("%s: monster %q hit_chance must be between 0 and 1",
				src, d.ID)
		}
		if d.SpawnCnt < 0 {
			return nil, nil, fmt.Errorf("%s: monster %q spawn_count must not be negative",
				src, d.ID)
		}
		if d.MinLevel < 1 {
			return nil, nil, fmt.Errorf("%s: monster %q min_level must be at least 1", src, d.ID)
		}
		if d.MaxLevel != 0 && d.MaxLevel < d.MinLevel {
			return nil, nil, fmt.Errorf("%s: monster %q max_level (%d) is below min_level (%d)",
				src, d.ID, d.MaxLevel, d.MinLevel)
		}

		registry[d.ID] = MonsterStats{
			Name:       d.Name,
			Pace:       pace,
			Spike:      d.Spike,
			HitChance:  d.HitChance,
			OpenDoors:  d.OpenDoors,
			MaxHP:      d.MaxHP,
			Shape:      shape,
			Color:      d.Color,
			Invisible:  d.Invisible,
			MinLevel:   d.MinLevel,
			MaxLevel:   d.MaxLevel,
			SpawnCount: d.SpawnCnt,
		}
		order = append(order, d.ID)
	}
	return registry, order, nil
}

// monsterByID returns the stats for a monster id.
func monsterByID(id string) (MonsterStats, bool) {
	ensureMonsters()
	monsterMu.RLock()
	defer monsterMu.RUnlock()
	st, ok := monsterReg[id]
	return st, ok
}

// monsterOverridePaths is where a disk monsters.yaml is looked for: the
// current directory first (so editing the file and running `go run .` works),
// then next to the executable (a deployed binary).
func monsterOverridePaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "monsters.yaml"))
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "monsters.yaml"))
	}
	return paths
}

// applyMonsterOverride swaps in a valid monsters.yaml from disk if one is
// found, otherwise it keeps the embedded data. A missing or broken file logs
// a warning rather than refusing to run. Called once from main, never tests.
func applyMonsterOverride() {
	paths := monsterOverridePaths()
	if loaded, n, src := applyMonsterOverrideAt(paths...); loaded {
		log.Printf("DC2D: loaded %d monster type(s) from %s", n, src)
		return
	}
	ensureMonsters()
	log.Printf("DC2D: using %d embedded monster type(s)", len(monsterOrder))
}

// applyMonsterOverrideAt tries each path in turn and applies the first valid
// file, reporting whether one was loaded.
func applyMonsterOverrideAt(paths ...string) (loaded bool, n int, src string) {
	for _, p := range paths {
		f, err := os.Open(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			log.Printf("DC2D: cannot read %s: %v", p, err)
			continue
		}
		stats, order, lerr := loadMonsterSet(f, p)
		f.Close()
		if lerr != nil {
			log.Printf("DC2D: %v", lerr)
			continue
		}
		monsterMu.Lock()
		monsterReg, monsterOrder = stats, order
		monsterMu.Unlock()
		return true, len(stats), p
	}
	return false, 0, ""
}
