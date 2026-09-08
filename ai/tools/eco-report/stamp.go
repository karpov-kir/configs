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
  refactor | refactor:partial(turnaround|cap) | refactor:skipped(not-applicable)
    partial         = the loop ended non-compliant
  security-review|edit [:skipped(turnaround|not-applicable)]
    turnaround      = trimmed to answer sooner; blocks the merge gate until an untrimmed pass
    not-applicable  = scope <base-ref> proved its condition unmet for this candidate
`

// The pass's account of the decision log, which `stamp` requires. Whether an entry was reached and is
// still true is a judgment no tool can take for the agent — `records.md` → **Every entry is dated and
// counted**. So what is enforced here is only that the pass said it worked the log. That is the
// guarantee a typed completion gives a stage that surfaced nothing, and it is here for the same reason:
// without it, a pass that never opened the record stamps exactly like one that pruned it.
//
// It lives in the stage-returns directory, so `invalidate` clears it along with everything else. It
// holds a word; typed stage results live in their own shared manifest.
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
	if problems := r.skipBlockReasons(entries); len(problems) > 0 {
		r.refuse("error: invalid scope evidence for skipped stages", strings.Join(problems, "\n"))
	}
	if problems := r.resultStagesProblems(entries); len(problems) > 0 {
		r.refuse("error: these stages are recorded as having run, but:", strings.Join(problems, "\n"))
	}
	// Last of the preconditions, so a pass missing both this and a stage's items is told about the
	// stage first — that one names which stage, and this one would send it to the record instead.
	if !r.hasPassMarker(decisionsMarker) {
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
	if manifest := r.readResultManifest(); manifest != nil {
		r.assertResultProjection(*manifest)
	}
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
	r.beginResultAttempt()
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
