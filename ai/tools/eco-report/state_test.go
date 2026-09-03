package ecoreport_test

// `state` and `list` are what `idsd-ship continue` routes on, so a token they cannot stand behind
// routes a live ship past a gate, and a listing that stops halfway reads exactly like a complete one.

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestTwoIntentsShipSideBySide(t *testing.T) {
	t.Parallel()
	// The whole point of the per-intent path: a second intent's init is not a collision, so neither
	// ship has to be finished before the other starts. And `check-ignore` first, as a real pass does,
	// or the report sits inside its own fingerprint and every state below reads `re-qualify`.
	f := newShip(t, "001-first-intent")
	f.runReport("init", "002-second-intent")
	f.record("a second intent gets its own report rather than a refusal",
		f.status == 0 && f.isFile(f.reportPath("001-first-intent")) && f.isFile(f.reportPath("002-second-intent")),
		f.evidence())

	f.runReport("gate")
	f.assertRefused("a subcommand refuses to guess which of two reports it means")
	f.assertReports("001-first-intent", "and lists both by name")
	f.assertReports("002-second-intent", "and lists the second too")

	f.runReport("invalidate", "002-second-intent")
	f.record("the named report is the only one acted on",
		f.status == 0 && containsLine(f.read(f.reportPath("002-second-intent")), "reviewed-tree: pending") &&
			containsLine(f.read(f.reportPath("001-first-intent")), "reviewed-tree: <hash>"),
		f.evidence())

	f.runReport("stage-returned", "code-review", "001-first-intent")
	f.runReport("invalidate", "002-second-intent")
	f.runReport("no-items", "code-review", "001-first-intent")
	f.record("one intent's invalidate leaves the other's stage markers standing", f.status == 0,
		f.evidence())

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
	f := newShip(t, "001-unreadable")
	f.runReport("init", "002-readable")
	if f.madeUnreadable(f.reportPath("001-unreadable"), "the unreadable-report cases") {
		f.runReport("state", "001-unreadable")
		f.assertRefused("state refuses a report it cannot read rather than answering resume")

		// And the listing is all-or-nothing: half a listing reads exactly like a complete one.
		f.runReport("list")
		f.record("list prints no ship's state when one report cannot be read",
			f.status != 0 && !strings.Contains(f.out, "002-readable"), f.evidence())
	}
	f.chmod(f.reportPath("001-unreadable"), 0o644)

	// The same invariant with the unreadable report ordered second, the only order that pins the
	// buffering. Reached first, nothing is printed whether the listing is buffered or streamed.
	buffered := newShip(t, "001-readable-first")
	buffered.runReport("init", "002-unreadable-second")
	if buffered.madeUnreadable(buffered.reportPath("002-unreadable-second"), "the buffering case") {
		buffered.runReport("list")
		buffered.record("list buffers, so a ship reached before the refusal is not printed either",
			buffered.status != 0 && !strings.Contains(buffered.out, "001-readable-first"),
			buffered.evidence())

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
			buffered.evidence())
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
	calls := f.countFingerprints()
	f.runReport("list")
	walks := *calls
	f.record("list fingerprints the tree once for every ship it lists", walks == 1,
		"fingerprint calls: "+strconv.Itoa(walks)+" (wanted 1)")

	// Priming must not be fatal on its own: an unstamped ship answers without the tree at all, so a
	// tree that cannot be fingerprinted must not silence a listing that never needed it.
	unstamped := newShip(t, "001-unstamped")
	unstamped.runReport("init", "002-unstamped")
	unstamped.write(unstamped.repo+"/blocker.txt", "unreadable\n")
	if unstamped.madeUnreadable(unstamped.repo+"/blocker.txt", "the priming case") {
		// "unfingerprintable" is the state under test, and nothing else here establishes it: both ships
		// are unstamped, so they answer `resume` without the tree either way and the case passes on a
		// perfectly readable tree. `gate` is the shortest path that has to fingerprint, so its 2 is the
		// failure to do so, where a readable tree gives 1, the freshness block.
		unstamped.runReport("gate", "001-unstamped")
		unstamped.record("fixture: a tree that cannot be fingerprinted", unstamped.status == 2,
			"gate exited "+strconv.Itoa(unstamped.status)+", wanted 2\n"+unstamped.out)
		unstamped.runReport("list")
		unstamped.record("an unfingerprintable tree does not silence a listing of ships that never needed it",
			unstamped.status == 0 && containsLine(unstamped.out, "001-unstamped\tresume") &&
				containsLine(unstamped.out, "002-unstamped\tresume"),
			unstamped.evidence())
	}
	unstamped.chmod(unstamped.repo+"/blocker.txt", 0o644)
}

func TestAScanThatDidNotRunIsNeverReadAsNothingOpen(t *testing.T) {
	t.Parallel()
	// `state`, `carry` and `close` share one reader, and the point of sharing it is that they cannot
	// drift apart, so all three are asserted here against the same broken gate. Broken means an exit
	// above 1, which read as "nothing open" would let a report still holding unrouted `- [ ]` pass the
	// merge gate.
	f := newShip(t, "001-scan-fails")
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
	f := newShip(t, "001-live")
	f.runReport("init", "002-live")
	f.runReport("state")
	f.assertRefused("state refuses rather than answering no-report with two ships open")
	f.record("and no-report appears nowhere in what it printed", !strings.Contains(f.out, "no-report"), "")

	// The stream-separation half that used to sit here is gone: the note it asserted on was legacyNote,
	// deleted in this change, so the assertion reduced to `stdout == "no-report"`, which an empty repo
	// answers regardless. That property is covered by TestAnOverrideMovesTheRootAndSaysSo, which asserts
	// `state` prints one bare token while the override note goes to stderr.
}

