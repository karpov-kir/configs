package ecoreport_test

// `discard` is the destructive path: it removes .idsd/ and deletes the intent file, in the mode that
// keeps no copy of either anywhere. Every case here is a deletion that must not happen.

import (
	"strconv"
	"strings"
	"testing"
)

func TestDiscardRemovesNothingItCouldNotRead(t *testing.T) {
	t.Parallel()
	// `discard` reads the slug to decide what to delete. An unreadable report yields no slug, so
	// without the readability refusal it deletes the report and orphans the intent: `state` then
	// answers no-report for work that was never built.
	f := newShip(t, "001-discarding")
	f.newIntentFile("001-discarding")
	if !f.madeUnreadable(f.reportPath("001-discarding"), "the discard case") {
		t.Skip("this process reads a mode-0 file regardless of the mode (root, or CAP_DAC_OVERRIDE), so an unreadable report cannot be built here")
	}
	f.runReport("discard", "001-discarding")
	f.assertRefused("discard refuses a report it cannot read")
	f.record("and removed neither the report nor the intent file",
		f.isFile(f.reportPath("001-discarding")) && f.isFile(f.scratch()+"/intents/001-discarding.md"), "")
	f.chmod(f.reportPath("001-discarding"), 0o644)
}

// The reports directory decides what survives. `survivingContent` keeps .idsd/ standing when another
// ship's report is in flight, and it learns that by listing this directory — so a listing that failed
// reads as "no other ship", and `discard` goes on to take the whole .idsd/, which in throwaway mode is
// the only copy anyone has. An empty directory and one this cannot open are the same shape and not the
// same fact.
func TestDiscardWillNotClearIdsdOnAReportListingItCouldNotRead(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-mine")
	f.newIntentFile("001-mine")
	// A second ship's report, in flight, with no other copy anywhere. It is what must survive.
	f.runReport("init", "002-yours")
	if !f.madeUnreadable(f.scratch()+"/qualify-reports", "the unreadable report-listing case") {
		t.Skip("this process lists a mode-0 directory regardless of the mode (root, or CAP_DAC_OVERRIDE), so an unreadable listing cannot be built here")
	}
	f.runReport("discard", "001-mine")
	f.chmod(f.scratch()+"/qualify-reports", 0o755)

	f.assertRefused("discard refuses a report listing it could not read")
	// The one thing throwaway mode keeps no copy of. This ship's own files are already gone by here,
	// and that is what discard is for; the refusal stands between the listing and `os.RemoveAll(.idsd)`.
	f.record("and left the other ship's report where it was",
		f.isFile(f.reportPath("002-yours")), f.evidence())
}

func TestDiscardReconcilesTheTwoNamesBeforeDeletingAnything(t *testing.T) {
	t.Parallel()
	// discard is addressed by the filename, and deletes the intent file the frontmatter names.
	// Disagreeing, it deletes another ship's in-flight intent, which throwaway mode keeps no copy of
	// anywhere.
	f := newShip(t, "001-mine")
	f.newIntentFile("002-yours")
	f.newIntentFile("001-mine")
	f.replaceLine(f.reportPath("001-mine"), "intent: 001-mine", "intent: 002-yours")
	f.runReport("discard", "001-mine")
	f.assertRefused("discard refuses when the filename and the frontmatter name different ships")
	f.record("and deleted neither intent file",
		f.isFile(f.scratch()+"/intents/002-yours.md") && f.isFile(f.scratch()+"/intents/001-mine.md"), "")
}

