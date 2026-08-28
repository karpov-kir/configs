package ecoreport_test

// `discard` is the destructive path: it removes .idsd/ and deletes the intent file, in the mode that
// keeps no copy of either anywhere. Every case here is a deletion that must not happen.

import (
	"strings"
	"syscall"
	"testing"
)

func TestDiscardRemovesNothingItCouldNotRead(t *testing.T) {
	t.Parallel()
	// `discard` reads the slug to decide what to delete. An unreadable report yields no slug, so
	// without the readability refusal it deletes the report and orphans the intent: `state` then
	// answers no-report for work that was never built.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-discarding")
	f.newIntentFile("001-discarding")
	if f.madeUnreadable(f.reportPath("001-discarding"), "the discard case") {
		f.runReport("discard", "001-discarding")
		f.assertRefused("discard refuses a report it cannot read")
		f.record("and removed neither the report nor the intent file",
			f.isFile(f.reportPath("001-discarding")) && f.isFile(f.repo+"/.idsd/intents/001-discarding.md"), "")
	}
	f.chmod(f.reportPath("001-discarding"), 0o644)
}

func TestDiscardReconcilesTheTwoNamesBeforeDeletingAnything(t *testing.T) {
	t.Parallel()
	// discard is addressed by the filename, and deletes the intent file the frontmatter names.
	// Disagreeing, it deletes another ship's in-flight intent, which throwaway mode keeps no copy of
	// anywhere.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-mine")
	f.newIntentFile("002-yours")
	f.newIntentFile("001-mine")
	f.replaceLine(f.reportPath("001-mine"), "intent: 001-mine", "intent: 002-yours")
	f.runReport("discard", "001-mine")
	f.assertRefused("discard refuses when the filename and the frontmatter name different ships")
	f.record("and deleted neither intent file",
		f.isFile(f.repo+"/.idsd/intents/002-yours.md") && f.isFile(f.repo+"/.idsd/intents/001-mine.md"), "")
}

func TestDiscardDestructivePath(t *testing.T) {
	t.Parallel()
	only := newRepo(t)
	only.runReport("check-ignore")
	only.runReport("init", "001-only-ship")
	only.newIntentFile("001-only-ship")
	only.runReport("invalidate", "001-only-ship")
	only.runReport("stage-returned", "code-review", "001-only-ship")
	markers := only.repo + "/.git/idsd-stage-returns/001-only-ship"
	only.runReport("discard", "001-only-ship")
	only.assertIdsdRemoved("discard removes the whole .idsd/ when this ship was the only thing in it")
	// The stage markers live in the git dir, so removing .idsd/ cannot reach them. They need their own
	// removal, or the next ship for this intent inherits a completed stage record and stamps for free.
	only.record("and the stage markers in the git dir, which removing .idsd/ never reaches", !only.exists(markers), "")
	// Zero traces means the local exclusion too, or .git/info/exclude keeps a line for a dir that is gone.
	only.record("and the local exclusion, so the throwaway leaves nothing at all", !only.hasLocalExclusion(), "")

	// A durable file is the human's, never this ship's scratch, so it keeps .idsd/ standing.
	charter := newRepo(t)
	charter.runReport("check-ignore")
	charter.runReport("init", "001-with-charter")
	charter.newDurableCharter()
	charter.runReport("discard", "001-with-charter")
	charter.record("discard keeps .idsd/ for a durable file while removing this ship's report",
		charter.status == 0 && charter.isFile(charter.repo+"/.idsd/charter.md") &&
			!charter.isFile(charter.reportPath("001-with-charter")), "exit "+itoa(charter.status)+"\n"+charter.out)
	charter.assertReports("charter.md", "and names what kept it standing")
	charter.record("and leaves the exclusion in place, since .idsd/ is still there to hide", charter.hasLocalExclusion(), "")

	// A parallel ship is another human's work in flight. Deleting its report is the unrecoverable case.
	parallel := newRepo(t)
	parallel.runReport("check-ignore")
	parallel.runReport("init", "001-going")
	parallel.runReport("init", "002-staying")
	parallel.newIntentFile("001-going")
	parallel.newIntentFile("002-staying")
	parallel.runReport("discard", "001-going")
	parallel.record("discard removes only the named ship, leaving a parallel one whole",
		parallel.status == 0 && parallel.isFile(parallel.reportPath("002-staying")) &&
			parallel.isFile(parallel.repo+"/.idsd/intents/002-staying.md") &&
			!parallel.isFile(parallel.repo+"/.idsd/intents/001-going.md"),
		"exit "+itoa(parallel.status)+"; reports: "+joinLines(parallel.entries(parallel.repo+"/.idsd/qualify-reports"))+
			"; intents: "+joinLines(parallel.entries(parallel.repo+"/.idsd/intents")))
	parallel.assertReports("other qualify report", "and names the parallel ship as what kept .idsd/ alive")

	// An intent that already built has its file in archive/ rather than intents/, and both are this ship's.
	built := newRepo(t)
	built.runReport("check-ignore")
	built.runReport("init", "001-archived")
	built.mkdirAll(built.repo + "/.idsd/archive")
	built.write(built.repo+"/.idsd/archive/001-archived.md", "# built\n")
	built.runReport("discard", "001-archived")
	built.assertIdsdRemoved("discard removes the intent file from archive/ as well as intents/")

	// Committed mode: .idsd/ is the durable record, so there is nothing here to discard and the refusal
	// is the only thing standing between this subcommand and the human's tracked files.
	committed := newCommittedRepo(t)
	committed.runReport("init", "001-committed")
	committed.runReport("discard", "001-committed")
	committed.assertRefused("discard refuses in committed mode, where .idsd/ is the durable record")
	committed.record("and deleted nothing",
		committed.isFile(committed.repo+"/.idsd/charter.md") && committed.isFile(committed.reportPath("001-committed")), "")

	// `discard` runs after `close`, the order `idsd-ship done` uses; reversed, `close` finds no report
	// and refuses. `close` deletes the report `discard` reads, and a `discard` that refuses on that
	// leaves the .idsd/ it was to clear standing.
	closed := newRepo(t)
	closed.runReport("check-ignore")
	closed.runReport("init", "001-closed-then-discarded")
	closed.newIntentFile("001-closed-then-discarded")
	closed.runReport("close", "001-closed-then-discarded")
	closed.runReport("discard", "001-closed-then-discarded")
	closed.assertIdsdRemoved("discard runs after close, with no report left to read")

	// Unnamed and with no report, there is nothing to identify. That refuses, and says naming the
	// intent is the way through, or a closed ship could never be discarded at all.
	unnamed := newRepo(t)
	unnamed.runReport("check-ignore")
	unnamed.runReport("init", "001-unnamed")
	unnamed.runReport("close", "001-unnamed")
	unnamed.runReport("discard")
	unnamed.assertRefused("discard refuses when no report is left and no intent is named")
	unnamed.assertReports("Name the intent", "and says naming the intent is what gets past it")
}

