package ecoreport_test

// The stamp is what the merge gate trusts, so every case here is one way a pass could claim a review
// it did not do.

import (
	"testing"
)

func TestAStampCannotOutliveThePassThatEarnedIt(t *testing.T) {
	t.Parallel()
	uninvalidated := newRepo(t)
	uninvalidated.runReport("check-ignore")
	uninvalidated.runReport("init", "review: stamp guard")
	uninvalidated.runReport("stamp", "code-review,security-review,tighten,refactor,retro")
	uninvalidated.assertRefused("stamp refuses before this pass has invalidated")
	uninvalidated.assertReports("never invalidated", "and names invalidate as what is missing")

	unrecorded := newRepo(t)
	unrecorded.runReport("check-ignore")
	unrecorded.runReport("init", "review: stage marker guard")
	unrecorded.runReport("invalidate")
	unrecorded.runReport("stage-returned", "code-review")
	unrecorded.runReport("stage-returned", "security-review")
	unrecorded.assertRefused("a second stage cannot be marked returned while the first's items are unrecorded")
	unrecorded.assertReports("has not moved since", "and says the report has not moved")

	// The same guard, met by the one caller it must not stop: streaming resumes a stage and takes its
	// return again with nothing recorded in between, so the outstanding stage is the one being marked.
	// The other four are cleared first so the re-marked one is the only stage the stamp below can block on.
	resumed := newRepo(t)
	resumed.runReport("check-ignore")
	resumed.runReport("init", "review: resumed stage")
	resumed.runReport("invalidate")
	for _, cleared := range []string{"security-review", "tighten", "refactor", "retro"} {
		resumed.runReport("stage-returned", cleared)
		resumed.runReport("no-items", cleared)
	}
	resumed.runReport("stage-returned", "code-review")
	resumed.runReport("stage-returned", "code-review")
	resumed.record("a resumed stage can be marked returned again", resumed.status == 0, "exit "+itoa(resumed.status)+"\n"+resumed.out)
	// Exit 0 alone would also be satisfied by a re-mark that cleared the gate. What must survive the
	// re-mark is the stamp's demand that the stage's items reach the report: it rewrites the same
	// checksum, so the report still has not moved, and a stamp taken here would record a review whose
	// findings were never written down.
	resumed.runReport("stamp", "code-review,security-review,tighten,refactor,retro")
	resumed.assertRefused("and the re-mark leaves the stamp still gated on that stage's unrecorded items")
	resumed.assertReports("unchanged since", "and names the unrecorded items as the reason")
	resumed.record("and nothing was stamped over them",
		containsLine(resumed.read(resumed.reportPath("")), "reviewed-tree: pending"), "")
	// The positive control for the refusal above: the same stamp lands once that one stage is accounted
	// for, so what blocked it was the unrecorded items and nothing else on the way.
	resumed.runReport("no-items", "code-review")
	resumed.runReport("stamp", "code-review,security-review,tighten,refactor,retro")
	resumed.record("and stamps once that stage's items are accounted for",
		resumed.status == 0 && !containsLine(resumed.read(resumed.reportPath("")), "reviewed-tree: pending"),
		"exit "+itoa(resumed.status)+"\n"+resumed.out)

	// The stamp's other per-stage refusal: an entry recorded as having run for a stage that was never
	// marked returned at all. `refactor,retro` is legally shaped whether or not either ran, so the
	// grammar check cannot see this and only the per-stage marker can. Four of the five are marked,
	// leaving `retro` as the one thing between this pass and a stamp it never earned.
	unmarked := newRepo(t)
	unmarked.runReport("check-ignore")
	unmarked.runReport("init", "review: unmarked stage")
	unmarked.runReport("invalidate")
	for _, cleared := range []string{"code-review", "security-review", "tighten", "refactor"} {
		unmarked.runReport("stage-returned", cleared)
		unmarked.runReport("no-items", cleared)
	}
	unmarked.runReport("stamp", "code-review,security-review,tighten,refactor,retro")
	unmarked.assertRefused("stamp refuses a stage recorded as having run that was never marked returned")
	unmarked.assertReports("never marked returned", "and names the stage-returned call that never happened")
	unmarked.record("and stamped nothing for the pass that skipped it",
		containsLine(unmarked.read(unmarked.reportPath("")), "reviewed-tree: pending"), "")
	// And the same stamp lands once that stage is marked too, so the refusal above was this guard alone.
	unmarked.runReport("stage-returned", "retro")
	unmarked.runReport("no-items", "retro")
	unmarked.runReport("stamp", "code-review,security-review,tighten,refactor,retro")
	unmarked.record("and stamps once every stage carries a marker",
		unmarked.status == 0 && !containsLine(unmarked.read(unmarked.reportPath("")), "reviewed-tree: pending"),
		"exit "+itoa(unmarked.status)+"\n"+unmarked.out)
}

func TestAStageNameThatIsNotAStageIsRefused(t *testing.T) {
	t.Parallel()
	// What these pin is the refusal itself: the usage line, and no marker written.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-stage-names")
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
		f.status == 0 && f.isFile(markers+"/code-review"), "exit "+itoa(f.status)+"\n"+f.out)
}
