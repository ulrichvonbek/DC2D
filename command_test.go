package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// commandGame builds a Game with a deterministic player facing east from tile
// (10,5) in an empty corridor.
func commandGame(t *testing.T) *Game {
	t.Helper()
	g := NewGame()
	g.level = testCorridor()
	g.player = combatTestPlayer(t)
	return g
}

// attackLine builds the expected two-run entry for an attack outcome.
func attackLine(side handSide, outcome string, style cmdStyle) logEntry {
	return logEntry{segments: []logSeg{
		{text: "Attack " + side.String() + " - ", style: cmdNormal},
		{text: outcome, style: style},
	}}
}

func TestAttackVerbShowsPreviewWithoutCommitting(t *testing.T) {
	g := commandGame(t)

	g.handleCmdInput(cmdInput{verbKey: true})
	if g.cmdVerb != verbAttack {
		t.Fatal("a j press should arm the attack verb")
	}
	if !g.cmdPending {
		t.Fatal("the preview line should be marked pending")
	}
	if !reflect.DeepEqual(g.inputLog, []logEntry{logNormal("Attack ?")}) {
		t.Fatalf("log = %v, want a bare [Attack ?] preview", g.inputLog)
	}
}

func TestAttackCommandResolvesOnLeftHand(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 1
	m := NewMonster(11, 5, spiderStats())
	before := m.HP
	g.level.Monsters = []*Monster{m}

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{handL: true})

	want := []logEntry{attackLine(handLeft, "Hit Spider!", cmdHit)}
	if !reflect.DeepEqual(g.inputLog, want) {
		t.Fatalf("log = %v, want %v", g.inputLog, want)
	}
	if g.cmdPending {
		t.Fatal("resolving must commit the command")
	}
	if g.cmdVerb != verbNone {
		t.Fatal("the pending verb should clear after completion")
	}
	if m.HP != before-g.player.Weapon.Damage {
		t.Fatalf("spider HP = %d, want %d", m.HP, before-g.player.Weapon.Damage)
	}
}

func TestAttackCommandNoTarget(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 1

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{handR: true})

	if !reflect.DeepEqual(g.inputLog, []logEntry{attackLine(handRight, "! MISS !", cmdMiss)}) {
		t.Fatalf("log = %v, want an empty-swing outcome line", g.inputLog)
	}
}

func TestAttackCommandMiss(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 0
	g.level.Monsters = []*Monster{NewMonster(11, 5, spiderStats())}

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{handL: true})

	if !reflect.DeepEqual(g.inputLog, []logEntry{attackLine(handLeft, "Miss Spider!", cmdMiss)}) {
		t.Fatalf("log = %v, want a whiffed-swing outcome line", g.inputLog)
	}
}

func TestAttackCommandKillRemovesMonster(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 1
	stats := spiderStats()
	stats.MaxHP = 1
	g.level.Monsters = []*Monster{NewMonster(11, 5, stats)}

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{handL: true})

	if len(g.level.Monsters) != 0 {
		t.Fatalf("level still holds %d monsters after the kill", len(g.level.Monsters))
	}
	if !reflect.DeepEqual(g.inputLog, []logEntry{attackLine(handLeft, "KILL SPIDER", cmdHit)}) {
		t.Fatalf("log = %v, want the killing-swing line", g.inputLog)
	}
}

// A killing blow reports KILL in caps rather than a plain hit.
func TestAttackKillOutcomeIsCaps(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 1
	stats := spiderStats()
	stats.MaxHP = 1
	g.level.Monsters = []*Monster{NewMonster(11, 5, stats)}

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{handR: true})

	if got := g.inputLog[0].segments[1].text; got != "KILL SPIDER" {
		t.Fatalf("outcome run = %q, want %q", got, "KILL SPIDER")
	}
	if g.inputLog[0].segments[1].style != cmdHit {
		t.Fatal("a kill is still a hit, so it must be green")
	}
}

