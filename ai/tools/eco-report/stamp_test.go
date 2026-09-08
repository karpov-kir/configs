package ecoreport_test

// The stamp is what the merge gate trusts, so every case here is one way a pass could claim a review
// it did not do.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAStampCannotOutliveThePassThatEarnedIt(t *testing.T) {
	t.Parallel()
	uninvalidated := newShip(t, "review: stamp guard")
	uninvalidated.runReport("stamp", allStagesStampedAs)
	uninvalidated.assertRefused("stamp refuses before this pass has invalidated")
	uninvalidated.assertReports("never invalidated", "and names invalidate as what is missing")

	unmarked := newShip(t, "review: unmarked stage")
	unmarked.runReport("invalidate")
	unmarked.runReport("decisions-reviewed")
	for _, cleared := range []string{"code-review", "security-review", "edit"} {
		unmarked.recordCleanStage(cleared)
	}
	unmarked.runReport("stamp", allStagesStampedAs)
	unmarked.assertRefused("stamp refuses a stage with no accepted result")
	unmarked.assertReports("no accepted result", "and names the missing stage result")
	unmarked.record("and stamped nothing for the pass that skipped it",
		containsLine(unmarked.read(unmarked.reportPath("")), "reviewed-tree: pending"), "")
	// The missing stage is the only thing preventing this stamp.
	unmarked.recordCleanStage("refactor")
	unmarked.runReport("stamp", allStagesStampedAs)
	unmarked.record("and stamps once every stage has an accepted result",
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
		f.recordCleanStage(stage)
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
		f.recordCleanStage(stage)
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
	f.runReport("invalidate")
	markers := f.repo + "/.git/idsd-stage-returns/001-inherited-markers"

	// A pass that got as far as accounting for the decision log and marking every stage, then died —
	// no invalidate, no close, no discard. The markers are on disk.
	f.runReport("decisions-reviewed")
	for _, stage := range allStages {
		f.recordCleanStage(stage)
	}
	manifest := f.stageResultsPath("001-inherited-markers")
	f.record("the abandoned pass left its markers behind",
		f.isFile(markers+"/decisions-reviewed"), f.evidence())

	// Someone starts over on the same intent.
	f.runReport("init", "001-inherited-markers", "--force")
	f.record("a fresh init clears them rather than adopting them",
		f.status == 0 && !f.isFile(markers+"/decisions-reviewed") && !f.exists(manifest), f.evidence())

	// The proof that matters: the new pass cannot reach a stamp on the dead one's account of the log.
	f.runReport("invalidate")
	for _, stage := range allStages {
		f.recordCleanStage(stage)
	}
	f.runReport("stamp", allStagesStampedAs)
	f.assertRefused("so the new pass must account for the decision log itself")
	f.assertReports("decisions-reviewed", "and is told which account is missing")
}

func TestAStageResultIsPrivateToItsOwner(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-result-modes")
	f.runReport("invalidate")
	f.recordCleanStage("code-review")
	manifest := f.stageResultsPath("001-result-modes")
	directory, directoryErr := os.Stat(filepath.Dir(manifest))
	f.record("results directory is reachable only by its owner", directoryErr == nil && directory.Mode().Perm() == 0o700, fmt.Sprint(directoryErr, " ", directory))
	result, resultErr := os.Stat(manifest)
	f.record("results are readable and writable only by their owner", resultErr == nil && result.Mode().Perm() == 0o600, fmt.Sprint(resultErr, " ", result))
}

func TestDecisionReviewEvidenceIsPrivateToItsOwner(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-decision-modes")
	f.runReport("decisions-reviewed")
	directory := f.repo + "/.git/idsd-stage-returns/001-decision-modes"
	info, err := os.Stat(directory)
	f.record("decision evidence directory is private", err == nil && info.Mode().Perm() == 0o700, fmt.Sprint(err, " ", info))
	info, err = os.Stat(directory + "/decisions-reviewed")
	f.record("decision review evidence is private", err == nil && info.Mode().Perm() == 0o600, fmt.Sprint(err, " ", info))
}

func TestAStageNameThatIsNotAStageIsRefused(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-stage-names")
	f.runReport("invalidate")
	for _, stage := range []string{"bogus", ""} {
		f.recordCleanStage(stage)
		f.assertRefused("refuses an unknown or empty stage")
	}
	f.recordCleanStage("code-review")
	f.record("a known stage is accepted", f.status == 0, f.evidence())
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
	f.record("invalidate removes the prior decision review marker",
		!f.exists(f.repo+"/.git/idsd-stage-returns/001-invalidating/decisions-reviewed"), "")
	report := f.read(f.reportPath("001-invalidating"))
	f.record("invalidate clears the reviewed tree", containsLine(report, "reviewed-tree: pending"), report)
	f.record("and the stage record beside it", containsLine(report, "reviewed-stages: pending"), report)

	// The gate reads that stage record on its own arm, so a record left standing keeps claiming a full
	// four stages for a pass whose own stamp says the review is not done.
	f.runReport("gate", "001-invalidating")
	f.record("so the gate blocks on the stage record, not on freshness alone",
		f.status == 1 && strings.Contains(f.out, "no reviewed-stages record"), f.evidence())

	// Typed receipts from the previous attempt cannot satisfy the new pass.
	f.runReport("stamp", allStagesStampedAs, "001-invalidating")
	f.assertRefused("and a stamp taken straight after invalidate is refused")
	f.assertReports("no accepted result", "because none of this pass's stages has returned")
	f.record("and nothing was stamped over the cleared record",
		containsLine(f.read(f.reportPath("001-invalidating")), "reviewed-tree: pending"), "")
}

func TestAStageResultWriteFailureIsNotReportedAsAccepted(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-unwritable-result")
	f.runReport("invalidate")
	result := f.cleanStageResultIn(cleanStageOptions{dir: f.repo, stage: "code-review", outcome: "complete", intent: ""})
	manifest := f.stageResultsPath("001-unwritable-result")
	f.remove(manifest)
	f.mkdirAll(manifest)
	f.runReport("stage-result", result)
	f.assertRefused("stage-result refuses when its manifest cannot be read or written")
	f.record("and reports no accepted result", !strings.Contains(f.out, "accepted stage result"), f.out)
}
