// A stable name for one clone: `<basename>-<digest>`, readable and unique. Two things key directories
// off it — the idsd scratch directory under a machine-local override root, and the owner's worktree
// directory — so both call FromSharedGitDir, and one shared git dir answers to one name whoever asks.
//
// That name also abbreviates, and the abbreviation is the prefix a session title carries. It
// identifies no clone — two clones of one repository share it, and so do two repositories whose names
// start alike — and that is the point: a human reading a sidebar wants every session in one project
// under one short word.
//
// What is shared is that resolution, not the finding of the git dir: a consumer that resolves its own
// path decides for itself what may relocate it.
package repokey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// How much of the realpath digest goes into a key. Six hex is 16.7M values over the handful of clones
// one machine holds, which puts an accidental collision around 5e-5 at a hundred of them. It is not a
// security boundary. Widening it renames every directory the consumers have already created, which is
// the cost that keeps it at six.
const digestLength = 6

// How much of an abbreviation reaches a title. Only a name built from many runs, or a long
// digit-carrying one, is long enough to hit it.
const abbrevLength = 7

// What a name with nothing usable left in it answers to. One literal for both projections, because
// two would drift into naming one degenerate clone two ways.
const fallbackName = "repo"

// The key for the clone whose SHARED git dir is at `shared` — the answer to `rev-parse
// --git-common-dir`, never `--git-path` or `--show-toplevel`.
//
// The digest is over that git dir's own realpath, the only sound identity available: two clones of one
// repository share a remote URL and often a basename, and never a git-dir realpath.
func FromSharedGitDir(shared string) (string, error) {
	canonical, err := canonicalGitDir(shared)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return nameOf(canonical) + "-" + hex.EncodeToString(digest[:])[:digestLength], nil
}

func abbrevFromSharedGitDir(shared string) (string, error) {
	canonical, err := canonicalGitDir(shared)
	if err != nil {
		return "", err
	}
	return abbrevOf(nameOf(canonical)), nil
}

// The clone's abbreviation: the initial of every alphanumeric run in its name, uppercased.
// `project-tracker-cache-cleanup` abbreviates to `PTCC`, `issue-tracker` to `IT`, and a name with
// no separator in it to its single letter.
//
// A run carrying a digit keeps its whole spelling instead — `github-action-deploy-k8s` is `GADK8s`
// and not `GADK`. The digits are the distinguishing half of a `k8s`, `v2` or `s3`, and an initial
// throws exactly that away.
//
// Initials collide by construction: `project-tracker` and `project-tools` both abbreviate to `PT`. This
// is a label a human groups sessions by and never an identity — FromSharedGitDir is what tells two
// clones apart, and a caller keying a directory off this instead would collide two repositories into
// one directory.
func abbrevOf(name string) string {
	initials := initialsOf(name)
	if initials == "" {
		// Nothing alphanumeric to take an initial from — `___` reaches here, because safeName leaves an
		// underscore alone.
		initials = initialsOf(fallbackName)
	}
	if len(initials) > abbrevLength {
		return initials[:abbrevLength]
	}
	return initials
}

// One initial per alphanumeric run, and a digit-carrying run's whole spelling — the rule abbrevOf
// states. isSeparator cuts at everything else, so a run can hold nothing but ASCII alphanumerics —
// which is what makes `run[:1]` whole and the answer safe to splice.
func initialsOf(name string) string {
	var initials strings.Builder
	for _, run := range strings.FieldsFunc(name, isSeparator) {
		initials.WriteString(strings.ToUpper(run[:1]))
		if strings.ContainsAny(run, "0123456789") {
			initials.WriteString(run[1:])
		}
	}
	return initials.String()
}

// Where one run of a name ends and the next begins: anything that is not alphanumeric, which covers
// both a raw `-`/`_`/`.` and anything else safeName has already flattened to a dash.
func isSeparator(r rune) bool {
	return !shell.IsAlnumRune(r)
}

// The shared git dir as a real path, or a refusal. The key and the abbreviation both start here, so a
// guard added to one of them and not the other cannot exist.
func canonicalGitDir(shared string) (string, error) {
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
	return canonical, nil
}

