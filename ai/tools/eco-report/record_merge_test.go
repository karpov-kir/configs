package ecoreport_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The test resolves every link from the file that holds it. One entry text lives in two files:
// the project record two levels under the root, and the ship's local record four levels under
// it, which finalize archives at that depth. A relative link written for one depth breaks in the
// other.
func TestAMergedEntrysLinksResolveFromTheProjectRecordAndTheArchive(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-linking")
	f.write(f.repo+"/scripts/x.go", "package main\n")
	f.runReport("record", "--intent", "001-linking", "append", "local-playbook",
		"run [the script](../../../../scripts/x.go#L3) before landing, as [the intent](../intent.md), [the ship](..) and [the docs](https://example.com/a) say")
	f.record("the local append lands", f.status == 0, f.evidence())

	f.runReport("record", "--intent", "001-linking", "merge", "local-playbook", "run [the script]")
	f.record("the merge lands", f.status == 0, f.evidence())
	f.runReport("finalize", "001-linking")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	script := canonical(f.repo + "/scripts/x.go")
	intent := canonical(f.archiveDir("001-linking") + "/intent.md")
	ship := canonical(f.archiveDir("001-linking"))
	for _, path := range []string{recordFile(f, "project-playbook"), f.archiveDir("001-linking") + "/for-agents/playbook.md"} {
		text := f.read(path)
		resolved := relativeLinkTargets(path, text)
		f.record(path+" links the script, the archived intent and the archived ship", len(resolved) == 3 && resolved[0] == script && resolved[1] == intent && resolved[2] == ship,
			"resolved: "+strings.Join(resolved, ", ")+"\n"+text)
		f.record(path+" keeps the line anchor and the absolute link as written",
			strings.Contains(text, "x.go#L3)") && strings.Contains(text, "(https://example.com/a)"), text)
	}
}

// The test checks that a merge keeps a decision's classification. The finalizing skill merges
// decisions upward. A promotion candidate under `## Decisions` reads as settled to later ships.
func TestAMergedPromotionCandidateStaysACandidate(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-classing")
	f.newIntentFile("001-classing")
	f.runReport("record", "--intent", "001-classing", "append", "local-decisions", "a rule worth promoting")
	f.runReport("record", "--intent", "001-classing", "classify", "local-decisions", "a rule worth promoting", "candidate")
	f.runReport("record", "--intent", "001-classing", "append", "local-decisions", "a settled choice")

	f.runReport("record", "--intent", "001-classing", "merge", "local-decisions", "a rule worth promoting")
	f.record("the candidate merges", f.status == 0, f.evidence())
	f.runReport("record", "--intent", "001-classing", "merge", "local-decisions", "a settled choice")
	f.record("the decision merges", f.status == 0, f.evidence())

	project := f.read(recordFile(f, "project-decisions"))
	candidates, decisions, _ := strings.Cut(project, "## Decisions")
	f.record("the candidate lands under the project's candidates",
		strings.Contains(candidates, "a rule worth promoting") && !strings.Contains(decisions, "a rule worth promoting"), project)
	f.record("and the decision under its decisions",
		strings.Contains(decisions, "a settled choice") && !strings.Contains(candidates, "a settled choice"), project)
}

// An exact restatement bumps the project's entry, and the project's own section stands: which of
// the two classifications wins is settled before the merge, never by it.
func TestAMergeThatRestatesAProjectDecisionBumpsItInPlace(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-restating")
	f.newIntentFile("001-restating")
	f.runReport("record", "append", "project-decisions", "a settled rule")
	f.runReport("record", "--intent", "001-restating", "append", "local-decisions", "a settled rule")
	f.runReport("record", "--intent", "001-restating", "classify", "local-decisions", "a settled rule", "candidate")

	f.runReport("record", "--intent", "001-restating", "merge", "local-decisions", "a settled rule")
	project := f.read(recordFile(f, "project-decisions"))
	_, decisions, _ := strings.Cut(project, "## Decisions")
	f.record("the project's entry is bumped and stays under its decisions",
		f.status == 0 && strings.Count(project, "a settled rule") == 1 && strings.Contains(decisions, "2x | "+today()+" | a settled rule"),
		f.evidence()+"\n"+project)
}

func TestAMergeLeavesLinksThatNeedNoRebase(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-keeping")
	f.newIntentFile("001-keeping")
	entry := "see [here](#here), [root](/scripts/x.go) and [spaced](<../x y.md>)"
	f.runReport("record", "--intent", "001-keeping", "append", "local-playbook", entry)

	f.runReport("record", "--intent", "001-keeping", "merge", "local-playbook", "see [here]")
	f.record("each link lands unchanged", f.status == 0 && strings.Contains(f.read(recordFile(f, "project-playbook")), " | "+entry+"\n"),
		f.evidence())
}

// A merge writes a project record, so it waits for the merge slot like any other project write.
func TestAMergeWaitsForTheSlotAnotherWorktreeHolds(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-waiting")
	f.newIntentFile("001-waiting")
	f.runReport("record", "--intent", "001-waiting", "append", "local-playbook", "landed while someone else was landing")
	f.takeMergeSlotFrom("002-landing", f.base+"/sibling-worktree")

	f.runReport("record", "--intent", "001-waiting", "merge", "local-playbook", "landed while")
	f.record("the merge exits with the slot's code", f.status == 4, f.evidence())
	f.record("and writes no project record", !f.exists(recordFile(f, "project-playbook")), f.evidence())
}

func TestAMergeRefusesWhatItCannotCarryUpward(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-refusing")
	f.newIntentFile("001-refusing")

	f.runReport("record", "--intent", "001-refusing", "merge", "local-playbook", "anything")
	f.record("a merge from a local record not yet written is refused", f.status != 0, f.evidence())
	f.record("and creates no project record", !f.exists(recordFile(f, "project-playbook")), f.evidence())

	f.runReport("record", "merge", "project-playbook", "anything")
	f.record("a merge naming a project record is refused", f.status != 0 && strings.Contains(f.out, "local-*"), f.evidence())
}

var markdownLinkTarget = regexp.MustCompile(`\]\(([^)\s]+)`)

/** Returns each relative link target in text, resolved against the directory of the file at path. */
func relativeLinkTargets(path, text string) []string {
	var resolved []string
	for _, match := range markdownLinkTarget.FindAllStringSubmatch(text, -1) {
		target := strings.SplitN(match[1], "#", 2)[0]
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "/") {
			continue
		}
		resolved = append(resolved, filepath.Join(filepath.Dir(path), target))
	}
	return resolved
}
