package ecoreport_test

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// The mechanical half of finalizing: a ship's own scratch goes, then the folder moves to archive/.
// The order is the point — moved first, the archived folder would carry three local records nothing
// prunes and a report that outlived its pass.
func TestFinalizeArchivesTheShipWithoutItsScratch(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-shipping")
	f.newIntentFile("001-shipping")
	f.runReport("record", "--intent", "001-shipping", "append", "local-decisions", "settled here")
	f.runReport("record", "--intent", "001-shipping", "append", "local-playbook", "run it like this")
	f.runReport("record", "--intent", "001-shipping", "append", "local-language", "a term")

	f.runReport("finalize", "001-shipping")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	archived := f.archiveDir("001-shipping")
	f.record("the intent is archived", f.isFile(archived+"/intent.md"), joinLines(f.find(f.scratch())))
	f.record("and its ship folder is gone from intents/", !f.exists(f.shipDir("001-shipping")),
		joinLines(f.find(f.scratch())))

	// The report goes: it recorded one pass and any later pass reproduces it.
	f.record("the archived folder carries no qualify-report.md", !f.exists(archived+"/for-agents/qualify-report.md"),
		joinLines(f.find(archived)))

	// The three records stay, and this is the half that has to be asserted rather than assumed. The
	// judging half may leave an entry unmerged — on a contradiction, or on a full project cap — so a
	// ship can reach finalize still holding the only copy. `language.md` is the one no later pass can
	// rebuild, a term's meaning not being recoverable from the code that used it.
	for _, kept := range []string{"decisions.md", "playbook.md", "language.md"} {
		f.record("and still carries "+kept+", which may hold the only copy of an unmerged entry",
			f.isFile(archived+"/for-agents/"+kept), joinLines(f.find(archived)))
	}
}

// The merge slot. Held by the stage every ship passes through rather than by a session, because a
// session's lock dies with its tab while the ships behind it keep running.
func TestFinalizeRefusesWhileAnotherShipHoldsTheMergeSlot(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-first")
	f.newIntentFile("001-first")
	// A second ship with a report of its own, or finalize refuses for want of one and the slot is
	// never reached — the case would pass on the wrong refusal.
	f.newIntentFile("002-second")
	f.runReport("init", "002-second")

	f.takeMergeSlot("001-first")
	f.runReport("finalize", "002-second")

	f.record("a second finalize is refused", f.status != 0, f.evidence())
	// Its own code, never the gate's 1 or the did-not-run 2: a session that cannot tell "wait your
	// turn" from "your tree is bad" re-runs gates that were fine, or sits on a red that is real.
	f.record("and says so with its own exit code, not a gate's",
		f.status == 4, "exit "+strconv.Itoa(f.status))
	f.record("and names the holder, so a caller can check it rather than trust it",
		strings.Contains(f.out, "001-first"), f.out)
	f.record("and nothing of the second ship moved",
		f.isFile(f.shipDir("002-second")+"/intent.md") && !f.exists(f.archiveDir("002-second")),
		joinLines(f.find(f.scratch())))
}

// A holder that is gone leaves a slot nothing else can break, and a queue nothing unblocks. The tool
// cannot see whether that session is alive — it is not a process this tool started — so the judgment
// is the caller's and --force is how it is carried out and recorded.
func TestAMergeSlotIsReclaimableOnceItsHolderIsKnownGone(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-only")
	f.newIntentFile("001-only")

	f.takeMergeSlot("002-abandoned")
	f.runReport("finalize", "--force", "001-only")

	f.record("--force reclaims the slot", f.status == 0, f.evidence())
	f.record("and says whose it took rather than doing it silently",
		strings.Contains(f.out, "reclaimed") && strings.Contains(f.out, "002-abandoned"), f.out)
	f.record("and the ship is archived", f.isFile(f.archiveDir("001-only")+"/intent.md"), f.evidence())
}

