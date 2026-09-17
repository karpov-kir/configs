package ecoreport_test

// Ignored has to mean ignored for everyone. A machine-local exclude answers the plain question while
// ignoring nothing on anybody else's clone, and the report carries a pass's security findings, so it
// is exactly the file that must not reach a commit.
//
// Every case here is COMMITTED mode, and that is the change: an external idsd no longer lives in the
// tree, so there is nothing there to ignore and `init` asks no ignore question at all. The rules below
// still govern the one mode where `.idsd/` is tracked and each ship's scratch has to be kept out of it.
// External mode's replacement property — the scratch is somewhere `git add -A` cannot reach — is
// pinned in scratch_location_test.go.

import (
	"strconv"
	"strings"
	"testing"
)

// A committed fixture with NO .gitignore entry — the state every case here is about. The shared
// newCommittedRepo writes that entry, which is the very thing these cases must find missing.
func newCommittedRepoUnignored(t *testing.T) *fixture {
	t.Helper()
	f := newRepo(t)
	f.newDurableCharter()
	f.track(".idsd/charter.md")
	f.assertFixtureIsCommitted()
	return f
}

func TestAGlobalExcludeDoesNotCountAsIgnoringTheReport(t *testing.T) {
	t.Parallel()
	f := newCommittedRepoUnignored(t)
	// An absolute source is what `check-ignore -v` names for a `core.excludesFile` outside the
	// repository, and an absolute path is what the tool keys on. The arrangement is stated rather than
	// probed for: that git really names the excludesFile that way is git's own, held in
	// `repo/exec_test.go`.
	f.ignoreShipFiles(f.base+"/global-exclude", "001-global-only")
	f.runReport("init", "001-global-only")
	f.assertRefused("init refuses when only a global core.excludesFile ignores the reports directory")
	f.assertReports("core.excludesFile", "and names the global exclude as what does not count")
	f.assertNoReportWritten("and wrote no report a clone would commit")
}

func TestIgnoredMeansIgnoredForEveryoneNotJustThisMachine(t *testing.T) {
	t.Parallel()
	// The common global setup: `git config --global core.excludesFile ~/.gitignore`. A `*/.gitignore`
	// arm matches it by name, so the guard against a machine-local exclude would pass for the very
	// configuration most people have.
	f := newCommittedRepoUnignored(t)
	f.ignoreShipFiles(f.base+"/home/.gitignore", "001-global-gitignore")
	f.runReport("init", "001-global-gitignore")
	f.assertRefused("init refuses a global core.excludesFile even when it is named .gitignore")
	f.assertNoReportWritten("and wrote no report a clone would commit")

	// The remedy `init` names has to agree with `init`, or the human is sent between the two forever.
	committed := newCommittedRepoUnignored(t)
	committed.ignoreShipFiles(committed.base+"/home2/.gitignore", "001-global-gitignore")
	committed.runReport("check-ignore")
	committed.record("check-ignore warns where init refuses, rather than reporting ok",
		committed.status == 1 && strings.Contains(committed.out, "NOT gitignored"),
		"exit "+strconv.Itoa(committed.status)+"; said: "+committed.out)

	// A linked worktree of a committed repo reads its ignore rules from a tracked .gitignore, which
	// travels. Init has to work there, and the report has to land in that worktree's own tree — a
	// committed .idsd/ is per-worktree by definition, since git checks it out into each one.
	t.Run("init works in a linked worktree of a committed repo", func(t *testing.T) {
		worktree := newCommittedRepoUnignored(t)
		worktree.write(worktree.repo+"/.gitignore", ignoreBlock())
		worktree.track(".gitignore")
		worktreeDir := worktree.newLinkedWorktree("wt")
		worktree.runReportIn(worktreeDir, "init", "001-in-a-worktree")
		worktree.record("init works in a linked worktree, writing into that worktree's own tree",
			worktree.status == 0 && worktree.isFile(worktreeDir+"/.idsd/intents/001-in-a-worktree/for-agents/qualify-report.md"),
			worktree.evidence())
	})
}

func TestAGitignoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine(t *testing.T) {
	t.Parallel()
	// `promote` appends to a .gitignore the human also keeps their own rules in. Two things can go
	// wrong in that append and both are silent: the entry accumulating one copy per run, and the entry
	// fusing onto a last line with no newline — after which neither the human's rule nor the entry
	// matches anything, while promote reports success and stages the report.

	f := newShip(t, "001-appending")
	f.newIntentFile("001-appending")
	gitignore := f.repo + "/.gitignore"
	// A rule of the human's own, deliberately left unterminated.
	f.write(gitignore, "# theirs\n*.scratch")

	f.runReport("promote")
	f.record("promote appends the entry as its own line, not onto an unterminated one",
		f.status == 0 && containsLine(f.read(gitignore), reportEntry()) &&
			containsLine(f.read(gitignore), "*.scratch"),
		"exit "+strconv.Itoa(f.status)+"; .gitignore now reads:\n"+f.read(gitignore))
	// Whether a fused line still matches is git's own question, and `repo/exec_test.go` holds git to it.
	// What this case can see is the file: two lines, each whole, which is the only state in which either
	// rule can match at all.
	f.record("and the human's own rule is still a line of its own, unfused",
		countLinesEqual(f.read(gitignore), "*.scratch") == 1 &&
			countLinesEqual(f.read(gitignore), reportEntry()) == 1,
		f.read(gitignore))

	// The dedupe, on a fixture where the append actually runs twice. Calling promote again would not
	// exercise it: once the repo is committed promote returns early and never reaches the append, so a
	// second call proves nothing about duplicates. A second ship in a fresh repo whose .gitignore
	// already carries the entry is the state that does.
	second := newShip(t, "001-again")
	second.newIntentFile("001-again")
	secondIgnore := second.repo + "/.gitignore"
	second.write(secondIgnore, "# theirs\n"+ignoreBlock())
	second.runReport("promote")
	second.record("an entry already present is not added a second time",
		second.status == 0 && countLinesEqual(second.read(secondIgnore), reportEntry()) == 1,
		strconv.Itoa(countLinesEqual(second.read(secondIgnore), reportEntry()))+" copies:\n"+second.read(secondIgnore))
}

func TestAMachineLocalExcludeDoesNotCountAsIgnoringTheReport(t *testing.T) {
	t.Parallel()
	// `.git/info/exclude` is one machine's file: it never leaves this clone, so a report ignored only
	// there is staged by the next `git add -A` on anybody else's. ignoredSourceTravels holds why the plain
	// question answers otherwise.
	f := newCommittedRepoUnignored(t)
	f.appendTo(f.repo+"/.git/info/exclude", ignoreBlock())
	f.ignoreShipFiles(".git/info/exclude", "001-locally-excluded")
	f.runReport("init", "001-locally-excluded")
	f.assertRefused("init refuses when only .git/info/exclude ignores the reports directory")
	f.assertNoReportWritten("and wrote no report a clone would commit")
	// The remedy has to work, or the human is sent between the two commands forever.
	f.write(f.repo+"/.gitignore", ignoreBlock())
	f.runReport("init", "001-locally-excluded")
	f.record("and accepts it once a tracked .gitignore is what ignores it",
		f.status == 0 && f.isFile(f.reportPath("001-locally-excluded")), f.evidence())
}
