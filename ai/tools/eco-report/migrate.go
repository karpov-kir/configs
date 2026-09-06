package ecoreport

import (
	"os"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// What the older in-tree layout left behind, and what this does about it. Throwaway scratch used to
// sit at `<repo>/.idsd`, hidden from `git add -A` by a rule in `.git/info/exclude` — so a repo that
// ever ran a throwaway ship carries both a directory and a rule for a location nothing writes any
// more. Neither is decided here; root.go decides where the scratch lives, and this reconciles what the
// last decision left.
//
// check-ignore reaches both, and it runs before anything else in every pass, so a repo cleans itself
// up the first time a ship touches it and no migration script has to be remembered. `promote` clears
// the rule again on its own path, because reading through a rule that hides untracked files under
// .idsd/ is what makes a promotion fail for a reason that blames the human.

// Throwaway mode holds nothing in the tree, so an in-tree `.idsd/` here is either a directory left by
// the older layout or one a human made by hand. Its content is the only copy of whatever it holds, so
// this refuses and says what to do rather than moving or merging anything.
//
// An empty directory is removed instead: it holds nothing to lose, and left standing it is a trace in
// the mode whose whole contract is leaving none.
func (r *run) reconcileTreeIdsdDir() {
	tree := r.treeIdsdDir()
	if r.idsdDir == tree || !shell.PathExists(tree) {
		return
	}
	if shell.IsSymlink(tree) {
		r.refuse("error: "+tree+" is a symlink -> "+shell.Oneline(readLink(tree))+" — nothing was read or written.",
			"  The scratch directory now lives at "+r.idsdDir+"; remove the link, then re-run.")
	}
	// Files, not directory entries. Moving the content out leaves `intents/` and `archive/` standing as
	// empty directories, and counting entries reads those as content — so a repo whose move is finished
	// is refused, with a message saying it holds content when it holds none.
	count, sample, err := filesUnder(tree)
	if err != nil {
		r.refuse("error: could not read "+tree+" ("+err.Error()+") — whether it holds anything is unknown, so nothing was read or written.",
			"  The scratch directory now lives at "+r.idsdDir+".")
	}
	if count == 0 {
		// RemoveAll, because what is left is the empty directory skeleton the move left behind rather
		// than a single empty dir. It holds nothing to lose: the walk above found no file under it.
		_ = os.RemoveAll(tree)
		return
	}
	r.refuse("error: "+tree+" still holds "+strconv.Itoa(count)+" file(s), and throwaway idsd scratch no longer lives in the tree — nothing was read or written.",
		"  The scratch directory is now "+r.idsdDir+", shared by every branch and worktree of this clone.",
		"  Still there: "+shell.Oneline(strings.Join(sample, " "))+sampleTail(count, len(sample)),
		"  Nothing here moves your files for you: copy what you still want into that directory, delete the rest of "+tree+", then re-run.")
}

// The line the old in-tree layout wrote into `.git/info/exclude` to hide the scratch from
// `git add -A`.
const staleExclusionEntry = ".idsd/"

// Remove that rule.
//
// Stale in both modes, so every caller may run it: throwaway writes nothing in the tree to hide, and
// in committed mode the rule hides a new intent from git. The file is shared across every worktree,
// and no worktree needs the rule any more.
//
// A failure here is reported rather than refused on: no correctness rests on the entry, so it must not
// block a pass. What it must not do is fail quietly.
func (r *run) removeStaleExclusion() {
	exclude := r.gitCommonPath("info/exclude")
	if !shell.IsRegularFile(exclude) {
		return
	}
	content, err := os.ReadFile(exclude)
	if err != nil {
		r.errLines("note: could not read " + exclude + " — a stale '" + staleExclusionEntry + "' rule may still be there; nothing was changed")
		return
	}
	var kept []string
	removed := 0
	for _, line := range shell.SplitLines(string(content)) {
		if line == staleExclusionEntry {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return
	}
	// Renamed over rather than written in place: an unwritable .git/info must fail without truncating
	// the file it could not replace, which holds the human's own rules.
	temp, err := os.CreateTemp(shell.DirName(exclude), ".exclude.")
	if err != nil {
		r.errLines("note: could not rewrite " + exclude + " — the stale '" + staleExclusionEntry + "' rule is still there")
		return
	}
	_, err = temp.Write(joinRecords(kept))
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = moveFile(temp.Name(), exclude)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
		r.errLines("note: could not replace " + exclude + " (" + err.Error() + ") — the stale '" + staleExclusionEntry + "' rule is still there")
		return
	}
	r.line("cleaned: removed the stale '%s' rule from %s (%d line(s)) — nothing writes there any more",
		staleExclusionEntry, exclude, removed)
}