// The write the slot was built for, asked of the slot at last. Until 2026-09-16 only `finalize` and an
// explicit `merge-slot take` consulted it, so a lane that appended to a project record bypassed the
// exclusion in silence and the ship holding the slot never learned. finalize.go carries where that was
// observed and where it was reproduced.
func TestASharedRecordWriteWaitsForTheSlotItsHolderTook(t *testing.T) {
	t.Parallel()

	// The holder is another worktree of this clone, which is the whole shape of the collision: one slot
	// per clone, and the lane that would overwrite is a sibling checkout, not this one.
	newForeignHolder := func(t *testing.T) *fixture {
		t.Helper()
		f := newShip(t, "001-writing")
		f.takeMergeSlotFrom("002-landing", f.base+"/sibling-worktree")
		return f
	}

	t.Run("refuses a project record while another worktree holds the slot", func(t *testing.T) {
		f := newForeignHolder(t)
		f.runReport("record", "append", "project-decisions", "settled while someone else was landing")

		// The slot's own code, so a caller already handling `merge-slot take`'s exit 4 handles this
		// without learning a second vocabulary for the same wait.
		f.record("and says so with the slot's exit code", f.status == 4, "exit "+strconv.Itoa(f.status)+"\n"+f.out)
		f.record("and names the holder and its worktree, so the waiter can check rather than trust",
			strings.Contains(f.out, "002-landing") && strings.Contains(f.out, "sibling-worktree"), f.out)
		f.record("and the record is untouched, so nothing half-wrote while it waited",
			!strings.Contains(f.read(recordFile(f, "project-decisions")), "settled while someone else"),
			f.read(recordFile(f, "project-decisions")))
	})

	// The control that makes the refusal above mean something: same clone, same slot, same write, held
	// by this worktree. Without it a tool that refused every project write would pass the case above.
	t.Run("writes freely under a slot this worktree holds", func(t *testing.T) {
		f := newShip(t, "001-writing")
		f.runReport("merge-slot", "take", "001-writing")
		f.runReport("record", "append", "project-decisions", "the holder's own judging half")

		f.record("the holder's own write succeeds", f.status == 0, f.evidence())
		f.record("and lands in the record",
			strings.Contains(f.read(recordFile(f, "project-decisions")), "the holder's own judging half"),
			f.read(recordFile(f, "project-decisions")))
	})

	// The second control: no slot at all. A clone where nobody brackets a merge is every single-ship
	// repository, and this fix must not make one of those wait on a file that is never written.
	t.Run("writes freely when no slot is held", func(t *testing.T) {
		f := newShip(t, "001-writing")
		f.runReport("record", "append", "project-decisions", "nobody is landing")

		f.record("a write with no slot standing succeeds", f.status == 0, f.evidence())
	})

	// A local record belongs to one ship and no other lane can reach it, so the slot has no business
	// stopping it. Refusing here would stall a build behind a landing it shares nothing with.
	t.Run("leaves a ship's own local record alone", func(t *testing.T) {
		f := newForeignHolder(t)
		f.runReport("record", "--intent", "001-writing", "append", "local-decisions", "this ship's own")

		f.record("a local write is not held back by another ship's slot", f.status == 0, f.evidence())
		f.record("and lands in the ship's own record",
			strings.Contains(f.read(localRecordFile(f, "001-writing", "local-decisions")), "this ship's own"),
			f.read(localRecordFile(f, "001-writing", "local-decisions")))
	})
}

func (f *fixture) takeMergeSlot(intent string) {
	f.t.Helper()
	f.takeMergeSlotFrom(intent, f.repo)
}

// The slot as a sibling worktree of this clone wrote it. takeMergeSlot above records this fixture's own
// repo, which every record write then reads as its own — so a case about the collision has to name a
// worktree that is not this one.
func (f *fixture) takeMergeSlotFrom(intent, worktree string) {
	f.t.Helper()
	f.write(f.mergeSlotPath(),
		intent+"\n"+worktree+"\n"+strconv.FormatInt(time.Now().Unix(), 10)+"\n")
}

func (f *fixture) mergeSlotPath() string {
	f.t.Helper()
	// `--git-common-dir` answers relative to the repo when asked from inside it, so joining it onto the
	// repo is what makes this the path the tool resolves rather than one under the test's own cwd.
	common := f.mustGit("rev-parse", "--git-common-dir")
	if !strings.HasPrefix(common, "/") {
		common = f.canonicalRepo() + "/" + common
	}
	return common + "/idsd-merge-slot"
}

// A slot the caller was already holding is the caller's to release. The finalizing skill takes it
// before finalize and releases it after step 4, because the judging half writes the project's records
// between them — outside this process, and unable to hold a lock of its own. finalize releasing it on
// the way out hands the rest of that bracket to whoever is waiting, which is the collision the slot
// exists to prevent rather than a slot merely leaked.
func TestFinalizeLeavesASlotItsCallerIsHolding(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-bracketed")
	f.newIntentFile("001-bracketed")

	f.runReport("merge-slot", "take", "001-bracketed")
	f.record("the caller can take the slot for its own ship", f.status == 0, f.evidence())

	f.runReport("finalize", "001-bracketed")
	f.record("finalize succeeds under a slot its caller already holds", f.status == 0, f.evidence())
	f.record("and the ship is archived, so the case reached the end it is about",
		f.isFile(f.archiveDir("001-bracketed")+"/intent.md"), joinLines(f.find(f.scratch())))
	f.record("and the slot is still held, because step 4 has not run",
		f.isFile(f.mergeSlotPath()), f.mergeSlotPath())
}

