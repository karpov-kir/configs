package density

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	gitrepo "configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

const barDidNotRun = " — the bar did NOT run"

func refusal(what string) error {
	return errors.New(what + barDidNotRun)
}

// A bad revision, an unborn HEAD and a missing object all fail the same call, and one sentence for the
// three sends a reader looking in the wrong place. The port's error already carries git's own words,
// which is what lets git's account ride under the refusal.
func gitRefusal(what string, err error) error {
	if reason := strings.TrimSpace(err.Error()); reason != "" {
		return errors.New(what + barDidNotRun + "\n  git said: " + reason)
	}
	return refusal(what)
}

// hostRepo is the repository under review; every path below is relative to root. cwd is where the caller
// ran, and the one place a pathspec they passed after `--` is relative to.
type hostRepo struct {
	git      gitrepo.Git
	root     string
	cwd      string
	maxBytes int64
	// The revision whole-file content is read at, empty for the working tree. Set only for a closed
	// range; see contentRevision.
	contentRev string
}

// git names a diff's files from the repository's top whatever directory it ran in, so reading them
// against cwd from a subdirectory finds none of them and the change set silently shrinks to its
// untracked half.
func newHostRepo(git gitrepo.Git, cwd string, maxBytes int64) (hostRepo, error) {
	root, err := git.TopLevel(cwd)
	if err != nil {
		return hostRepo{}, refusal(cwd + " is not inside a git repository")
	}
	return hostRepo{git: git, root: root, cwd: cwd, maxBytes: maxBytes}, nil
}

