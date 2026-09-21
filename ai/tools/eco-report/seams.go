package ecoreport

import (
	"io"
	"strconv"

	"configs/ai/tools/shell"
)

// This package owns every seam except the tree fingerprint, which belongs to
// `ai/tools/tree-fingerprint/` and runs in process. The code here calls that package instead of
// reimplementing it, and newRun, the constructor, records what recomputing the fingerprint costs. The
// open-item scan belongs to this package now, in openitems.go.

// The open-item scan over this run's own report.
func (r *run) reportOpenItems() (string, error) {
	return openItemsIn(r.report)
}

// The report's open `- [ ]`, for every caller that must refuse rather than read a failed scan as
// "nothing open". The consequence is the caller's, since what is then unknown differs at each one.
//
// Every one of those callers resolves its report through requireReport, which probes the file first.
// This refusal is the backstop behind that probe. Drop the probe and `carry` prints an empty item
// list for a report it failed to open, which reads exactly like a report with no open items.
func (r *run) readOpenTodos(consequence string) {
	items, err := r.reportOpenItems()
	if err != nil {
		r.refuse("error: the open-item scan did not run — " + shell.Oneline(err.Error()) + "; " + consequence)
	}
	r.openTodos = items
}

// Whether anything is open in either file the merge gate reads. The routing token has to ask the gate's
// own question: a `ready` over a `- [ ]` sitting in the ICE's `## Follow-ups` routes the human to a merge
// the gate then refuses. `carry` deliberately does not use this — the report's items are what a
// re-qualify must not lose, and the ICE carries its own to the build that owns them.
func (r *run) anyOpenItemsBeforeMerge(consequence string) bool {
	r.readOpenTodos(consequence)
	if r.openTodos != "" {
		return true
	}
	intent := r.intentFilePath()
	if intent == "" {
		return false
	}
	// No upstream step opens the intent file. intentFilePath, the path lookup, checks only that the
	// path is a regular file, excluding a symlink. This is the only open-item refusal a caller reaches
	// directly.
	items, err := openItemsIn(intent)
	if err != nil {
		r.refuse("error: the open-item scan of " + intent + " did not run — " + shell.Oneline(err.Error()) + "; " + consequence)
	}
	return items != ""
}

func (r *run) openItemsPhrase() string {
	items, err := r.reportOpenItems()
	switch {
	case err != nil:
		return "an unknown number of open '- [ ]' — the scan did not run (" + shell.Oneline(err.Error()) + ")"
	case items == "":
		return "no open '- [ ]'"
	}
	return strconv.Itoa(countPrintedLines(items)) + " open '- [ ]'"
}

// The fingerprint the freshness gate compares. Fails loudly rather than printing an empty tree, which
// would match the next equally-failed reading and report "tree fresh" — and loudly rather than
// falling back to a local recipe.
func (r *run) currentTree(errOut io.Writer) (string, bool) {
	if !isExecutable(r.fingerprintBin) {
		errLinesTo(errOut,
			"error: "+r.fingerprintBin+" is missing or not executable — the tree could not be fingerprinted.",
			"  The recipe itself runs in process, from ai/tools/tree-fingerprint/; this guard is about the install being complete. There is deliberately no local fallback, because a second copy is what put untracked working files in .git/objects for good.")
		return "", false
	}
	tree, err := r.fingerprint(r.root)
	if err != nil || tree == "" {
		reason := "returned no tree"
		if err != nil {
			reason = err.Error()
		}
		errLinesTo(errOut, "error: the tree could not be fingerprinted — "+reason)
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
