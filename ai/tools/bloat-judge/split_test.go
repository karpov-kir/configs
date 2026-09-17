// Cases for turning a text into units, and for applying a verdict back onto it.
package bloatjudge

import (
	"strings"
	"testing"
)

func TestSplitSourceOffersWholeBlocks(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	units, view := Split(lines, commentBlocks(lines), all)
	if len(units) != 3 {
		t.Fatalf("got %d units, want 3 (header block, on a(), trailing)", len(units))
	}
	if units[0].Line != 1 || units[0].Span != 2 {
		t.Fatalf("the header block is line %d span %d, want 1 span 2", units[0].Line, units[0].Span)
	}
	if !strings.Contains(view, "   1| // file header\n   .| // second line\n") {
		t.Fatalf("the view does not mark the continuation line:\n%s", view)
	}
	if !strings.Contains(view, "    | *ptr = 1\n") {
		t.Fatalf("a dereference was offered as a unit:\n%s", view)
	}
}

func TestSplitProseHoldsAFenceAsOneUnit(t *testing.T) {
	lines := []string{"para", "```xml", "<a/>", "<b/>", "```", "", "after"}
	units, view := Split(lines, proseBlocks(lines), all)
	if len(units) != 3 {
		t.Fatalf("got %d units, want 3 (para, fence, after)", len(units))
	}
	if units[1].Line != 2 || units[1].Span != 4 {
		t.Fatalf("the fence is line %d span %d, want 2 span 4", units[1].Line, units[1].Span)
	}
	if strings.Count(view, "   .| ") != 3 {
		t.Fatalf("the fence body is not marked as continuation:\n%s", view)
	}
}

func TestApplyDeletesTheWholeSpanAndKeepsTheTrailingNewline(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	units, _ := Split(lines, commentBlocks(lines), all)
	got := Apply(lines, units, []int{1})
	want := "\nfunc a() {}\n// on a()\n*ptr = 1\n// trailing\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A `/*` block whose continuation lines carry no leading `*` is still one comment. Split as one unit per
// comment-looking line, deleting it took the first line alone and left `kept for history. */` to break
// the file.
func TestSplitSourceHoldsABlockCommentWhole(t *testing.T) {
	lines := []string{"/* Legacy block comment", "   kept for history. */", "code()", "/* one-liner */", "code()", "// after"}
	units, view := Split(lines, commentBlocks(lines), all)
	if len(units) != 3 || units[0].Span != 2 || units[1].Span != 1 || units[2].Span != 1 {
		t.Fatalf("got %d units with spans %v, want 3 with spans 2, 1, 1", len(units), spansOf(units))
	}
	if !strings.Contains(view, "   1| /* Legacy block comment\n   .|    kept for history. */\n") {
		t.Fatalf("the closing line is not marked as the block's continuation:\n%s", view)
	}
}

func TestSplitProseHoldsAWrappedParagraphAsOneUnit(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(wrapped, "\n"), "\n")
	units, view := Split(lines, proseBlocks(lines), all)
	if len(units) != 3 {
		t.Fatalf("got %d units, want 3 (subject, body, trailer)", len(units))
	}
	if units[1].Line != 3 || units[1].Span != 3 {
		t.Fatalf("the body is line %d span %d, want 3 span 3", units[1].Line, units[1].Span)
	}
	if !strings.Contains(view, "   2| A pass measured") || !strings.Contains(view, "   .| result up as") {
		t.Fatalf("the body's wrapped lines are not marked as one unit:\n%s", view)
	}
}

func TestSplitProseKeepsEachListItemItsOwnUnit(t *testing.T) {
	lines := []string{"Lead-in text", "- first item that wraps", "  onto a second line", "- second item", "1. numbered", "2. also numbered", "# heading", "| a | b |"}
	units, _ := Split(lines, proseBlocks(lines), all)
	want := []Unit{{Line: 1, Span: 1}, {Line: 2, Span: 2}, {Line: 4, Span: 1}, {Line: 5, Span: 1}, {Line: 6, Span: 1}, {Line: 7, Span: 1}, {Line: 8, Span: 1}}
	if len(units) != len(want) {
		t.Fatalf("got %d units, want %d: %v", len(units), len(want), units)
	}
	for i, unit := range units {
		if unit != want[i] {
			t.Fatalf("unit %d is %+v, want %+v", i+1, unit, want[i])
		}
	}
}

// A paragraph opening in bold is prose, not a list: `**Bold**` and `--- so` both start with a marker
// character and neither carries the space that makes one.
func TestSplitProseDoesNotReadBoldOrADashAsAListItem(t *testing.T) {
	lines := []string{"**Bold** opens this", "--- and this continues it", "*emphasis* too"}
	units, _ := Split(lines, proseBlocks(lines), all)
	if len(units) != 1 || units[0].Span != 3 {
		t.Fatalf("got %v, want one unit spanning 3 lines", units)
	}
}

