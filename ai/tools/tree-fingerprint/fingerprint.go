// A fingerprint of the working tree — tracked and untracked content, ignored paths and untracked
// nested repositories excluded — so a ledger can name the tree it was written against
// (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**).
//
// Go rather than shell, and importable rather than only runnable, because the caller that runs this
// most is another Go tool. A spawn cost a bash, an mktemp and a rev-parse on top of the git calls that
// do the actual work; in process those calls are all that is left.
//
// Those calls stay git. Building a tree object means hashing every file the ignore rules admit and
// writing them into the object store in git's own format — reimplementing that would be a second
// answer to "what is in this tree", and the whole point of a fingerprint is that it is the same answer
// every time.
package treefingerprint

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fingerprint answers the tree hash for the repository at root.
//
// The error is what the caller prints, so each says which way it could not be answered rather than
// that it failed: a fingerprint that did not run must never be mistaken for one that came back the
// same as last time, which is a stale ledger passing as a valid resume point.
func Fingerprint(root string) (string, error) {
	if root == "" {
		root = "."
	}
	if !isRepository(root) {
		return "", fmt.Errorf("not a git repository: %s", root)
	}

	// Private scratch for everything this run writes. The index goes here rather than in a shared temp
	// file because git recreates that file at 0644, and on a shared /tmp that hands every path in the
	// tree to any local user. MkdirTemp is 0o700.
	scratch, err := os.MkdirTemp("", "tree-fingerprint")
	if err != nil {
		return "", errors.New("could not create a temporary directory")
	}
	defer os.RemoveAll(scratch)

	// `add -A` hashes every untracked, un-ignored file into the object store. No ref points at those
	// blobs, so nothing ever collects them: fingerprinting a tree would leave the caller's working
	// files, a live credential among them, recoverable from `.git/objects` for good. They go to a
	// throwaway store instead. No git call below READS an object, so the real store needs no alternate
	// and the hash is the same.
	//
	// Git will not create GIT_OBJECT_DIRECTORY, and a missing one fails repository discovery itself
	// ("fatal: not a git repository"), so this mkdir is load-bearing.
	objects := filepath.Join(scratch, "objects")
	if err := os.MkdirAll(objects, 0o700); err != nil {
		return "", errors.New("could not create the throwaway object store")
	}
	// Nothing creates the index path: git rejects an existing 0-byte file ("index file smaller than
	// expected"), so it must not be pre-made.
	index := filepath.Join(scratch, "index")

	// This seed is not an optimisation. Git applies ignore rules only to paths the index does not
	// already hold, so an index built from nothing treats every tracked file as untracked — and a
	// tracked file matching an ignore rule is then dropped from the walk entirely. Such a file could be
	// rewritten between two runs with the fingerprint unmoved: the skill reads the ledger head as
	// matching, resumes, and skips every file it believes already has a verdict.
	//
	// Seeded from HEAD rather than from the caller's index, which this must never read or write. No
	// commit means nothing is tracked and there is nothing to seed, so an unborn HEAD is not a failure;
	// a HEAD that resolves and still cannot be read is, because the walk would then silently miss
	// everything committed. `read-tree` writes only the index, so it takes no throwaway object store —
	// and it needs the real one, where HEAD's trees live.
	if hasHead(root) {
		if _, err := run(root, []string{"GIT_INDEX_FILE=" + index}, "read-tree", "HEAD"); err != nil {
			return "", fmt.Errorf("could not read HEAD into the throwaway index for %s: %w", root, err)
		}
	}

	env := []string{"GIT_INDEX_FILE=" + index, "GIT_OBJECT_DIRECTORY=" + objects}

	// Asked of the seeded index, so "untracked" means here what it will mean to the walk below. A
	// declared submodule is then one HEAD holds, never one the caller happens to have staged.
	nested, err := untrackedRepositories(root, env)
	if err != nil {
		return "", fmt.Errorf("could not list the untracked paths in %s: %w", root, err)
	}

	if _, err := run(root, env, addArgs(nested)...); err != nil {
		return "", fmt.Errorf("could not fingerprint the tree in %s: %w", root, err)
	}
	tree, err := run(root, env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("could not fingerprint the tree in %s: %w", root, err)
	}
	if tree == "" {
		return "", fmt.Errorf("git wrote no tree for %s", root)
	}
	return tree, nil
}

// The untracked repositories sitting inside this one, as repository-relative paths.
//
// Another repository's state is not this tree's content, and `add -A` disagrees twice over. It records
// one as a gitlink holding that repository's HEAD, so the fingerprint moves the moment anyone commits
// in there and moves for nothing else that happens in there — a ledger stamped with it cannot tell
// "this tree changed" from "a session working next door committed". And one with no commit yet aborts
// the whole `add` ("does not have a commit checked out"), so no fingerprint can be taken at all while
// a sibling worktree is being set up. Both reach this repository through the worktrees agent sessions
// open inside a checkout.
//
// Untracked ones only: a gitlink HEAD already holds is a declared submodule, which IS part of what
// this repository tracks, and its pointer moving is a change to this tree.
//
// `--others` will not walk into another repository and prints the directory itself with a trailing
// slash, where every other untracked directory comes back as the files inside it — so the trailing
// slash is the answer, and nothing here goes looking for a `.git`. `-z` because a path holding a
// newline would otherwise arrive as two paths, and it also turns off the quoting `core.quotePath`
// applies. `--full-name` and `:/` because root may be a subdirectory, while the walk this feeds covers
// the whole repository either way.
func untrackedRepositories(root string, env []string) ([]string, error) {
	out, err := run(root, env, "ls-files", "--others", "--exclude-standard", "--full-name", "-z", "--", ":/")
	if err != nil {
		return nil, err
	}
	var nested []string
	for _, path := range strings.Split(out, "\x00") {
		if strings.HasSuffix(path, "/") {
			nested = append(nested, strings.TrimSuffix(path, "/"))
		}
	}
	return nested, nil
}

// `add -A` over the whole repository, with each nested repository held out of it.
//
// `:/` states the scope a bare `add -A` already has, because an exclude pathspec needs something
// positive beside it. `literal` so a nested repository named `wt*` holds out itself and not every
// sibling its name globs over, and `top` because these paths are named from the repository root while
// git would otherwise read them from root's own directory.
func addArgs(nested []string) []string {
	args := []string{"add", "-A", "--", ":/"}
	for _, path := range nested {
		args = append(args, ":(exclude,top,literal)"+path)
	}
	return args
}

// A repository is one with a `.git` at the root or above it. Read rather than asked, because
// `rev-parse --show-toplevel` was a whole child process to answer a question the filesystem shows —
// and this is the check that runs even on the paths that then refuse.
func isRepository(root string) bool {
	probe, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if info, err := os.Stat(probe); err != nil || !info.IsDir() {
		return false
	}
	for {
		if _, err := os.Lstat(filepath.Join(probe, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return false
		}
		probe = parent
	}
}

// Whether HEAD names a commit. An unborn HEAD is the ordinary state of a repository with no commit,
// and not a failure — see the seed comment above for why the difference matters.
func hasHead(root string) bool {
	_, err := run(root, nil, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

// One git call. stderr is captured rather than inherited, because a caller may read the hash through
// a combined stream and git's warnings would land in it; a warning that did not stop the walk does not
// change the answer, so a successful run drops it and a failed one carries it in the error.
func run(root string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", errors.New(strings.TrimRight(string(exit.Stderr), "\n"))
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}