func TestDiscardDeletesNothingForAShipThatIsNotHere(t *testing.T) {
	t.Parallel()
	// What the guard reaches is a slug naming nothing. A slug that names a real ship is still
	// discardable, report closed or never made, and has to be: that is the close-then-discard case. So
	// the residual exposure is a wrong slug that happens to name a live ship, which no guard can tell
	// from an intended one.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-real")
	f.newIntentFile("001-real")
	f.runReport("discard", "002-nothing-of-mine")
	f.assertRefused("discard refuses a slug that names no report and no intent file")
	// Asserted on the refusal's own effect, not on the sibling surviving: the sibling's report keeps
	// .idsd/ alive either way, so "it is untouched" holds with the guard gone.
	f.record("and the reports directory it would have torn down still stands",
		!f.exists(f.repo+"/.idsd/intents/002-nothing-of-mine.md") &&
			f.exists(f.repo+"/.idsd/qualify-reports") && f.isFile(f.repo+"/.idsd/intents/001-real.md"), "")
	f.assertReports("Looked for", "and names every path it looked in")

	// A typo must not tear down a directory. `decisions.md` alone does not keep .idsd/ alive by design,
	// so without the guard this reports "zero traces" for a ship that never existed.
	typo := newRepo(t)
	typo.runReport("check-ignore")
	typo.mkdirAll(typo.repo + "/.idsd")
	typo.write(typo.repo+"/.idsd/decisions.md", "# decisions\n")
	typo.runReport("discard", "999-typo")
	typo.assertRefused("discard refuses a typo rather than removing the directory around it")
	typo.record("and the decision log survives", typo.isFile(typo.repo+"/.idsd/decisions.md"), "")

	// A repo that never used idsd has no .idsd/ and no exclusion to lose; the guard is what stops
	// discard reaching the exclusion teardown there.
	empty := newRepo(t)
	empty.runReport("check-ignore")
	empty.runReport("discard", "001-never-existed")
	empty.assertRefused("discard refuses in a repo that holds no ship at all")
	empty.record("and leaves the local exclusion alone", empty.hasLocalExclusion(), "")

	// The guard must not break the case it was built around: a landed ship whose report `close` retired
	// is still identified by its intent file, so close-then-discard survives.
	landed := newRepo(t)
	landed.runReport("check-ignore")
	landed.runReport("init", "001-landed")
	landed.newIntentFile("001-landed")
	landed.runReport("close", "001-landed")
	landed.runReport("discard", "001-landed")
	landed.assertIdsdRemoved("and a closed ship is still discardable, identified by its intent file")
}

