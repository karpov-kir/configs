// Package repo is where these tools ask a git repository a question.
//
// Every tool used to spawn `git` itself, and a suite then had to build a real repository per case. A
// process costs about 100ms on the machine these are written on, since CrowdStrike Falcon inspects
// every exec. A twenty-row table test paid twenty spawns and a suite ran for minutes.
// `ai/kk-flavor/standards/testing.md` rule 6 puts the whole suite under 100 seconds, and a case that
// forks to assert anything cannot reach it.
//
// Exec runs git and is what production wires in. repotest.Fake answers from a table and the suites
// drive it. exec_test.go drives Exec against a real repository.
package repo

// Worktree is one entry of `git worktree list`: its checkout directory and the git dir it uses.
type Worktree struct {
	// Path is the worktree's own directory, absolute as git prints it.
	Path string
	// Head is the commit it has checked out, empty for an unborn branch.
	Head string
	// Bare says the entry is the clone's bare repository and holds no checkout.
	Bare bool
	// Prunable is git's own verdict that the entry is stale — its directory is gone, or its lock has
	// expired — and that `worktree prune` would drop it. Neither this nor Bare is a tree to write
	// into: a run that wrote into a prunable one would be acting on metadata git is about to discard.
	Prunable bool
}

// Git is every question this repository's tools ask of a git repository.
//
// The methods name questions in git's words. A port shaped as `Run(args ...string)` would make every
// fake parse git's grammar, and such a fake agrees with itself (testing.md rule 5). A question is
// added when production asks it, so every method here is one the suites drive too.
type Git interface {
	// TopLevel is `rev-parse --show-toplevel`: the root of the working tree dir sits in.
	TopLevel(dir string) (string, error)
	// CommonDir is `rev-parse --git-common-dir` made absolute against dir — the store every linked
	// worktree of one clone shares.
	CommonDir(dir string) (string, error)
	// GitDir is this worktree's own git directory, which differs from CommonDir inside a linked worktree.
	GitDir(dir string) (string, error)
	// GitPath is where git would put a file of that name for this worktree. The port asks git, because
	// a linked worktree does not keep every such file under one git dir.
	GitPath(dir, name string) (string, error)
	// Prefix is `rev-parse --show-prefix`: dir's path below the working tree root, "" at the root and
	// otherwise slash-terminated as git prints it.
	Prefix(dir string) (string, error)
	// ConfigValue is `config --get <key>`, and whether the key is set.
	//
	// git spells unset as a non-zero exit with empty stdout, and a failure looks the same, so no error comes
	// back. A key set to the EMPTY STRING is still set, and the two mean opposite things. core.hooksPath,
	// the config key, set empty sends git to the worktree root for hooks, and the installed hook never runs.
	ConfigValue(dir, key string) (value string, isSet bool)

	// Resolve is `rev-parse --verify --quiet <rev>`: the object id, or "" with a nil error for a
	// revision git cannot find. An unborn HEAD and a typo are the same answer here, which is git's own.
	Resolve(dir, rev string) (string, error)
	// MergeBase is the best common ancestor of left and right.
	MergeBase(dir, left, right string) (string, error)

	// Tracked is `ls-files`: the index's paths, relative to the working tree root. An empty pathspec
	// asks about the whole tree.
	Tracked(dir string, pathspec ...string) ([]string, error)
	// Untracked is `ls-files --others --exclude-standard`: the files git would call untracked, with
	// the ignored ones left out.
	Untracked(dir string, pathspec ...string) ([]string, error)
	// NamesAt is `ls-tree -r --name-only <rev>`: what a commit holds.
	NamesAt(dir, rev string) ([]string, error)
	// Changed is `diff --name-only` over revisions, deletions dropped. No revision means "against the
	// working tree", which is the caller's to spell as HEAD if that is what it means.
	Changed(dir string, revisions, pathspec []string) ([]string, error)
	// ChangedWithStatus is `diff --raw --no-renames`: one entry per path, with git's status letter and
	// the destination blob. A caller reads a file's new content off that blob, with no second listing.
	ChangedWithStatus(dir string, revisions, pathspec []string) ([]Change, error)
	// Patch is the change set as unified diff text.
	//
	// The adapter pins the shape a parser keys off — `+++ b/<path>`, a leading `+` — against the things
	// a reader's own git config can turn on.
	Patch(dir string, revisions, pathspec []string) ([]byte, error)
	// Status is `status --porcelain -uall`, one entry per line.
	Status(dir string) ([]string, error)

	// Show is `show <rev>:<path>`: one file's content at a revision.
	Show(dir, rev, path string) ([]byte, error)
	// ContentsAt is Show for a whole list, and it costs one process whatever the list's length. visit
	// runs once per path the revision holds, in the order asked. A path the revision does not hold is
	// passed over in silence. A file it holds EMPTY arrives as a visit with no content, so a caller
	// counting files can tell the two apart. An object over maxBytes is passed over unread by visit.
	ContentsAt(dir, rev string, paths []string, maxBytes int64, visit func(path string, content []byte)) error
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

	// Add is `add -- <paths>`, and the only write this port makes: `report.sh promote` stages what it
	// wrote.
	Add(dir string, paths []string) error
}

// Change is one entry of a raw diff: git's status letter, the path, and both sides of what changed.
type Change struct {
	Status string
	Path   string
	// OldMode and NewMode carry the file's mode on each side. A file that was executable at the base
	// and is a regular file now holds the same content on both sides, and a symlink turned regular is
	// the same shape. No content check recovers either change, so eco-report's scope scan reads these
	// two. A mode is empty where that side is absent, and an addition has no source.
	OldMode string
	NewMode string
	// OldBlob and Blob are the object ids of each side. `Show` is no substitute for the base blob,
	// because it applies `--textconv` and the reader's own git config can turn that on. A blob is
	// empty where that side is absent, and a deletion has no destination.
	OldBlob string
	Blob    string
}
