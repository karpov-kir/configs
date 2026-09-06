package ecoreport_test

// `promote` writes to the human's index, and every refusal past the point it MOVES the scratch into
// the tree owes a move back — left off, the intents sit untracked in the working tree while the human
// has been told the promotion did not happen.

import (
	"strings"
	"testing"
)

func TestCheckIgnoreHoldsBeforeQualifyReportsExists(t *testing.T) {
	t.Parallel()
	// `check-ignore` runs before the first write into `.idsd/`, and its exit 1 blocks that write. So it
	// has to answer correctly while qualify-reports/ does not exist yet, and that is where the trailing
	// slash in the ignore surface earns its keep: without it, `git check-ignore -q
	// .idsd/qualify-reports` exits 1 on a directory that is not there.
	f := newCommittedRepo(t)
	if !f.exists(f.scratch()+"/intents") && f.runReportStdout("repo-mode") == "committed" {
		f.runReport("check-ignore")
		f.record("check-ignore passes in committed mode before qualify-reports/ is created",
			f.status == 0, f.evidence())
	} else {
		f.record("fixture is not a committed repo with qualify-reports/ absent", false, "")
	}
}

func TestPromoteReportsTheModeNotTheAdd(t *testing.T) {
	t.Parallel()
	// qualify-reports/ is ignored by the entry promote itself writes, and `git add` on a directory
	// whose every file is ignored stages nothing and exits 0. With nothing else under .idsd/, reading
	// success off that add leaves repo-mode still saying throwaway, and the next check-ignore
	// re-excludes .idsd/, silently undoing the promotion.
	f := newShip(t, "001-nothing-durable")
	f.runReport("promote")
	f.assertRefused("promote refuses when everything under .idsd/ is ignored")
	f.record("and the repo is still a throwaway, as the refusal says", f.runReportStdout("repo-mode") == "throwaway", "")
	// The refusal's real obligation now that promote MOVES the scratch: it must not have moved it. A
	// half-done promotion leaves the intents in the tree, untracked and unignored, with the tool saying
	// it refused.
	f.record("and left the scratch where it was, with nothing in the working tree",
		f.treeIsFreeOfScratch() && f.exists(f.sharedIdsd()), f.evidence())

	// The same repo with one durable file: promote now has something to stage, and the mode flips.
	f.write(f.scratch()+"/intents-placeholder.md", "# intent\n")
	f.runReport("promote")
	f.record("promote succeeds once .idsd/ holds something that is not ignored",
		f.status == 0 && f.runReportStdout("repo-mode") == "committed", f.evidence())

	// Committed mode takes the other check-ignore branch entirely: the one that asks git rather than
	// writing an exclusion, and the only one that can confirm the entry instead of creating it.
	f.runReport("check-ignore")
	f.record("committed mode confirms qualify-reports/ is gitignored",
		f.status == 0 && strings.Contains(f.out, "gitignored"), f.evidence())

	// And the warning fires when it is not: the entry is what keeps a report out of `git add -A`.
	f.write(f.repo+"/.gitignore", "")
	f.runReport("check-ignore")
	f.record("and warns when the entry is missing",
		f.status == 1 && strings.Contains(f.out, "NOT gitignored"), f.evidence())
}

func TestNoRefusalLeavesTheScratchStrandedInTheTree(t *testing.T) {
	t.Parallel()
	// promote moves the scratch into the tree and then stages it. A refusal after the move and before
	// the staging is the dangerous shape: the reports are in the tree, and the human was told the
	// promotion did not happen.
	f := newShip(t, "001-promoting")
	f.newIntentFile("001-promoting")
	f.write(f.repo+"/.git/index.lock", "")
	f.runReport("promote")
	f.assertRefused("promote refuses when it cannot stage")
	// The move is undone, so "not promoted" is the whole truth rather than half of it.
	f.record("and put the scratch back rather than stranding it in the tree",
		f.treeIsFreeOfScratch() && f.exists(f.sharedIdsd()), f.evidence())
	f.assertReports("back at", "and says so, so the human is not left guessing where their intents went")
	f.remove(f.repo + "/.git/index.lock")

	// promote needs one report as its evidence a ship happened here, which is why it does not go
	// through the report requirement every other reader opens with.
	bare := newRepo(t)
	bare.runReport("check-ignore")
	bare.runReport("promote")
	bare.assertRefused("promote refuses with no report at all")
	// Asserted on the message: without this refusal `promote` runs on and `git add .idsd` fails because
	// the directory is not there, so it still exits 2, for a reason that is not this guard.
	bare.assertReports("nothing to promote", "and names the missing report as the reason")
}

func TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex(t *testing.T) {
	t.Parallel()
	// The mode decides whether .idsd/ is durable, so every caller that acts on it owes the check.
	f := newShip(t, "001-modes")
	if f.madeUnreadable(f.repo+"/.git/index", "the unreadable-index cases") {
		// Again the message rather than the exit: without the assertion both subcommands still exit 2,
		// because a later git call fails on the same unreadable index. Only the message tells the two
		// apart, and only the assertion stops the mode being read as "throwaway", the answer that deletes.
		f.runReport("promote")
		f.assertRefused("promote refuses when the repo mode cannot be read")
		f.assertReports("repo mode is unknown", "and names the unreadable mode as the reason")
		f.runReport("check-ignore")
		f.assertRefused("check-ignore refuses when the repo mode cannot be read")
		f.assertReports("repo mode is unknown", "and names it there too")
	}
	f.chmod(f.repo+"/.git/index", 0o644)
}

func TestPromoteIsIdempotentOverACommittedRepo(t *testing.T) {
	t.Parallel()
	// `promote` is run by hand and by `idsd-ship`, so it meets repos already promoted. Past the mode
	// check it appends to .gitignore, moves the scratch into the tree and runs `git add` — and over an
	// already-durable .idsd/ that add is the human's own staging area being written for nothing.
	f := newCommittedRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-already-durable")
	// An unstaged edit to a tracked file under .idsd/, which is what `git add .idsd` would sweep up.
	// It is the human's work, and nothing here asked for it to be staged.
	f.appendTo(f.scratch()+"/charter.md", "a line the human has not staged\n")
	before := f.indexState()

	f.runReport("promote")
	f.record("promote reports the repo already committed and exits 0",
		f.status == 0 && strings.Contains(f.out, "already committed"), f.evidence())
	f.record("and stages nothing, leaving the human's index as it was",
		f.indexState() == before, "before:\n"+before+"\nafter:\n"+f.indexState())
}

func TestPromoteWritesNoGitignoreThroughALink(t *testing.T) {
	t.Parallel()
	// promote claims two things: .gitignore names qualify-reports/, and git acts on it. Three ways
	// that claim fails, each of which would otherwise be reported as a promotion that happened — a
	// link that takes the write out of the repo, a write that cannot land, and an entry git does not
	// act on. The last is the one that matters most: it leaves the report stageable.
	linked := newShip(t, "001-linked-gitignore")
	linked.newIntentFile("001-linked-gitignore")
	outside := linked.base + "/outside.gitignore"
	linked.write(outside, "# not the repo's\n")
	linked.symlink(outside, linked.repo+"/.gitignore")
	linked.runReport("promote")
	linked.assertRefused("promote refuses a symlinked .gitignore")
	linked.assertReports("is a symlink", "and names the link rather than the git answer downstream of it")
	linked.record("and wrote nothing through it, so the file outside the repo is untouched",
		linked.read(outside) == "# not the repo's\n", "it now reads:\n"+linked.read(outside))
	linked.record("and left the scratch unmoved, with nothing in the working tree",
		linked.treeIsFreeOfScratch() && linked.exists(linked.sharedIdsd()), linked.evidence())

	// A write that cannot land, by construction rather than by a mode bit: .gitignore is a directory,
	// so the append fails with EISDIR for every user, root included.
	unwritable := newShip(t, "001-unwritable-gitignore")
	unwritable.newIntentFile("001-unwritable-gitignore")
	unwritable.mkdirAll(unwritable.repo + "/.gitignore")
	unwritable.runReport("promote")
	unwritable.assertRefused("promote refuses when the .gitignore entry cannot be written")
	// The message carries this one: without the guard promote runs on to the next check, which also
	// refuses, for a reason that sends the human to look at git rather than at the failed write.
	unwritable.assertReports("could not add", "and names the write that failed")
	unwritable.record("and left the scratch unmoved", unwritable.treeIsFreeOfScratch() && unwritable.exists(unwritable.sharedIdsd()), unwritable.evidence())

	// An entry written that git does not act on. The instance the guard names is a .gitignore git
	// cannot read, which cannot be built without a mode bit; a negation reaches the same state by
	// construction — the entry is in the root .gitignore and the surface is stageable anyway.
	//
	// Seeded in the ROOT .gitignore, with the entry already present so promote's append dedupes and
	// the negation after it is what git acts on (last match wins). A negation inside the scratch dir
	// would ride along with promote's move and land in the tree, and the target-is-clear guard would
	// refuse first if it were seeded in the tree — either way the case would pass for the wrong reason.
	unread := newShip(t, "001-negated")
	unread.newIntentFile("001-negated")
	unread.write(unread.repo+"/.gitignore", ignoreBlock()+"!"+reportEntry()+"\n")
	unread.runReport("promote")
	unread.assertRefused("promote refuses when the entry is written but git still does not ignore the surface")
	unread.assertReports("git still does not ignore", "and says the entry landed without taking effect")
	// The harm, asserted where it lands. The report carries the pass's security findings, and a
	// promotion taken here puts it in the index on its way to a commit.
	staged, _ := unread.git("diff", "--cached", "--name-only")
	unread.record("and staged nothing, the report included",
		!strings.Contains(staged, "qualify-report.md"), "staged:\n"+staged)
	unread.record("and left the scratch unmoved, so no report reached the tree",
		unread.treeIsFreeOfScratch() && unread.exists(unread.sharedIdsd()), unread.evidence())
}