func TestDiscardDestructivePath(t *testing.T) {
	t.Parallel()
	only := newShip(t, "001-only-ship")
	only.newIntentFile("001-only-ship")
	only.runReport("invalidate", "001-only-ship")
	only.runReport("stage-returned", "code-review", "001-only-ship")
	markers := only.repo + "/.git/idsd-stage-returns/001-only-ship"
	only.runReport("discard", "001-only-ship")
	only.assertIdsdRemoved("discard removes the whole .idsd/ when this ship was the only thing in it")
	// The stage markers live in the git dir, so removing .idsd/ cannot reach them. They need their own
	// removal, or the next ship for this intent inherits a completed stage record and stamps for free.
	only.record("and the stage markers in the git dir, which removing .idsd/ never reaches", !only.exists(markers), "")
	// Zero traces means the working tree too. There is no exclusion to drop any more, so what has to
	// hold is that nothing was ever put in the tree for one to hide.
	only.record("and left nothing in the working tree either", only.treeIsFreeOfScratch(), "")

	// A durable file is the human's, never this ship's scratch, so it keeps .idsd/ standing.
	charter := newShip(t, "001-with-charter")
	charter.newDurableCharterInScratch()
	charter.runReport("discard", "001-with-charter")
	charter.record("discard keeps .idsd/ for a durable file while removing this ship's report",
		charter.status == 0 && charter.isFile(charter.scratch()+"/charter.md") &&
			!charter.isFile(charter.reportPath("001-with-charter")), charter.evidence())
	charter.assertReports("charter.md", "and names what kept it standing")
	charter.record("and the surviving charter is outside the working tree, so nothing hides it", charter.treeIsFreeOfScratch(), "")

	// A parallel ship is another human's work in flight. Deleting its report is the unrecoverable case.
	parallel := newShip(t, "001-going")
	parallel.runReport("init", "002-staying")
	parallel.newIntentFile("001-going")
	parallel.newIntentFile("002-staying")
	parallel.runReport("discard", "001-going")
	parallel.record("discard removes only the named ship, leaving a parallel one whole",
		parallel.status == 0 && parallel.isFile(parallel.reportPath("002-staying")) &&
			parallel.isFile(parallel.scratch()+"/intents/002-staying.md") &&
			!parallel.isFile(parallel.scratch()+"/intents/001-going.md"),
		"exit "+strconv.Itoa(parallel.status)+"; reports: "+joinLines(parallel.entries(parallel.scratch()+"/qualify-reports"))+
			"; intents: "+joinLines(parallel.entries(parallel.scratch()+"/intents")))
	parallel.assertReports("other qualify report", "and names the parallel ship as what kept .idsd/ alive")

	// An intent that already built has its file in archive/ rather than intents/, and both are this ship's.
	built := newShip(t, "001-archived")
	built.mkdirAll(built.scratch() + "/archive")
	built.write(built.scratch()+"/archive/001-archived.md", "# built\n")
	built.runReport("discard", "001-archived")
	built.assertIdsdRemoved("discard removes the intent file from archive/ as well as intents/")

	// Committed mode: .idsd/ is the durable record, so there is nothing here to discard and the refusal
	// is the only thing standing between this subcommand and the human's tracked files.
	committed := newCommittedRepo(t)
	committed.runReport("init", "001-committed")
	committed.runReport("discard", "001-committed")
	committed.assertRefused("discard refuses in committed mode, where .idsd/ is the durable record")
	committed.record("and deleted nothing",
		committed.isFile(committed.scratch()+"/charter.md") && committed.isFile(committed.reportPath("001-committed")), "")

	// `discard` runs after `close`, the order `idsd-ship done` uses; reversed, `close` finds no report
	// and refuses. `close` deletes the report `discard` reads, and a `discard` that refuses on that
	// leaves the .idsd/ it was to clear standing.
	closed := newShip(t, "001-closed-then-discarded")
	closed.newIntentFile("001-closed-then-discarded")
	closed.runReport("close", "001-closed-then-discarded")
	closed.runReport("discard", "001-closed-then-discarded")
	closed.assertIdsdRemoved("discard runs after close, with no report left to read")

	// Unnamed and with no report, there is nothing to identify. That refuses, and says naming the
	// intent is the way through, or a closed ship could never be discarded at all.
	unnamed := newShip(t, "001-unnamed")
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
	f := newShip(t, "001-real")
	f.newIntentFile("001-real")
	f.runReport("discard", "002-nothing-of-mine")
	f.assertRefused("discard refuses a slug that names no report and no intent file")
	// Asserted on the refusal's own effect, not on the sibling surviving: the sibling's report keeps
	// .idsd/ alive either way, so "it is untouched" holds with the guard gone.
	f.record("and the reports directory it would have torn down still stands",
		!f.exists(f.scratch()+"/intents/002-nothing-of-mine.md") &&
			f.exists(f.scratch()+"/qualify-reports") && f.isFile(f.scratch()+"/intents/001-real.md"), "")
	f.assertReports("Looked for", "and names every path it looked in")

	// A typo must not tear down a directory. `decisions.md` alone does not keep .idsd/ alive by design,
	// so without the guard this reports "zero traces" for a ship that never existed.
	typo := newRepo(t)
	typo.runReport("check-ignore")
	typo.mkdirAll(typo.scratch() + "")
	typo.write(typo.scratch()+"/decisions.md", "# decisions\n")
	typo.runReport("discard", "999-typo")
	typo.assertRefused("discard refuses a typo rather than removing the directory around it")
	typo.record("and the decision log survives", typo.isFile(typo.scratch()+"/decisions.md"), "")

	// A repo that never used idsd has nothing to lose; the guard is what stops discard tearing down a
	// scratch dir it never created.
	empty := newRepo(t)
	empty.runReport("check-ignore")
	empty.runReport("discard", "001-never-existed")
	empty.assertRefused("discard refuses in a repo that holds no ship at all")
	empty.record("and wrote nothing into the working tree on the way to refusing", empty.treeIsFreeOfScratch(), "")

	// The guard must not break the case it was built around: a landed ship whose report `close` retired
	// is still identified by its intent file, so close-then-discard survives.
	landed := newShip(t, "001-landed")
	landed.newIntentFile("001-landed")
	landed.runReport("close", "001-landed")
	landed.runReport("discard", "001-landed")
	landed.assertIdsdRemoved("and a closed ship is still discardable, identified by its intent file")
}