func TestTheDestructivePathCarriesTheGuardsTheWritePathHas(t *testing.T) {
	t.Parallel()
	// An unreadable index fails the index read, which reads as "nothing tracked", so a committed repo
	// reports throwaway and discard's committed-mode refusal never fires.
	f := newRepo(t)
	f.newDurableCharter()
	f.newIntentFile("002-tracked")
	f.write(f.repo+"/.gitignore", ".idsd/qualify-reports/\n")
	f.mustGit("add", ".gitignore", ".idsd/charter.md", ".idsd/intents/002-tracked.md")
	f.commit("committed idsd")
	// Built by hand rather than through the committed-repo builder, because this one needs the intent
	// file tracked, so it owes the same assertion that builder carries. Checked before the index is made
	// unreadable, since `repo-mode` cannot answer afterwards.
	f.assertFixtureIsCommitted()
	if f.madeUnreadable(f.repo+"/.git/index", "the unreadable-index case") {
		f.runReport("discard", "002-tracked")
		f.assertRefused("discard refuses when the repo mode cannot be read, rather than assuming throwaway")
		f.record("and the tracked intent file survives", f.isFile(f.repo+"/.idsd/intents/002-tracked.md"), "")
	}
	f.chmod(f.repo+"/.git/index", 0o644)

	// Without init's symlink guard here, an untracked .idsd link lets discard delete through to a
	// target outside the repo.
	linked := newRepo(t)
	linked.runReport("check-ignore")
	linked.mkdirAll(linked.base + "/outside-discard/intents")
	linked.write(linked.base+"/outside-discard/intents/003-elsewhere.md", "# not ours to delete\n")
	linked.symlink(linked.base+"/outside-discard", linked.repo+"/.idsd")
	linked.runReport("discard", "003-elsewhere")
	linked.assertRefused("discard refuses a symlinked .idsd rather than deleting through it")
	linked.record("and the file outside the repo is still there",
		linked.isFile(linked.base+"/outside-discard/intents/003-elsewhere.md"), "")
	linked.remove(linked.repo + "/.idsd")
}

func TestAStandaloneReviewCanStillBeTornDownAfterItIsClosed(t *testing.T) {
	t.Parallel()
	// `review` is the one stem with no intent file, and idsd-qualify's SKILL.md tells the agent to run
	// `close review`, so this sequence is the documented one. Without the `review` exception it ends in
	// a permanent refusal, leaving an empty .idsd/ and its exclusion standing in the mode whose whole
	// contract is zero traces.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "review: a standalone pass")
	f.runReport("close", "review")
	f.runReport("discard", "review")
	present := "gone"
	if f.exists(f.repo + "/.idsd") {
		present = "present"
	}
	f.record("discard tears down a closed standalone review, exclusion included",
		f.status == 0 && !f.exists(f.repo+"/.idsd") && !f.hasLocalExclusion(),
		"exit "+itoa(f.status)+"; .idsd: "+present)
}

func TestTheTeardownReportsTheExclusionFromTheResultNotTheAttempt(t *testing.T) {
	t.Parallel()
	// With the exclusion teardown's return discarded, "zero traces" prints at exit 0 over a surviving
	// entry. That is the one claim here a human acts on without checking.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-teardown")
	f.newIntentFile("001-teardown")
	f.chmod(f.repo+"/.git/info", 0o500)
	if syscall.Access(f.repo+"/.git/info", 0x2) == nil {
		f.t.Logf("skip  chmod does not restrict this user (root?) — the exclusion-failure case cannot run")
		f.chmod(f.repo+"/.git/info", 0o755)
		return
	}
	f.runReport("discard", "001-teardown")
	f.chmod(f.repo+"/.git/info", 0o755)
	f.record("discard does not claim zero traces when the exclusion could not be removed",
		f.status != 0 && !strings.Contains(f.out, "zero traces"), "exit "+itoa(f.status)+"; said: "+f.out)
}

