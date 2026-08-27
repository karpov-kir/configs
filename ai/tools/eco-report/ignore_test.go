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