// Git's own rule, and the reason for it: with no block above it, the last block is the subject line,
// and a subject shaped like `Fix: the thing` is the message rather than metadata about it.
func TestASubjectLineShapedLikeATrailerIsStillJudged(t *testing.T) {
	if withheld := trailerLines([]string{"Fix: the thing"}); withheld != nil {
		t.Fatalf("a lone subject was withheld as a trailer: %v", withheld)
	}
	if withheld := trailerLines([]string{"Subject", "", "Body text"}); withheld != nil {
		t.Fatalf("an ordinary closing paragraph was withheld as a trailer: %v", withheld)
	}
	withheld := trailerLines([]string{"Subject", "", "Signed-off-by: A <a@b>", "Co-Authored-By: C <c@d>"})
	if len(withheld) != 2 || !withheld[3] || !withheld[4] {
		t.Fatalf("the trailer block is %v, want lines 3 and 4", withheld)
	}
}

// Git writes lines into the trailer block that are not `Token: value`, and the block has to survive
// them: all-or-nothing, a cherry-pick note hands the sign-off and the co-author line back to a vote
// told to delete provenance.
func TestATrailerBlockSurvivesTheLinesGitPutsInIt(t *testing.T) {
	for name, block := range map[string][]string{
		"a cherry-pick note": {"Signed-off-by: A <a@b>", "(cherry picked from commit deadbee)"},
		"a bare issue ref":   {"Co-Authored-By: C <c@d>", "Fixes #123"},
		"a folded value":     {"Co-Authored-By: C", "  <c@d>"},
	} {
		t.Run(name, func(t *testing.T) {
			withheld := trailerLines(append([]string{"Subject", "", "Body.", ""}, block...))
			if len(withheld) != len(block) || !withheld[5] || !withheld[6] {
				t.Fatalf("withheld %v, want lines 5 and 6", withheld)
			}
		})
	}
	// Still nothing to withhold where no line in the block is a trailer at all.
	if withheld := trailerLines([]string{"Subject", "", "Body.", "", "Closing thought.", "Another line."}); withheld != nil {
		t.Fatalf("an ordinary closing paragraph was withheld: %v", withheld)
	}
}

// The shape of a message piped from `git log`, which ends in blank lines.
func TestTrailingBlanksDoNotHideTheTrailerBlock(t *testing.T) {
	withheld := trailerLines([]string{"Subject", "", "Body.", "", "Co-Authored-By: C <c@d>", "", ""})
	if len(withheld) != 1 || !withheld[5] {
		t.Fatalf("the trailer block is %v, want line 5 alone", withheld)
	}
}

func TestOpensBlockTakesAMarkerOnlyWithTheSpaceAfterIt(t *testing.T) {
	// Inside a paragraph, which is where a stray marker does the damage.
	for line, want := range map[string]bool{
		"# heading": true, "#no-space": true, "> quoted": true, "| a | b |": true,
		"- item": true, "* item": true, "+ item": true, "1. item": true, "1) item": true,
		"-\titem": true, "1.\titem": true,
		"**Bold** opens a paragraph": false, "--- a comparison": false, "*emphasis*": false,
		"-": false, "12": false, "12.": false, "0": false, "2026-09-16 was the date": false,
		"plain continuation": false, "": false,
	} {
		if got := opensBlock(line, false); got != want {
			t.Errorf("opensBlock(%q, inList=false) = %v, want %v", line, got, want)
		}
	}
}

// CommonMark lets an ordered list interrupt a paragraph only where it numbers from one. Without that,
// a commit message wrapping onto a line like `163. Three wordings were tried` is split mid-sentence
// by the very rule that exists to stop that — this repo's own commit a1eb712f is where it was found.
func TestAnOrderedMarkerSplitsAParagraphOnlyAtOneOrInsideAList(t *testing.T) {
	for _, line := range []string{"163. Three wordings were tried", "2) and then"} {
		if opensBlock(line, false) {
			t.Errorf("%q opened a block mid-paragraph, cutting the sentence it belongs to", line)
		}
		if !opensBlock(line, true) {
			t.Errorf("%q did not open its own item inside a list", line)
		}
	}
	lines := []string{"A pass measured the range while HEAD was somewhere else, then wrote it up as", "163. Three wordings were tried and the shortest won.", "", "1. first", "2. second"}
	units, _ := Split(lines, proseBlocks(lines), all)
	want := []Unit{{Line: 1, Span: 2}, {Line: 4, Span: 1}, {Line: 5, Span: 1}}
	if len(units) != len(want) {
		t.Fatalf("got %v, want %v", units, want)
	}
	for i, unit := range units {
		if unit != want[i] {
			t.Fatalf("unit %d is %+v, want %+v", i+1, unit, want[i])
		}
	}
}

