// What a run says as it goes and what it says at the end: the unit table, one unit's key material, a
// unit's own line, the closing tally and the tail of a failure's output. Apart from run.go because
// that file decides what happens and this one only reports it — and because the exit status is
// decided here, at the bottom of reportRun, where the counts it reads are in view.
package gate

import (
	"fmt"
	"strings"
	"time"
)

func (g *gate) printUnits() int {
	fmt.Fprintf(g.out, "%-34s %-9s %-8s %s\n", "UNIT", "KIND", "STATE", "INPUTS")
	for _, u := range g.units {
		key, _ := g.keyMaterial(u)
		state := "stale"
		if g.hasRecord(u, key) {
			state = "fresh"
		}
		fmt.Fprintf(g.out, "%-34s %-9s %-8s %s\n", u.id, u.kind, state, strings.Join(u.inputs, " "))
	}
	return 0
}

func (g *gate) printWhy(want string) int {
	for _, u := range g.units {
		if u.id != want {
			continue
		}
		key, lines := g.keyMaterial(u)
		fmt.Fprintf(g.out, "%s  (%s)\n", u.id, u.kind)
		fmt.Fprintf(g.out, "  command: %s\n", u.cmd)
		fmt.Fprintf(g.out, "  key:     %s\n", key)
		fmt.Fprintln(g.out, "  inputs:")
		for _, line := range lines {
			fmt.Fprintf(g.out, "    %s\n", line)
		}
		return 0
	}
	return g.fail("no unit is called '%s' — run --units for the list", want)
}

func (g *gate) unitLine(state, id, detail string) {
	fmt.Fprintf(g.out, "  %-11s %-32s %s\n", state, id, detail)
}

func (g *gate) reportRun(started time.Time, deferredIDs []string, tally runTally) int {
	if len(deferredIDs) > 0 {
		fmt.Fprintln(g.out, "\nDEFERRED — these have inputs that moved, and the fast path did not run them:")
		for _, id := range deferredIDs {
			fmt.Fprintf(g.out, "    %s\n", id)
		}
		fmt.Fprintln(g.out, "  Mutation is a statement about whether the suites can fail, not about this change, and on this")
		fmt.Fprintln(g.out, "  machine it costs minutes per script. CI runs the full sweep on every push. To settle them here:")
		fmt.Fprintln(g.out, "      ai/gate.sh --mutants")
	}

	fmt.Fprintf(g.out, "\n%d unit(s): %d ran, %d fresh from cache, %d deferred, %d failed, %d that never measured, %d with no inputs, %ds wall clock\n",
		len(g.units), tally.ran, tally.fresh, tally.deferred, tally.failed, tally.unmeasured, tally.empty,
		int(time.Since(started).Round(time.Second).Seconds()))

	if tally.empty > 0 {
		fmt.Fprintf(g.errOut, "%d unit(s) resolved to no input file — the gate narrowed itself and cannot report on them. Exit 2, and this is not a pass.\n", tally.empty)
		return 2
	}
	if tally.ran == 0 && tally.fresh == 0 {
		fmt.Fprintln(g.errOut, "nothing was measured and nothing was answered from cache — exit 2, and this is not a pass.")
		return 2
	}
	// A finding about the code outranks one about the machine. Exit 2 alone means nothing was found
	// wrong and something never ran, which a caller may never read as a pass.
	if tally.failed > 0 {
		return 1
	}
	if tally.unmeasured > 0 {
		fmt.Fprintf(g.errOut, "%d %s — nothing is known about them, and this is not a pass.\n", tally.unmeasured, didNotMeasureLine)
		return 2
	}
	return 0
}

func (g *gate) tail(output string, n int, indent string) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, line := range lines {
		fmt.Fprintf(g.out, "%s%s\n", indent, line)
	}
}