// The control, and the half that makes the case above mean something: a finalize that took the slot
// itself still drops it. Without this, a tool that never released anything would pass that test.
func TestFinalizeReleasesASlotItTookItself(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-unbracketed")
	f.newIntentFile("001-unbracketed")

	f.runReport("finalize", "001-unbracketed")
	f.record("finalize succeeds having taken its own slot", f.status == 0, f.evidence())
	f.record("and leaves no slot behind, or the next ship waits on nobody",
		!f.exists(f.mergeSlotPath()), f.mergeSlotPath())
}

// The subcommand a caller runs to hold the slot across the judging half of a merge. takeMergeSlot
// above writes the slot file directly, so this path — the only one a caller actually takes — was
// driven by nothing, and it recorded the word `take` as the holder.
func TestTakingTheMergeSlotHoldsItForTheShipNamed(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-holding")
	f.newIntentFile("001-holding")

	f.runReport("merge-slot", "take", "001-holding")
	f.record("merge-slot take reports the ship it was given",
		f.status == 0 && strings.Contains(f.out, "001-holding"), f.evidence())

	// The harm, where it lands. finalize's already-ours arm compares the recorded holder against the
	// intent it is finalizing, so a slot recorded under any other name turns away the ship that
	// legitimately holds it — exit 4, and nothing in the clone finalizes again without --force.
	f.runReport("finalize", "001-holding")
	f.record("and finalize runs under the slot that ship already holds",
		f.status == 0 && f.isFile(f.archiveDir("001-holding")+"/intent.md"), f.evidence())

	// Release has to reach the same file the take wrote, or a caller that brackets its own merge leaves
	// the slot standing and every later ship waits behind a holder that finished.
	released := newShip(t, "002-releasing")
	released.newIntentFile("002-releasing")
	released.runReport("merge-slot", "take", "002-releasing")
	released.runReport("merge-slot", "release")
	released.record("release gives the slot up", released.status == 0, released.evidence())
	released.runReport("merge-slot", "take", "003-someone-else")
	released.runReport("finalize", "002-releasing")
	released.record("and the slot is takeable again by whoever comes next",
		released.status == 4 && strings.Contains(released.out, "003-someone-else"), released.evidence())
}

// The verb is the subcommand's own first argument now, so the forms that name none have to refuse
// rather than index past the end of the slice.
func TestMergeSlotRefusesEveryFormThatNamesNoSlotToAct(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-usage")
	for _, args := range [][]string{
		{"merge-slot"},
		{"merge-slot", "hold"},
		{"merge-slot", "take"},
		{"merge-slot", "take", "--force"},
	} {
		form := strings.Join(args[1:], " ")
		f.runReport(args...)
		f.assertRefused("merge-slot refuses '" + form + "'")
		f.assertReports("usage: report.sh merge-slot", "and prints the grammar for '"+form+"'")
	}
	// And none of them took a slot on the way to refusing: one written by a refused call is one nothing
	// releases, and every later finalize in the clone waits behind it.
	f.newIntentFile("001-usage")
	f.runReport("finalize", "001-usage")
	f.record("and no refused call left a slot standing", f.status == 0, f.evidence())
}

// The records have to reach the index, not merely the disk: nothing under the archive path is tracked
// or ignored, so without staging the pathspec of whatever the ship commits next decides all three.
func TestFinalizeStagesTheArchivedRecordsInCommittedMode(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-shipping")
	f.runReport("record", "--intent", "001-shipping", "append", "local-decisions", "settled here")
	f.runReport("record", "--intent", "001-shipping", "append", "local-playbook", "run it like this")
	f.runReport("record", "--intent", "001-shipping", "append", "local-language", "a term")

	f.runReport("finalize", "001-shipping")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	staged, _ := f.git("diff", "--name-only", "--cached")
	for _, kept := range []string{"decisions.md", "playbook.md", "language.md"} {
		path := ".idsd/archive/001-shipping/for-agents/" + kept
		f.record("the archived "+kept+" is staged, so no later pathspec can drop it",
			containsLine(staged, path), "staged:\n"+staged)
	}
}

// The other mode, where the same staging would be wrong: an external .idsd/ lives under the git dir,
// outside any tree git could add. A `git add` there fails, and a finalize that refused on it would
// break every repo that never promoted its idsd.
func TestFinalizeStagesNothingInExternalMode(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-shipping")
	f.newIntentFile("001-shipping")
	f.runReport("record", "--intent", "001-shipping", "append", "local-decisions", "settled here")

	f.runReport("finalize", "001-shipping")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	staged, _ := f.git("diff", "--name-only", "--cached")
	f.record("and staged nothing, the scratch being outside the tree", staged == "", "staged:\n"+staged)
}

