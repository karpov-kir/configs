package ecoreport_test

// What `init` refuses. The report's filename is built from the intent, and an intent can be seeded
// from a fetched ticket, so the value reaches a path; every write it makes goes through two
// directories and a staged file, so a link at any of the three would put the write outside the repo.

import (
	"os"
	"strings"
	"testing"
)

func TestInitRefusesRatherThanWritingThroughALink(t *testing.T) {
	t.Parallel()
	link := newRepo(t)
	link.runReport("check-ignore")
	link.mkdirAll(link.scratch() + "/qualify-reports")
	link.mkdirAll(link.base + "/elsewhere")
	link.symlink(link.base+"/elsewhere/stolen.md", link.reportPath(""))
	link.runReport("init", "review: symlinked report")
	link.assertRefused("init refuses a symlinked report")
	link.assertReports("is a symlink", "and says the report was not initialized")
	link.record("nothing was written through the link", !link.exists(link.base+"/elsewhere/stolen.md"), "")

	idsd := newRepo(t)
	idsd.runReport("check-ignore")
	idsd.mkdirAll(idsd.base + "/outside")
	idsd.symlink(idsd.base+"/outside", idsd.scratch()+"")
	idsd.runReport("init", "review: symlinked idsd dir")
	idsd.assertRefused("init refuses a symlinked .idsd directory")
	idsd.record("nothing was written outside the repo through .idsd", !idsd.exists(idsd.base+"/outside/qualify-reports"), "")

	// The second directory every write goes through, so it needs a case of its own: the symlink test
	// on the report file cannot see a link one level up.
	reports := newRepo(t)
	reports.runReport("check-ignore")
	reports.mkdirAll(reports.scratch() + "")
	reports.mkdirAll(reports.base + "/outside-reports")
	reports.symlink(reports.base+"/outside-reports", reports.scratch()+"/qualify-reports")
	reports.runReport("init", "review: symlinked reports dir")
	reports.assertRefused("init refuses a symlinked qualify-reports directory")
	reports.record("nothing was written outside the repo through qualify-reports",
		!reports.exists(reports.base+"/outside-reports/review-qualify-report.md"), "")
	// Asserted on which refusal it is, because the exit alone cannot tell them apart: git refuses any
	// pathspec beyond a symbolic link, so without this directory's own link test the ignore check
	// refuses instead — for want of an ignore rule, naming `check-ignore` as the remedy.
	reports.assertReports("is a symlink", "and names the link rather than blaming the ignore rules")
	// And that remedy is already satisfied, so the other refusal is a loop with no exit: check-ignore
	// reports ok, changes nothing about the link, and init refuses again on the next run.
	reports.runReport("check-ignore")
	reports.record("the step that other refusal would name reports ok and leaves the link standing",
		reports.status == 0 && strings.HasPrefix(reports.out, "ok:") && reports.isSymlink(reports.scratch()+"/qualify-reports"),
		reports.evidence())
}

func TestAnIntentValueCannotNameAFileOutsideQualifyReports(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")
	// Two forms, because two separate rules refuse them: the leading dot stops `../../escaped` on its
	// own, so only a value that starts with a legal character reaches the charset.
	for _, escaping := range []string{"../../escaped", "ok/../../escaped"} {
		f.runReport("init", escaping)
		f.assertRefused("init refuses the intent '" + escaping + "', whose path could escape the directory")
	}
	// The escape lands at <repo>/escaped-qualify-report.md, two levels up from qualify-reports/, not at
	// the scratch base. Asserting only on the base, or on the repo's parent, lets a widened charset through.
	escaped := f.exists(f.repo+"/escaped-qualify-report.md") ||
		f.exists(f.scratch()+"/escaped-qualify-report.md") ||
		f.exists(f.base+"/escaped-qualify-report.md")
	f.record("no report was written outside qualify-reports/", !escaped, strings.Join(f.find(f.base), "\n"))

	// What the refusals prevent is invisibility rather than the write itself: a listing never matches a
	// leading dot, so a dot-named report would stand open while `list` says "no reports". So `list` is
	// asserted here as well as the three exits.
	dotted := newRepo(t)
	dotted.runReport("check-ignore")
	for _, dotIntent := range []string{"..", ".", ".hidden"} {
		dotted.runReport("init", dotIntent)
		dotted.assertRefused("init refuses the intent '" + dotIntent + "', which no listing could ever see")
	}
	dotted.runReport("list")
	unseen := containsLine(dotted.out, "no reports") && len(dotted.entries(dotted.scratch()+"/qualify-reports")) == 0
	dotted.record("and no dot-named report was left on disk for list to miss", unseen,
		strings.Join(dotted.entries(dotted.scratch()+"/qualify-reports"), "\n"))
}

