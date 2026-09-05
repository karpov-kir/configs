package ecoreport_test

// The stamp is what the merge gate trusts, so every case here is one way a pass could claim a review
// it did not do.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestAStampCannotOutliveThePassThatEarnedIt(t *testing.T) {
	t.Parallel()
	uninvalidated := newShip(t, "review: stamp guard")
	uninvalidated.runReport("stamp", allStagesStampedAs)
	uninvalidated.assertRefused("stamp refuses before this pass has invalidated")
	uninvalidated.assertReports("never invalidated", "and names invalidate as what is missing")

	unrecorded := newShip(t, "review: stage marker guard")
	unrecorded.runReport("invalidate")
	unrecorded.runReport("stage-returned", "code-review")
	unrecorded.runReport("stage-returned", "security-review")
	unrecorded.assertRefused("a second stage cannot be marked returned while the first's items are unrecorded")
	unrecorded.assertReports("has not moved since", "and says the report has not moved")

	// The same guard, met by the one caller it must not stop: streaming resumes a stage and takes its
	// return again with nothing recorded in between, so the outstanding stage is the one being marked.
	// The other three are cleared first so the re-marked one is the only stage the stamp below can block on.
	resumed := newShip(t, "review: resumed stage")
	resumed.runReport("invalidate")
	resumed.runReport("decisions-reviewed")
	for _, cleared := range []string{"security-review", "tighten", "refactor"} {
		resumed.runReport("stage-returned", cleared)
		resumed.runReport("no-items", cleared)
	}
	resumed.runReport("stage-returned", "code-review")
	resumed.runReport("stage-returned", "code-review")
	resumed.record("a resumed stage can be marked returned again", resumed.status == 0, resumed.evidence())
	// Exit 0 alone would also be satisfied by a re-mark that cleared the gate. What must survive the
	// re-mark is the stamp's demand that the stage's items reach the report: it rewrites the same
	// checksum, so the report still has not moved, and a stamp taken here would record a review whose
	// findings were never written down.
	resumed.runReport("stamp", allStagesStampedAs)
	resumed.assertRefused("and the re-mark leaves the stamp still gated on that stage's unrecorded items")
	resumed.assertReports("unchanged since", "and names the unrecorded items as the reason")
	resumed.record("and nothing was stamped over them",
		containsLine(resumed.read(resumed.reportPath("")), "reviewed-tree: pending"), "")
	// The positive control for the refusal above: the same stamp lands once that one stage is accounted
	// for, so what blocked it was the unrecorded items and nothing else on the way.
	resumed.runReport("no-items", "code-review")
	resumed.runReport("stamp", allStagesStampedAs)
	resumed.record("and stamps once that stage's items are accounted for",
		resumed.status == 0 && !containsLine(resumed.read(resumed.reportPath("")), "reviewed-tree: pending"),
		resumed.evidence())

	// The stamp's other per-stage refusal: an entry recorded as having run for a stage that was never
	// marked returned at all. `refactor` is legally shaped whether or not it ran, so the grammar
	// check cannot see this and only the per-stage marker can. Three of the four are marked, leaving
	// `refactor` as the one thing between this pass and a stamp it never earned.
	unmarked := newShip(t, "review: unmarked stage")
	unmarked.runReport("invalidate")
	unmarked.runReport("decisions-reviewed")
	for _, cleared := range []string{"code-review", "security-review", "tighten"} {
		unmarked.runReport("stage-returned", cleared)
		unmarked.runReport("no-items", cleared)
	}
	unmarked.runReport("stamp", allStagesStampedAs)
	unmarked.assertRefused("stamp refuses a stage recorded as having run that was never marked returned")
	unmarked.assertReports("never marked returned", "and names the stage-returned call that never happened")
	unmarked.record("and stamped nothing for the pass that skipped it",
		containsLine(unmarked.read(unmarked.reportPath("")), "reviewed-tree: pending"), "")
	// And the same stamp lands once that stage is marked too, so the refusal above was this guard alone.
	unmarked.runReport("stage-returned", "refactor")
	unmarked.runReport("no-items", "refactor")
	unmarked.runReport("stamp", allStagesStampedAs)
	unmarked.record("and stamps once every stage carries a marker",
		unmarked.status == 0 && !containsLine(unmarked.read(unmarked.reportPath("")), "reviewed-tree: pending"),
		unmarked.evidence())
}

