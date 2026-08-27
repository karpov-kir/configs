package ecoreport_test

// The index group comes first because nothing undoes what it catches. The fingerprint recipe belongs
// to `~/.kk-flavor/scripts/tree-fingerprint.sh`, whose own failure modes are pinned in
// `~/.kk-flavor/scripts/tree-fingerprint-test.sh`. What these cases pin is this side of the seam:
// that a subcommand which fingerprints leaves the human's staged-versus-unstaged split exactly as
// they left it, and that the fingerprint really follows the tree. Git records nothing about what was
// staged before, so no later refusal puts a wrecked split back
// (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**). `init` fingerprints nothing, so the case
// beside them guards only that the subcommand stays out of the index.

import (
	"regexp"
	"strings"
	"testing"
)

func TestTheHumanIndexIsNeverTouched(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")

	// The split: one path staged, one tracked path modified but not staged, one path untracked. A
	// `git add -A` on the real index collapses all three into "staged".
	f.write(f.repo+"/staged.txt", "staged\n")
	f.mustGit("add", "staged.txt")
	f.write(f.repo+"/tracked.txt", "base\nmodified\n")
	f.write(f.repo+"/untracked.txt", "untracked\n")
	before := f.indexState()
	// The three cases below compare this split before and after, and an empty split compares equal
	// just as well. So a fixture that staged nothing would pass all three while the state they protect
	// is not there.
	established := strings.HasPrefix(before, "staged:staged.txt") &&
		strings.Contains(before, "\nunstaged:tracked.txt")
	f.record("fixture: the staged/unstaged split the three cases below compare", established, before)

	f.runReport("init", "review: index isolation")
	afterInit := f.indexState()
	f.record("init stages nothing (a regression guard, not a fingerprint one)",
		before == afterInit, "before -> "+before+"\nafter  -> "+afterInit)

	// `gate` reaches the fingerprint immediately after resolving the report, so it is the shortest
	// path to it. It is expected to block here; what matters is the index afterwards.
	f.runReport("gate")
	afterGate := f.indexState()
	f.record("the gate's fingerprint leaves the split exactly as it was",
		before == afterGate, "before -> "+before+"\nafter  -> "+afterGate)

	// The fingerprint has to be a real reading of the whole tree, or the case above would also pass on
	// a fingerprint of nothing at all.
	f.runReport("gate")
	firstTree := currentTreeIn(f.out)
	f.appendTo(f.repo+"/untracked.txt", "changed after the first reading\n")
	f.runReport("gate")
	secondTree := currentTreeIn(f.out)
	f.record("the fingerprint moves when an untracked file changes",
		firstTree != "" && secondTree != "" && firstTree != secondTree,
		"first -> "+orNone(firstTree)+"\nsecond -> "+orNone(secondTree))
}

func TestAMissingFingerprintScriptRefusesInsteadOfRecomputing(t *testing.T) {
	t.Parallel()
	// The one thing this must not do when the sibling script is gone is fingerprint the tree itself: a
	// local copy of the recipe puts every untracked file's content in the human's own object store,
	// recoverable for good. The fixture is the positive control — with the sibling reachable, `gate`
	// gets past the fingerprint and blocks on freshness (exit 1), so the refusal below is the missing
	// script and not some earlier guard.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "review: missing fingerprint")
	f.runReport("gate")
	f.record("fixture: gate reaches the fingerprint and blocks on freshness while the sibling is installed",
		f.status == 1, "gate exited "+itoa(f.status)+", wanted 1\n"+f.out)

	// The path is resolved through HOME, so an empty one is how the script goes missing without
	// touching the real install.
	f.home = f.base + "/nohome"
	f.mkdirAll(f.home)
	f.runReport("gate")
	f.assertRefused("gate refuses when the fingerprint script is not installed")
	f.assertReports("tree-fingerprint.sh", "and names the path it wanted")
	f.assertReports("no local fallback", "and says it will not fall back to a recipe of its own")
}

func currentTreeIn(output string) string {
	return regexp.MustCompile(`current [0-9a-f]{40}`).FindString(output)
}

func orNone(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}
