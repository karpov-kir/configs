package ecoreport_test

// `promote` writes to the human's index, and every refusal past the point it drops the local
// exclusion owes a restore — left off, the next `git add -A` stages the whole scratch dir.

import (
	"strings"
	"testing"
)

func TestCheckIgnoreHoldsBeforeQualifyReportsExists(t *testing.T) {
	t.Parallel()
	// `check-ignore` runs before the first write into `.idsd/`, and its exit 1 blocks that write. So it
	// has to answer correctly while qualify-reports/ does not exist yet, and that is where the trailing
	// slash in the ignore surface earns its keep: without it, `git check-ignore -q
	// .idsd/qualify-reports` exits 1 on a directory that is not there.
	f := newCommittedRepo(t)
	if !f.exists(f.repo+"/.idsd/qualify-reports") && f.runReportStdout("repo-mode") == "committed" {
		f.runReport("check-ignore")
		f.record("check-ignore passes in committed mode before qualify-reports/ is created",
			f.status == 0, "exit "+itoa(f.status)+"\n"+f.out)
	} else {
		f.record("fixture is not a committed repo with qualify-reports/ absent", false, "")
	}
}

func TestPromoteReportsTheModeNotTheAdd(t *testing.T) {
	t.Parallel()
	// qualify-reports/ is ignored by the entry promote itself writes, and `git add` on a directory
	// whose every file is ignored stages nothing and exits 0. With nothing else under .idsd/, reading
	// success off that add leaves repo-mode still saying throwaway, and the next check-ignore
	// re-excludes .idsd/, silently undoing the promotion.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-nothing-durable")
	f.runReport("promote")
	f.assertRefused("promote refuses when everything under .idsd/ is ignored")
	f.record("and the repo is still a throwaway, as the refusal says", f.runReportStdout("repo-mode") == "throwaway", "")
	// Read straight from the exclude file: routing this through check-ignore would pass either way,
	// since check-ignore re-adds the entry itself in throwaway mode.
	f.record("and promote put the local exclusion back, so nothing scratch can be staged", f.hasLocalExclusion(), "")

	// The same repo with one durable file: promote now has something to stage, and the mode flips.
	f.write(f.repo+"/.idsd/intents-placeholder.md", "# intent\n")
	f.runReport("promote")
	f.record("promote succeeds once .idsd/ holds something that is not ignored",
		f.status == 0 && f.runReportStdout("repo-mode") == "committed", "exit "+itoa(f.status)+"\n"+f.out)

	// Committed mode takes the other check-ignore branch entirely: the one that asks git rather than
	// writing an exclusion, and the only one that can confirm the entry instead of creating it.
	f.runReport("check-ignore")
	f.record("committed mode confirms qualify-reports/ is gitignored",
		f.status == 0 && strings.Contains(f.out, "gitignored"), "exit "+itoa(f.status)+"\n"+f.out)

	// And the warning fires when it is not: the entry is what keeps a report out of `git add -A`.
	f.write(f.repo+"/.gitignore", "")
	f.runReport("check-ignore")
	f.record("and warns when the entry is missing",
		f.status == 1 && strings.Contains(f.out, "NOT gitignored"), "exit "+itoa(f.status)+"\n"+f.out)
}

func TestNoRefusalLeavesIdsdExposedToGitAddAll(t *testing.T) {
	t.Parallel()
	// Without a restore on the `git add` refusal, the exclusion is gone and `git status` lists .idsd/.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-promoting")
	f.newIntentFile("001-promoting")
	f.write(f.repo+"/.git/index.lock", "")
	f.runReport("promote")
	f.assertRefused("promote refuses when it cannot stage")
	f.record("and put the local exclusion back, so .idsd/ stays invisible to 'git add -A'", f.hasLocalExclusion(), "")
	f.remove(f.repo + "/.git/index.lock")

	// promote needs one report as its evidence a ship happened here, which is why it does not go
	// through the report requirement every other reader opens with.
	bare := newRepo(t)
	bare.runReport("check-ignore")
	bare.runReport("promote")
	bare.assertRefused("promote refuses with no report at all")
	// Asserted on the message: without this refusal `promote` runs on and `git add .idsd` fails because
	// the directory is not there, so it still exits 2, for a reason that is not this guard.
	bare.assertReports("nothing to promote", "and names the missing report as the reason")
}

func TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex(t *testing.T) {
	t.Parallel()
	// The mode decides whether .idsd/ is durable, so every caller that acts on it owes the check.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-modes")
	if f.madeUnreadable(f.repo+"/.git/index", "the unreadable-index cases") {
		// Again the message rather than the exit: without the assertion both subcommands still exit 2,
		// because a later git call fails on the same unreadable index. Only the message tells the two
		// apart, and only the assertion stops the mode being read as "throwaway", the answer that deletes.
		f.runReport("promote")
		f.assertRefused("promote refuses when the repo mode cannot be read")
		f.assertReports("repo mode is unknown", "and names the unreadable mode as the reason")
		f.runReport("check-ignore")
		f.assertRefused("check-ignore refuses when the repo mode cannot be read")
		f.assertReports("repo mode is unknown", "and names it there too")
	}
	f.chmod(f.repo+"/.git/index", 0o644)
}
