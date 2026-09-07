package ecoreport_test

// The intent-ready gate. It is the only thing standing between a build and an ICE that still holds
// the template's own words, so the clean pass is asserted first: every block below would also pass
// against a gate that blocks on everything.

import (
	"strings"
	"testing"
)

// A filled ICE — what `intent-ready` must let through. Built as lines because the gherkin fences make
// a raw string impossible.
func readyIntent(extraFrontmatter ...string) string {
	lines := []string{
		"---",
		"title: Fast search",
		"milestone: mvp",
		"status: draft",
	}
	lines = append(lines, extraFrontmatter...)
	return strings.Join(append(lines, []string{
		"---",
		"",
		"# Search returns results fast enough to keep shoppers on the page",
		"",
		"> Why this matters: slow search is the top reason sessions end before a purchase.",
		"",
		"## Constraints",
		"",
		"- search returns in under 300ms at p95",
		"",
		"## Success scenarios",
		"",
		"```gherkin",
		"Scenario: a common query",
		"  Given the catalogue is indexed",
		"  When a shopper searches for boots",
		"  Then results appear within 300ms",
		"```",
		"",
		"## Failure scenarios",
		"",
		"```gherkin",
		"Scenario: the index is cold",
		"  Given the index has not been built",
		"  When a shopper searches for boots",
		"  Then the shopper is told search is unavailable",
		"```",
		"",
	}...), "\n")
}

func (f *fixture) writeIntent(slug, body string) {
	f.t.Helper()
	f.mkdirAll(f.scratch() + "/intents")
	f.write(f.shipDir(slug)+"/intent.md", body)
}

func (f *fixture) writeArchivedIntent(slug, body string) {
	f.t.Helper()
	f.write(f.archiveDir(slug)+"/intent.md", body)
}

func TestIntentReadyClearsAFilledIceAndBlocksOnEachDefect(t *testing.T) {
	t.Parallel()
	f := newRepo(t)

	f.writeIntent("001-search", readyIntent())
	f.runReport("intent-ready", "001-search")
	f.record("a filled ICE is ready", f.status == 0 && strings.Contains(f.out, "intent ready"), f.evidence())

	f.record("and the clean line names each check the gate makes, and claims no sign-off",
		strings.Contains(f.out, "no placeholders") &&
			strings.Contains(f.out, "every required section filled") &&
			strings.Contains(f.out, "dependencies built") &&
			strings.Contains(f.out, "sibling declaring it goes first") &&
			!strings.Contains(f.out, "sign-off"), f.evidence())

	// The defect the gate exists for: an ICE still carrying the words the template shipped with.
	f.writeIntent("001-search", strings.Replace(readyIntent(),
		"- search returns in under 300ms at p95",
		`- <constraint, prefer measurable, e.g. "search returns in < 300ms">`, 1))
	f.runReport("intent-ready", "001-search")
	f.record("an unfilled constraint blocks", f.status == 1 && strings.Contains(f.out, "unfilled placeholder"), f.evidence())
	f.assertReports("intent not ready", "and names the file it judged")

	// Fences are scanned, not skipped: the gherkin skeleton is where the template's own placeholders
	// sit, so a scan that stepped over fenced text would pass an untouched scenario block.
	f.writeIntent("001-search", strings.Replace(readyIntent(),
		"Scenario: a common query", "Scenario: <name>", 1))
	f.runReport("intent-ready", "001-search")
	f.record("a placeholder inside a gherkin fence blocks",
		f.status == 1 && strings.Contains(f.out, "<name>"), f.evidence())

	// The two shapes that are not placeholders. A code span is how an intent writes a literal angle
	// bracket, and a comparison opens on a space.
	f.writeIntent("001-search", strings.Replace(readyIntent(),
		"- search returns in under 300ms at p95",
		"- the feed element is `<entry>` and latency stays < 300ms and > 1ms", 1))
	f.runReport("intent-ready", "001-search")
	f.record("a code span and a comparison are not placeholders",
		f.status == 0, f.evidence())

	// And a comparison does not hide the placeholder after it. The `>` closing `< 300ms` is `<state>`'s
	// own, so a scan that skipped to it would step over the one thing it is looking for — and report an
	// ICE still holding template text as ready to build.
	f.writeIntent("001-search", strings.Replace(readyIntent(),
		"- search returns in under 300ms at p95",
		"- latency stays < 300ms for <state>", 1))
	f.runReport("intent-ready", "001-search")
	f.record("a placeholder standing after a comparison on the same line still blocks",
		f.status == 1 && strings.Contains(f.out, "<state>"), f.evidence())

	f.writeIntent("001-search", strings.Replace(readyIntent(), "## Failure scenarios", "## Nothing in particular", 1))
	f.runReport("intent-ready", "001-search")
	f.record("a missing required section blocks",
		f.status == 1 && strings.Contains(f.out, "no '## Failure scenarios' section"), f.evidence())

	// Deleting the placeholder is the obvious way to "answer" one, so emptiness is asked separately.
	f.writeIntent("001-search", strings.Replace(readyIntent(), "- search returns in under 300ms at p95", "", 1))
	f.runReport("intent-ready", "001-search")
	f.record("an empty required section blocks",
		f.status == 1 && strings.Contains(f.out, "'## Constraints' is empty"), f.evidence())
}

