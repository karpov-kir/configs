package projectsetup

import (
	"os"
	"os/exec"
	"strings"

	"kk-flavor/tools/repo"
)

// Git is the four questions this installer asks of a git repository. A port rather than calls, so the
// worktree cases below are driven from a table instead of from a repository a suite has to build — on
// the machine these are written on a `git init` plus a commit plus a `worktree add` is five processes
// at about 100ms each, and the shell suite this replaces spent 819 seconds that way.
//
// Its own port rather than ai/tools/repo's. That package answers three of these four and cannot answer
// the fourth — nothing there reads a config value, and its worktree listing drops the `prunable` flag
// this installer has to honour. Widening it is not this change's to do; folding the two together once
// it is free is worth doing, and is named in this change's report.
type Git interface {
	// TopLevel is `rev-parse --show-toplevel`: the root of the working tree dir sits in. An error says
	// dir is not a readable worktree, which is how a project that is not a repository at all is told
	// from one this installer may not touch.
	TopLevel(dir string) (string, error)
	// CommonDir is `rev-parse --path-format=absolute --git-common-dir`: the store every linked worktree
	// of one clone shares, and where this installer keeps its own per-project state.
	CommonDir(dir string) (string, error)
	// HooksPath is `config --get core.hooksPath`, and whether it is set at all. Set to the empty string
	// is still set: git then looks for hooks in the worktree root, so a hook written where this
	// installer would put one never runs — and an installer that reported success would be promising
	// skill links that never arrive.
	HooksPath(dir string) (value string, isSet bool)
	// Worktrees is `worktree list --porcelain`, including the flags that say an entry is not a checkout
	// this installer should write into.
	Worktrees(dir string) ([]Worktree, error)
}

// Worktree is one entry of the listing.
type Worktree struct {
	// Path is the worktree's own directory, absolute as git prints it.
	Path string
	// IsBare says the entry is the bare repository rather than a checkout, and IsPrunable that git
	// itself considers the entry stale. Neither is a tree to write skills into, and a run that wrote
	// into a prunable one would be acting on metadata git is about to drop.
	IsBare     bool
	IsPrunable bool
}

// NewGit is the adapter a real run asks through.
func NewGit() Git {
	return gitCommand{}
}

type gitCommand struct{}

// Every call drops the location variables out of the environment. GIT_DIR and GIT_COMMON_DIR override
// `-C`, so a run started from inside a hook — which is exactly when this installer's sync runs — would
// otherwise be asked about the repository the hook belongs to rather than the worktree it was handed.
// repo.WithoutGitLocation is what decides which ones those are, and carries why the rest stay.
func (g gitCommand) ask(dir string, args ...string) (string, error) {
	run := exec.Command("git", append([]string{"-C", dir}, args...)...)
	run.Env = repo.WithoutGitLocation(os.Environ())
	out, err := run.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (g gitCommand) TopLevel(dir string) (string, error) {
	return g.ask(dir, "rev-parse", "--show-toplevel")
}

func (g gitCommand) CommonDir(dir string) (string, error) {
	return g.ask(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

// git exits non-zero when the key is unset, which is the whole of what separates "unset" from "set to
// the empty string" — and those two mean opposite things here.
func (g gitCommand) HooksPath(dir string) (string, bool) {
	value, err := g.ask(dir, "config", "--get", "core.hooksPath")
	if err != nil {
		return "", false
	}
	return value, true
}

func (g gitCommand) Worktrees(dir string) ([]Worktree, error) {
	out, err := g.ask(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var listed []Worktree
	for _, block := range strings.Split(out, "\n\n") {
		var one Worktree
		found := false
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				one.Path, found = strings.TrimPrefix(line, "worktree "), true
			case line == "bare":
				one.IsBare = true
			// git writes `prunable <reason>`, so the prefix and not the whole word.
			case strings.HasPrefix(line, "prunable"):
				one.IsPrunable = true
			}
		}
		if found {
			listed = append(listed, one)
		}
	}
	return listed, nil
}