// `records.md` → **Every entry is dated and counted** makes the bump a judgment no tool can take for
// the agent, so the stamp enforces the one thing it can: that the pass said it worked the log. Without
// it a pass that never opened the record stamps exactly like one that pruned it.
func TestAStampDemandsThePassAccountForTheDecisionLog(t *testing.T) {
	t.Parallel()
	f := newShip(t, "review: decision log account")
	f.runReport("invalidate")
	for _, stage := range allStages {
		f.runReport("stage-returned", stage)
		f.runReport("no-items", stage)
	}
	f.runReport("stamp", allStagesStampedAs)
	f.assertRefused("stamp refuses before this pass has accounted for the decision log")
	f.assertReports("decisions-reviewed", "and names the subcommand that answers it")
	f.record("and stamped nothing",
		containsLine(f.read(f.reportPath("")), "reviewed-tree: pending"), "")

	// The positive control: the same stamp lands once the account is on record, so nothing else on the
	// way was blocking it.
	f.runReport("decisions-reviewed")
	f.runReport("stamp", allStagesStampedAs)
	f.record("and stamps once the account is recorded",
		f.status == 0 && !containsLine(f.read(f.reportPath("")), "reviewed-tree: pending"), f.evidence())

	// The account belongs to the pass that made it. `invalidate` starts the next one, and a marker left
	// standing would let that pass stamp on this one's reading of the log.
	f.runReport("invalidate")
	for _, stage := range allStages {
		f.runReport("stage-returned", stage)
		f.runReport("no-items", stage)
	}
	f.runReport("stamp", allStagesStampedAs)
	f.assertRefused("and the next pass cannot inherit it")
	f.assertReports("decisions-reviewed", "and is sent to account for the log again")
}

// A stem's markers are cleared by invalidate, close and discard. `init` is the fourth way onto a
// stem, and the only one that does not follow a pass ending — a crash, a deleted report, a --force.
// Every marker is a precondition `stamp` reads instead of re-checking the stage, so one surviving
// into a fresh report is a claim about a pass that never ran.
func TestAFreshReportInheritsNoStageMarkerFromTheOneBeforeIt(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-inherited-markers")
	markers := f.repo + "/.git/idsd-stage-returns/001-inherited-markers"

	// A pass that got as far as accounting for the decision log and marking every stage, then died —
	// no invalidate, no close, no discard. The markers are on disk.
	f.runReport("decisions-reviewed")
	for _, stage := range allStages {
		f.runReport("stage-returned", stage)
		f.runReport("no-items", stage)
	}
	f.record("the abandoned pass left its markers behind",
		f.isFile(markers+"/decisions-reviewed"), f.evidence())

	// Someone starts over on the same intent.
	f.runReport("init", "001-inherited-markers", "--force")
	f.record("a fresh init clears them rather than adopting them",
		f.status == 0 && !f.isFile(markers+"/decisions-reviewed"), f.evidence())

	// The proof that matters: the new pass cannot reach a stamp on the dead one's account of the log.
	f.runReport("invalidate")
	for _, stage := range allStages {
		f.runReport("stage-returned", stage)
		f.runReport("no-items", stage)
	}
	f.runReport("stamp", allStagesStampedAs)
	f.assertRefused("so the new pass must account for the decision log itself")
	f.assertReports("decisions-reviewed", "and is told which account is missing")
}

