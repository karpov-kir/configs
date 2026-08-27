// Package ecostats is the ecosystem size ledger: the numbers that decide whether a reduction pass is
// worth running, measured rather than estimated — prose words, script words, the always-loaded tier
// and the skill count. With --append it writes one dated row of them into kk-reduce's stats.md,
// seeding that file with its own header when there is none.
//
// It is a library with a thin command beside it, for the reason ecocheck is one: the suite that
// proves it drives it once per case, and a process spawn per case is the cost that makes a mutation
// run take hours. Nothing here writes to os.Stdout or calls os.Exit — Run reports through the writers
// it is handed and returns the code the command exits on — and nothing here holds state between
// calls, so two runs in one process cannot see each other's refusal counters.
//
// It exits 0 on success and 2 when it could not measure, and never anything between: a measurement
// that did not run is not a zero, and a figure known to be short must not reach the ledger, because
// every delta a later pass reads is taken off the rows below it.
//
// The shared regions it holds byte-identical with check.sh live in kk-flavor/tools/shell, one copy
// for both ports. A change here needs a case in `~/.claude/skills/kk-reduce/scripts/stats-test.sh`
// and its twin in stats_test.go beside it, and a mutation in gomutate is what shows that case can
// fail.
package ecostats

import (
	"fmt"
	"io"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// The most a row's note takes. A note is read by the next pass off a table cell, not by a human
// reading a report, so the bar is on what survives being read that way.
const noteWordCap = 40

// Run measures the tree under root and writes the report to out. args is the command line without
// its program name; self is the program name itself, which is where the ledger is looked for. An
// empty root means the two candidates the shell version tried, in order.
func Run(self string, args []string, out, errOut io.Writer) int {
	note, rest, ok := noteFrom(args, errOut)
	if !ok {
		return 2
	}
	root := ""
	if len(rest) > 0 {
		root = rest[0]
	}
	root, ok = resolveRoot(root)
	if !ok {
		fmt.Fprintln(errOut, "stats.sh: no root holding both kk-flavor/ and skills/")
		fmt.Fprintln(errOut, "stats.sh: exit 2 — nothing was measured. Fix the invocation; do not read this as no change.")
		return 2
	}

	s := &stats{root: root, rootCanon: shell.CanonicalDir(root), home: os.Getenv("HOME")}
	s.measure(errOut)

	if s.prose <= 0 {
		fmt.Fprintf(errOut, "stats.sh: measured 0 words of prose under %s — the scan did not work\n", root)
		return 2
	}
	// The refusals were named above as they happened; this is the one place their count decides
	// anything. Nothing is printed and no row is appended, because the always-loaded figure is short
	// by an amount nothing here can state.
	if s.budgetRefusals != 0 {
		fmt.Fprintf(errOut, "stats.sh: %d budget file(s) refused above — exit 2, the always-loaded figure is short by an unknown amount and no row was appended.\n",
			s.budgetRefusals)
		return 2
	}

	s.report(out)
	if note == "" {
		return 0
	}
	return s.appendRow(self, note, out, errOut)
}

// The shell's `case "${1:-}" in --append) …`: the note is one argument, so an unquoted one leaves its
// first word as the note and its second read as the root. An absent or empty note is the default,
// never an empty cell.
func noteFrom(args []string, errOut io.Writer) (note string, rest []string, ok bool) {
	if len(args) == 0 || args[0] != "--append" {
		return "", args, true
	}
	args = args[1:]
	note = "upkeep"
	if len(args) > 0 {
		if args[0] != "" {
			note = args[0]
		}
		args = args[1:]
	}
	note = sanitiseNote(note)
	words := len(shell.SplitFields(note))
	if words > noteWordCap {
		fmt.Fprintf(errOut, "stats.sh: the note is %d words; %d is the most a row takes. Nothing was appended.\n", words, noteWordCap)
		fmt.Fprintln(errOut, "stats.sh: keep what a later pass must act on — what ran, and what is open. The reasoning belongs in your reply to the human, which is read once.")
		return "", args, false
	}
	return note, args, true
}

// The note is stats.md's last table cell: a newline in it forges a whole row, a bare `|` forges extra
// columns, and a later pass reads either as a measurement. Backslashes before pipes, never the other
// way round, or the escape added here is escaped in turn and the pipe comes back through.
func sanitiseNote(note string) string {
	note = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, note)
	note = strings.ReplaceAll(note, `\`, `\\`)
	return strings.ReplaceAll(note, "|", `\|`)
}

// Resolved the way check.sh resolves it, so both tools always describe the same tree.
func resolveRoot(root string) (string, bool) {
	if root == "" {
		for _, candidate := range []string{".", "./ai"} {
			if shell.IsDir(shell.Join(candidate, "kk-flavor")) && shell.IsDir(shell.Join(candidate, "skills")) {
				root = candidate
				break
			}
		}
	}
	if root == "" || !shell.IsDir(shell.Join(root, "kk-flavor")) || !shell.IsDir(shell.Join(root, "skills")) {
		return "", false
	}
	return root, true
}
