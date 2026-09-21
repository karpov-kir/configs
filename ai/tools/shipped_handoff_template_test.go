// The shipped handoff prompt template, run through the gate that reads a draft written from it.
//
// It lives in this package with the other cases that read the shipped checkout, for the reason
// shipped_tree_test.go gives.
//
// The gate's own cases stay beside it and run over drafts they build. What they cannot say is anything
// about the file this repository actually ships.
package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	handoffcheck "configs/ai/tools/handoff-check"
	"configs/ai/tools/repo/repotest"
)

const shippedHandoffTemplate = repoRoot + "/ai/kk-flavor/skills/kk-handoff/handoff-prompt.md"

// The template and the gate each hold the seven headings, and this case is the only comparison of the
// two. Rename one heading in either and every future draft is refused, with the mismatch surfacing
// only at the next real handoff. The comparison is a gate run over the shipped template. Its leftover
// comments prove the scan reached the slots, and neither mismatch finding may appear.
func TestShippedTemplateMatchesTheHeadingsTheGateRequires(t *testing.T) {
	if _, err := os.Stat(shippedHandoffTemplate); err != nil {
		t.Fatalf("cannot reach %s, so the drift case did not run: %v", shippedHandoffTemplate, err)
	}
	dir, git := handoffRepository(t)

	var out, errOut bytes.Buffer
	code := handoffcheck.Run("handoff-check.sh", shippedHandoffTemplate, dir, git, &out, &errOut)
	said := out.String() + errOut.String()

	if code != 1 {
		t.Errorf("the gate exited %d over the shipped template, wanted 1 — output: %s", code, said)
	}
	// The control. A gate that had stopped reading the file would report none of the three, and the two
	// mismatch findings would each be satisfied by silence.
	if !strings.Contains(said, "template comment left") {
		t.Errorf("the gate reported no leftover template comment, so it did not reach the slots and the two "+
			"drift findings below are absent for the wrong reason — output: %s", said)
	}
	for _, drift := range []string{"missing section:", "unknown section:"} {
		if strings.Contains(said, drift) {
			t.Errorf("the gate reported %q over the template this repository ships, so the two have drifted "+
				"and every draft written from it is refused — output: %s", drift, said)
		}
	}
}

// A directory the gate can be pointed at, and the port that answers for it. The port reports a work
// tree holding one commit, a clean tree, and a shared git dir whose parent name is what `repo-key`
// abbreviates. Two things are real on disk and both have to be. The gate resolves the path a draft
// must name, and `repo-key` refuses a git dir with no HEAD in it.
func handoffRepository(t *testing.T) (string, *repotest.Fake) {
	t.Helper()
	// Distinctive, not "repo": the gate refuses a draft naming the repository by basename, and a fixture
	// called "repo" would satisfy that on the word "repo" appearing anywhere.
	dir := filepath.Join(t.TempDir(), "handoff-fixture")
	git := repotest.New(dir)
	if err := os.MkdirAll(git.Git, 0o755); err != nil {
		t.Fatalf("building the repository fixture: %v — nothing was tested", err)
	}
	if err := os.WriteFile(filepath.Join(git.Git, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("building the repository fixture: %v — nothing was tested", err)
	}
	// Twelve hex, the length a session writes an abbreviated SHA down at.
	return dir, git.Commit("9f2a1c0b7de4", nil)
}
