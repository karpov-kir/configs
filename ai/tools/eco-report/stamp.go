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

// A marker holds the report's checksum as that stage returned, and the stamp demands the report have
// moved since. Serialising the marks is what makes that per-stage: the round's stages return
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
	err := r.rewriteReport(
		"the stamp was NOT cleared; it still describes an older tree",
		"could not clear the stamp in "+r.report+" — it still describes an older tree",
		rewriteInvalidated)
	if err != nil {
		r.exit(2)
	}
	// Last pass's stage returns would otherwise satisfy this pass's stamp for free.
	_ = os.RemoveAll(r.stageReturnsDir)
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