// The move has two halves and only one is a new file: intent.md is tracked under intents/ while the ship
// is active, so the rename leaves it deleted on disk and still in the index there. Staging the archive
// alone writes a tree holding the intent at both paths, which every later checkout then restores.
func TestFinalizeStagesTheMoveRatherThanACopy(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-shipping")
	f.mustGit("add", ".idsd/intents/001-shipping/intent.md")
	f.commit("the active intent")

	f.runReport("finalize", "001-shipping")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	staged, _ := f.git("diff", "--name-status", "--cached", "--no-renames")
	f.record("the vacated path is staged as deleted, so the commit is a move",
		containsLine(staged, "D\t.idsd/intents/001-shipping/intent.md"), "staged:\n"+staged)
	f.record("and the archived intent is staged as added",
		containsLine(staged, "A\t.idsd/archive/001-shipping/intent.md"), "staged:\n"+staged)
}

// Only the ship's own files are named, so nothing else that reached the folder rides into the index.
func TestFinalizeStagesNoStrayFileFoundInTheShipFolder(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-shipping")
	f.runReport("record", "--intent", "001-shipping", "append", "local-decisions", "settled here")
	f.write(f.shipDir("001-shipping")+"/for-agents/captured.log", "whatever an agent dropped here\n")

	f.runReport("finalize", "001-shipping")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	staged, _ := f.git("diff", "--name-only", "--cached")
	f.record("the record is staged",
		containsLine(staged, ".idsd/archive/001-shipping/for-agents/decisions.md"), "staged:\n"+staged)
	f.record("and the stray file beside it is not",
		!containsLine(staged, ".idsd/archive/001-shipping/for-agents/captured.log"), "staged:\n"+staged)
}

// The silent half of the same loss: an ignore rule reaching the archive path would lose the records again
// while the tool printed that it staged them. Named explicitly, the add refuses and so does finalize.
func TestFinalizeRefusesWhenAnArchivedRecordIsIgnored(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-shipping")
	f.runReport("record", "--intent", "001-shipping", "append", "local-decisions", "settled here")
	f.write(f.repo+"/.gitignore", ignoreBlock()+".idsd/archive/*/for-agents/*.md\n")

	f.runReport("finalize", "001-shipping")
	f.assertRefused("a record git will not stage refuses rather than passing silently")
	f.assertReports("git said:", "and quotes git's own account of why")
	f.record("and the refusal stays on one line per message, so nothing in a path forges another",
		countLines(f.out, func(line string) bool { return strings.HasPrefix(line, "  git said: ") }) == 1, f.out)
}

// The branch where staging fails outright. The archive is already on disk by then, which is why the
// refusal says nothing needs re-running: a re-run would only meet "already exists".
func TestFinalizeRefusesWhenTheArchiveCannotBeStaged(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-shipping")
	f.runReport("record", "--intent", "001-shipping", "append", "local-decisions", "settled here")
	// What a concurrent `git add` in this worktree leaves behind. Reads still answer, so the repo mode
	// still resolves and the staging branch is the one that fails.
	f.write(f.repo+"/.git/index.lock", "")

	f.runReport("finalize", "001-shipping")
	f.assertRefused("a failed staging refuses")
	f.assertReports("stage "+f.archiveDir("001-shipping")+" yourself", "and says what to do by hand")
	f.record("the archive is on disk regardless, so the refusal is not a re-run instruction",
		f.isFile(f.archiveDir("001-shipping")+"/for-agents/decisions.md"), f.evidence())
}

// The removal is staged as a directory pathspec, so it covers every tracked file under the ship folder.
// `for-agents/supporting/` is where an intent's handoffs and evidence go, and nothing ignores them — a
// named add list that misses one stages its deletion with no addition, and the commit drops the file.
func TestFinalizeStagesASupportingFileTheShipHadCommitted(t *testing.T) {
	t.Parallel()
	f := newCommittedShip(t, "001-shipping")
	f.write(f.shipDir("001-shipping")+"/for-agents/supporting/handoff.md", "evidence the ship committed\n")
	f.mustGit("add", ".idsd/intents/001-shipping/intent.md", ".idsd/intents/001-shipping/for-agents/supporting/handoff.md")
	f.commit("the active intent and its evidence")

	f.runReport("finalize", "001-shipping")
	f.record("finalize succeeds", f.status == 0, f.evidence())

	staged, _ := f.git("diff", "--name-status", "--cached", "--no-renames")
	f.record("the vacated supporting file is staged as deleted",
		containsLine(staged, "D\t.idsd/intents/001-shipping/for-agents/supporting/handoff.md"), "staged:\n"+staged)
	f.record("and its archived copy is staged as added, so the commit moves it rather than dropping it",
		containsLine(staged, "A\t.idsd/archive/001-shipping/for-agents/supporting/handoff.md"), "staged:\n"+staged)
}
