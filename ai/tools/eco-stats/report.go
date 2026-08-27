package ecostats

import (
	"fmt"
	"io"

	"kk-flavor/tools/shell"
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
	return fmt.Sprintf("  (+ %d uncounted import(s): %s)", len(s.uncounted), shell.UncountedNames(s.uncounted))
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
	fmt.Fprintf(out, "always-loaded:%6d words  = %d router + %d descriptions across %d of %d skills%s\n",
		s.alwaysLoaded(), s.alwaysLoadedWords, s.descriptionWords, s.routedSkills, s.skills, s.budgetNote())
}
