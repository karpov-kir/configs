package ecostats

import (
	"fmt"
	"io"

	ecoroot "kk-flavor/tools/eco-root"
)

func (s *stats) alwaysLoaded() int { return s.alwaysLoadedWords + s.descriptionWords }

// A `+` on the always-loaded figure marks it a lower bound: a figure that silently excludes an import
// teaches a later pass the tier held still while it grew. The mark is what the row carries; the names
// behind it go to the printed line only, where there is room for them.
func (s *stats) budgetMark() string {
	if len(s.uncounted) == 0 {
		return ""
	}
	return "+"
}

func (s *stats) budgetNote() string {
	if len(s.uncounted) == 0 {
		return ""
	}
	return fmt.Sprintf("  (+ %d uncounted import(s): %s)", len(s.uncounted), ecoroot.UncountedNames(s.uncounted))
}

// A refused budget file is a member of the tier that was never read, so this figure is not the lower
// bound a `+` marks — it is short by an amount nothing here can state. It is marked apart from `+`
// for exactly that reason: a reader who takes the two the same way reads "at least this much" off a
// figure carrying no such floor. The printed line only, because a run with a refusal appends no row
// for a mark to sit on.
func (s *stats) refusalNote() string {
	if s.budgetRefusals == 0 {
		return ""
	}
	return fmt.Sprintf("  (SHORT: %d budget file(s) refused and not counted — not a measurement)", s.budgetRefusals)
}

func (s *stats) report(out io.Writer) {
	fmt.Fprintf(out, "prose:        %6d words (%d .md files, ledger excluded)\n", s.prose, s.proseFiles)
	fmt.Fprintf(out, "scripts:      %6d words\n", s.scripts)
	fmt.Fprintf(out, "ledger:       %6d words  (stats.md — a record, not instructions; step 1 reads it in full, so it costs context like the always-loaded tier)\n",
		s.ledgerWords)
	s.reportMountedOutside(out)
	fmt.Fprintf(out, "always-loaded:%6d words  = %d router + %d descriptions across %d of %d skills%s%s\n",
		s.alwaysLoaded(), s.alwaysLoadedWords, s.descriptionWords, s.routedSkills, s.skills, s.budgetNote(), s.refusalNote())
	s.reportUnreadable(out)
}

// A run that skipped the scan and a tree with nothing mounted from outside printed the same nothing,
// and a reader takes the absence for the second. Outside the install this is not a zero, it is no
// measurement — budget.go → mountedOutside is where it stops and why. With that line printing, no line
// at all means the scan ran and found none.
func (s *stats) reportMountedOutside(out io.Writer) {
	if !s.outsideMeasured {
		fmt.Fprintf(out, "mounted outside:  not measured  (no scan ran here: this checkout is not the install, or its skills mount would not list)\n")
		return
	}
	if s.outsideSkills == 0 {
		return
	}
	fmt.Fprintf(out, "mounted outside:%4d words  (%d skill(s) this tree cannot shrink; bundled and plugin skills are unmeasurable)\n",
		s.outsideWords, s.outsideSkills)
}

// A path that could not be read shortens whichever figures counted over it, and which ones those are
// is not knowable from here — a directory that would not list takes its subtree out of prose, scripts
// and the census together. So the note sits under the whole report rather than on one line, and it
// says the figures are not measurements: the exit code carries the same fact to a caller that reads
// status, and this carries it to the one reading stdout.
func (s *stats) reportUnreadable(out io.Writer) {
	if s.unreadable == 0 {
		return
	}
	fmt.Fprintf(out, "SHORT: %d path(s) %s could not be read — every figure above counts over what was reachable, not over the tree. Not a measurement.\n",
		s.unreadable, s.unreadableWhere())
}