func TestDiscardRefusesWhenTheRepoModeCannotBeRead(t *testing.T) {
	t.Parallel()
	// An unreadable index fails the index read, which reads as "nothing tracked", so a committed repo
	// reports throwaway and discard's committed-mode refusal never fires.
	f := newRepo(t)
	// Both files in the tree, because this fixture is COMMITTED: git can only track what the tree holds,
	// and the scratch-side helpers write outside it.
	f.newDurableCharter()
	f.mkdirAll(f.treeIdsd() + "/intents")
	f.write(f.treeIdsd()+"/intents/002-tracked.md", "# intent\n")
	f.write(f.repo+"/.gitignore", ".idsd/qualify-reports/\n")
	f.mustGit("add", ".gitignore", ".idsd/charter.md", ".idsd/intents/002-tracked.md")
	f.commit("committed idsd")
	// Built by hand rather than through the committed-repo builder, because this one needs the intent
	// file tracked, so it owes the same assertion that builder carries. Checked before the index is made
	// unreadable, since `repo-mode` cannot answer afterwards.
	f.assertFixtureIsCommitted()
	if !f.madeUnreadable(f.repo+"/.git/index", "the unreadable-index case") {
		t.Skip("this process reads a mode-0 file regardless of the mode (root, or CAP_DAC_OVERRIDE), so an unreadable index cannot be built here")
	}
	f.runReport("discard", "002-tracked")
	f.assertRefused("discard refuses when the repo mode cannot be read, rather than assuming throwaway")
	// The REASON, not just the exit. Without the mode assertion the run reads throwaway, walks on, and is
	// refused a few lines later for having no ship at the resolved location — also exit 2, from a guard
	// that says nothing about the unreadable index. Asserting the code alone observes neither.
	f.assertReports("could not read the index", "and names the unreadable index as why")
	// Named directly rather than through f.scratch(): that asks git which mode this is, and the index it
	// would ask is the very thing this case made unreadable.
	f.record("and the tracked intent file survives", f.isFile(f.treeIdsd()+"/intents/002-tracked.md"), "")
	f.chmod(f.repo+"/.git/index", 0o644)
}

func TestDiscardRefusesASymlinkedIdsdRatherThanDeletingThroughIt(t *testing.T) {
	t.Parallel()
	// Without init's symlink guard here, an untracked .idsd link lets discard delete through to a
	// target outside the repo.
	linked := newRepo(t)
	linked.runReport("check-ignore")
	linked.mkdirAll(linked.base + "/outside-discard/intents")
	linked.write(linked.base+"/outside-discard/intents/003-elsewhere.md", "# not ours to delete\n")
	linked.symlink(linked.base+"/outside-discard", linked.scratch()+"")
	linked.runReport("discard", "003-elsewhere")
	linked.assertRefused("discard refuses a symlinked .idsd rather than deleting through it")
	linked.record("and the file outside the repo is still there",
		linked.isFile(linked.base+"/outside-discard/intents/003-elsewhere.md"), "")
	linked.remove(linked.scratch() + "")
}

