package ecoreport_test

// The merge gate itself, and the two things that read the same stamp: the routing token and the
// carry list. The gate has four block reasons and one clean line, and a gate that never clears is
// as useless as one that never blocks — so the clean line is asserted before any of the blocks.

import (
	"os"
	"strings"
	"testing"
)

func TestGateBlocksOnEachOfItsReasonsAndClearsOnNone(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-gating")
	f.stampFullPass("001-gating")

	// The clean gate first, because nothing below means anything without it: every block could be a
	// gate that blocks on everything, and each case would pass on a gate that never let a merge through.
	f.runReport("gate", "001-gating")
	f.record("gate clears a full pass on a fresh tree with nothing open",
		f.status == 0 && strings.Contains(f.out, "gate clean"), f.evidence())

	// The report is git-ignored, so editing it moves nothing the fingerprint reads: the freshness arm
	// stays clear and each arm below is the only one that can block.
	f.appendTo(f.reportPath("001-gating"), "- [ ] a decision nobody routed\n")
	f.runReport("gate", "001-gating")
	f.record("gate blocks on an open item, with no override",
		f.status == 1 && strings.Contains(f.out, "open TODOs"), f.evidence())
	f.assertReports("a decision nobody routed", "and prints the item it blocked on")
	f.record("and does not also report itself clean", !strings.Contains(f.out, "gate clean"), f.out)
	f.dropLines(f.reportPath("001-gating"), "- [ ] a decision")

	// A stage record that says nothing. The two shapes are the template's placeholder and no line at
	// all, and both mean no untrimmed pass stands behind this report — a reader that knows only one of
	// them lets the other through.
	f.replaceLine(f.reportPath("001-gating"), "reviewed-stages:", "reviewed-stages: <stages>")
	f.runReport("gate", "001-gating")
	f.record("gate blocks on a report carrying the template's stage placeholder",
		f.status == 1 && strings.Contains(f.out, "no reviewed-stages record"), f.evidence())
	f.dropLines(f.reportPath("001-gating"), "reviewed-stages:")
	f.runReport("gate", "001-gating")
	f.record("and on a report carrying no stage line at all",
		f.status == 1 && strings.Contains(f.out, "no reviewed-stages record"), f.evidence())

	// A scan that did not run is not a scan that found nothing: read as one, a report still holding
	// unrouted items passes the merge gate.
	f.write(f.todoGatePath(), "#!/bin/sh\nexit 3\n")
	f.chmod(f.todoGatePath(), 0o755)
	f.runReport("gate", "001-gating")
	f.record("gate blocks when the open-item scan did not run",
		f.status == 1 && strings.Contains(f.out, "todo-gate.sh exited 3"), f.evidence())

	// The seam is checked before it is run, the way the fingerprint script's is: both are located from
	// this program's own path, so a slash-less argv[0] resolves either against the invocation's
	// directory instead of the skill's. Asserted on the phrase rather than the exit, because the
	// unchecked path also blocks — it just blocks quoting whatever exec returned, with no name in it.
	f.chmod(f.todoGatePath(), 0o644)
	if info, err := os.Stat(f.todoGatePath()); err == nil && info.Mode()&0o111 != 0 {
		// A filesystem holding no execute bit to drop leaves the script runnable, and the assertion
		// below would go red for the fixture rather than for the tool.
		t.Logf("skip  this filesystem does not hold the execute bit — the unexecutable case cannot run")
	} else {
		f.runReport("gate", "001-gating")
		f.record("gate names an open-item scan that is there but not executable",
			f.status == 1 && strings.Contains(f.out, "is missing or not executable"), f.evidence())
	}
	f.chmod(f.todoGatePath(), 0o755)
}

// The ICE's own `## Follow-ups` are the build's loose ends, and they live outside the report. Nothing
// else reads them before a merge, so a gate scanning only the report lets every one of them through.
func TestGateScansTheShipsIntentFileAsWellAsItsReport(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-follow-ups")
	f.newIntentFile("001-follow-ups")
	f.stampFullPass("001-follow-ups")

	f.runReport("gate", "001-follow-ups")
	f.record("gate clears with an intent file holding nothing open",
		f.status == 0 && strings.Contains(f.out, "gate clean"), f.evidence())

	// Written into the intent, not the report — and the intent is git-ignored scratch here, so the
	// freshness arm stays clear and this is the only thing that can block.
	f.appendTo(f.shipDir("001-follow-ups")+"/intent.md",
		"\n## Follow-ups\n\n- [ ] tell the consumers the payload changed\n")
	f.runReport("gate", "001-follow-ups")
	f.record("gate blocks on an open item in the intent file",
		f.status == 1 && strings.Contains(f.out, "clear each in the intent"), f.evidence())
	f.assertReports("tell the consumers the payload changed", "and prints the item it blocked on")

	// And the routing token reads the same two files. Answering `ready` here would send `continue` to
	// present a merge the gate refuses one command later.
	f.runReport("state", "001-follow-ups")
	f.record("the routing token sees an item the report does not hold",
		f.status == 0 && strings.TrimSpace(f.out) == "decide", f.evidence())

	// With no intent file the three intent arms have nothing to read, and the clean line must not claim
	// they did — "approved intent" over a ship whose ICE is still being authored is the one sentence a
	// reader takes the gate's word for.
	f.remove(f.shipDir("001-follow-ups") + "/intent.md")
	f.runReport("gate", "001-follow-ups")
	f.record("gate clears a ship with no intent file, claiming no approval it did not read",
		f.status == 0 && strings.Contains(f.out, "no intent to check"), f.evidence())
	f.record("and never calls that an approved intent",
		!strings.Contains(f.out, "approved intent"), f.out)
}

