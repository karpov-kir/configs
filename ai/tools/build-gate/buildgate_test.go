package buildgate_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildgate "configs/ai/tools/build-gate"
	"configs/ai/tools/repo/repotest"
)

// worktree is a fake git with a real git dir, so the record lands on disk. A case moves the
// fingerprint by hand to stand for an edit.
type worktree struct {
	t    *testing.T
	git  *repotest.Fake
	tree string
	fail error
}

func newWorktree(t *testing.T) *worktree {
	root := t.TempDir()
	git := repotest.New(root)
	if err := os.MkdirAll(git.Git, 0o755); err != nil {
		t.Fatal(err)
	}
	return &worktree{t: t, git: git, tree: "tree-1"}
}

func (w *worktree) fingerprint(root string) (string, error) {
	if root != w.git.Root {
		w.t.Fatalf("fingerprinted %q, want the worktree root %q", root, w.git.Root)
	}
	return w.tree, w.fail
}

type result struct {
	code        int
	out, errOut string
}

func (w *worktree) run(args ...string) result {
	var out, errOut bytes.Buffer
	code := buildgate.Run(buildgate.Options{Args: args, Dir: w.git.Root, Git: w.git, Fingerprint: w.fingerprint, Out: &out, ErrOut: &errOut})
	return result{code: code, out: out.String(), errOut: errOut.String()}
}

func (w *worktree) recordPath() string {
	return filepath.Join(w.git.Git, "kk-build-gate")
}

func (w *worktree) want(code int, args ...string) result {
	w.t.Helper()
	r := w.run(args...)
	if r.code != code {
		w.t.Fatalf("build-gate %s exited %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.code, code, r.out, r.errOut)
	}
	return r
}

func TestAnOpenBuildBlocksUntilAPassStampsIt(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-267")
	r := w.want(1, "check")
	if !strings.Contains(r.errOut, "kk-qualify") {
		t.Fatalf("the refusal does not name the pass it is waiting for: %s", r.errOut)
	}
	w.want(1, "close")
	w.want(0, "stamp")
	w.want(0, "check")
}

func TestAnEditAfterTheStampReopensTheGate(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-268")
	w.want(0, "stamp")
	w.tree = "tree-2"
	w.git.StatusLines = []string{" M f"}
	w.want(1, "check")
	w.want(1, "close")
	w.want(0, "stamp")
	w.want(0, "check")
}

func TestTheHumansSkipPassesAndBindsTheTreeItWasGivenFor(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-222")
	w.want(2, "skip")
	w.want(0, "skip", "ship", "it", "unqualified")
	r := w.want(0, "check")
	if !strings.Contains(r.out, "ship it unqualified") {
		t.Fatalf("a skipped pass does not say whose words waived it: %s", r.out)
	}
	w.tree = "tree-2"
	w.want(1, "check")
}

func TestCloseRetiresALandedBuildAfterThePullMovesTheTree(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-267")
	w.want(0, "stamp")
	w.tree = "tree-after-pull"
	w.git.StatusLines = []string{" M f"}
	w.want(1, "close")
	w.git.StatusLines = nil
	w.want(0, "close")
	w.want(0, "check")
}

func TestARecordPlantedAsALinkIsRefusedAndNeverFollowed(t *testing.T) {
	w := newWorktree(t)
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(target, w.recordPath()); err != nil {
		t.Fatal(err)
	}
	w.want(2, "check")
	w.want(2, "open", "FE-267")
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the gate wrote through the link: %v", err)
	}
}

func TestAStatusThatFailsLeavesTheRecord(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-267")
	w.want(0, "stamp")
	w.tree = "tree-2"
	w.git.Fail["Status"] = errors.New("git is gone")
	w.want(2, "close")
	if _, err := os.Stat(w.recordPath()); err != nil {
		t.Fatalf("a failed status lost the record: %v", err)
	}
}

func TestCloseRetiresALandedBuild(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-267")
	w.want(0, "stamp")
	w.want(0, "close")
	w.tree = "tree-2"
	w.want(0, "check")
	if _, err := os.Stat(w.recordPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("close left the record behind: %v", err)
	}
}

func TestAWorktreeWithNoBuildPasses(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "check")
	w.want(0, "close")
	w.want(1, "stamp")
	w.want(1, "skip", "no reason")
}

func TestOpenIsIdempotentForOneRequirementAndRefusesASecond(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-267")
	w.want(0, "stamp")
	w.want(0, "open", "FE-267")
	w.want(0, "check")
	w.want(1, "open", "FE-268")
}

func TestARecordItCannotReadIsNotAPass(t *testing.T) {
	for name, body := range map[string]string{
		"garbage":       "not json",
		"unknown state": `{"requirement":"x","state":"done"}`,
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorktree(t)
			if err := os.WriteFile(w.recordPath(), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			w.want(2, "check")
		})
	}
}

func TestAFingerprintThatFailsIsNotAPass(t *testing.T) {
	w := newWorktree(t)
	w.want(0, "open", "FE-267")
	w.want(0, "stamp")
	w.fail = errors.New("git is gone")
	r := w.want(2, "check")
	if !strings.Contains(r.errOut, "did NOT run") {
		t.Fatalf("a failed fingerprint does not say the gate did not run: %s", r.errOut)
	}
}

func TestAMalformedInvocationRefusesWithTheUsageLine(t *testing.T) {
	for _, args := range [][]string{nil, {"nope"}, {"open"}, {"open", "  "}, {"check", "extra"}, {"stamp", "x"}, {"close", "x"}} {
		w := newWorktree(t)
		r := w.want(2, args...)
		if !strings.Contains(r.errOut, "usage: build-gate.sh") {
			t.Fatalf("%q refused without the usage line: %s", args, r.errOut)
		}
	}
}