func TestEveryDurableFileKeepsIdsdStanding(t *testing.T) {
	t.Parallel()
	// The durable four are a table in the source, and what a row buys is that .idsd/ survives this
	// ship's discard: those files are the human's own, never the ship's scratch. A row dropped from
	// that list deletes the file it names and reports zero traces, so every row gets a fixture.
	for _, durable := range []string{"charter.md", "constitution.md", "language.md", "playbook.md"} {
		f := newRepo(t)
		f.runReport("check-ignore")
		f.runReport("init", "001-durable")
		f.write(f.repo+"/.idsd/"+durable, "# the human's own\n")
		f.runReport("discard", "001-durable")
		f.record(durable+" alone keeps .idsd/ standing through a discard",
			f.status == 0 && f.isFile(f.repo+"/.idsd/"+durable) && !f.isFile(f.reportPath("001-durable")),
			"exit "+itoa(f.status)+"; left: "+joinLines(f.find(f.repo+"/.idsd"))+"\n"+f.out)
		f.assertReports(durable, "and discard names "+durable+" as what kept it")
	}

	// Another ship's intent file is the only thing under .idsd/ that identifies that ship once its own
	// report is closed, and throwaway mode keeps no copy of it anywhere. Nothing else here survives
	// this discard, so the intents/-and-archive/ arm is the only thing standing between the two.
	sibling := newRepo(t)
	sibling.runReport("check-ignore")
	sibling.runReport("init", "001-going")
	sibling.newIntentFile("001-going")
	sibling.newIntentFile("002-still-in-flight")
	sibling.runReport("discard", "001-going")
	sibling.record("another ship's intent file keeps .idsd/ standing",
		sibling.status == 0 && sibling.isFile(sibling.repo+"/.idsd/intents/002-still-in-flight.md") &&
			!sibling.isFile(sibling.repo+"/.idsd/intents/001-going.md"),
		"exit "+itoa(sibling.status)+"; left: "+joinLines(sibling.find(sibling.repo+"/.idsd"))+"\n"+sibling.out)
	sibling.assertReports("1 other intent(s)", "and counts it as an intent rather than as stray content")

	// The label is the whole deliverable for what is left below: .idsd/ stands either way, and what
	// differs is what the human is told is in it. "1 other intent(s)" for a stray .DS_Store says
	// another ship is in flight when none is.
	stray := newRepo(t)
	stray.runReport("check-ignore")
	stray.runReport("init", "001-stray")
	stray.newIntentFile("001-stray")
	stray.write(stray.repo+"/.idsd/intents/.DS_Store", "not an intent\n")
	stray.runReport("discard", "001-stray")
	stray.record("a stray file under intents/ keeps .idsd/ standing",
		stray.status == 0 && stray.exists(stray.repo+"/.idsd/intents"), "exit "+itoa(stray.status)+"\n"+stray.out)
	stray.assertReports("unrecognised content under intents/", "and is reported as unrecognised, never counted as an intent")
	stray.record("and no number of intents is claimed for it",
		!strings.Contains(stray.out, "other intent(s)"), stray.out)

	// `find -type f` semantics: a symlink to an intent file is not one. Counted as one, the label
	// claims a ship in flight for a link that points at nothing.
	linked := newRepo(t)
	linked.runReport("check-ignore")
	linked.runReport("init", "001-linked")
	linked.newIntentFile("001-linked")
	linked.symlink(linked.base+"/no-such-intent.md", linked.repo+"/.idsd/intents/002-link.md")
	linked.runReport("discard", "001-linked")
	linked.record("a symlink named like an intent keeps .idsd/ standing",
		linked.status == 0 && linked.exists(linked.repo+"/.idsd/intents"), "exit "+itoa(linked.status)+"\n"+linked.out)
	linked.assertReports("unrecognised content under intents/", "and a symlink is not counted as an intent file")
	linked.record("and no number of intents is claimed for a link",
		!strings.Contains(linked.out, "other intent(s)"), linked.out)
}

func TestASecondWorktreeKeepsTheSharedExclusion(t *testing.T) {
	t.Parallel()
	// .git/info/exclude is one file for every worktree of a repository, so only the last discard may
	// drop the '.idsd/' line. Dropped from the first, a parallel throwaway ship's .idsd/ becomes
	// visible to the next `git add -A` in the other worktree, which is how it reaches a commit.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-shared")
	f.mustGit("worktree", "add", "-q", f.base+"/second-worktree", "-b", "second")
	// The fixture's own precondition: with one worktree the case below passes on a discard that never
	// consults the count at all.
	f.record("fixture: git reports two worktrees",
		countLinesWithPrefix(f.mustGit("worktree", "list", "--porcelain"), "worktree ") == 2, "")

	f.runReport("discard", "001-shared")
	f.record("discard removes this ship's .idsd/ with a second worktree open",
		f.status == 0 && !f.exists(f.repo+"/.idsd"), "exit "+itoa(f.status)+"\n"+f.out)
	f.record("and keeps the exclusion the other worktree shares", f.hasLocalExclusion(), "")
	f.assertReports("kept the shared exclusion", "and says so rather than claiming zero traces")

	// The positive control: the same discard does drop the exclusion once this is the only worktree
	// left, so what kept it above was the count and not a discard that never drops it.
	f.mustGit("worktree", "remove", f.base+"/second-worktree")
	f.runReport("check-ignore")
	f.runReport("init", "002-last")
	f.runReport("discard", "002-last")
	f.record("and the last worktree's discard drops it, leaving zero traces",
		f.status == 0 && !f.hasLocalExclusion(), "exit "+itoa(f.status)+"\n"+f.out)
	f.assertReports("zero traces", "and claims zero traces only there")
}
