package density

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

const barDidNotRun = " — the bar did NOT run"

func refusal(what string) error {
	return errors.New(what + barDidNotRun)
}

// A bad revision, an unborn HEAD and a missing object all fail the same call, and one sentence for the
// three sends a reader looking in the wrong place. Output() keeps stderr on the ExitError, which is
// what lets git's own account ride under the refusal.
func gitRefusal(what string, err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if reason := strings.TrimSpace(string(exit.Stderr)); reason != "" {
			return errors.New(what + barDidNotRun + "\n  git said: " + reason)
		}
	}
	return refusal(what)
}

// hostRepo is the repository under review; every path below is relative to root. cwd is where the caller
// ran, and the one place a pathspec they passed after `--` is relative to.
type hostRepo struct {
	root     string
	cwd      string
	maxBytes int64
}

// git names a diff's files from the repository's top whatever directory it ran in, so reading them
// against cwd from a subdirectory finds none of them and the change set silently shrinks to its
// untracked half.
func newHostRepo(cwd string, maxBytes int64) (hostRepo, error) {
	out, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return hostRepo{}, refusal(cwd + " is not inside a git repository")
	}
	return hostRepo{root: strings.TrimSpace(string(out)), cwd: cwd, maxBytes: maxBytes}, nil
}

func gitOutput(dir string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
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
	_, err := gitOutput(h.root, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

// listSources keeps the source files a git listing prints. `-z` is the caller's to pass: without it git
// C-quotes a path holding a non-ASCII byte. dir is where git runs, and the paths it prints must still
// be root-relative: `diff` prints them so only with `--no-relative`, and `ls-files` only with
// `--full-name`.
func (h hostRepo) listSources(dir string, args ...string) ([]string, error) {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" && !isProseOrData(name) {
			paths = append(paths, name)
		}
	}
	return paths, nil
}

// `git ls-files`, not a filesystem walk: a walk would pull in vendored trees and build output nobody
// here commented.
func (h hostRepo) trackedSources() ([]string, error) {
	paths, err := h.listSources(h.root, "ls-files", "-z")
	if err != nil {
		return nil, gitRefusal("could not list the repo's tracked files", err)
	}
	sort.Strings(paths)
	return paths, nil
}

// changedSources is the set the bar judges: every source file the diff names, deleted ones aside, as it
// sits in the working tree. Named by `--name-only`, not by added lines: the pass this mode serves cuts
// comments, and a file the change only deleted from would otherwise leave the set and join the baseline.
// Untracked files join only with no revisions, as in Run; without them a set of new files reads as empty.
// A narrowed listing runs from cwd, where its pathspec is relative to. An unnarrowed one runs at root:
// `ls-files` lists only what sits under the directory git ran in, and from a subdirectory the untracked
// files elsewhere in the tree would silently leave the set.
func (h hostRepo) changedSources(revisions, pathspec []string) ([]string, error) {
	dir := h.root
	if len(pathspec) > 0 {
		dir = h.cwd
	}
	// `--no-relative` for the same reason `--no-ext-diff` is here: both override something the
	// reviewer's own git config can set. Under `diff.relative=true` a listing from a subdirectory
	// names `a.go` for `pkg/a.go` and drops every changed file outside that directory, so readCapped
	// finds nothing, the baseline keeps files the change touched, and the bar measures another tree.
	diffArgs := []string{"diff", "--name-only", "-z", "--no-ext-diff", "--no-relative", "--diff-filter=d"}
	if len(revisions) == 0 {
		diffArgs = append(diffArgs, "HEAD")
	} else {
		diffArgs = append(diffArgs, revisions...)
	}
	paths, err := h.listSources(dir, withPathspec(diffArgs, pathspec)...)
	if err != nil {
		// A repository with no commit fails the same diff, and "rejected these arguments" would send its
		// reader to arguments they never passed.
		if !h.hasCommit() {
			return nil, refusal("this repository has no commit yet, so no file outside the change can set a rate")
		}
		return nil, gitRefusal("git rejected these arguments", err)
	}
	if len(revisions) == 0 {
		listing := []string{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"}
		untracked, err := h.listSources(dir, withPathspec(listing, pathspec)...)
		if err != nil {
			return nil, gitRefusal("could not list untracked files", err)
		}
		paths = append(paths, untracked...)
	}
	sort.Strings(paths)
	return paths, nil
}

// withPathspec ends the listing in `--` whether or not a pathspec follows. Without it, a file named
// HEAD in the working tree makes `git diff HEAD` "ambiguous" and the bar exits 2. The branch under
// review can commit that file, switching the bar off for everyone who reviews it.
func withPathspec(args, pathspec []string) []string {
	return append(append(args, "--"), pathspec...)
}

// newSinceBase is the changed files the diff's base did not hold: the ones perFileCeiling judges on
// their own.
func (h hostRepo) newSinceBase(revisions, changed []string) (map[string]bool, error) {
	base, err := h.baseRevision(revisions)
	if err != nil {
		return nil, err
	}
	held, err := h.listSources(h.root, "ls-tree", "-r", "-z", "--name-only", base)
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
		out, err := gitOutput(h.root, "merge-base", left, right)
		if err != nil {
			return "", gitRefusal(fmt.Sprintf("%s and %s have no merge base", left, right), err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	left, _, _ := strings.Cut(first, "..")
	if left == "" {
		return "HEAD", nil
	}
	return left, nil
}

// Lstat, so a symlink is skipped rather than followed. The paths come from the branch under review,
// and a link it plants at a file outside the repository would otherwise be read from the reviewer's
// machine and its line counts reported.
func (h hostRepo) readCapped(rel string) (string, bool) {
	path := shell.Join(h.root, rel)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > h.maxBytes {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil || strings.IndexByte(string(raw), 0) >= 0 {
		return "", false
	}
	return string(raw), true
}