// The markers ARE the merge precondition: `stamp` reads them rather than re-checking the stage. A
// mode the umask decides is one another local account can write, and a forged marker there buys a
// stamp the pass never earned.
func TestAStageMarkerIsNotWritableByAnyoneElseOnTheMachine(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-marker-modes")
	f.runReport("decisions-reviewed")
	markers := f.repo + "/.git/idsd-stage-returns/001-marker-modes"

	dir, dirErr := os.Stat(markers)
	f.record("the marker directory is reachable only by its owner",
		dirErr == nil && dir.Mode().Perm() == 0o700, fmt.Sprint(dirErr, " ", dir))
	marker, markerErr := os.Stat(markers + "/decisions-reviewed")
	f.record("and the marker itself is writable only by its owner",
		markerErr == nil && marker.Mode().Perm() == 0o600, fmt.Sprint(markerErr, " ", marker))
}

func TestAStageNameThatIsNotAStageIsRefused(t *testing.T) {
	t.Parallel()
	// What these pin is the refusal itself: the usage line, and no marker written.
	f := newShip(t, "001-stage-names")
	markers := f.repo + "/.git/idsd-stage-returns/001-stage-names"
	// An omitted stage name and an empty one arrive by the same path, so the empty form covers both; a
	// separate case for the omitted one could not fail differently.
	for _, badStage := range []string{"bogus", ""} {
		for _, subcommand := range []string{"stage-returned", "no-items"} {
			f.runReport(subcommand, badStage)
			f.assertRefused(subcommand + " refuses the stage name '" + badStage + "'")
			f.assertReports("usage: report.sh "+subcommand, "and prints the stage vocabulary for '"+badStage+"'")
		}
	}
	f.record("and no stage marker was written for any of them", len(f.entries(markers)) == 0, joinLines(f.entries(markers)))
	// Last, so the marker check above cannot pass on a fixture where nothing was markable in the first place.
	f.runReport("stage-returned", "code-review")
	f.record("while a real stage name on the same fixture is recorded",
		f.status == 0 && f.isFile(markers+"/code-review"), f.evidence())
}

func TestInvalidateClearsThePassItStarts(t *testing.T) {
	t.Parallel()
	// `invalidate` is what separates one pass from the next, and it clears two things. The frontmatter
	// has to stop describing the last pass, because the merge gate reads it; and the last pass's stage
	// markers have to go, or this pass's stamp is satisfied by returns it never took.
	f := newShip(t, "001-invalidating")
	f.stampFullPass("001-invalidating")
	f.runReport("gate", "001-invalidating")
	f.record("fixture: a full pass stands, and the gate clears it",
		f.status == 0 && strings.Contains(f.out, "gate clean"), f.evidence())

	f.runReport("invalidate", "001-invalidating")
	report := f.read(f.reportPath("001-invalidating"))
	f.record("invalidate clears the reviewed tree", containsLine(report, "reviewed-tree: pending"), report)
	f.record("and the stage record beside it", containsLine(report, "reviewed-stages: pending"), report)

	// The gate reads that stage record on its own arm, so a record left standing keeps claiming a full
	// four stages for a pass whose own stamp says the review is not done.
	f.runReport("gate", "001-invalidating")
	f.record("so the gate blocks on the stage record, not on freshness alone",
		f.status == 1 && strings.Contains(f.out, "no reviewed-stages record"), f.evidence())

	// And the markers. Left behind, the next stamp is satisfied by the previous pass's returns, so a
	// pass that has run nothing since stamps a full review over the tree as it now stands.
	f.runReport("stamp", allStagesStampedAs, "001-invalidating")
	f.assertRefused("and a stamp taken straight after invalidate is refused")
	f.assertReports("never marked returned", "because none of this pass's stages has returned")
	f.record("and nothing was stamped over the cleared record",
		containsLine(f.read(f.reportPath("001-invalidating")), "reviewed-tree: pending"), "")
}

