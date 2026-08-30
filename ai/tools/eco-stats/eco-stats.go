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
// What it once held byte-identical with check.sh through a shared region now lives in
// kk-flavor/tools/shell and kk-flavor/tools/eco-root, one copy for both tools. A change here needs a
// case in stats_test.go beside it, and a mutation in go-mutate is what shows that case can fail.
// `stats.sh` in kk-reduce's scripts/ is the stub that reaches this binary.
package ecostats

import (
	"fmt"
	"io"
	"strings"

	ecoroot "kk-flavor/tools/eco-root"
	"kk-flavor/tools/shell"
)

// The most a row's note takes. A note is read by the next pass off a table cell, not by a human
// reading a report, so the bar is on what survives being read that way.
const noteWordCap = 40

// How many budget-file refusals are named on stderr before the rest are summarised. Named where the
// relation to the suppression note is visible: written as a bare 5 and 6, a change to one silently
// outruns the other and the note never prints.
const budgetRefusalCap = 5

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
	resolved, ok := ecoroot.New(root)
	if !ok {
		fmt.Fprintln(errOut, "stats.sh: no root holding both kk-flavor/ and skills/")
		fmt.Fprintln(errOut, "stats.sh: exit 2 — nothing was measured. Fix the invocation; do not read this as no change.")
		return 2
	}

	s := &stats{root: resolved}
	s.measure(errOut)

	if s.prose <= 0 {
		fmt.Fprintf(errOut, "stats.sh: measured 0 words of prose under %s — the scan did not work\n", s.root.Named())
		return 2
	}
	// Reported before the refusals below, not after them: prose, scripts, the ledger and skills all
	// measured, and withholding four sound figures to protect the one short figure leaves the caller
	// nothing to read at all. Each short figure states its own shortfall on the line that carries it.
	s.report(out)

	// A path the run could not read at all — a directory it could not list, a file it could not open.
	// Wider than the budget refusal below, which is one tier short: this shortens prose, scripts and the
	// census alike, and nothing here can say by how much. One `chmod 000` on `kk-flavor/standards/`
	// moved prose from 32714 words to 20633 and the always-loaded tier from 1695 to 962, printed both
	// at full confidence and exited 0 — and every delta a later pass reads is taken off these rows.
	if s.unreadable != 0 {
		fmt.Fprintf(errOut, "stats.sh: %d path(s) %s could not be read above — exit 2, the figures are short by an unknown amount and no row was appended. Do not read this as no change.\n",
			s.unreadable, s.unreadableWhere())
		return 2
	}

	// The refusals were named above as they happened; this is the one place their count decides
	// anything. No row is appended, because the always-loaded figure is short by an amount nothing
	// here can state — what a refusal withholds is the record, not the reading.
	if s.budgetRefusals != 0 {
		fmt.Fprintf(errOut, "stats.sh: %d budget file(s) refused above — exit 2, the always-loaded figure is short by an unknown amount and no row was appended.\n",
			s.budgetRefusals)
		return 2
	}

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
//
// Every control byte goes, not only the two that forge rows. The note is committed, read back by a
// later pass, and printed to a terminal on the way past, so an ESC left in it edits the display of
// everyone who later reads the ledger. shell.Oneline is the guard the reported names already get:
// a byte barred from a message has no claim to the record.
func sanitiseNote(note string) string {
	note = shell.Oneline(note)
	note = strings.ReplaceAll(note, `\`, `\\`)
	return strings.ReplaceAll(note, "|", `\|`)
}
