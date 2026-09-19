# AGENTS.md

Guidance for coding agents working in this repository. Applies to the whole repo.

## Project

DC2D is a grid-based, real-time dungeon crawler in Go, rendered with Ebiten. The
world is tiles; the player moves and turns on a grid while monsters act on
individual "beat" timers. Everything lives in a single `package main` at the repo
root — there are no subpackages. Module `DC2D`, Go 1.26.7. Key deps:
`github.com/hajimehoshi/ebiten/v2`, `go.yaml.in/yaml/v3`, `golang.org/x/image`,
and `pgregory.net/rapid` (property tests).

## Commands

```sh
gofmt -l .        # must print nothing
go vet ./...
go test ./...
go build .        # produces ./DC2D (gitignored)
timeout 5 ./DC2D  # smoke run; logs "loaded N monster type(s)" at startup
```

Definition of done: `gofmt` clean, `go vet` clean, `go test ./...` passing, and
`go build .` succeeding. Run all four before reporting a task complete.

## Layout (by concern, all `package main`)

- `main.go` — entry point; `errQuit` clean-exit sentinel; loads the monster
  override before `NewGame`.
- `game.go` — `Game` state, the `Update`/`Draw` loop, fades, faint/death
  handling, and `reset()`/`restart()`.
- `level.go` — dungeon generation, spawning (`spawnPool`), `UpdateVision` hooks.
- `vision.go` — line-of-sight / light cone and visibility.
- `camera.go` — viewport scrolling.
- `player.go` — movement, turning, attack targeting.
- `command.go` — the verb + hand command system (`j`, `[`, `]`, ...).
- `monsters.go` / `monsters.yaml` — data-driven monster definitions: embed, YAML
  parse, validation, registry, disk override.
- `monster.go` — `Monster` behavior plus the code-defined sprite silhouettes.
- `weapons.go` — `Weapon` profiles and the `bareHands` tuning baseline.
- `statusbar.go` — `HeartRate` model, thresholds, pulse square, bar rendering.
- `info.go` / `help.go` — info panel and help modal; the UI color palette.
- `death.go` — the "you died" end screen(s).

Tests mirror their subject (`*_test.go`, same package). `property_test.go` uses
`pgregory.net/rapid`; `visual_test.go` and `debug_test.go` render offscreen with
`ebiten.NewImage`.

## Conventions

- Match the existing doc-comment style: nearly every function has a short
  comment explaining intent, not mechanics.
- Keep shared colors/palette values centralized in `info.go` / `statusbar.go`
  rather than inlined at call sites.
- Build expensive images once via `sync.Once` or `sync.Map` (see `statusbar.go`,
  `monster.go`).
- Keep game logic pure and testable. Tests must be deterministic: pin randomness
  by emptying the level (`g.level.Monsters = nil`) when asserting exact values.
- Tuning constants are contracts: regression-pin them in tests (spider/snake
  stats, `bareHands`, HR thresholds) so drift is intentional.

## Domain contracts

### Heart rate
`baseHR = 80`. Level-0 thresholds: pass out at `hrPassOut = 180`, wake at
`hrWake = 150`, death at `hrDeath = 200`, hard flatline at `hrFlatline = 220`,
movement ceiling `hrAsymptote = 205`. They scale with player level via
`hrThresholds`. Movement ramps BPM toward the ceiling; resting recovers toward
base. Rationale: the HR economy is the core risk system — running and swinging
tax the heart, and the pass-out/death bands create escalating pressure.

### Fainting
At the pass-out threshold the player blacks out (quadrant mask) and input stops.
The status bar masks the real BPM as `???` and the pulse square hammers at the
pass-out cadence instead of the true, recovering rate; waking (BPM at or below
the wake threshold) restores the real number immediately. Rationale: while
unconscious the actual number would leak how close to death you are, so it is
deliberately hidden.

### Death (two tiers)
- **Overload, 200-220**: HR freezes in place, the bar stays `???`, and the pulse
  keeps hammering. After a 3s beat (`deathReady`) the end screen appears.
- **Flatline, above 220**: HR crashes to `0`, the pulse stops, and the bar shows
  a stark `0`.

Rationale: a barely-fatal hit should be indistinguishable from a faint right up
to the end screen, while a devastating hit is unmistakable. The end screen
offers **R** to restart and **ESC** to quit cleanly (`errQuit`).

### Monsters
Definitions live in `monsters.yaml` (embedded; an on-disk copy in the working
dir or beside the executable overrides it). Fields: `id`, `name`, `pace`,
`spike`, `hit_chance`, `open_doors`, `max_hp`, `shape`, `color`, `invisible`,
`min_level`, `max_level` (0 = no cap), `spawn_count`. Spawning (`spawnPool`) is
purely data-driven: each type contributes `spawn_count` placements on depths in
its window, and no monster spawns closer than 10 Manhattan tiles to the start.
`shape` selects a code-defined silhouette; `color` tints it.

### Weapons
`bareHands` (1 damage, 0.8 to-hit, 10 HR on a hit / 5 on a miss) is the tuning
anchor. Future weapons (dagger < sword < axe/hammer) should scale damage and HR
cost up from it so heavier hits are proportionally more taxing.

## Gotchas

- Adding a monster type is three steps: add the YAML entry, register the shape
  in `monster.go` (`buildShapeSprite`, `knownShape`, `knownShapes`), and update
  the embedded-count assertions in `monsters_test.go`.
- Do not change `monsterSize`; `visual_test.go` pins sprite centering to it.
- The dead branch in `game.go` does not call `HeartRate.Update`, so BPM stays
  frozen. Use `HammerPassOut` to keep the overload pulse beating.
- `main.go` touches more than one feature, so commits that try to split features
  can hit awkward interleaving.
- `HRDisplay` caches its text and owns the faint/fatal masking; route bar-number
  changes through `HeartRate.Die` and the `fainted`/`masked` state, not around
  it.

## Git

- Remote: `origin` -> `git@github.com:ulrichvonbek/DC2D.git`, branch `main`.
- Never commit, amend, push, or open a PR unless explicitly asked.
- Commit messages: a capitalized summary line; add a short body for multi-part
  changes.
- `DC2D` and `dungeon-crawl` binaries and `testdata/rapid/` are gitignored.
