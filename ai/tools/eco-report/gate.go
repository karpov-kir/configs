package ecoreport

import (
	"io"
	"strconv"

	"kk-flavor/tools/shell"
)

// What the report is read for: the merge gate, the items a re-qualify must carry, and the routing
// token `idsd-ship continue` acts on.

// What every overridable block says about being overridden. One literal, because five call sites said
// it identically and a sixth would have said it slightly differently — and the sentence is load-bearing:
// a comment pass found the earlier wording promised a flag (`--override`, `--force`, `-f`) that has
// never existed, so a reader was being pointed at a mechanism rather than told the truth.
const overridableByAHumanOnly = "No flag overrides this; overriding means a human deciding to merge anyway."

func (r *run) cmdGate() {
	r.requireReport(r.arg(1))
	// Every reason is asked, and none short-circuits the rest: a gate that stopped at the first block
	// would send the human back for one more round per reason it never printed.
	stale := r.blocksOnFreshness()
	trimmed := r.blocksOnStages()
	open := r.blocksOnOpenTodos()
	if stale || trimmed || open {
		r.exit(1)
	}
	r.line("gate clean: tree fresh, full qualify, no open TODOs")
	r.exit(0)
}

// True when this report's review does not vouch for this tree, here. Two questions, and the fingerprint
// answers only the first: without the second, a worktree that ran nothing reads a sibling's stamp as its
// own and gates clean. worktree.go holds why the fingerprint cannot answer it.
func (r *run) blocksOnFreshness() bool {
	current, ok := r.currentTree(r.errOut)
	if !ok {
		r.exit(2)
	}
	reviewed := r.reviewedTree()
	blocked := false
	if current != reviewed {
		if reviewed == "" {
			reviewed = "<unstamped>"
		}
		r.errLines("BLOCK (freshness): tree changed since last qualify (current " + current + " != reviewed " + reviewed + "). Re-qualify. " + overridableByAHumanOnly)
		blocked = true
	}
	// Whether the sentence below may claim identical trees. It is reachable with a STALE tree — an
	// unstamped `reviewed-tree` beside a real token, which an older binary's invalidate could leave — and
	// unguarded it claims the fingerprints match directly under a BLOCK saying they differ.
	sameTree := ""
	if current == reviewed {
		sameTree = " The fingerprints match because both trees are identical, which is not the same as this tree having been reviewed."
	}
	// Every value quoted below is collapsed: none of them is text this tool chose. The recorded worktree
	// is a line out of a hand-editable report, and the path beside it comes from the checkout's own name,
	// which git hands back with any control byte in it intact. An ESC in a BLOCK line rewrites the lines
	// printed above it, and what reads this output is an agent as often as a person.
	switch vouch, mine := r.worktreeVouch(); vouch {
	case noReviewingWorktreeRecorded:
		r.errLines("BLOCK (freshness): this report records a reviewed tree but no usable reviewing worktree (reviewed-worktree: " + shell.Oneline(fieldValue(r.report, "reviewed-worktree")) + "). Which worktree the review vouches for is unknown, so it cannot vouch for this one. Re-qualify here. " + overridableByAHumanOnly)
		blocked = true
	case reviewNotYetStamped:
		// Unstamped is already blocked above, on freshness and on stages. What that pair does not say is
		// WHY it will stay unstamped: with no establishable identity here, `stamp` refuses, so the reader
		// is sent to re-run four stages that refuse again, and the explanation reaches them only in
		// stamp's stderr. It goes here because this is where the human meets the block.
		if _, established := r.worktreeToken(); !established {
			r.errLines("NOTE: and it will stay unstamped — this worktree has no establishable identity (" + r.gitPath("idsd-worktree-id") + " is not writable), which is what stamp refuses on. Re-running the stages will not help until that path is writable.")
		}
	case thisWorktreeHasNoIdentity:
		r.errLines("BLOCK (freshness): this tree's identity could not be established (" + r.gitPath("idsd-worktree-id") + " is not writable), so whether this worktree is the one that was reviewed is unknown. Fix that path, then re-qualify. " + overridableByAHumanOnly)
		blocked = true
	case reviewedInAnotherWorktree:
		r.errLines("BLOCK (freshness): this pass was reviewed in another worktree (" + shell.Oneline(fieldValue(r.report, "reviewed-worktree")) + ", not " + mine + " " + shell.Oneline(r.currentWorktreePath()) + ")." + sameTree + " If that path is gone or renamed, the worktree it named is gone too — re-qualify here. " + overridableByAHumanOnly)
		blocked = true
	}
	return blocked
}

// Whether the stage record says a full pass stands behind this report.
func (r *run) blocksOnStages() bool {
	stages := r.reviewedStages()
	trims := r.fastTrims()
	switch {
	case isUnstamped(stages):
		r.errLines("BLOCK (stages): no reviewed-stages record — run a full qualify (it stamps the stage set). " + overridableByAHumanOnly)
		return true
	case trims != "":
		r.errLines("BLOCK (stages): trimmed for turnaround (" + trims + ") — run a full qualify before merge. " + overridableByAHumanOnly)
		return true
	}
	return false
}

