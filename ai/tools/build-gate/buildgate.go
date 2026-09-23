// Package buildgate holds a kk-build change back from a commit, a push or a PR. A kk-qualify pass
// over its tree releases it. So does a skip the human gave.
package buildgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"configs/ai/tools/repo"
	treefingerprint "configs/ai/tools/tree-fingerprint"
)

const stubName = "build-gate.sh"

const usage = "usage: " + stubName + " open <requirement> | stamp | skip <the human's words> | check | close"

// Git resolves this name under `--git-path` into the worktree's own git dir. A `git add -A` never sees
// that dir, and each worktree of a clone has its own.
const recordName = "kk-build-gate"

const (
	stateOpen      = "open"
	stateQualified = "qualified"
	stateSkipped   = "skipped"
)

// Maps each verb that releases the gate to the state it records.
var settledBy = map[string]string{"stamp": stateQualified, "skip": stateSkipped}

type record struct {
	Requirement string `json:"requirement"`
	State       string `json:"state"`
	// Tree is the fingerprint a stamp or a skip covers. It stays empty while the build is open.
	Tree string `json:"tree,omitempty"`
	// Skip holds the human's own words for a skip.
	Skip string `json:"skip,omitempty"`
}

type Fingerprinter func(root string) (string, error)

type Options struct {
	Args []string
	// Dir is any directory inside the worktree the gate guards.
	Dir         string
	Git         repo.Git
	Fingerprint Fingerprinter
	Out, ErrOut io.Writer
}

// Runs one invocation from the working directory against the real git and fingerprint.
// treefingerprint.Fingerprint runs git with this process's environment, so Command clears the
// variables that relocate git before the record or the fingerprint reads the repository.
func Command(args []string, out, errOut io.Writer) int {
	repo.ClearGitLocation()
	return Run(Options{Args: args, Dir: ".", Git: repo.Exec{}, Fingerprint: treefingerprint.Fingerprint, Out: out, ErrOut: errOut})
}

// Runs one invocation and answers 0 on a pass or a written record, 1 when the gate refuses, or 2
// when it could not run.
func Run(options Options) int {
	args, errOut := options.Args, options.ErrOut
	if len(args) == 0 {
		return refuse(errOut, usage)
	}
	verb, rest := args[0], args[1:]
	text := strings.TrimSpace(strings.Join(rest, " "))
	switch verb {
	case "open", "skip":
		if text == "" {
			return refuse(errOut, usage)
		}
	case "stamp", "check", "close":
		if len(rest) > 0 {
			return refuse(errOut, usage)
		}
	default:
		return refuse(errOut, usage)
	}

	path, err := options.Git.GitPath(options.Dir, recordName)
	if err != nil {
		return refuse(errOut, err.Error())
	}
	current, err := load(path)
	if err != nil {
		return refuse(errOut, err.Error())
	}

	g := gate{Options: options, path: path}
	switch verb {
	case "open":
		return g.open(current, text)
	case "stamp", "skip":
		return g.settle(current, verb, text)
	case "check":
		return g.check(current)
	default:
		return g.close(current)
	}
}

type gate struct {
	Options
	path string
}

// kk-build's Phase 1 allows one requirement per worktree. open refuses a second one and keeps the
// record for a resumed build of the same one.
func (g gate) open(current *record, requirement string) int {
	if current != nil {
		if current.Requirement == requirement {
			g.say("already open for %q (%s)", requirement, current.State)
			return 0
		}
		return g.block(fmt.Sprintf("this worktree already holds the build %q; one requirement is one worktree", current.Requirement))
	}
	if err := save(g.path, record{Requirement: requirement, State: stateOpen}); err != nil {
		return refuse(g.ErrOut, err.Error())
	}
	g.say("open for %q; commit, push and PR wait for a kk-qualify pass or the human's skip", requirement)
	return 0
}

