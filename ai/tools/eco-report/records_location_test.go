package ecoreport_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRecordsLandWhereTheRepoModePutsThem(t *testing.T) {
	t.Parallel()
	external := newRepo(t)
	external.runReport("record", "append", "project-decisions", "kept outside")
	external.record("in external mode the record is outside the working tree",
		external.isFile(external.sharedIdsd()+"/for-agents/decisions.md") && !external.exists(external.treeIdsd()+"/for-agents/decisions.md"),
		external.evidence())
	external.record("and the tree stays clean", external.treeIsFreeOfScratch(), external.indexState())

	committed := newCommittedRepo(t)
	committed.runReport("record", "append", "project-decisions", "committed")
	committed.record("in committed mode it is the tracked one in the tree",
		committed.isFile(committed.treeIdsd()+"/for-agents/decisions.md"), committed.evidence())
}

func TestOnlyAnAppendCreatesARecord(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	for _, op := range []string{"bump", "evict"} {
		f.runReport("record", op, "project-playbook", "nothing is here yet")
		f.assertRefused(op + " on a record that is not there is refused")
		f.assertReports("there is no", op+" says the record is not there")
		// Not tidiness. `playbook.md` is on survivingContent's durable list, so a nought-byte one left
		// by a typo reads as the human's own work for good, and a `discard` they ask for then leaves
		// the whole directory standing on a file a refusal created.
		f.record("and "+op+" leaves no empty record behind",
			!f.exists(recordFile(f, "project-playbook")), joinLines(f.find(f.scratch())))
	}
	// Nor the directory: a refusal that had already created one is a command that did nothing and still
	// left a trace, the same shape as the nought-byte record above.
	f.record("and neither op created the scratch directory", !f.exists(f.scratch()), joinLines(f.find(f.repo+"/.git")))

	f.runReport("record", "amend", "project-playbook", "an operation nobody spelled right")
	f.assertRefused("an unknown operation is refused")
	f.record("and it too leaves no scratch directory behind", !f.exists(f.scratch()), joinLines(f.find(f.repo+"/.git")))

	// With the scratch directory already there, an absent record is the ONLY thing left in the way — so
	// this is the case that observes the `creating` gate itself. Above, the open fails on the missing
	// parent whether or not O_CREATE is set, which is why those assertions pass with the gate removed.
	existing := newRepo(t)
	existing.mkdirAll(existing.scratch() + "/for-agents")
	existing.runReport("record", "bump", "project-playbook", "still nothing is here")
	existing.assertRefused("a bump into an existing scratch directory is still refused when the record is absent")
	existing.assertReports("there is no", "and says the record is not there, not that no entry matched")
	existing.record("and creates no empty record in it",
		!existing.exists(recordFile(existing, "project-playbook")), joinLines(existing.find(existing.scratch())))

	f.runReport("record", "append", "project-playbook", "and this is the call that creates it")
	f.record("while an append creates it", f.status == 0 && f.isFile(recordFile(f, "project-playbook")), f.evidence())
	// 0700, as `init` creates this tree: the records carry a project's decisions and are read from a
	// shared scratch root, so nothing outside this account has business in them.
	mode, err := os.Stat(f.scratch())
	f.record("and the scratch directory it created is reachable by nobody else",
		err == nil && mode.Mode().Perm()&0o077 == 0,
		fmt.Sprintf("%v mode %v", err, mode.Mode().Perm()))
}

func TestARecordIsNeverWrittenWhereGitCanReachIt(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.writeOverride("root " + f.repo + "/inside-the-tree\n")
	f.runReport("check-ignore")
	f.assertRefused("fixture: check-ignore refuses a scratch root inside the working tree")

	// `record` is the only command outside `init` that creates a file under the scratch directory, so
	// it has to ask the same question. Without that it writes where check-ignore refuses to, and the
	// entries land in the human's own `git add -A`.
	f.runReport("record", "append", "project-decisions", "this must not reach the working tree")
	f.assertRefused("and record refuses that layout rather than writing into it")
	f.record("so nothing of the record reaches the working tree at all",
		!f.exists(f.repo+"/inside-the-tree"), joinLines(f.entries(f.repo)))
}

// The split the whole merge design rests on: a ship's record and the project's are two files, not one
// addressed two ways. Asserted from both ends, because a recordPath that ignored the scope would put
// both entries in one file and every assertion about content alone would still read as sensible.
func TestALocalRecordAndTheProjectsAreSeparateFiles(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-splitting")
	f.newIntentFile("001-splitting")

	f.runReport("record", "--intent", "001-splitting", "append", "local-decisions", "this ship settled it")
	f.record("a local append lands", f.status == 0, f.evidence())
	f.runReport("record", "append", "project-decisions", "the project settled it")
	f.record("a project append lands", f.status == 0, f.evidence())

	local := f.read(localRecordFile(f, "001-splitting", "local-decisions"))
	project := f.read(recordFile(f, "project-decisions"))

	f.record("the ship's entry is in the ship's folder and not in the project's",
		strings.Contains(local, "this ship settled it") && !strings.Contains(project, "this ship settled it"),
		"local:\n"+local+"\nproject:\n"+project)
	f.record("and the project's entry is in the project's file and not in the ship's",
		strings.Contains(project, "the project settled it") && !strings.Contains(local, "the project settled it"),
		"local:\n"+local+"\nproject:\n"+project)

	// The weight mechanism finalize reads. A local entry restating a project one is allowed and stays a
	// separate line here — absorbed into a bump now, finalize would never see the duplicate that makes
	// two ships needing one thing visible as a count.
	f.runReport("record", "--intent", "001-splitting", "append", "local-decisions", "the project settled it")
	local = f.read(localRecordFile(f, "001-splitting", "local-decisions"))
	stillProject := f.read(recordFile(f, "project-decisions"))
	f.record("a local entry restating a project one is kept rather than bumped across the boundary",
		f.status == 0 && strings.Count(local, "the project settled it") == 1 &&
			strings.Contains(stillProject, "1x | ") && !strings.Contains(stillProject, "2x | "),
		"exit "+strconv.Itoa(f.status)+"; local:\n"+local+"\nproject:\n"+stillProject)

	// Each file states its own bound and its own pruning point, which records.md requires of a record
	// and which six records under two roots cannot get from one skill.
	f.record("the ship's record states its own bound and pruning point",
		strings.Contains(local, "Full at 25 entries") && strings.Contains(local, "Pruned by the ship that owns it"), local)
	f.record("and the project's states its own",
		strings.Contains(project, "Full at 100 entries") && strings.Contains(project, "Pruned only when a ship is finalized"), project)
}