// The one reason that cannot be overridden at all, so a scan that did not run has to block as loudly as
// items found.
func (r *run) blocksOnOpenTodos() bool {
	// 0 = nothing open, 1 = items printed. Anything else yields empty output, which the test below
	// would read as "no open TODOs" and pass the merge gate on a scan that never ran.
	todos, status := r.runTodoGate()
	switch {
	case status > 1:
		r.errLines("BLOCK (open TODOs): the scan did not run — todo-gate.sh exited " + strconv.Itoa(status) + ". Fix the invocation; this one cannot be overridden.")
		return true
	case todos != "":
		r.errLines("BLOCK (open TODOs): clear each before merge; this one cannot be overridden.")
		r.errLines(todos)
		return true
	}
	return false
}

func (r *run) cmdCarry() {
	r.requireReport(r.arg(1))
	r.readOpenTodos("prior open items are unknown.")
	if r.openTodos != "" {
		r.line("%s", r.openTodos)
	}
}

func (r *run) cmdState() {
	resolved := r.resolveReport(r.arg(1))
	// Several open ships have no single state, and `continue` must not act on one of them picked at
	// random. `list` is what answers here, and the message names it.
	if resolved == reportAmbiguous {
		r.refuseAmbiguous("no single state. Run report.sh list, then report.sh state <intent>")
	}
	// The archive is read BEFORE the report's absence decides, because `close` retires a landed ship's
	// report and the archived intent file is then the only record that it landed. Absence alone routes
	// `continue <intent>` to "start ship <intent>" — a rebuild of work already merged.
	if resolved == reportResolved && shell.IsRegularFile(r.idsdDir+"/archive/"+stemOfReportPath(r.report)+".md") {
		r.line("done")
		r.exit(0)
	}
	if resolved != reportResolved || !shell.IsRegularFile(r.report) {
		r.line("no-report")
		r.exit(0)
	}
	// `state` resolves for itself rather than through requireReport, because a missing report is a
	// token here and a refusal there. Unreadable is neither: it is a report whose state is unknown.
	r.assertReportIsReadable("its state is unknown (permissions?), and 'resume' is what an unread report looks like")
	r.line("%s", r.stateToken())
}

// The `continue` routing token for the report the caller resolved. Every path either returns a token
// or refuses: a token it cannot stand behind would route `continue` past a live gate.
func (r *run) stateToken() string {
	// A built intent's file has moved to archive/ (a standalone `review:` has no slug, so this is
	// skipped).
	if slug := r.intentSlug(); slug != "" && shell.IsRegularFile(r.idsdDir+"/archive/"+slug+".md") {
		return "done"
	}
	reviewed := r.reviewedTree()
	if isUnstamped(reviewed) {
		// Never stamped, or invalidated mid-pass → quality stages haven't completed.
		return "resume"
	}
	current, ok := r.currentTreeCached(r.errOut)
	if !ok {
		r.exit(2)
	}
	if current != reviewed {
		return "re-qualify" // reviewed once, tree moved since
	}
	// Same tree, and the review still has to be this worktree's — a sibling, an unestablishable
	// identity, and a stamped tree with no worktree line all read as "not verifiably here", so each
	// routes to re-qualify rather than letting a worktree resume a ship it never ran.
	if vouch, _ := r.worktreeVouch(); vouch != vouchesForThisWorktree {
		return "re-qualify"
	}
	r.readOpenTodos("the state is unknown.")
	if r.openTodos != "" {
		return "decide" // quality done, tree fresh, open `- [ ]` remain
	}
	if isUnstamped(r.reviewedStages()) || r.fastTrims() != "" {
		// Stages trimmed (or unrecorded) and fresh, nothing open — a full qualify remains.
		return "finalize"
	}
	return "ready" // full-reviewed, tree fresh, nothing open → merge-ready
}

// One line per open ship, so `continue` can route with several in flight.
func (r *run) cmdList() {
	names := r.reportNames()
	r.noteUnnameableReports()
	if len(names) == 0 {
		r.line("no reports")
		r.exit(0)
	}
	// Primed before the loop so every ship is scored against the same tree. Failing to prime is not
	// fatal — an unstamped ship answers without the tree at all, and refusing here would let one
	// unreadable file anywhere in the repo silence the whole listing. Each stateToken that does need
	// the tree retries and refuses on its own behalf, which is why this read is silent.
	r.currentTreeCached(io.Discard)
	// Built whole, then printed. Printing as it goes puts the ships already reached on stdout before a
	// later one refuses, and a truncated listing reads exactly like a complete one.
	listing := ""
	for _, name := range names {
		r.setReportPaths(name)
		r.assertReportIsReadable("nothing was printed, this listing included")
		// A refusal inside stateToken exits the process rather than returning a blank token, so this
		// needs no guard against one.
		listing += name + "\t" + r.stateToken() + "\n"
	}
	writeAll(r.out, listing)
}
