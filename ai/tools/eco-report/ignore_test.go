package ecoreport_test

// Ignored has to mean ignored for everyone. A machine-local exclude answers the plain question while
// ignoring nothing on anybody else's clone, and the report carries a pass's security findings, so it
// is exactly the file that must not reach a commit.

import (
	"strings"
	"testing"
)

func TestAGlobalExcludeDoesNotCountAsIgnoringTheReport(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	globalExclude := f.base + "/global-exclude"
	f.write(globalExclude, ".idsd/\n")
	f.mustGit("config", "core.excludesFile", globalExclude)
	if _, status := f.git("check-ignore", "-q", ".idsd/qualify-reports/"); status == 0 {
		f.runReport("init", "001-global-only")
		f.assertRefused("init refuses when only a global core.excludesFile ignores the reports directory")
		f.assertReports("core.excludesFile", "and names the global exclude as what does not count")
		f.assertNoReportWritten("and wrote no report a clone would commit")
	} else {
		f.record("fixture did not establish a global-exclude-only state", false, "")
	}
}

func TestIgnoredMeansIgnoredForEveryoneNotJustThisMachine(t *testing.T) {
	t.Parallel()
	// The common global setup: `git config --global core.excludesFile ~/.gitignore`. A `*/.gitignore`
	// arm matches it by name, so the guard against a machine-local exclude would pass for the very
	// configuration most people have.
	f := newRepo(t)
	homeGitignore := f.base + "/home/.gitignore"
	f.mkdirAll(f.base + "/home")
	f.write(homeGitignore, ".idsd/\n")
	f.mustGit("config", "core.excludesFile", homeGitignore)
	if _, status := f.git("check-ignore", "-q", ".idsd/qualify-reports/"); status == 0 {
		f.runReport("init", "001-global-gitignore")
		f.assertRefused("init refuses a global core.excludesFile even when it is named .gitignore")
		f.assertNoReportWritten("and wrote no report a clone would commit")
	} else {
		f.record("fixture did not establish a global-excludesFile state", false, "")
	}

	// The remedy `init` names has to agree with `init`, or the human is sent between the two forever.
	// This needs committed mode: throwaway `check-ignore` writes an exclusion and never asks the
	// question, so a throwaway fixture passes whatever the committed branch does.
	committed := newRepo(t)
	committed.newDurableCharter()
	committed.mustGit("add", ".idsd/charter.md")
	committed.commit("committed idsd, no gitignore")
	secondHomeGitignore := committed.base + "/home2/.gitignore"
	committed.mkdirAll(committed.base + "/home2")
	committed.write(secondHomeGitignore, ".idsd/qualify-reports/\n")
	committed.mustGit("config", "core.excludesFile", secondHomeGitignore)
	_, ignoredGlobally := committed.git("check-ignore", "-q", ".idsd/qualify-reports/")
	if committed.runReportStdout("repo-mode") == "committed" && ignoredGlobally == 0 {
		committed.runReport("check-ignore")
		committed.record("check-ignore warns where init refuses, rather than reporting ok",
			committed.status == 1 && strings.Contains(committed.out, "NOT gitignored"),
			"exit "+itoa(committed.status)+"; said: "+committed.out)
	} else {
		committed.record("fixture did not establish a committed repo ignored only globally", false, "")
	}

	// A linked worktree's info/exclude is absolute, so rejecting absolutes alone would break every worktree.
	worktree := newRepo(t)
	worktree.runReport("check-ignore")
	worktreeDir := worktree.base + "/wt"
	worktree.mustGit("worktree", "add", "-q", worktreeDir, "-b", "wt-branch")
	if worktree.exists(worktreeDir) {
		worktree.runReportIn(worktreeDir, "init", "001-in-a-worktree")
		worktree.record("init works in a linked worktree, whose info/exclude is an absolute path",
			worktree.status == 0 && worktree.isFile(worktreeDir+"/.idsd/qualify-reports/001-in-a-worktree-qualify-report.md"),
			"exit "+itoa(worktree.status)+"\n"+worktree.out)
	} else {
		worktree.t.Logf("skip  git worktree add unavailable — the absolute-info/exclude case cannot run")
	}
}

func TestAnIgnoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine(t *testing.T) {
	t.Parallel()
	// `check-ignore` is run at the start of every pass, so the exclusion is appended over and over to
	// a file the human also keeps their own rules in. Two things can go wrong in that append and both
	// are silent: the entry accumulating one copy per pass, and the entry fusing onto a last line with
	// no newline — after which neither the human's rule nor the entry matches anything.
	f := newRepo(t)
	exclude := f.repo + "/.git/info/exclude"
	// A rule of the human's own, deliberately left unterminated. git init writes an exclude file whose
	// last line is a comment with a newline, so the unterminated state has to be built here.
	f.write(exclude, "# theirs\n*.scratch")

	f.runReport("check-ignore")
	f.record("check-ignore appends the exclusion as its own line, not onto an unterminated one",
		containsLine(f.read(exclude), ".idsd/") && containsLine(f.read(exclude), "*.scratch"),
		"exclude now reads:\n"+f.read(exclude))
	// git is the authority on whether the append took effect: a fused line is still a line, and only
	// git's own answer distinguishes a rule that matches from one that reads like it should.
	_, ignored := f.git("check-ignore", "-q", ".idsd/")
	_, theirs := f.git("check-ignore", "-q", "keep.scratch")
	f.record("and git ignores both .idsd/ and the rule that was already there",
		ignored == 0 && theirs == 0, "check-ignore .idsd/ exited "+itoa(ignored)+", keep.scratch exited "+itoa(theirs))

	// Every pass runs check-ignore again. One entry per pass would grow the file without bound and
	// leave the human reading their own rules out of a wall of duplicates.
	f.runReport("check-ignore")
	f.runReport("check-ignore")
	f.record("and three runs leave exactly one '.idsd/' line",
		countLinesEqual(f.read(exclude), ".idsd/") == 1,
		itoa(countLinesEqual(f.read(exclude), ".idsd/"))+" copies:\n"+f.read(exclude))
}