// splitPathspec divides the arguments at `--`. RefuseNonRevisions tells a caller to put paths after it,
// so this is the form the tool itself asks for; left among the revisions, `--` becomes the base every
// listing fails on.
func splitPathspec(args []string) (revisions, pathspec []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func (h hostRepo) hasCommit() bool {
	id, err := h.git.Resolve(h.root, "HEAD")
	return err == nil && id != ""
}

// Sorted, because a change set is two listings appended and a report over it must not depend on which
// order they arrived in. A discovery path must not read another repository's source copied in as a
// fixture, which is what notThisRepositorysSource holds out.
func sourcesOf(paths []string) []string {
	kept := make([]string, 0, len(paths))
	for _, name := range paths {
		if !notThisRepositorysSource(name) {
			kept = append(kept, name)
		}
	}
	sort.Strings(kept)
	return kept
}

// The tracked listing, not a filesystem walk: a walk would pull in vendored trees and build output
// nobody here commented. With content pinned to a revision the listing moves there too — a list taken
// from today's index names files that revision never held, and every one of them reads as unreadable and
// leaves the baseline without a word, which is the defect this pinning exists to end, moved one step.
func (h hostRepo) trackedSources() ([]string, error) {
	if h.contentRev != "" {
		held, err := h.git.NamesAt(h.root, h.contentRev)
		if err != nil {
			return nil, gitRefusal("could not list the files at "+h.contentRev, err)
		}
		return sourcesOf(held), nil
	}
	tracked, err := h.git.Tracked(h.root)
	if err != nil {
		return nil, gitRefusal("could not list the repo's tracked files", err)
	}
	return sourcesOf(tracked), nil
}

// changedSources is the set the bar judges: every source file the diff names, deleted ones aside, as it
// sits in the working tree. Named by the changed listing, not by added lines: the pass this mode serves
// cuts comments, and a file the change only deleted from would otherwise leave the set and join the
// baseline. Untracked files join only with no revisions, as in Run; without them a set of new files
// reads as empty. A narrowed listing is asked from cwd, where its pathspec is relative to. An unnarrowed
// one is asked at root: the untracked listing names only what sits under the directory git ran in, and
// from a subdirectory the untracked files elsewhere in the tree would silently leave the set.
func (h hostRepo) changedSources(revisions, pathspec []string) ([]string, error) {
	dir := h.root
	if len(pathspec) > 0 {
		dir = h.cwd
	}
	named := revisions
	if len(named) == 0 {
		named = []string{"HEAD"}
	}
	paths, err := h.git.Changed(dir, named, pathspec)
	if err != nil {
		// A repository with no commit fails the same diff, and "rejected these arguments" would send its
		// reader to arguments they never passed.
		if !h.hasCommit() {
			return nil, refusal("this repository has no commit yet, so no file outside the change can set a rate")
		}
		return nil, gitRefusal("git rejected these arguments", err)
	}
	if len(revisions) == 0 {
		untracked, err := h.git.Untracked(dir, pathspec...)
		if err != nil {
			return nil, gitRefusal("could not list untracked files", err)
		}
		paths = append(paths, untracked...)
	}
	return sourcesOf(paths), nil
}

// newSinceBase is the changed files the diff's base did not hold: the ones perFileCeiling judges on
// their own.
func (h hostRepo) newSinceBase(revisions, changed []string) (map[string]bool, error) {
	base, err := h.baseRevision(revisions)
	if err != nil {
		return nil, err
	}
	held, err := h.git.NamesAt(h.root, base)
	if err != nil {
		return nil, gitRefusal(fmt.Sprintf("could not list the files at %s", base), err)
	}
	return setOf(without(changed, held)), nil
}

func setOf(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func without(names, excluded []string) []string {
	skip := setOf(excluded)
	kept := make([]string, 0, len(names))
	for _, name := range names {
		if !skip[name] {
			kept = append(kept, name)
		}
	}
	return kept
}

// baseRevision is the commit `git diff <revisions>` compared the working tree or the second revision
// against: HEAD with none, the left side of `a..b` or `a b`, and the merge base of `a...b`.
func (h hostRepo) baseRevision(revisions []string) (string, error) {
	if len(revisions) == 0 {
		return "HEAD", nil
	}
	first := revisions[0]
	if left, right, symmetric := strings.Cut(first, "..."); symmetric {
		if left == "" {
			left = "HEAD"
		}
		if right == "" {
			right = "HEAD"
		}
		base, err := h.git.MergeBase(h.root, left, right)
		if err != nil {
			return "", gitRefusal(fmt.Sprintf("%s and %s have no merge base", left, right), err)
		}
		return base, nil
	}
	left, _, _ := strings.Cut(first, "..")
	if left == "" {
		return "HEAD", nil
	}
	return left, nil
}

// readCappedInTree reads a file from the working tree. Lstat first, so a symlink is skipped rather than
// followed: the paths come from the branch under review, and a link it plants at a file outside the
// repository would otherwise be read from the reviewer's machine and its line counts reported.
func (h hostRepo) readCappedInTree(rel string) (string, bool) {
	path := shell.Join(h.root, rel)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > h.maxBytes {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// The revision whole-file content belongs to, or empty when that is the working tree. Only a closed
// range pins content to a commit: `a..b` and `a...b` both ask about what the right-hand side holds, an
// empty side meaning HEAD. Everything else — no revision, or a single one — measures work the tree still
// holds, and reading a commit there would drop exactly the uncommitted lines the caller is asking about.
func contentRevision(revisions []string) string {
	switch {
	case len(revisions) == 0:
		return ""
	// `git diff a b` is the same closed comparison as `a..b`, and baseRevision already reads it that
	// way, so its content is b. Three or more is git's combined-merge form, whose content is the merge
	// named first rather than any parent — left on the tree rather than read off the wrong side.
	case len(revisions) == 2:
		return revisions[1]
	case len(revisions) > 2:
		return ""
	}
	// `...` first: it contains `..`, so cutting on the shorter separator would read a symmetric range's
	// right side as ".b" and resolve nothing.
	right, closed := "", false
	if _, after, found := strings.Cut(revisions[0], "..."); found {
		right, closed = after, true
	} else if _, after, found := strings.Cut(revisions[0], ".."); found {
		right, closed = after, true
	}
	if !closed {
		return ""
	}
	if right == "" {
		return "HEAD"
	}
	return right
}