// A frame in a command sequence belongs to the command: it is claimed (returns
// true) so the game drops all other input, making input atomic. Neutral frames
// are unclaimed.
func TestCommandClaimContract(t *testing.T) {
	g := commandGame(t)

	if g.handleCmdInput(cmdInput{}) {
		t.Error("an idle neutral frame should not be claimed")
	}
	if g.handleCmdInput(cmdInput{handL: true}) {
		t.Error("a stray hand key with no verb should not be claimed")
	}
	if g.handleCmdInput(cmdInput{cancel: true}) {
		t.Error("cancel with no pending verb should not be claimed")
	}

	if !g.handleCmdInput(cmdInput{verbKey: true}) {
		t.Error("arming a verb should claim its frame")
	}
	if !g.handleCmdInput(cmdInput{}) {
		t.Error("a frame spent waiting on a pending verb should be claimed")
	}
	if !g.handleCmdInput(cmdInput{other: true}) {
		t.Error("an invalid completion should claim its frame")
	}

	g.handleCmdInput(cmdInput{verbKey: true})
	if !g.handleCmdInput(cmdInput{handL: true}) {
		t.Error("completing a command should claim its frame")
	}
	if !g.handleCmdInput(cmdInput{verbKey: true}) || !g.handleCmdInput(cmdInput{cancel: true}) {
		t.Error("canceling a command should claim its frame")
	}
}

// j followed by a key that isn't [ or ] settles the command as Invalid with
// no mechanical effect: no swing, no HR cost, nothing removed.
func TestAttackCommandInvalidKey(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 1
	m := NewMonster(11, 5, spiderStats())
	g.level.Monsters = []*Monster{m}

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{other: true})

	want := logEntry{segments: []logSeg{
		{text: "Attack ", style: cmdNormal},
		{text: "Invalid", style: cmdMiss},
	}}
	if !reflect.DeepEqual(g.inputLog, []logEntry{want}) {
		t.Fatalf("log = %v, want an Invalid outcome line", g.inputLog)
	}
	if g.cmdVerb != verbNone || g.cmdPending {
		t.Fatal("Invalid should settle the command")
	}
	if m.HP != spiderStats().MaxHP {
		t.Fatalf("monster HP = %d, want untouched %d", m.HP, spiderStats().MaxHP)
	}
	if len(g.level.Monsters) != 1 {
		t.Fatal("an invalid attack must not remove monsters")
	}
	if g.heartRate.BPM() != NewHeartRate(baseHR).BPM() {
		t.Fatal("an invalid attack must not cost HR")
	}
}

func TestAttackCommandCancelRemovesPreview(t *testing.T) {
	g := commandGame(t)

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{cancel: true})
	if g.cmdVerb != verbNone {
		t.Fatal("cancel should clear the pending verb")
	}
	if g.cmdPending || len(g.inputLog) != 0 {
		t.Fatalf("cancel must remove the preview, got pending=%v log=%v", g.cmdPending, g.inputLog)
	}

	// A stray hand key with nothing pending does nothing.
	g.handleCmdInput(cmdInput{handL: true})
	if len(g.inputLog) != 0 {
		t.Fatalf("a stray hand key must not act, log = %v", g.inputLog)
	}
}

// A pending command is never discarded on its own: it must persist through
// arbitrary idle frames and still complete with a hand key much later.
func TestAttackVerbStaysPendingUntilResolved(t *testing.T) {
	g := commandGame(t)

	g.handleCmdInput(cmdInput{verbKey: true})
	if g.cmdVerb != verbAttack {
		t.Fatal("a j press should arm the attack verb")
	}

	// Many frames of no command input must not dismiss the preview.
	for i := 0; i < 1000; i++ {
		g.handleCmdInput(cmdInput{})
	}
	if g.cmdVerb != verbAttack || !g.cmdPending {
		t.Fatal("idle frames must not auto-cancel a pending command")
	}
	if !reflect.DeepEqual(g.inputLog, []logEntry{logNormal("Attack ?")}) {
		t.Fatalf("log = %v, want the preview to persist", g.inputLog)
	}

	// A hand key pressed long after arming still completes the command.
	g.player.Weapon.ToHit = 1
	g.level.Monsters = []*Monster{NewMonster(11, 5, spiderStats())}
	g.handleCmdInput(cmdInput{handL: true})
	if !reflect.DeepEqual(g.inputLog, []logEntry{attackLine(handLeft, "Hit Spider!", cmdHit)}) {
		t.Fatalf("log = %v, want the resolve line", g.inputLog)
	}
}

func TestPendingCommandStaysOnTopWhileOtherLogsAccumulate(t *testing.T) {
	g := commandGame(t)
	g.player.Weapon.ToHit = 1
	g.level.Monsters = []*Monster{NewMonster(11, 5, spiderStats())}

	g.handleCmdInput(cmdInput{verbKey: true})
	g.pushLog(logNormal("Move Forward"))
	g.pushLog(logNormal("Turn Right"))

	// The preview must stay the newest line; other actions slot beneath it
	// without committing it.
	want := []logEntry{logNormal("Attack ?"), logNormal("Turn Right"), logNormal("Move Forward")}
	if !reflect.DeepEqual(g.inputLog, want) {
		t.Fatalf("log = %v, want %v", g.inputLog, want)
	}
	if !g.cmdPending {
		t.Fatal("the preview should still be pending")
	}

	g.handleCmdInput(cmdInput{handL: true})
	want = []logEntry{attackLine(handLeft, "Hit Spider!", cmdHit), logNormal("Turn Right"), logNormal("Move Forward")}
	if !reflect.DeepEqual(g.inputLog, want) {
		t.Fatalf("log = %v, want %v", g.inputLog, want)
	}
	if g.cmdPending {
		t.Fatal("resolving should commit the command")
	}
}

