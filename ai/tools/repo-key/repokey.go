// A stable name for one clone: `<basename>-<digest>`, readable and unique. Two things key directories
// off it — the idsd scratch directory under a machine-local override root, and the owner's worktree
// directory — so both call FromSharedGitDir, and one shared git dir answers to one name whoever asks.
//
// What is shared is that function, not the finding of the git dir: a consumer that resolves its own
// path decides for itself what may relocate it.
package repokey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kk-flavor/tools/shell"
)

// How much of the realpath digest goes into a key. Six hex is 16.7M values over the handful of clones
// one machine holds, which puts an accidental collision around 5e-5 at a hundred of them. It is not a
// security boundary. Widening it renames every directory the consumers have already created, which is
// the cost that keeps it at six.
const digestLength = 6

// The key for the clone whose SHARED git dir is at `shared` — the answer to `rev-parse
// --git-common-dir`, never `--git-path` or `--show-toplevel`.
//
// The basename comes from that directory's PARENT. `--show-toplevel` answers the worktree's own
// directory, so a key built from it hands every worktree a different key: reaching for it looks like
// a fix and is the bug.
//
// The digest is over that git dir's own realpath, the only sound identity available: two clones of one
// repository share a remote URL and often a basename, and never a git-dir realpath.
func FromSharedGitDir(shared string) (string, error) {
	canonical := shell.CanonicalDir(shared)
	if canonical == "" {
		// Not a fallback to the unresolved path: that would key one clone two ways depending on how the
		// caller reached it, and each name would hold half the work.
		return "", errors.New("could not resolve " + shell.Oneline(shared) + " to a real path")
	}
	// An argument that is not a git dir still has a parent and still hashes, so without this a wrong
	// path answers a well-formed key for a directory nobody meant. HEAD is the probe because every git
	// dir holds one, worktrees and bare repositories included.
	if !shell.IsRegularFile(canonical + "/HEAD") {
		return "", errors.New(shell.Oneline(shared) + " is not a git directory (no HEAD in it) — pass the answer to `rev-parse --git-common-dir`")
	}
	digest := sha256.Sum256([]byte(canonical))
	readable := safeName(shell.BaseName(shell.DirName(canonical)))
	return readable + "-" + hex.EncodeToString(digest[:])[:digestLength], nil
}

// The readable half, reduced to what is safe to splice into a path or a command line. A clone's
// directory name is whatever its owner typed — a space, a glob, a newline, a leading dash — and one
// consumer splices the key into a `git worktree add` line a human reads and runs.
func safeName(name string) string {
	var safe strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			safe.WriteRune(r)
		default:
			safe.WriteByte('-')
		}
	}
	// Leading dashes and dots separately: a name starting with a dash reads as an option wherever the
	// key reaches a command, and one starting with a dot makes the directory hidden. Trimming either
	// cannot collide two clones that the digest keeps apart.
	flattened := strings.TrimLeft(safe.String(), "-.")
	if flattened == "" {
		return "repo"
	}
	return flattened
}

// The key for the clone containing `root`, asking git where the shared git dir is. For a caller that
// holds no resolved path of its own.
func Resolve(root string) (string, error) {
	command := exec.Command("git", "rev-parse", "--git-common-dir")
	command.Dir = root
	// Dir alone does NOT select the repository: git reads its location from the environment first, so an
	// inherited GIT_DIR wins over the path this was handed. A hook in a linked worktree is given one, so
	// it would key its own clone, and a consumer would create, write and later remove directories under
	// that name.
	command.Env = withoutGitLocation(os.Environ())
	out, err := command.Output()
	if err != nil {
		return "", errors.New("could not ask git for the shared git dir of " + shell.Oneline(root) + " (git rev-parse --git-common-dir)")
	}
	shared := strings.TrimSpace(string(out))
	// `--git-common-dir` answers relative to the caller in an ordinary repo — a bare `.git` — so a
	// relative answer would resolve against whatever directory the next caller happened to stand in.
	if !filepath.IsAbs(shared) {
		shared = filepath.Join(root, shared)
	}
	return FromSharedGitDir(filepath.Clean(shared))
}

// The environment with the two variables that relocate git's idea of the repository removed. Exactly
// two: only GIT_DIR and GIT_COMMON_DIR point `rev-parse --git-common-dir` at another repository.
// GIT_WORK_TREE moves `--show-toplevel` but not this, and GIT_OBJECT_DIRECTORY and
// GIT_DISCOVERY_ACROSS_FILESYSTEM move neither, so stripping any of them would read as a guard while
// guarding nothing.
//
// GIT_CEILING_DIRECTORIES stays in the caller's environment on purpose. Set on the repository's own
// root it stops discovery rather than redirecting it, so honouring it costs a refusal and never a
// wrong key, and a refusal is what this tool is for. `eco-report/layout.go` reads it the same way.
func withoutGitLocation(env []string) []string {
	relocates := map[string]bool{"GIT_DIR": true, "GIT_COMMON_DIR": true}
	kept := make([]string, 0, len(env))
	for _, entry := range env {
		if name, _, found := strings.Cut(entry, "="); !found || !relocates[name] {
			kept = append(kept, entry)
		}
	}
	return kept
}
