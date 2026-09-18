package readerjudge

import (
	"strings"
	"testing"
)

func TestAMajorityLabelWinsOutright(t *testing.T) {
	if got := MajorityLabel([]string{"obvious", "obvious", "keep"}); got != "obvious" {
		t.Errorf("three rolls, two obvious, answered %q", got)
	}
	if got := MajorityLabel([]string{"keep", "keep", "obvious"}); got != "keep" {
		t.Errorf("three rolls, two keep, answered %q", got)
	}
}

// A block carrying two defects splits the rolls between two correct labels. The rolls agree it is
// bad. Precedence decides which action it takes, and a split would otherwise escape as `keep`.
func TestRollsThatAgreeABlockIsBadTakeThePrecedence(t *testing.T) {
	if got := MajorityLabel([]string{"obvious", "coined", "obvious", "coined"}); got != "obvious" {
		t.Errorf("a split between obvious and coined answered %q, and obvious outranks coined", got)
	}
	if got := MajorityLabel([]string{"stale", "unclear", "coined", "unclear"}); got != "stale" {
		t.Errorf("a three-way split answered %q, and stale outranks both", got)
	}
}

// Precision over recall: where the rolls do not agree the block is bad, it is left alone.
func TestABlockAMajorityDidNotCallBadIsKept(t *testing.T) {
	if got := MajorityLabel([]string{"keep", "keep", "obvious", "coined"}); got != "keep" {
		t.Errorf("two keep of four answered %q", got)
	}
	if got := MajorityLabel(nil); got != "keep" {
		t.Errorf("no rolls at all answered %q", got)
	}
}

func TestEveryBlockMustCarryAVerdict(t *testing.T) {
	if _, err := ParseLabels("1 obvious\n2 keep\n", 3); err == nil {
		t.Error("a reply leaving block 3 unanswered was taken, and the caller counts lines against blocks")
	}
	if _, err := ParseLabels("1 obvious\n1 keep\n", 1); err == nil {
		t.Error("a reply answering block 1 twice was taken")
	}
	if _, err := ParseLabels("1 unreadable\n", 1); err == nil {
		t.Error("a verdict outside the closed set was taken")
	}
	if _, err := ParseLabels("1 obvious\n2 keep\n", 2); err != nil {
		t.Errorf("a complete reply was refused: %v", err)
	}
}

// A delete kind with an empty offer prints the artifact, because its output is the artifact. A
// verdict kind's output is one label line per block, so the same path would hand its caller a file to
// read as verdicts. Reached whenever `--changed` narrows a file to no changed block.
func TestAVerdictRunWithNothingOfferedPrintsNoArtifact(t *testing.T) {
	withTheRecordedVerdictKind(t)
	path := write(t, "package p\n\nfunc a() {}\n")
	var out, errOut strings.Builder
	called := false
	call := func(string, string) (string, error) { called = true; return "", nil }
	if code := Run("reader-judge.sh", []string{"comment-verdict", path}, nil, &out, &errOut, call, nil); code != exitClean {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if out.String() != "" {
		t.Errorf("a verdict run with no block offered printed %q", out.String())
	}
	if called {
		t.Error("the model was called for a file offering no block")
	}
}