func TestRepeatedVerbDoesNotDuplicatePreview(t *testing.T) {
	g := commandGame(t)

	g.handleCmdInput(cmdInput{verbKey: true})
	g.handleCmdInput(cmdInput{verbKey: true})
	if len(g.inputLog) != 1 {
		t.Fatalf("log = %v, want exactly one preview line", g.inputLog)
	}
	if !g.cmdPending || g.inputLog[0].segments[0].text != "Attack ?" {
		t.Fatalf("preview line = %v (pending=%v), want Attack ?", g.inputLog[0], g.cmdPending)
	}
}

// The attack line splits into a neutral command run and a styled outcome run,
// and the outcome dims as it scrolls into history.
func TestAttackOutcomeRunColors(t *testing.T) {
	line := attackLine(handLeft, "Hit Spider!", cmdHit)
	if len(line.segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(line.segments))
	}
	if line.segments[0].text != "Attack Left - " || line.segments[0].style != cmdNormal {
		t.Fatalf("command run = %+v, want neutral 'Attack Left - '", line.segments[0])
	}
	if line.segments[1].style != cmdHit {
		t.Fatalf("outcome style = %d, want cmdHit", line.segments[1].style)
	}

	if textColor(cmdHit, true) != infoHitBright {
		t.Error("a newest hit should be bright green")
	}
	if textColor(cmdHit, false) != infoHitDim {
		t.Error("an older hit should dim toward dull green")
	}
	if textColor(cmdMiss, true) != infoMissBright {
		t.Error("a newest miss should be bright red")
	}
	if textColor(cmdMiss, false) != infoMissDim {
		t.Error("an older miss should dim toward dull red")
	}
	if textColor(cmdNormal, true) != infoLatest {
		t.Error("a newest neutral run should be the bright default")
	}
	if textColor(cmdNormal, false) != infoText {
		t.Error("an older neutral run should be the gray history color")
	}
}

// A wall bump splits into a neutral movement run and a red !OUCH! run that
// dims as it scrolls into history, matching the attack outcome styling.
func TestBumpEntry(t *testing.T) {
	e := bumpEntry("Move Forward")
	if len(e.segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(e.segments))
	}
	if e.segments[0].text != "Move Forward " || e.segments[0].style != cmdNormal {
		t.Fatalf("command run = %+v, want neutral 'Move Forward '", e.segments[0])
	}
	if e.segments[1].text != "!OUCH!" || e.segments[1].style != cmdMiss {
		t.Fatalf("outcome run = %+v, want red !OUCH!", e.segments[1])
	}
	if textColor(cmdMiss, true) != infoMissBright {
		t.Error("a newest !OUCH! should be bright red")
	}
	if textColor(cmdMiss, false) != infoMissDim {
		t.Error("an older !OUCH! should dim toward dull red")
	}
}

// Passing out logs a red !PASSING OUT! on the way down, a neutral COMING TO
// on the way up, and nothing while the state is steady.
func TestFaintLogEdges(t *testing.T) {
	if e := faintLog(false, true); e == nil {
		t.Fatal("falling unconscious should log an event")
	} else if e.segments[0].text != "!!PASSING OUT!!" || e.segments[0].style != cmdMiss {
		t.Fatalf("faint entry = %+v, want red !!PASSING OUT!!", e.segments[0])
	}
	if e := faintLog(true, false); e == nil {
		t.Fatal("waking should log an event")
	} else if e.segments[0].text != "COMING TO" || e.segments[0].style != cmdNormal {
		t.Fatalf("wake entry = %+v, want neutral COMING TO", e.segments[0])
	}
	for _, steady := range [][2]bool{{false, false}, {true, true}} {
		if e := faintLog(steady[0], steady[1]); e != nil {
			t.Fatalf("steady %v->%v must not log, got %+v", steady[0], steady[1], *e)
		}
	}
}

