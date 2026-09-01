package ecoreport

import (
	"io"
	"strconv"

	"kk-flavor/tools/shell"
)

// What the report is read for: the merge gate, the items a re-qualify must carry, and the routing
// token `idsd-ship continue` acts on.

func (r *run) cmdGate() {
	r.requireReport(r.arg(1))
	blocked := 0
	current, ok := r.currentTree(r.errOut)
	if !ok {
		r.exit(2)
	}
	reviewed := r.reviewedTree()
	if current != reviewed {
		if reviewed == "" {
			reviewed = "<unstamped>"
		}
		r.errLines("BLOCK (freshness): tree changed since last qualify (current " + current + " != reviewed " + reviewed + "). Re-qualify, or the human may explicitly override this one.")
		blocked = 1
	}
	stages := r.reviewedStages()
	trims := r.fastTrims()
	switch {
	case isUnstamped(stages):
		r.errLines("BLOCK (stages): no reviewed-stages record — run a full qualify (it stamps the stage set), or the human may explicitly override this one.")
		blocked = 1
	case trims != "":
		r.errLines("BLOCK (stages): trimmed for turnaround (" + trims + ") — run a full qualify before merge, or the human may explicitly override this one.")
		blocked = 1
	}
	// 0 = nothing open, 1 = items printed. Anything else yields empty output, which the test below
	// would read as "no open TODOs" and pass the merge gate on a scan that never ran.
	todos, status := r.runTodoGate()
	switch {
	case status > 1:
		r.errLines("BLOCK (open TODOs): the scan did not run — todo-gate.sh exited " + strconv.Itoa(status) + ". No override; fix the invocation.")
		blocked = 1
	case todos != "":
		r.errLines("BLOCK (open TODOs): clear each before merge — no override.")
		r.errLines(todos)
		blocked = 1
	}
	if blocked == 0 {
		r.line("gate clean: tree fresh, full qualify, no open TODOs")
	}
	r.exit(blocked)
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
	// the tree retries and refuses on its own behalf, which is why the priming reading is silent.
	r.currentTreeCached(io.Discard)
	// Built whole, then printed. Printing as it goes puts the ships already reached on stdout before a
	// later one refuses, and a truncated listing reads exactly like a complete one.
	listing := ""
	for _, name := range names {
		r.setReportPaths(name)
		r.assertReportIsReadable("nothing was printed, this listing included")
		// A refusal inside stateToken leaves through this loop, printing nothing. The shell version
		// ran it in a command substitution, where an exit killed only that subshell, and needed a
		// second guard here against the blank token that left behind; there is no subshell to lose it in.
		listing += name + "\t" + r.stateToken() + "\n"
	}
	writeAll(r.out, listing)
}
