// A fingerprint of the working tree — tracked and untracked content, ignored paths excluded — so a
// ledger can name the tree it was written against (`~/.kk-flavor/standards/skill-protocol.md` →
// **Queue**).
//
// Go rather than shell, and importable rather than only runnable, because the caller that runs this
// most is another Go tool: `eco-report` spawned the script about 110 times across its suite, and each
// spawn cost a bash, an mktemp and a rev-parse on top of the three git calls that do the actual work.
// In process those three are all that is left.
//
// The three git calls stay git. Building a tree object means hashing every file the ignore rules admit
// and writing them into the object store in git's own format — reimplementing that would be a second
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
	// throwaway store instead. Both git calls below only WRITE objects, so the real store needs no
	// alternate and the hash is the same.
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
	// rewritten between two runs with the fingerprint unmoved, which is a stale ledger passing as a
	// valid resume point: the skill reads the head as matching, resumes, and skips every file it
	// believes already has a verdict.
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
	if _, err := run(root, env, "add", "-A"); err != nil {
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