// A bullet that wraps is the same half-sentence hazard one context over: an ordered marker after a
// bullet item begins a fresh ordered list, whose first item must be numbered 1 to interrupt anything.
func TestAWrappedBulletIsNotSplitByANumberOnItsNextLine(t *testing.T) {
	lines := []string{"- a sentence that wraps onto", "163. Three wordings were tried", "and keeps going"}
	units, _ := Split(lines, proseBlocks(lines), all)
	if len(units) != 1 || units[0] != (Unit{Line: 1, Span: 3}) {
		t.Fatalf("got %v, want one unit spanning all three lines", units)
	}
	// What must keep working: a real ordered list still numbers past one, and a bullet list still
	// gives every bullet its own unit.
	for name, lines := range map[string][]string{
		"an ordered list":            {"1. first", "2. second", "3. third"},
		"an ordered list that wraps": {"1. first", "   onto a second line", "2. second"},
		"a bullet list":              {"- a", "- b"},
		"a bullet then an ordered":   {"- a", "1. b"},
	} {
		units, _ := Split(lines, proseBlocks(lines), all)
		if len(units) < 2 {
			t.Errorf("%s collapsed into %v", name, units)
		}
	}
}

// A heading takes nothing with it, so deleting the paragraph under one leaves the heading standing.
// A quote does wrap, by markdown's own lazy continuation.
func TestAHeadingAndATableRowDoNotSwallowTheLineBelow(t *testing.T) {
	for name, lines := range map[string][]string{
		"a heading":   {"# Heading", "Some prose here.", "More prose."},
		"a table row": {"| a | b |", "Some prose here.", "More prose."},
	} {
		units, _ := Split(lines, proseBlocks(lines), all)
		if len(units) != 2 || units[0].Span != 1 || units[1] != (Unit{Line: 2, Span: 2}) {
			t.Errorf("%s gave %v, want it alone then the paragraph below it", name, units)
		}
	}
	lines := []string{"> quoted", "lazy continuation"}
	units, _ := Split(lines, proseBlocks(lines), all)
	if len(units) != 1 || units[0].Span != 2 {
		t.Fatalf("a quote's lazy continuation gave %v, want one unit spanning both", units)
	}
}

// Git's own shape for a subject-only commit. Withholding the one block there is would leave nothing
// to judge, and the run would report clean over text no roll ever read.
func TestASubjectOnlyMessageIsStillJudged(t *testing.T) {
	if withheld := subjectLines([]string{"Fix the thing"}); withheld != nil {
		t.Fatalf("a subject-only message withheld %v, leaving nothing to judge", withheld)
	}
	if withheld := subjectLines([]string{"Subject", "", "Body."}); len(withheld) != 1 || !withheld[1] {
		t.Fatalf("the subject block is %v, want line 1", withheld)
	}
}

// A unit is a thing the model may delete, and Apply drops every line of one. A script's interpreter
// directive reads as a `#` comment, so it would join the header below it — and a vote against that
// header would take `#!/usr/bin/env bash` with it and leave a file the kernel will not run.
func TestAShebangIsNeverOfferedAsAUnit(t *testing.T) {
	lines := []string{"#!/usr/bin/env bash", "# What this script does.", "# A second line.", "set -euo pipefail"}
	units := commentBlocks(lines)
	for _, unit := range units {
		if unit.Line == 1 {
			t.Fatalf("the shebang was offered as a unit: %+v", unit)
		}
	}
	if len(units) != 1 || units[0].Line != 2 || units[0].Span != 2 {
		t.Fatalf("want one unit over lines 2-3; got %+v", units)
	}
	// Deleting everything on offer leaves the script runnable.
	if kept := Apply(lines, units, []int{1}); !strings.HasPrefix(kept, "#!/usr/bin/env bash\n") {
		t.Fatalf("a vote against the header took the shebang with it:\n%s", kept)
	}
}

// Only at the top of the file. A `#!` further down is an ordinary comment and stays offerable.
func TestAHashBangBelowTheFirstLineIsAnOrdinaryComment(t *testing.T) {
	lines := []string{"code", "#!not-a-shebang", "more code"}
	units := commentBlocks(lines)
	if len(units) != 1 || units[0].Line != 2 {
		t.Fatalf("want the line-2 comment offered; got %+v", units)
	}
}