func TestAnExistingReportIsNotSilentlyReplaced(t *testing.T) {
	t.Parallel()
	f := newShip(t, "review: first")
	f.runReport("init", "review: second")
	f.assertRefused("init refuses over an existing report without --force")
	f.record("and the first report is left untouched", strings.Contains(f.read(f.reportPath("")), "review: first"), "")

	f.appendTo(f.reportPath(""), "- [ ] an open item nobody has routed\n")
	f.runReport("init", "review: third", "--force")
	// The listing comes from todo-gate.sh, and it is now the only record of what --force discarded: no
	// copy is kept beside the report. A --force that replaces a report while printing none of its open
	// items is how routed work silently disappears.
	f.assertReports("an open item nobody has routed", "--force lists the open items it discards")
	f.record("and the new report is in place", strings.Contains(f.read(f.reportPath("")), "review: third"), "")
}

func TestTheFrontmatterCannotBeForgedThroughTheIntentValue(t *testing.T) {
	t.Parallel()
	// The intent value can come from a fetched ticket. A newline in it would otherwise write a second
	// frontmatter line, and `reviewed-tree:` is exactly what a forged one would claim.
	f := newShip(t, "review: forged\nreviewed-tree: 0000000000000000000000000000000000000000")
	report := f.read(f.reportPath(""))
	stamps := 0
	for _, line := range strings.Split(report, "\n") {
		if strings.HasPrefix(line, "reviewed-tree:") {
			stamps++
		}
	}
	f.record("a newline in the intent writes no second reviewed-tree line",
		stamps == 1 && containsLine(report, "reviewed-tree: <hash>"), report)
}

func TestTheFilenameAndTheFrontmatterNameTheSameShip(t *testing.T) {
	t.Parallel()
	// When they differ, one intent gets two reports so the ambiguity refusal never fires, `discard`
	// deletes another ship's in-flight intent, and `state` answers `done` for an open one.
	f := newShip(t, "  002-spaced")
	filed := f.isFile(f.reportPath("002-spaced")) && !f.isFile(f.reportPath("review")) &&
		containsLine(f.read(f.reportPath("002-spaced")), "intent: 002-spaced")
	f.record("a whitespace-led intent is filed and recorded under the same slug", filed,
		strings.Join(f.entries(f.scratch()+"/qualify-reports"), "\n"))
	f.runReport("state", "002-spaced")
	f.record("and it is addressable by the slug it recorded", f.out == "resume", "said '"+f.out+"'")
	// And by the value `init` was given, whitespace and all — the name reaches a subcommand from the
	// same place the intent reached init. Truncated before the whitespace is trimmed it names nothing,
	// falls to the `review` stem, and `state` answers `no-report` for a ship that is open: the token
	// `idsd-ship continue` routes to "start ship <intent>", rebuilding work already in flight.
	f.runReport("state", "  002-spaced")
	f.record("and by the value init was given, leading whitespace and all", f.out == "resume", "said '"+f.out+"'")

	// Asserted on disk as well as on the exit: the harm is the scaffolded report, whose blank `intent:`
	// every reader treats as a standalone review.
	blank := newShip(t, " ")
	blank.assertRefused("init refuses a whitespace-only intent")
	blank.assertNoReportWritten("and wrote no report for it")
}