func TestStateAnswersEveryTokenItRoutesOn(t *testing.T) {
	t.Parallel()
	// `idsd-ship continue` routes on this one word, so the deliverable is the whole mapping rather
	// than the tokens a scenario happened to reach: every token, and the state that earns it. Answered
	// for the wrong state, one of these sends `continue` past a live gate or starts a fresh ship over
	// work already merged.
	none := newRepo(t)
	none.runReport("state")
	none.record("no-report where no ship has started", none.out == "no-report", "said '"+none.out+"'")

	f := newShip(t, "001-token")
	f.runReport("state", "001-token")
	f.record("resume for a report that has never been stamped", f.out == "resume", "said '"+f.out+"'")

	f.stampFullPass("001-token")
	f.runReport("state", "001-token")
	f.record("ready for a full pass on a fresh tree with nothing open", f.out == "ready", "said '"+f.out+"'")

	// The report is git-ignored, so editing it moves nothing the fingerprint reads: the freshness arm
	// stays clear and the open item is the only thing that can change the answer.
	f.appendTo(f.reportPath("001-token"), "- [ ] an item nobody routed\n")
	f.runReport("state", "001-token")
	f.record("decide for a full pass still holding an open item", f.out == "decide", "said '"+f.out+"'")
	f.dropLines(f.reportPath("001-token"), "- [ ] an item")

	// The fingerprint covers untracked content too, so a new file is enough to move the tree. Ordered
	// after `decide` deliberately: with the tree moved, freshness answers first and would mask it.
	f.write(f.repo+"/moved.txt", "the tree has moved since the stamp\n")
	f.runReport("state", "001-token")
	f.record("re-qualify for a stamped pass whose tree moved since", f.out == "re-qualify", "said '"+f.out+"'")

	trimmed := newShip(t, "001-trimmed-token")
	trimmed.runReport("invalidate", "001-trimmed-token")
	for _, stage := range []string{"code-review", "tighten", "refactor"} {
		trimmed.runReport("stage-returned", stage, "001-trimmed-token")
		trimmed.runReport("no-items", stage, "001-trimmed-token")
	}
	trimmed.runReport("stamp", "code-review,security-review:skipped(turnaround),tighten,refactor", "001-trimmed-token")
	trimmed.runReport("state", "001-trimmed-token")
	trimmed.record("finalize for a fresh pass with a stage trimmed for turnaround",
		trimmed.out == "finalize", "said '"+trimmed.out+"'")

	// An intent file that has moved to archive/ is the only record a landed ship leaves once its
	// report is closed. Read through `list` as well as `state`: `list` is the surface that reaches the
	// token's own archive arm, since `state` answers from the report's filename a step earlier, and the
	// two are different ships whenever the frontmatter names a slug the filename does not.
	archived := newShip(t, "001-landed")
	archived.mkdirAll(archived.scratch() + "/archive")
	archived.write(archived.scratch()+"/archive/001-landed.md", "# built, and merged\n")
	archived.runReport("list")
	archived.record("done for a ship whose intent file has reached archive/",
		stateOf(archived.out, "001-landed") == "done", archived.out)
	archived.runReport("state", "001-landed")
	archived.record("and state answers done for it too", archived.out == "done", "said '"+archived.out+"'")
}

// A stem comes off the directory listing without passing reportNameFor, and a filename can hold a
// newline. `ev<LF>fakeship<TAB>ready<LF>il-qualify-report.md` put a whole forged `fakeship  ready` row
// into the listing `idsd-ship continue` routes on — a ship that does not exist, reported merge-ready.
func TestAFilenameCannotForgeAListingRow(t *testing.T) {
	t.Parallel()
	f := newShip(t, "realship")
	forged := f.scratch() + "/qualify-reports/ev\nfakeship\tready\nil" + "-qualify-report.md"
	if err := os.WriteFile(forged, []byte("---\nintent: x\n---\n"), 0o644); err != nil {
		t.Skipf("this filesystem refused a newline in a filename, so this case cannot run here: %v", err)
	}

	listing := f.runReportStdout("list")
	// The control: the real ship is still listed, so the assertions below are about the forged row and
	// not about a listing that failed outright.
	f.record("the real ship is still listed", strings.Contains(listing, "realship\t"), listing)
	f.record("no forged row reached the listing", !strings.Contains(listing, "fakeship\tready"), listing)
	f.record("and the row count is one", strings.Count(strings.TrimRight(listing, "\n"), "\n") == 0, listing)

	// Skipping in silence would be the other failure: a ship the listing does not mention is one
	// nothing resumes.
	f.runReport("list")
	f.record("the skipped file is said out loud", strings.Contains(f.out, "were NOT listed"), f.out)
}