func TestIntentReadyBlocksOnADependencyThatHasNotShipped(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	dependent := readyIntent("links:", "  - depends-on 001 — needs the index it builds")

	f.writeIntent("001-indexing", readyIntent())
	f.writeIntent("003-search", dependent)
	f.runReport("intent-ready", "003-search")
	f.record("a depends-on edge onto a draft intent blocks",
		f.status == 1 && strings.Contains(f.out, "depends-on 001 is not built"), f.evidence())

	f.writeIntent("001-indexing", strings.Replace(readyIntent(), "status: draft", "status: built", 1))
	f.runReport("intent-ready", "003-search")
	f.record("and clears once that intent is built", f.status == 0, f.evidence())

	// Archived is built — the file moves there at merge, so the status line is no longer what says so.
	f.remove(f.shipDir("001-indexing") + "/intent.md")
	f.writeArchivedIntent("001-indexing", readyIntent())
	f.runReport("intent-ready", "003-search")
	f.record("an archived dependency counts as built", f.status == 0, f.evidence())

	f.remove(f.archiveDir("001-indexing"))
	f.runReport("intent-ready", "003-search")
	f.record("an edge naming no intent blocks",
		f.status == 1 && strings.Contains(f.out, "names no intent"), f.evidence())
}

// Never mirror 001's `blocks 003` with a `depends-on 001` in 003. A two-sided edge survives the
// removal of either side, so the case below that drops the declaration would clear and prove nothing.
func TestIntentReadyBlocksOnASiblingThatDeclaredItGoesFirst(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	blocker := readyIntent("links:", "  - blocks 003 — moves the paths 003 would rewrite")

	f.writeIntent("001-indexing", blocker)
	f.writeIntent("003-search", readyIntent())
	f.runReport("intent-ready", "003-search")
	f.record("a sibling declaring blocks on this intent blocks it",
		f.status == 1 && strings.Contains(f.out, "001-indexing declares blocks 003"), f.evidence())

	f.writeIntent("001-indexing", readyIntent())
	f.runReport("intent-ready", "003-search")
	f.record("and clears when that sole declaration is removed", f.status == 0, f.evidence())

	f.writeIntent("001-indexing", strings.Replace(blocker, "status: draft", "status: built", 1))
	f.runReport("intent-ready", "003-search")
	f.record("a built blocker does not block", f.status == 0, f.evidence())

	f.remove(f.shipDir("001-indexing") + "/intent.md")
	f.writeArchivedIntent("001-indexing", blocker)
	f.runReport("intent-ready", "003-search")
	f.record("an archived blocker does not block, since archived is built", f.status == 0, f.evidence())

	f.remove(f.archiveDir("001-indexing"))
	f.writeIntent("003-search", readyIntent("links:", "  - blocks 003 — a self-edge nobody should act on"))
	f.runReport("intent-ready", "003-search")
	f.record("an intent's own blocks entry does not block itself", f.status == 0, f.evidence())
}