func TestNoItemsDemandsTheStageHaveReturnedFirst(t *testing.T) {
	t.Parallel()
	// `no-items` is the one way to clear a stage's marker without editing the report, which makes it
	// the shortest path from a pass that ran nothing to a stamped full review. Its demand that the
	// stage have been marked returned first is the whole of what stands in that path.
	f := newShip(t, "001-no-items")
	f.runReport("invalidate", "001-no-items")
	markers := f.repo + "/.git/idsd-stage-returns/001-no-items"
	for _, stage := range allStages {
		f.runReport("no-items", stage, "001-no-items")
		f.assertRefused("no-items refuses " + stage + " before it has been marked returned")
		f.assertReports("was never marked returned", "and names the stage-returned call missing for "+stage)
	}
	f.record("and wrote no marker for any of them", len(f.entries(markers)) == 0, joinLines(f.entries(markers)))
	f.runReport("stamp", allStagesStampedAs, "001-no-items")
	f.assertRefused("so a pass that ran nothing cannot stamp")
	f.record("and nothing was stamped",
		containsLine(f.read(f.reportPath("001-no-items")), "reviewed-tree: pending"), "")

	// The positive control: the same call lands once that stage has returned, so what refused above
	// was this guard and not something else on the way.
	f.runReport("stage-returned", "code-review", "001-no-items")
	f.runReport("no-items", "code-review", "001-no-items")
	f.record("and no-items records a stage that has been marked returned",
		f.status == 0 && f.read(markers+"/code-review") == "no-items\n",
		"exit "+strconv.Itoa(f.status)+"; marker holds '"+f.read(markers+"/code-review")+"'\n"+f.out)
}

func TestAMarkerIsThePosixCksumOfTheReport(t *testing.T) {
	t.Parallel()
	// A marker holds the report as cksum(1) saw it, and it has to be cksum(1)'s digest rather than
	// merely this tool's own: a marker written by one version of the tool is read by the other across
	// a swap, and any self-consistent digest passes every comparison the tool makes against itself.
	f := newShip(t, "001-marker")
	f.runReport("invalidate", "001-marker")
	marker := f.repo + "/.git/idsd-stage-returns/001-marker/code-review"
	f.runReport("stage-returned", "code-review", "001-marker")
	f.record("stage-returned writes the stage's marker",
		f.status == 0 && f.isFile(marker), f.evidence())
	held, want, ok := f.markerAgainstCksum(marker, f.reportPath("001-marker"))
	if ok {
		f.record("and it holds the report's POSIX cksum — crc and byte count", held == want,
			"the marker holds '"+held+"'; cksum(1) says '"+want+"'")
	}

	// And it has to follow the report, or the stamp's "this stage's items reached the report" test is
	// satisfied by a report nobody wrote to.
	f.appendTo(f.reportPath("001-marker"), "- a finding this stage recorded\n")
	f.runReport("stage-returned", "code-review", "001-marker")
	held, want, ok = f.markerAgainstCksum(marker, f.reportPath("001-marker"))
	if ok {
		f.record("and the digest follows the report when the report changes", held == want,
			"the marker holds '"+held+"'; cksum(1) says '"+want+"'")
	}

	// The marker write can fail while the report is perfectly readable, and a "recorded" line printed
	// over a marker that was never written dead-ends the pass: `no-items` then refuses for a stage the
	// human just watched return. Unwritable by construction, never by a mode bit — the marker path is
	// a directory, so the write fails with EISDIR for every user, root included.
	blocked := newShip(t, "001-unwritable-marker")
	blocked.runReport("invalidate", "001-unwritable-marker")
	blocked.mkdirAll(blocked.repo + "/.git/idsd-stage-returns/001-unwritable-marker/code-review")
	blocked.runReport("stage-returned", "code-review", "001-unwritable-marker")
	blocked.assertRefused("stage-returned refuses when the marker cannot be written")
	blocked.assertReports("is NOT marked", "and says the stage is not marked")
	blocked.record("and reports no return the pass could act on",
		!strings.Contains(blocked.out, "recorded return"), blocked.out)
}