// FaintUpdate drives the pass-out state machine: it logs each edge exactly
// once, and passing out discards any command whose preview is waiting in the
// log.
func TestFaintUpdateLogsOnceAndCancelsPendingCommand(t *testing.T) {
	// A bare Game needs no level to exercise faintUpdate and the command log.
	g := &Game{}
	g.handleCmdInput(cmdInput{verbKey: true})
	if !g.cmdPending || len(g.inputLog) != 1 {
		t.Fatalf("preview not armed: pending=%v log=%v", g.cmdPending, g.inputLog)
	}

	g.faintUpdate(true)
	want := logStyled("!!PASSING OUT!!", cmdMiss)
	if !reflect.DeepEqual(g.inputLog, []logEntry{want}) {
		t.Fatalf("log = %v, want the faint line to replace the preview", g.inputLog)
	}
	if g.cmdPending || g.cmdVerb != verbNone {
		t.Fatal("passing out must abandon an in-progress command")
	}

	g.faintUpdate(true)
	if !reflect.DeepEqual(g.inputLog, []logEntry{want}) {
		t.Fatalf("a steady faint must not log again, got %v", g.inputLog)
	}

	g.faintUpdate(false)
	wantLog := []logEntry{logNormal("COMING TO"), logStyled("!!PASSING OUT!!", cmdMiss)}
	if !reflect.DeepEqual(g.inputLog, wantLog) {
		t.Fatalf("log = %v, want COMING TO on top of the faint line", g.inputLog)
	}

	g.faintUpdate(false)
	if !reflect.DeepEqual(g.inputLog, wantLog) {
		t.Fatalf("a steady wake must not log again, got %v", g.inputLog)
	}
}

// The blackout mask falls layer by layer at maskStep: each step covers the
// next maskRank until every dungeon square is dark, and waking removes the
// layers in mirror order at the same cadence.
func TestMaskCoversAndClears(t *testing.T) {
	g := &Game{}
	now := time.Time{}

	g.maskUpdate(true, false, now) // begin the faint
	if g.maskProgress != 0 {
		t.Fatalf("progress on the faint edge = %d, want 0", g.maskProgress)
	}
	for i := 1; i <= 4; i++ {
		now = now.Add(maskStep)
		g.maskUpdate(true, true, now)
		if g.maskProgress != i {
			t.Fatalf("progress after step %d = %d, want %d", i, g.maskProgress, i)
		}
	}
	// A steady fully-dark frame must not step past the last layer.
	g.maskUpdate(true, true, now.Add(maskStep))
	if g.maskProgress != 4 {
		t.Fatalf("progress = %d, want it to hold at 4", g.maskProgress)
	}

	g.maskUpdate(false, true, now) // begin waking
	if g.maskProgress != 4 {
		t.Fatalf("progress on the wake edge = %d, want 4", g.maskProgress)
	}
	for i := 3; i >= 0; i-- {
		now = now.Add(maskStep)
		g.maskUpdate(false, false, now)
		if g.maskProgress != i {
			t.Fatalf("progress after %d clears = %d, want %d", 4-i, g.maskProgress, i)
		}
	}
	// A steady fully-clear frame must not step below zero.
	g.maskUpdate(false, false, now.Add(maskStep))
	if g.maskProgress != 0 {
		t.Fatalf("progress = %d, want it to hold at 0", g.maskProgress)
	}
}

// maskLayerString renders the additive layers over an 8x8 tile window from
// maskRank, so it can be compared character-for-character with the tiled
// dither masks.
func maskLayerString(layer int) string {
	var b strings.Builder
	for ty := 0; ty < 8; ty++ {
		for tx := 0; tx < 8; tx++ {
			if maskRank(tx, ty) <= layer {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// maskRank must reproduce the four dither masks exactly, each the previous
// with one more parity class of dungeon squares added.
func TestMaskLayersReproduceTheDitherMasks(t *testing.T) {
	want := []string{
		"#.#.#.#.\n........\n#.#.#.#.\n........\n#.#.#.#.\n........\n#.#.#.#.\n........\n",
		"#.#.#.#.\n.#.#.#.#\n#.#.#.#.\n.#.#.#.#\n#.#.#.#.\n.#.#.#.#\n#.#.#.#.\n.#.#.#.#\n",
		"########\n.#.#.#.#\n########\n.#.#.#.#\n########\n.#.#.#.#\n########\n.#.#.#.#\n",
		"########\n########\n########\n########\n########\n########\n########\n########\n",
	}
	for i, w := range want {
		if got := maskLayerString(i + 1); got != w {
			t.Fatalf("layer %d mask renders:\n%s\nwant:\n%s", i+1, got, w)
		}
	}
}
