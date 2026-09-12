package main

import "strings"

// handSide names the two equipment hands. The verb system targets a hand
// with the bracket keys: [ is left, ] is right.
type handSide int

const (
	handLeft handSide = iota
	handRight
)

func (h handSide) String() string {
	if h == handRight {
		return "Right"
	}
	return "Left"
}

// verb is a command waiting for its hand selector.
type verb int

const (
	verbNone verb = iota
	verbAttack
)

// cmdInput is one frame of raw command-key state, gathered from inpututil so
// the parser logic itself can be unit-tested without a real keyboard.
type cmdInput struct {
	verbKey bool // j: begin a verb sequence
	handL   bool // [ : target the left hand
	handR   bool // ] : target the right hand
	cancel  bool // Space or Esc: abandon a pending verb
	other   bool // any key other than the command keys was just pressed
}

// handleCmdInput feeds one frame through the verb+hand parser and reports
// whether the frame was claimed by the command system. A verb press puts its
// preview line (e.g. "Attack ?") at the top of the input log without
// committing it; a hand press resolves it in place; a stray key settles it as
// Invalid. The command stays pending until the player completes or cancels it
// — there is no timeout. While a command is armed every frame is claimed: all
// other input (movement, turns) is ignored, so input is atomic — it either
// forms a valid command or is invalid, never partially executed.
func (g *Game) handleCmdInput(in cmdInput) bool {
	if in.cancel {
		if g.cmdVerb != verbNone {
			if g.cmdPending {
				g.cancelCommand()
			}
			g.cmdVerb = verbNone
			return true
		}
		return false
	}
	if g.cmdVerb == verbNone {
		if in.verbKey {
			g.cmdVerb = verbAttack
			g.armCommand(logNormal("Attack ?"))
			return true
		}
		return false
	}
	switch {
	case in.handL:
		g.finishAttack(handLeft)
		g.cmdVerb = verbNone
	case in.handR:
		g.finishAttack(handRight)
		g.cmdVerb = verbNone
	case in.other:
		g.finalizeCommand(logEntry{segments: []logSeg{
			{text: "Attack ", style: cmdNormal},
			{text: "Invalid", style: cmdMiss},
		}})
		g.cmdVerb = verbNone
	}
	return true
}

// finishAttack swings the equipped weapon with the given hand, resolves the
// pending preview into the command plus its outcome, applies the HR stress,
// and clears any corpse. The outcome run is tinted by result: green hits,
// red whiffs; the command itself stays neutral.
func (g *Game) finishAttack(side handSide) {
	res := g.player.Attack(g.level, side)

	var outcome string
	var style cmdStyle
	switch {
	case res.Killed != nil:
		outcome, style = "KILL "+strings.ToUpper(res.Target), cmdHit
	case res.Target == "":
		outcome, style = "! MISS !", cmdMiss
	case res.Hit:
		outcome, style = "Hit "+res.Target+"!", cmdHit
	default:
		outcome, style = "Miss "+res.Target+"!", cmdMiss
	}
	g.finalizeCommand(logEntry{segments: []logSeg{
		{text: "Attack " + side.String() + " - ", style: cmdNormal},
		{text: outcome, style: style},
	}})

	if res.Cost > 0 {
		g.heartRate.Bump(res.Cost)
	}
	if res.Killed != nil {
		g.level.RemoveMonster(res.Killed)
	}
}

// armCommand puts a command preview on top of the input log, replacing one
// already pending. The preview is not committed yet: resolveCommand turns it
// into a real history line, cancelCommand removes it entirely.
func (g *Game) armCommand(preview logEntry) {
	if g.cmdPending {
		g.inputLog[0] = preview
		return
	}
	if len(g.inputLog) >= infoContentRows {
		g.inputLog = g.inputLog[:infoContentRows-1]
	}
	g.inputLog = append([]logEntry{preview}, g.inputLog...)
	g.cmdPending = true
}

// finalizeCommand commits the pending preview line with its final text.
func (g *Game) finalizeCommand(final logEntry) {
	if len(g.inputLog) > 0 {
		g.inputLog[0] = final
	}
	g.cmdPending = false
}

// cancelCommand discards the pending preview entirely.
func (g *Game) cancelCommand() {
	if len(g.inputLog) > 0 {
		g.inputLog = g.inputLog[1:]
	}
	g.cmdPending = false
}
