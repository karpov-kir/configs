package ecoreport

import (
	"io"
	"strconv"
)

// The two scripts this tool calls rather than reimplements. Both own a recipe that is dangerous to
// get half right, and both are located by the same rule the shell version used, so a copied skill
// directory still resolves its own.

// The open-item scan. 0 = nothing open, 1 = items on stdout, anything else = the scan did not run,
// and its output is then empty — which read as "nothing open" would pass the merge gate on a scan
// that never happened.
//
// Checked before it is run, the way currentTree checks the fingerprint script below. Both seams answer
// the same question and answering it two ways is the defect: `todoGate` is built from argv[0], so a
// slash-less one resolves it against the invocation's directory rather than the skill's, and what the
// caller then reads is a bare exit status with no name in it. The status alone already fails closed —
// every reader here treats anything above 1 as "did not run" — so this buys the reason, not the
// refusal.
func (r *run) runTodoGate() (string, int) {
	if !isExecutable(r.todoGate) {
		errLinesTo(r.errOut,
			"error: "+r.todoGate+" is missing or not executable — the open-item scan did not run.",
			"  It is located from this program's own path, so an invocation that renamed argv[0] resolves it somewhere else entirely.")
		return "", 2
	}
	return r.capture(r.errOut, r.todoGate, r.report)
}

// The report's open `- [ ]`, for every caller that must refuse rather than read a failed scan as
// "nothing open". The consequence is the caller's, since what is then unknown differs at each one.
func (r *run) readOpenTodos(consequence string) {
	items, status := r.runTodoGate()
	if status > 1 {
		r.refuse("error: the open-item scan did not run — todo-gate.sh exited " + strconv.Itoa(status) + "; " + consequence)
	}
	r.openTodos = items
}

func (r *run) openItemsPhrase() string {
	items, status := r.runTodoGate()
	switch status {
	case 0:
		return "no open '- [ ]'"
	case 1:
		return strconv.Itoa(countPrintedLines(items)) + " open '- [ ]'"
	}
	return "an unknown number of open '- [ ]' — the scan did not run (todo-gate.sh exited " + strconv.Itoa(status) + ")"
}

// The fingerprint the freshness gate compares. Fails loudly rather than printing an empty tree, which
// would match the next equally-failed reading and report "tree fresh" — and loudly rather than
// falling back to a local recipe.
func (r *run) currentTree(errOut io.Writer) (string, bool) {
	if !isExecutable(r.fingerprintBin) {
		errLinesTo(errOut,
			"error: "+r.fingerprintBin+" is missing or not executable — the tree could not be fingerprinted.",
			"  It owns the fingerprint recipe; there is deliberately no local fallback, because a second copy is what put untracked working files in .git/objects for good.")
		return "", false
	}
	// Its stderr is the caller's, so git's own account of a failed walk reaches them.
	tree, status := r.capture(errOut, r.fingerprintBin, r.root)
	if status != 0 || tree == "" {
		errLinesTo(errOut, "error: "+r.fingerprintBin+" exited "+strconv.Itoa(status)+" without a tree — the tree could not be fingerprinted")
		return "", false
	}
	return tree, true
}

// One fingerprint per invocation. `list` scores every report against the same working tree, so the
// walk currentTree does is the same walk each time. Only a success is cached, so a failed reading
// reports itself again for the next caller rather than going quiet after the first.
func (r *run) currentTreeCached(errOut io.Writer) (string, bool) {
	if r.cachedTree != "" {
		return r.cachedTree, true
	}
	tree, ok := r.currentTree(errOut)
	if ok {
		r.cachedTree = tree
	}
	return tree, ok
}
