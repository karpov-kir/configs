package ecoreport_test

// `state` and `list` are what `idsd-ship continue` routes on, so a token they cannot stand behind
// routes a live ship past a gate, and a listing that stops halfway reads exactly like a complete one.

import (
	"strings"
	"testing"
)

func TestTwoIntentsShipSideBySide(t *testing.T) {
	t.Parallel()
	// The whole point of the per-intent path: a second intent's init is not a collision, so neither
	// ship has to be finished before the other starts. And `check-ignore` first, as a real pass does,
	// or the report sits inside its own fingerprint and every state below reads `re-qualify`.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-first-intent")
	f.runReport("init", "002-second-intent")
	f.record("a second intent gets its own report rather than a refusal",
		f.status == 0 && f.isFile(f.reportPath("001-first-intent")) && f.isFile(f.reportPath("002-second-intent")),
		"exit "+itoa(f.status)+"\n"+f.out)

	f.runReport("gate")
	f.assertRefused("a subcommand refuses to guess which of two reports it means")
	f.assertReports("001-first-intent", "and lists both by name")
	f.assertReports("002-second-intent", "and lists the second too")

	f.runReport("invalidate", "002-second-intent")
	f.record("the named report is the only one acted on",
		f.status == 0 && containsLine(f.read(f.reportPath("002-second-intent")), "reviewed-tree: pending") &&
			containsLine(f.read(f.reportPath("001-first-intent")), "reviewed-tree: <hash>"),
		"exit "+itoa(f.status)+"\n"+f.out)

	f.runReport("stage-returned", "code-review", "001-first-intent")
	f.runReport("invalidate", "002-second-intent")
	f.runReport("no-items", "code-review", "001-first-intent")
	f.record("one intent's invalidate leaves the other's stage markers standing", f.status == 0,
		"exit "+itoa(f.status)+"\n"+f.out)

	// The state column is asserted by value, not by "a tab follows the name". The looser form is
	// satisfied by a listing that emits an empty token for every ship, or `BOGUS` where `resume`
	// belongs. `list` is the surface `idsd-ship continue` routes on with several ships in flight.
	f.runReport("list")
	first, second := stateOf(f.out, "001-first-intent"), stateOf(f.out, "002-second-intent")
	f.record("list prints one line per open ship, each with its state", first == "resume" && second == "resume",
		"001 -> '"+first+"', 002 -> '"+second+"'")

	// And the states must be the ships' own, not one ship's answer repeated: stamping only 001 must
	// move only 001's column.
	f.stampFullPass("001-first-intent")
	f.runReport("list")
	f.record("and each state is that ship's own, not one answer repeated",
		stateOf(f.out, "001-first-intent") == "ready" && stateOf(f.out, "002-second-intent") == "resume", "")
}

func TestAnUnreadableReportIsNotAState(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-unreadable")
	f.runReport("init", "002-readable")
	if f.madeUnreadable(f.reportPath("001-unreadable"), "the unreadable-report cases") {
		f.runReport("state", "001-unreadable")
		f.assertRefused("state refuses a report it cannot read rather than answering resume")

		// And the listing is all-or-nothing: half a listing reads exactly like a complete one.
		f.runReport("list")
		f.record("list prints no ship's state when one report cannot be read",
			f.status != 0 && !strings.Contains(f.out, "002-readable"), "exit "+itoa(f.status)+"\n"+f.out)
	}
	f.chmod(f.reportPath("001-unreadable"), 0o644)

	// The same invariant with the unreadable report ordered second, the only order that pins the
	// buffering. Reached first, nothing is printed whether the listing is buffered or streamed.
	buffered := newRepo(t)
	buffered.runReport("check-ignore")
	buffered.runReport("init", "001-readable-first")
	buffered.runReport("init", "002-unreadable-second")
	if buffered.madeUnreadable(buffered.reportPath("002-unreadable-second"), "the buffering case") {
		buffered.runReport("list")
		buffered.record("list buffers, so a ship reached before the refusal is not printed either",
			buffered.status != 0 && !strings.Contains(buffered.out, "001-readable-first"),
			"exit "+itoa(buffered.status)+"\n"+buffered.out)

		// `carry` is where an unread report loses work silently: it prints the open items a re-qualify
		// must keep, and an unreadable one prints none, which reads exactly like a report with nothing open.
		buffered.runReport("carry", "002-unreadable-second")
		buffered.assertRefused("carry refuses a report it cannot read rather than reporting no open items")
		// The message carries this one: with the readability refusal deleted, `carry` still exits 2,
		// because todo-gate.sh cannot read the file either and the scan reader refuses in its place.
		// Only this assertion tells the two apart; the discard case pins the same guard where nothing
		// else refuses for it.
		buffered.assertReports("its state is unknown", "and it is that guard refusing, not the scan failing behind it")
		buffered.runReport("carry", "001-readable-first")
		buffered.record("and a readable sibling still carries normally", buffered.status == 0,
			"exit "+itoa(buffered.status)+"\n"+buffered.out)
	}
	buffered.chmod(buffered.reportPath("002-unreadable-second"), 0o644)
}

