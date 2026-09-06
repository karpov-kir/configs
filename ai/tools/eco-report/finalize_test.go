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

	for _, scratch := range []string{"decisions.md", "playbook.md", "language.md", "qualify-report.md"} {
		f.record("the archived folder carries no "+scratch, !f.exists(archived+"/"+scratch),
			joinLines(f.find(archived)))
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

// The slot names a worktree because that is what a session owns for its whole life — the name a
// caller can check against its own live sessions rather than trust.
func (f *fixture) takeMergeSlot(intent string) {
	f.t.Helper()
	// `--git-common-dir` answers relative to the repo when asked from inside it, so joining it onto the
	// repo is what makes this the path the tool resolves rather than one under the test's own cwd.
	common := f.mustGit("rev-parse", "--git-common-dir")
	if !strings.HasPrefix(common, "/") {
		common = f.canonicalRepo() + "/" + common
	}
	f.write(common+"/idsd-merge-slot",
		intent+"\n"+f.repo+"\n"+strconv.FormatInt(time.Now().Unix(), 10)+"\n")
}
