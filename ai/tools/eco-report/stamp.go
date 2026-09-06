package ecoreport

import (
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// The pass's own bookkeeping: what invalidate clears, what a stage's return records, and what the
// stamp demands before it will write a reviewed tree into the report the merge gate trusts.

const stampUsage = `usage: report.sh stamp "<all four stages, comma-separated>"
  code-review                                   always runs, always bare
  refactor | refactor:partial(turnaround|cap)   partial = the loop ended non-compliant
  security-review|tighten [:skipped(turnaround|not-applicable)]
    turnaround      = trimmed to answer sooner; blocks the merge gate until an untrimmed pass
    not-applicable  = its condition was unmet
`

// Serialising the marks is what makes stages.go's checksum rule per-stage: the round's stages return
// together, so back-to-back marks share one checksum and one edit would clear them all.
func (r *run) cmdStageReturned() {
	r.requireReport(r.arg(2))
	stage := r.arg(1)
	r.assertValidStage(stage, "stage-returned")
	outstanding := r.outstandingStage()
	// Its own name is the one exception. A streamed stage returns, gets resumed with what landed, and
	// returns again with nothing recorded in between — the report has not moved, so it is itself the
	// outstanding stage. Re-marking it rewrites the same checksum, so this is idempotent; what the guard
	// exists to stop is a *second* stage being waved through on the first's unrecorded return.
	if outstanding != "" && outstanding != stage {
		r.refuse("error: " + outstanding + " is marked returned and the report has not moved since — record its items, or run report.sh no-items " + outstanding + ", before marking a stage returned.")
	}
	r.writeStageMarker(stage, r.reportChecksum())
	r.line("recorded return of %s — record its items, or report.sh no-items %s, before taking the next stage's return", stage, stage)
}

// The escape hatch for a stage that genuinely surfaced nothing. It demands a marker: without that, a
// pass that ran nothing can declare every stage empty and stamp.
func (r *run) cmdNoItems() {
	r.requireReport(r.arg(2))
	stage := r.arg(1)
	r.assertValidStage(stage, "no-items")
	if !r.stageWasMarkedReturned(stage) {
		r.refuse("error: " + stage + " was never marked returned — run report.sh stage-returned " + stage + " when it returns, then report.sh no-items " + stage + " once you have read its findings.")
	}
	r.writeStageMarker(stage, noItemsMarker)
	r.line("recorded %s as having surfaced nothing for the report", stage)
}

// The pass's account of the decision log, which `stamp` requires. Whether an entry was reached and is
// still true is a judgment no tool can take for the agent — `records.md` → **Every entry is dated and
// counted**. So what is enforced here is only that the pass said it worked the log. That is the
// guarantee `no-items` gives a stage that surfaced nothing, and it is here for the same reason:
// without it, a pass that never opened the record stamps exactly like one that pruned it.
//
// It lives in the stage-returns directory, so `invalidate` clears it along with everything else. It
// holds a word rather than a checksum, which keeps `outstandingStage` from taking it for a stage
// awaiting its items.
const decisionsMarker = "decisions-reviewed"

func (r *run) cmdDecisionsReviewed() {
	r.requireReport(r.arg(1))
	r.writeStageMarker(decisionsMarker, "accounted")
	r.line("recorded this pass's account of the decision log")
}

func (r *run) cmdStamp() {
	r.requireReport(r.arg(2))
	entries := r.arg(1)
	if entries == "" {
		writeAll(r.errOut, stampUsage)
		r.exit(2)
	}
	entries = removeWhitespace(entries)
	if problems := validateStampEntries(entries); len(problems) > 0 {
		r.refuse("error: invalid stage record", strings.Join(problems, "\n"))
	}
	if !hasField(r.report, "reviewed-tree") {
		r.refuse("error: no 'reviewed-tree:' line in frontmatter")
	}
	// `invalidate` is what separates one pass from the next, so it comes before the per-stage check
	// below, whose markers mean nothing until it is known which pass made them.
	if stamped := r.reviewedTree(); stamped != "pending" {
		// Collapsed for the reason gate.go collapses the same field: it is a line out of a hand-editable
		// report, and it is a fingerprint only when the stamp wrote it.
		stamped = shell.Oneline(stamped)
		if stamped == "" {
			stamped = "<empty>"
		}
		r.refuse("error: this pass never invalidated — reviewed-tree still reads '" + stamped + "', not 'pending'. Run report.sh invalidate first, or the stamp and the stage markers standing here are the previous pass's, not this one's.")
	}
	// `refactor` is legally shaped whether or not it ran, hence the per-stage check.
	var blockReasons []string
	for _, entry := range strings.Split(entries, ",") {
		if strings.Contains(entry, ":skipped(") {
			continue
		}
		stage, _, _ := strings.Cut(entry, ":")
		if reason := r.stageBlockReason(stage); reason != "" {
			blockReasons = append(blockReasons, "  "+stage+": "+reason)
		}
	}
	if len(blockReasons) > 0 {
		r.refuse("error: these stages are recorded as having run, but:", strings.Join(blockReasons, "\n"))
	}
	// Last of the preconditions, so a pass missing both this and a stage's items is told about the
	// stage first — that one names which stage, and this one would send it to the record instead.
	if !r.stageWasMarkedReturned(decisionsMarker) {
		r.refuse("error: this pass has not accounted for the decision log — NOT stamped.",
			"  Re-evaluate every entry against the tree: bump what this pass reached and found still true,",
			"  evict what its subject has left, and leave what this pass never went near.",
			"  Then: report.sh decisions-reviewed")
	}
	tree, ok := r.currentTree(r.errOut)
	if !ok {
		r.exit(2)
	}
	worktree, established := r.currentWorktreeRecord()
	if !established {
		r.refuse("error: could not establish which worktree this pass ran in ("+r.gitPath("idsd-worktree-id")+" is not writable) — NOT stamped.",
			"  gate reads that identity to tell this tree's review from a sibling's; recorded as unknown, two worktrees would gate clean off each other's review.",
			"  Make that path writable, then stamp again. Nothing else about the pass is lost.")
	}
	err := r.rewriteReport(
		"nothing was stamped",
		"could not write the stamp into "+r.report+" — reviewed-tree is unchanged",
		rewriteStamp(tree, worktree, entries))
	if err != nil {
		r.exit(2)
	}
	r.line("stamped reviewed-tree: %s (stages: %s)", tree, entries)
}

func (r *run) cmdInvalidate() {
	r.requireReport(r.arg(1))
	// Before the stamp, not after: the sentence below is only true while the stamp still stands. Clearing
	// it first and failing here leaves a report that reads as invalidated under a refusal saying it is not.
	// Last pass's stage returns would otherwise satisfy this pass's stamp for free.
	if err := os.RemoveAll(r.stageReturnsDir); err != nil {
		r.refuse("error: could not clear " + r.stageReturnsDir + " (" + err.Error() + ") — the pass is NOT invalidated, and the markers still standing would let the next stamp skip what it has to re-earn.")
	}
	err := r.rewriteReport(
		"the stamp was NOT cleared; it still describes an older tree",
		"could not clear the stamp in "+r.report+" — it still describes an older tree",
		rewriteInvalidated)
	if err != nil {
		r.exit(2)
	}
	r.line("invalidated reviewed-tree — restamp when the pass completes")
}

// `tr -d '[:space:]'`, so a stage record pasted across two lines still reads as one.
func removeWhitespace(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if !shell.IsSpaceByte(value[i]) {
			out.WriteByte(value[i])
		}
	}
	return out.String()
}