func TestListWalksTheTreeOnceAndNeverStreamsAPartialAnswer(t *testing.T) {
	t.Parallel()
	// Three stamped reports, so the state token reaches the fingerprint for each. One walk has to
	// answer all three, or two ships could be scored against different trees.
	f := newRepo(t)
	f.runReport("check-ignore")
	for _, ship := range []string{"001-a", "002-b", "003-c"} {
		f.runReport("init", ship)
		f.stampFullPass(ship)
	}
	// Stamped is the load-bearing word. An unstamped ship answers `resume` without reading the tree at
	// all, so the count below would be the priming call alone, and would pass with no ship ever
	// reaching the cache this pins. Asserted through `list`'s own output, the surface the case reads.
	f.runReport("list")
	f.record("fixture: three stamped ships, each one reaching the fingerprint",
		countLinesEndingWith(f.out, "\tready") == 3, "")
	log := f.newCountingFingerprintHome()
	f.runReport("list")
	walks := f.countLines(log)
	f.record("list fingerprints the tree once for every ship it lists", walks == 1,
		"fingerprint calls: "+itoa(walks)+" (wanted 1)")

	// Priming must not be fatal on its own: an unstamped ship answers without the tree at all, so a
	// tree that cannot be fingerprinted must not silence a listing that never needed it.
	unstamped := newRepo(t)
	unstamped.runReport("check-ignore")
	unstamped.runReport("init", "001-unstamped")
	unstamped.runReport("init", "002-unstamped")
	unstamped.write(unstamped.repo+"/blocker.txt", "unreadable\n")
	if unstamped.madeUnreadable(unstamped.repo+"/blocker.txt", "the priming case") {
		// "unfingerprintable" is the state under test, and nothing else here establishes it: both ships
		// are unstamped, so they answer `resume` without the tree either way and the case passes on a
		// perfectly readable tree. `gate` is the shortest path that has to fingerprint, so its 2 is the
		// failure to do so, where a readable tree gives 1, the freshness block.
		unstamped.runReport("gate", "001-unstamped")
		unstamped.record("fixture: a tree that cannot be fingerprinted", unstamped.status == 2,
			"gate exited "+itoa(unstamped.status)+", wanted 2\n"+unstamped.out)
		unstamped.runReport("list")
		unstamped.record("an unfingerprintable tree does not silence a listing of ships that never needed it",
			unstamped.status == 0 && containsLine(unstamped.out, "001-unstamped\tresume") &&
				containsLine(unstamped.out, "002-unstamped\tresume"),
			"exit "+itoa(unstamped.status)+"\n"+unstamped.out)
	}
	unstamped.chmod(unstamped.repo+"/blocker.txt", 0o644)
}

func TestThePreScopingPathIsReportedNeverPassedOverInSilence(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.repo + "/.idsd")
	f.write(f.repo+"/.idsd/ship-report.md", "---\nintent: 002-live\n---\n\n# Decide\n\n- [ ] a live decision\n")
	// The pre-rename directory is the second historical path, and the note must name it too: a repo
	// mid-ship across the rename has its report there, which the `ship-report.md` entry does not cover.
	f.mkdirAll(f.repo + "/.idsd/ship-reports")
	f.write(f.repo+"/.idsd/ship-reports/003-live-ship-report.md", "---\nintent: 003-live\n---\n")
	f.runReport("state")
	f.record("state names the pre-scoping report rather than answering a bare no-report", f.out != "no-report", "")
	f.runReport("list")
	f.assertReports("ship-report.md", "list names the pre-scoping report it cannot see")
	f.assertReports("ship-reports", "and names the pre-rename directory too")
	f.runReport("gate", "002-live")
	f.assertReports("ship-report.md", "and so does a refusal for a named report that is not there")
}

func TestAScanThatDidNotRunIsNeverReadAsNothingOpen(t *testing.T) {
	t.Parallel()
	// `state`, `carry` and `close` share one reader, and the point of sharing it is that they cannot
	// drift apart, so all three are asserted here against the same broken gate. Broken means an exit
	// above 1, which read as "nothing open" would let a report still holding unrouted `- [ ]` pass the
	// merge gate.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-scan-fails")
	f.stampFullPass("001-scan-fails")
	// The positive control, while the scan still works: this fixture reaches the open-item scan and
	// answers `ready`. Without it, every refusal below could belong to an earlier guard and pin nothing.
	f.runReport("state", "001-scan-fails")
	f.record("the fixture reaches the open-item scan while the scan still works", f.out == "ready", "said '"+f.out+"'")

	f.write(f.todoGatePath(), "#!/bin/sh\nexit 3\n")
	f.chmod(f.todoGatePath(), 0o755)
	for _, scanReader := range []string{"state", "carry", "close"} {
		f.runReport(scanReader, "001-scan-fails")
		f.assertRefused(scanReader + " refuses when the open-item scan exits 3")
		f.assertReports("todo-gate.sh exited 3", "and "+scanReader+" names the exit rather than reporting nothing open")
	}
	// close is the destructive one: a scan it could not run must leave the report where it is.
	f.record("and close retired nothing on a scan it could not run", f.isFile(f.reportPath("001-scan-fails")), "")
}

func TestStateNeverAnswersATokenItCannotStandBehind(t *testing.T) {
	t.Parallel()
	// The worst-token failure: with two ships open and no intent named, an unguarded `state` prints
	// `no-report` and exits 0, because the resolution refuses before any report path is set and the arm
	// falls into the absence branch. `idsd-ship continue` routes that to "start a fresh ship", over two
	// live ones.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-live")
	f.runReport("init", "002-live")
	f.runReport("state")
	f.assertRefused("state refuses rather than answering no-report with two ships open")
	f.record("and no-report appears nowhere in what it printed", !strings.Contains(f.out, "no-report"), "")

	// `state`'s stdout is parsed as exactly one token, so every note it emits must go to stderr.
	noted := newRepo(t)
	noted.mkdirAll(noted.repo + "/.idsd")
	noted.write(noted.repo+"/.idsd/ship-report.md", "---\nintent: 002-old\n---\n\n# Decide\n")
	stdout := noted.runReportStdout("state")
	noted.record("state's stdout is one token even while it notes the pre-scoping report on stderr",
		stdout == "no-report", "stdout was: "+stdout)
}