// A relation is read at the head of a `links:` entry and never inside another entry's why. Both
// directions go through one parser, so both would invent an edge: the forward one a dependency of the
// author's own making, the reverse one a blocker sitting in a SIBLING's file, which the blocked
// intent's author cannot clear from their own.
func TestIntentReadyReadsALinkFromTheEntryHeadAndNotFromItsWhy(t *testing.T) {
	t.Parallel()
	f := newRepo(t)

	// 002 is deliberately absent: a phantom edge onto it would be reported as naming no intent, so the
	// case fails loudly rather than quietly reading the right number for the wrong reason.
	f.writeIntent("001-indexing", strings.Replace(readyIntent(), "status: draft", "status: built", 1))
	f.writeIntent("003-search", readyIntent("links:", "  - blocks 001 — indexing waits on this; nothing here depends-on 002 any more"))
	f.runReport("intent-ready", "003-search")
	f.record("a relation named in an entry's why is not a depends-on edge", f.status == 0, f.evidence())

	f.writeIntent("002-ranking", readyIntent("links:", "  - depends-on 001 — 001 blocks 003 too, so ranking follows it"))
	f.runReport("intent-ready", "003-search")
	f.record("and a sibling's why naming this intent is not a blocks edge", f.status == 0, f.evidence())

	// The control. Same words, same draft sibling, moved to the head of its own entry: it blocks. Without
	// this the two cases above would pass against a scan that had stopped running.
	f.writeIntent("002-ranking", readyIntent("links:", "  - blocks 003 — ranking rewrites what search reads"))
	f.runReport("intent-ready", "003-search")
	f.record("while the same words at an entry's head do block",
		f.status == 1 && strings.Contains(f.out, "002-ranking declares blocks 003"), f.evidence())
}

// The reverse scan opens files this tool never named, so the two ways that read can fail get the
// answer the forward scan gives: refuse. Clearing an intent because the siblings could not be read is
// the false green the scan was added to close.
func TestIntentReadyRefusesWhenItCannotReadTheSiblingsThatMightBlockIt(t *testing.T) {
	t.Parallel()
	f := newRepo(t)

	f.writeIntent("003-search", readyIntent())
	f.runReport("intent-ready", "003-search")
	f.record("fixture: it reads ready with the siblings readable", f.status == 0, f.evidence())

	f.writeIntent("001-indexing", readyIntent())
	f.remove(f.shipDir("001-indexing") + "/intent.md")
	f.symlink(f.shipDir("003-search")+"/intent.md", f.shipDir("001-indexing")+"/intent.md")
	f.runReport("intent-ready", "003-search")
	f.record("a sibling intent reached through a symlink refuses",
		f.status == 2 && strings.Contains(f.out, "is a symlink"), f.evidence())

	f.remove(f.shipDir("001-indexing"))
	dir := f.scratch() + "/intents"
	if !f.madeUnlistable(dir, "the unlistable-siblings case") {
		return
	}
	f.runReport("intent-ready", "003-search")
	f.chmod(dir, 0o755)
	f.record("an intents directory that cannot be listed refuses rather than clearing",
		f.status == 2 && strings.Contains(f.out, "whether a sibling blocks 003 is unknown"), f.evidence())
}

func TestIntentReadyRefusesRatherThanJudgingWhatItCannotRead(t *testing.T) {
	t.Parallel()
	f := newRepo(t)

	f.runReport("intent-ready")
	f.assertRefused("a bare intent-ready refuses")

	// Every link between intents is drawn on the number, so a name without one cannot be judged against
	// its siblings — and running three of the four checks under a line that claims all four is worse
	// than refusing.
	f.writeIntent("no-number", readyIntent())
	f.runReport("intent-ready", "no-number")
	f.assertRefused("a slug with no leading number refuses")
	f.assertReports("must open with the intent's number", "and says the number is what it lacked")

	// The slug is joined into a path, so the charset is the whole of what keeps this read inside
	// intents/.
	f.runReport("intent-ready", "../../etc/passwd")
	f.assertRefused("a slug outside the charset refuses")

	f.runReport("intent-ready", "004-absent")
	f.assertRefused("an intent that is not there refuses")
	f.assertReports("no intent at", "and says which path it looked at")

	f.writeArchivedIntent("005-shipped", readyIntent())
	f.runReport("intent-ready", "005-shipped")
	f.assertRefused("an archived intent refuses rather than reporting itself ready")
	f.assertReports("is archived", "and says it is already built")

	f.writeIntent("006-real", readyIntent())
	f.symlink(f.shipDir("006-real")+"/intent.md", f.shipDir("007-link")+"/intent.md")
	f.runReport("intent-ready", "007-link")
	f.assertRefused("a symlinked intent refuses")
}