func TestAStandaloneReviewCanStillBeTornDownAfterItIsClosed(t *testing.T) {
	t.Parallel()
	// `review` is the one stem with no intent file, and idsd-qualify's SKILL.md tells the agent to run
	// `close review`, so this sequence is the documented one. Without the `review` exception it ends in
	// a permanent refusal, leaving an empty .idsd/ and its exclusion standing in the mode whose whole
	// contract is zero traces.
	f := newShip(t, "review: a standalone pass")
	f.runReport("close", "review")
	f.runReport("discard", "review")
	present := "gone"
	if f.exists(f.sharedIdsd()) {
		present = "present"
	}
	f.record("discard tears down a closed standalone review, leaving the tree clean",
		f.status == 0 && !f.exists(f.sharedIdsd()) && f.treeIsFreeOfScratch(),
		"exit "+strconv.Itoa(f.status)+"; scratch: "+present)
}

func TestEveryDurableFileKeepsIdsdStanding(t *testing.T) {
	t.Parallel()
	// The durable four are a table in the source, and what a row buys is that .idsd/ survives this
	// ship's discard: those files are the human's own, never the ship's scratch. A row dropped from
	// that list deletes the file it names and reports zero traces, so every row gets a fixture.
	for _, durable := range []string{"charter.md", "constitution.md", "language.md", "playbook.md"} {
		f := newShip(t, "001-durable")
		f.write(f.scratch()+"/"+durable, "# the human's own\n")
		f.runReport("discard", "001-durable")
		f.record(durable+" alone keeps .idsd/ standing through a discard",
			f.status == 0 && f.isFile(f.scratch()+"/"+durable) && !f.isFile(f.reportPath("001-durable")),
			"exit "+strconv.Itoa(f.status)+"; left: "+joinLines(f.find(f.scratch()+""))+"\n"+f.out)
		f.assertReports(durable, "and discard names "+durable+" as what kept it")
	}

	// Another ship's intent file is the only thing under .idsd/ that identifies that ship once its own
	// report is closed, and throwaway mode keeps no copy of it anywhere. Nothing else here survives
	// this discard, so the intents/-and-archive/ arm is the only thing standing between the two.
	sibling := newShip(t, "001-going")
	sibling.newIntentFile("001-going")
	sibling.newIntentFile("002-still-in-flight")
	sibling.runReport("discard", "001-going")
	sibling.record("another ship's intent file keeps .idsd/ standing",
		sibling.status == 0 && sibling.isFile(sibling.scratch()+"/intents/002-still-in-flight.md") &&
			!sibling.isFile(sibling.scratch()+"/intents/001-going.md"),
		"exit "+strconv.Itoa(sibling.status)+"; left: "+joinLines(sibling.find(sibling.scratch()+""))+"\n"+sibling.out)
	sibling.assertReports("1 other intent(s)", "and counts it as an intent rather than as stray content")

	// The label is the whole deliverable for what is left below: .idsd/ stands either way, and what
	// differs is what the human is told is in it. "1 other intent(s)" for a stray .DS_Store says
	// another ship is in flight when none is.
	stray := newShip(t, "001-stray")
	stray.newIntentFile("001-stray")
	stray.write(stray.scratch()+"/intents/.DS_Store", "not an intent\n")
	stray.runReport("discard", "001-stray")
	stray.record("a stray file under intents/ keeps .idsd/ standing",
		stray.status == 0 && stray.exists(stray.scratch()+"/intents"), stray.evidence())
	stray.assertReports("unrecognised content under intents/", "and is reported as unrecognised, never counted as an intent")
	stray.record("and no number of intents is claimed for it",
		!strings.Contains(stray.out, "other intent(s)"), stray.out)

	// `find -type f` semantics: a symlink to an intent file is not one. Counted as one, the label
	// claims a ship in flight for a link that points at nothing.
	linked := newShip(t, "001-linked")
	linked.newIntentFile("001-linked")
	linked.symlink(linked.base+"/no-such-intent.md", linked.scratch()+"/intents/002-link.md")
	linked.runReport("discard", "001-linked")
	linked.record("a symlink named like an intent keeps .idsd/ standing",
		linked.status == 0 && linked.exists(linked.scratch()+"/intents"), linked.evidence())
	linked.assertReports("unrecognised content under intents/", "and a symlink is not counted as an intent file")
	linked.record("and no number of intents is claimed for a link",
		!strings.Contains(linked.out, "other intent(s)"), linked.out)
}