func TestInitStagedWriteIsNotAWayOutOfTheRepo(t *testing.T) {
	t.Parallel()
	// The staged copy writes to `<report>.new`, one more path the symlink guard chain has to cover. A
	// link planted there is committable, so it arrives through someone else's branch, and the copy
	// would overwrite its target while `init` reported success.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.mkdirAll(f.scratch() + "/qualify-reports")
	f.mkdirAll(f.base + "/victim")
	f.write(f.base+"/victim/keep.md", "PRECIOUS\n")
	f.symlink(f.base+"/victim/keep.md", f.reportPath("review")+".new")
	// The planted link is the fixture here, and unlike every other symlink case this one expects `init`
	// to succeed. So a failed link leaves PRECIOUS intact and the report written, and both cases below
	// pass with nothing ever planted.
	f.record("fixture: a link planted at the staged path", f.isSymlink(f.reportPath("review")+".new"), "")
	f.runReport("init", "review: staged write")
	f.record("init writes no template through a link planted at the staged path",
		strings.HasPrefix(f.read(f.base+"/victim/keep.md"), "PRECIOUS"), "")
	f.record("and still initialized the report itself",
		f.isFile(f.reportPath("")) && strings.Contains(f.read(f.reportPath("")), "review: staged write"), "")

	// That clearing is `rm -f`, which takes a leftover file and refuses a directory — and `init` reads
	// the refusal as "something is in the way". os.Remove is not `rm -f`: it takes an empty directory
	// happily, so a directory there would be removed and the report written over ground `init` never
	// looked at. Empty by construction, since a directory with anything in it fails the removal too
	// and the case would pass on a mutation that changed the answer.
	occupied := newRepo(t)
	occupied.runReport("check-ignore")
	occupied.mkdirAll(occupied.reportPath("") + ".new")
	occupied.runReport("init", "review: a directory in the staged path")
	occupied.assertRefused("init refuses when a directory sits where it stages its copy")
	occupied.assertReports("could not clear", "and names the path it could not clear")
	occupied.record("and wrote no report over it",
		!occupied.isFile(occupied.reportPath("")) && occupied.exists(occupied.reportPath("")+".new"),
		joinLines(occupied.entries(occupied.scratch()+"/qualify-reports")))
}

func TestInitWillNotWriteAReportIntoItsOwnFingerprint(t *testing.T) {
	t.Parallel()
	// Skipping `check-ignore`, the documented first step, is silent. The report lands inside the tree it
	// fingerprints, so `state` answers `re-qualify` straight after a complete four-stage stamp and
	// `gate` blocks on freshness with nothing that can clear it.
	//
	// Committed mode, because that is now the only mode where the report CAN land inside the tree.
	// Throwaway scratch sits outside it by construction, which is the stronger guarantee — there is no
	// ignore rule left to skip. The throwaway side of this is TestScratchSitsWhereGitAddAllCannotReachIt.
	f := newCommittedRepoUnignored(t)
	f.runReport("init", "001-unignored")
	f.assertRefused("init refuses when git does not ignore the reports directory")
	f.assertReports("report.sh check-ignore", "and names the step that was skipped")
	f.assertNoReportWritten("and wrote no report that could never gate clean")

	// Committed mode reaches the same precondition through a `.gitignore` entry rather than a local
	// exclusion, so the assertion has to accept it, or it blocks every committed idsd repo.
	committed := newCommittedRepo(t)
	committed.runReport("init", "001-committed-ok")
	committed.record("and accepts a committed repo, where .gitignore is what ignores it",
		committed.status == 0 && committed.isFile(committed.reportPath("001-committed-ok")), "")
}

func TestAnIntentValueCarriesNoControlByteIntoTheReport(t *testing.T) {
	t.Parallel()
	f := newShip(t, "review: seeded\x1b[2KFORGED")

	written, err := os.ReadFile(f.reportPath(""))
	if err != nil {
		t.Fatalf("no report to read: %v", err)
	}
	// The control: without it, an init that refused outright would satisfy the assertion below.
	f.record("the intent still reaches the report", strings.Contains(string(written), "seeded"), string(written))
	f.record("no control byte reached the frontmatter", !strings.ContainsRune(string(written), 0x1b), string(written))
	f.record("nor this tool's own output", !strings.ContainsRune(f.out, 0x1b), f.out)

	// The half that was already there: CR/LF collapse to a space rather than opening a second line.
	g := newShip(t, "review: a\nreviewed-tree: forged")
	body, err := os.ReadFile(g.reportPath(""))
	if err != nil {
		t.Fatalf("no report to read: %v", err)
	}
	g.record("a newline still cannot forge a frontmatter line",
		!strings.Contains(string(body), "\nreviewed-tree: forged"), string(body))
}