// The clone's directory name, taken from the git dir's PARENT. `--show-toplevel` answers the worktree's
// own directory, so a name built from it hands every worktree a different one: reaching for it looks
// like a fix and is the bug.
func nameOf(canonical string) string {
	return safeName(shell.BaseName(shell.DirName(canonical)))
}

// The readable half, reduced to what is safe to splice into a path or a command line. A clone's
// directory name is whatever its owner typed — a space, a glob, a newline, a leading dash — and one
// consumer splices the key into a `git worktree add` line a human reads and runs.
func safeName(name string) string {
	var safe strings.Builder
	for _, r := range name {
		switch {
		case shell.IsAlnumRune(r), r == '.', r == '_', r == '-':
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
		return fallbackName
	}
	return flattened
}

// The key for the clone containing `root`, asking git where the shared git dir is. For a caller that
// holds no resolved path of its own.
func resolveKey(git repo.Git, root string) (string, error) {
	shared, err := sharedGitDir(git, root)
	if err != nil {
		return "", err
	}
	return FromSharedGitDir(shared)
}

// ResolveAbbrev is the abbreviation for the clone containing `root`, asking git where the shared git
// dir is. It is what `repo-key.sh --abbrev` prints, and what a caller holding a repository path — the
// handoff gate holds one — compares a written-down prefix against. A root that is not inside a clone
// this process can read comes back as an error and never as an abbreviation.
func ResolveAbbrev(git repo.Git, root string) (string, error) {
	shared, err := sharedGitDir(git, root)
	if err != nil {
		return "", err
	}
	return abbrevFromSharedGitDir(shared)
}

// Where git says the shared git dir of `root` is. The port answers absolute, which matters: git's own
// answer in an ordinary repository is a bare `.git`, and a relative one would resolve against whatever
// directory the next caller happened to stand in.
//
// What the port must be given, and what CommandGit below hands it, is an environment with GIT_DIR and
// GIT_COMMON_DIR taken out. A directory does NOT select the repository on its own: git reads its
// location from the environment first, so a run from a hook in a linked worktree — which is given
// those — would key its own clone, and a consumer would create, write and later remove directories
// under that name.
func sharedGitDir(git repo.Git, root string) (string, error) {
	shared, err := git.CommonDir(root)
	if err != nil {
		return "", errors.New("could not ask git for the shared git dir of " + shell.Oneline(root) + " (git rev-parse --git-common-dir)")
	}
	return shared, nil
}

// CommandGit is the port every command here runs with. GIT_CEILING_DIRECTORIES stays in the
// environment on purpose: set on the repository's own root it stops discovery rather than redirecting
// it, so honouring it costs a refusal and never a wrong key, and a refusal is what this tool is for.
// `eco-report/layout.go` reads it the same way.
func CommandGit() repo.Git {
	return repo.Exec{Env: repo.WithoutGitLocation(os.Environ())}
}

// The stub this command runs behind, written out rather than read from argv[0]. `stub_usage_test.go`
// compares the usage line below against the one the stub's own header documents, and a name that
// changes with how the binary was reached leaves it nothing stable to compare.
const stubName = "repo-key.sh"

const usage = "usage: " + stubName + " [--abbrev] [<repo path>]"

// The command behind the stub. `--abbrev` prints the clone's abbreviation, no flag prints the key, and
// the root defaults to the working directory.
//
// There is no exit 1: a key either names this clone or it is nothing, and a caller that read a refusal
// as a key would write into a directory belonging to no repository.
func Run(args []string, git repo.Git, out, errOut io.Writer) int {
	resolve := resolveKey
	if len(args) > 0 && args[0] == "--abbrev" {
		resolve, args = ResolveAbbrev, args[1:]
	}
	// A second argument is a caller who does not know which clone they are asking about. Whatever is
	// left once the flag is taken is a path even when it starts with a dash: a directory may
	// legitimately be named `-rf`, and TestTheCommandsArgumentTable's dash-leading row fails on the
	// edit that changes that.
	if len(args) > 1 {
		return refuse(errOut, usage)
	}
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	answer, err := resolve(git, root)
	if err != nil {
		return refuse(errOut, err.Error())
	}
	fmt.Fprintln(out, answer)
	return 0
}

func refuse(errOut io.Writer, reason string) int {
	fmt.Fprintf(errOut, "%s: %s\n", stubName, reason)
	return 2
}
