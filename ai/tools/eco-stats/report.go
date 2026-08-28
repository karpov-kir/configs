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
	if s.outsideSkills != 0 {
		fmt.Fprintf(out, "mounted outside:%4d words  (%d skill(s) this tree cannot shrink; bundled and plugin skills are unmeasurable)\n",
			s.outsideWords, s.outsideSkills)
	}
	fmt.Fprintf(out, "always-loaded:%6d words  = %d router + %d descriptions across %d of %d skills%s%s\n",
		s.alwaysLoaded(), s.alwaysLoadedWords, s.descriptionWords, s.routedSkills, s.skills, s.budgetNote(), s.refusalNote())
}
