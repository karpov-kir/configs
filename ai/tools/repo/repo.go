// Package repo is the one place these tools ask a git repository a question.
//
// Every tool here used to spawn `git` directly, and its suite then had to build a real repository per
// case to have anything to drive. On the machine these are written on a process costs about 100ms —
// CrowdStrike Falcon inspects every exec — so a table test with twenty rows paid twenty spawns and a
// suite ran for minutes. `ai/kk-flavor/standards/testing.md` rule 6 puts the whole suite under 100
// seconds, and that is unreachable while a case has to fork to assert anything.
//
// So the questions are named here and the answers come from somewhere. Git execs git and is what runs
// in production. repotest.Fake answers from a table and is what the suites drive.
//
// The interface names QUESTIONS, never git's command line. A port shaped as `Run(args ...string)`
// would make every fake parse git's grammar, which is the fake agreeing with itself rather than with
// git — testing.md rule 5. Shaped as questions, the fake holds data and the one thing that could
// disagree with git is the adapter below, which exec_test.go drives against a real repository.
package repo

// Worktree is one entry of `git worktree list`: where it is checked out and which git dir it uses.
type Worktree struct {
	// Path is the worktree's own directory, absolute as git prints it.
	Path string
	// Head is the commit it has checked out, empty for an unborn branch.
	Head string
	// Bare says the entry is the bare repository rather than a checkout.
	Bare bool
}

// Git is every question this repository's tools ask of a git repository. Each method takes the
// directory to ask from, because git answers relative to where it runs and several of these tools ask
// about more than one tree in a run.
//
// Small on purpose: a question is added here when production asks it, never in advance. The suites
// drive repotest.Fake, so a method nobody calls is a fake nobody exercises.
type Git interface {
	// TopLevel is `rev-parse --show-toplevel`: the root of the working tree dir sits in.
	TopLevel(dir string) (string, error)
	// CommonDir is `rev-parse --git-common-dir` made absolute against dir — the store every linked
	// worktree of one clone shares.
	CommonDir(dir string) (string, error)
	// GitDir is `rev-parse --git-dir`: this worktree's own git directory, which differs from CommonDir
	// inside a linked worktree.
	GitDir(dir string) (string, error)
	// GitPath is `rev-parse --git-path <name>`: where git would put a file of that name for this
	// worktree. Asked rather than joined, because a linked worktree's answer is not its git dir.
	GitPath(dir, name string) (string, error)
	// Prefix is `rev-parse --show-prefix`: dir's path below the working tree root, "" at the root and
	// otherwise slash-terminated as git prints it.
	Prefix(dir string) (string, error)

	// Resolve is `rev-parse --verify --quiet <rev>`: the object id, or "" with a nil error where the
	// revision names nothing. An unborn HEAD and a typo are the same answer here, which is git's own.
	Resolve(dir, rev string) (string, error)
	// MergeBase is `merge-base <left> <right>`.
	MergeBase(dir, left, right string) (string, error)

	// Tracked is `ls-files`: the index's paths, relative to the working tree root. An empty pathspec
	// asks about the whole tree.
	Tracked(dir string, pathspec ...string) ([]string, error)
	// Untracked is `ls-files --others --exclude-standard`: files present and not ignored.
	Untracked(dir string, pathspec ...string) ([]string, error)
	// NamesAt is `ls-tree -r --name-only <rev>`: what a commit holds.
	NamesAt(dir, rev string) ([]string, error)
	// Changed is `diff --name-only` over revisions, deletions dropped. No revision means "against the
	// working tree", which is the caller's to spell as HEAD if that is what it means.
	Changed(dir string, revisions, pathspec []string) ([]string, error)
	// ChangedWithStatus is `diff --raw --no-renames`: one entry per path with the status letter git
	// prints for it and the destination blob, so a caller can read a file's new content without a
	// second listing.
	ChangedWithStatus(dir string, revisions, pathspec []string) ([]Change, error)
	// Status is `status --porcelain -uall`, one entry per line.
	Status(dir string) ([]string, error)

	// Show is `show <rev>:<path>`: one file's content at a revision.
	Show(dir, rev, path string) ([]byte, error)
	// Blob is `cat-file blob <id>` with the object's size, so a caller can refuse a large one without
	// reading it.
	Blob(dir, id string) (content []byte, size int64, err error)

	// Ignored is `check-ignore --stdin` over paths: the subset git would ignore.
	Ignored(dir string, paths []string) (map[string]bool, error)
	// IgnoreSource is `check-ignore -v <path>`: the file and line the decision came from, empty where
	// the path is not ignored.
	IgnoreSource(dir, path string) (string, error)

	// Worktrees is `worktree list --porcelain`.
	Worktrees(dir string) ([]Worktree, error)

	// Add is `add -- <paths>`. The one write in this port: `report.sh promote` stages what it wrote.
	Add(dir string, paths []string) error
}

// Change is one entry of a raw diff: git's status letter, the path, and the blob the path holds on the
// right-hand side ("" where the letter is a deletion).
type Change struct {
	Status string
	Path   string
	Blob   string
}
