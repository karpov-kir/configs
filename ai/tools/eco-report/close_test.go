package ecoreport_test

// `close` retires a landed ship's scratch. Its open-item refusal is the only thing keeping a
// hand-run from dropping a decision nobody routed: no copy of the report is kept anywhere.

import (
	"testing"
)

func TestCloseRetiresOneShipScratchAndNothingElse(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-closing")
	f.runReport("init", "002-staying")
	f.appendTo(f.reportPath("001-closing"), "- [ ] a decision nobody routed\n")
	f.runReport("close", "001-closing")
	f.assertRefused("close refuses while an open '- [ ]' stands")
	f.assertReports("a decision nobody routed", "and names the item it would have discarded")
	f.record("and the report is still there", f.isFile(f.reportPath("001-closing")), "")

	f.runReport("close", "001-closing", "--force")
	f.record("--force closes the named report and leaves the sibling ship alone",
		f.status == 0 && !f.isFile(f.reportPath("001-closing")) && f.isFile(f.reportPath("002-staying")),
		"exit "+itoa(f.status)+"\n"+f.out)

	// `--force` shares its charset with a legal slug, so read positionally it resolves as an intent
	// name and closes a report that does not exist, reporting success while the real one stands.
	alone := newRepo(t)
	alone.runReport("check-ignore")
	alone.runReport("init", "review: force alone")
	alone.runReport("close", "--force")
	alone.record("close reads --force as a flag, not as the intent name",
		alone.status == 0 && !alone.isFile(alone.reportPath("")), "exit "+itoa(alone.status)+"\n"+alone.out)

	// The stage markers sit in the git dir, which removing the report never reaches, and they are keyed
	// by the report stem — so the next ship for the same intent inherits them. Asserted on the dir as
	// well as on its effect: `discard` has its own case for this, `close` had none.
	relanding := newRepo(t)
	relanding.runReport("check-ignore")
	relanding.runReport("init", "001-relanding")
	relanding.runReport("stage-returned", "code-review", "001-relanding")
	markers := relanding.repo + "/.git/idsd-stage-returns/001-relanding"
	relanding.record("fixture: the closing ship marked a stage returned", relanding.isFile(markers+"/code-review"),
		"exit "+itoa(relanding.status)+"\n"+relanding.out)
	relanding.runReport("close", "001-relanding")
	relanding.record("close takes the ship's stage markers with its report",
		relanding.status == 0 && !relanding.exists(markers), joinLines(relanding.find(markers)))
	// What an inherited marker does to the next ship. A report scaffolded from the same template for
	// the same intent is byte-identical to the one just closed, so the stale marker's checksum matches
	// it: the first stage to return is refused for a return the closed ship made, and nothing in the
	// new pass can clear a marker it does not know is there.
	relanding.runReport("init", "001-relanding")
	relanding.runReport("stage-returned", "security-review", "001-relanding")
	relanding.record("so the next ship for that intent is not refused for the closed one's return",
		relanding.status == 0, "exit "+itoa(relanding.status)+"\n"+relanding.out)
}

func TestCloseOnACleanReportThePathDoneRuns(t *testing.T) {
	t.Parallel()
	// The unforced path, the one `idsd-ship done` invokes, on a report whose items are all cleared.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-landed")
	f.runReport("close", "001-landed")
	f.record("close needs no --force once nothing is open",
		f.status == 0 && !f.isFile(f.reportPath("001-landed")), "exit "+itoa(f.status)+"\n"+f.out)

	// `close` retires a landed ship's report, and the archived intent file is then the only record it
	// landed. Read absence alone and `state` answers `no-report`, which `idsd-ship continue` routes to
	// "start ship <intent>": rebuilding work already merged.
	archived := newRepo(t)
	archived.runReport("check-ignore")
	archived.runReport("init", "001-landed-and-archived")
	archived.mkdirAll(archived.repo + "/.idsd/archive")
	archived.write(archived.repo+"/.idsd/archive/001-landed-and-archived.md", "# built and archived\n")
	archived.runReport("close", "001-landed-and-archived")
	archived.runReport("state", "001-landed-and-archived")
	archived.record("state answers done for a closed report whose intent is archived",
		archived.out == "done", "said '"+archived.out+"'")

	// And an intent that was never archived still reads no-report once closed. The archive is the fact
	// being read, not the closing.
	archived.runReport("init", "002-closed-unbuilt")
	archived.runReport("close", "002-closed-unbuilt")
	archived.runReport("state", "002-closed-unbuilt")
	archived.record("and no-report still answers for a closed report with no archived intent",
		archived.out == "no-report", "said '"+archived.out+"'")
}