func TestATrimmedPassIsNotAFullOne(t *testing.T) {
	t.Parallel()
	// `(turnaround)` is the one word that marks a stage trimmed to answer sooner, which is why the vocabulary is
	// closed rather than pattern-matched: any other word for the same trim would record a trimmed pass
	// as a full one, and the merge gate reads that record.
	f := newShip(t, "001-trimmed")
	f.runReport("invalidate", "001-trimmed")
	f.runReport("decisions-reviewed", "001-trimmed")
	// Every stage but the trimmed one is marked. A skipped entry is not required to have returned, and
	// leaving security-review unmarked is what pins that.
	for _, stage := range []string{"code-review", "tighten", "refactor"} {
		f.runReport("stage-returned", stage, "001-trimmed")
		f.runReport("no-items", stage, "001-trimmed")
	}
	f.runReport("stamp", "code-review,security-review:skipped(turnaround),tighten,refactor", "001-trimmed")
	f.record("a stage skipped for turnaround stamps without having returned",
		f.status == 0, f.evidence())

	f.runReport("state", "001-trimmed")
	f.record("state answers finalize for a trimmed pass, never ready", f.out == "finalize", "said '"+f.out+"'")

	f.runReport("gate", "001-trimmed")
	f.record("gate blocks a pass trimmed for turnaround",
		f.status == 1 && strings.Contains(f.out, "trimmed for turnaround"), f.evidence())
	f.assertReports("security-review:skipped(turnaround)", "and names the stage that was trimmed")
}

func TestCarryPrintsTheItemsARequalifyMustNotLose(t *testing.T) {
	t.Parallel()
	// `carry` is what a re-qualify reads to keep the last pass's open items, and no copy of them is kept
	// anywhere else. Printing nothing is indistinguishable from a report with nothing open.
	f := newShip(t, "001-carrying")
	f.appendTo(f.reportPath("001-carrying"), "- [ ] an item the next pass must carry\n")
	printed := f.runReportStdout("carry", "001-carrying")
	f.record("carry prints the open items on stdout",
		strings.Contains(printed, "an item the next pass must carry"), "stdout was: "+printed)
	f.runReport("carry", "001-carrying")
	f.record("and exits 0 having printed them", f.status == 0, f.evidence())
}

// The gate's fourth reason, and the only one that reads the ICE rather than the report: an intent that
// never reached `approved` is one whose gap rounds did not close, so what it leaves open was answered
// by an implementer alone. The clearing arm is asserted first — a gate that blocked on every intent
// would pass every block below while letting nothing merge.
func TestGateBlocksAnIntentTheGapRoundsNeverApproved(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-gating")
	f.stampFullPass("001-gating")

	// The intent sits in the git-ignored scratch, so writing it moves nothing the fingerprint reads and
	// the freshness arm stays clear. It carries no `- [ ]`, so the open-item arm stays clear too.
	writeIntent := func(frontmatter string) {
		f.mkdirAll(f.scratch() + "/intents")
		f.write(f.shipDir("001-gating")+"/intent.md", "---\ntitle: t\n"+frontmatter+"---\n\n# intent\n")
	}

	writeIntent("status: approved\n")
	f.runReport("gate", "001-gating")
	f.record("gate clears a ship whose intent is approved",
		f.status == 0 && strings.Contains(f.out, "gate clean"), f.evidence())

	writeIntent("status: draft\n")
	f.runReport("gate", "001-gating")
	f.record("gate blocks a ship whose intent is still draft",
		f.status == 1 && strings.Contains(f.out, "BLOCK (intent)"), f.evidence())
	f.assertReports("status: draft", "and names the status it read rather than only that it was wrong")
	f.record("and does not also report itself clean", !strings.Contains(f.out, "gate clean"), f.out)

	// Absent is not approved. Read as one, an ICE nobody filled in reaches the archive — which is the
	// whole reason this arm exists.
	writeIntent("")
	f.runReport("gate", "001-gating")
	f.record("gate blocks an intent carrying no status line at all",
		f.status == 1 && strings.Contains(f.out, "status: <none>"), f.evidence())

	// The template documents `status:` on the field's own line. Blank the value and read it raw, and
	// that explanation IS the status — a string none of the gate's arms match, so it blocks naming that
	// string rather than naming the intent as unfilled.
	writeIntent("status:              # draft → approved → built\n")
	f.runReport("gate", "001-gating")
	f.record("gate reads a blanked status carrying only the field's own comment as no status",
		f.status == 1 && strings.Contains(f.out, "status: <none>"), f.evidence())

	writeIntent("status: approved     # draft → approved → built\n")
	f.runReport("gate", "001-gating")
	f.record("and clears an approved status that kept that same comment after the value",
		f.status == 0 && strings.Contains(f.out, "gate clean"), f.evidence())

	// `built` is what Phase 5 sets on its way to archive/, and it is not a licence to merge a second
	// time: the arm asks for the one state the gap rounds produce.
	writeIntent("status: built\n")
	f.runReport("gate", "001-gating")
	f.record("gate blocks an intent already marked built but still sitting in intents/",
		f.status == 1 && strings.Contains(f.out, "BLOCK (intent)"), f.evidence())

	// `ready` means the gate will clear. A token that said so over an intent this gate refuses would
	// route `continue` to a merge, and the human would meet the block one command later.
	writeIntent("status: draft\n")
	f.runReport("state", "001-gating")
	f.record("the routing token sends an unapproved intent back to build rather than to the merge",
		f.status == 0 && strings.TrimSpace(f.out) == "resume", f.evidence())

	writeIntent("status: approved\n")
	f.runReport("state", "001-gating")
	f.record("and answers ready once it is approved, matching the gate it stands in front of",
		f.status == 0 && strings.TrimSpace(f.out) == "ready", f.evidence())
}