func (g gate) settle(current *record, verb, words string) int {
	if current == nil {
		return g.block("no build is open in this worktree, so there is nothing to " + verb)
	}
	tree, code := g.tree()
	if code != 0 {
		return code
	}
	current.State, current.Tree, current.Skip = settledBy[verb], tree, words
	if err := save(g.path, *current); err != nil {
		return refuse(g.ErrOut, err.Error())
	}
	g.say("%s %q at tree %s", current.State, current.Requirement, tree)
	return 0
}

// A commit leaves the working tree's content as it was, so the fingerprint that check compares still
// matches at the push and the PR.
func (g gate) check(current *record) int {
	if current == nil {
		g.say("pass, no build is open in this worktree")
		return 0
	}
	if current.State == stateOpen {
		return g.block(fmt.Sprintf("the build %q has had no kk-qualify pass. Run kk-qualify over its change set, then `build-gate.sh stamp`. Only the human can waive the pass: `build-gate.sh skip <their words>`", current.Requirement))
	}
	tree, code := g.tree()
	if code != 0 {
		return code
	}
	if tree != current.Tree {
		return g.block(fmt.Sprintf("the tree changed after the build %q was %s. Qualify the change again and stamp, or ask the human for a skip of this tree", current.Requirement, current.State))
	}
	if current.State == stateSkipped {
		g.say("pass, the human skipped the pass for %q: %s", current.Requirement, current.Skip)
		return 0
	}
	g.say("pass, %q is qualified at this tree", current.Requirement)
	return 0
}

// Retires the worktree's build, or blocks one no pass has read. A pull or a merge after landing moves
// the fingerprint. Every commit already passed check, so close retires a settled build over a clean tree.
func (g gate) close(current *record) int {
	if current == nil {
		g.say("no build is open in this worktree")
		return 0
	}
	if current.State == stateOpen {
		return g.check(current)
	}
	tree, code := g.tree()
	if code != 0 {
		return code
	}
	if tree != current.Tree {
		dirty, err := g.Git.Status(g.Dir)
		if err != nil {
			return refuse(g.ErrOut, err.Error())
		}
		if len(dirty) > 0 {
			return g.block(fmt.Sprintf("the tree holds uncommitted changes made after the build %q was %s. Qualify them and stamp, or set them aside (`git stash -u`, or discard them) before closing", current.Requirement, current.State))
		}
	}
	if err := os.Remove(g.path); err != nil {
		return refuse(g.ErrOut, err.Error())
	}
	g.say("closed %q", current.Requirement)
	return 0
}

func (g gate) tree() (string, int) {
	root, err := g.Git.TopLevel(g.Dir)
	if err != nil {
		return "", refuse(g.ErrOut, err.Error())
	}
	tree, err := g.Fingerprint(root)
	if err != nil {
		return "", refuse(g.ErrOut, "cannot fingerprint the tree, so the gate did NOT run: "+err.Error())
	}
	return tree, 0
}

func (g gate) say(format string, args ...any) {
	fmt.Fprintf(g.Out, "build-gate: "+format+"\n", args...)
}

func (g gate) block(reason string) int {
	fmt.Fprintf(g.ErrOut, "%s: blocked: %s\n", stubName, reason)
	return 1
}

// Returns the record, or nil when there is none. A missing record passes check, so load reports a
// record it cannot parse as an error.
func load(path string) (*record, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file, so the gate did NOT run", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r record
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("%s does not parse, so the gate did NOT run: %v", path, err)
	}
	switch r.State {
	case stateOpen, stateQualified, stateSkipped:
		return &r, nil
	}
	return nil, fmt.Errorf("%s holds the unknown state %q, so the gate did NOT run", path, r.State)
}

func save(path string, r record) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	// A rename replaces a link planted at the path. A plain write would follow it out of the git dir.
	tmp, err := os.CreateTemp(filepath.Dir(path), recordName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func refuse(errOut io.Writer, reason string) int {
	fmt.Fprintf(errOut, "%s: %s\n", stubName, reason)
	return 2
}
