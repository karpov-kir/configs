package ecoreport_test

// Where throwaway scratch lives, which is the whole of what this change moved. The defect: the
// `.idsd/` exclusion was shared across every worktree of a clone while the directory it excluded was
// not, so a ship authored from the main checkout was invisible in every worktree and vice versa.
//
// Two properties carry all of it, and each has a case here that fails if it breaks silently:
// the location resolves the same from every worktree and across branch switches, and it sits where
// `git add -A` cannot reach it. Committed mode is deliberately untouched — git already gives a tracked
// `.idsd/` both properties — so its case here asserts that nothing moved.

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEveryWorktreeOfACloneSeesTheOneScratchDirectory(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-shared")
	f.newIntentFile("001-shared")
	second := f.base + "/second-worktree"
	f.mustGit("worktree", "add", "-q", second, "-b", "second")
	if !f.exists(second) {
		t.Skip("git worktree add is unavailable here, so this case cannot be built")
	}
	f.record("fixture: git reports two worktrees",
		countLinesWithPrefix(f.mustGit("worktree", "list", "--porcelain"), "worktree ") == 2, "")

	// Asked from the second worktree, which is the vantage point that used to see nothing.
	fromSecond := f.runReportStdoutIn(second, "root")
	fromFirst := f.runReportStdout("root")
	f.record("both worktrees resolve the same scratch directory",
		fromFirst == fromSecond && fromFirst != "", "first: "+fromFirst+"\nsecond: "+fromSecond)

	// The location alone is not the property the human cares about — being able to read the other
	// worktree's ship is.
	f.runReportIn(second, "list")
	f.record("and the ship authored in the first worktree is listed from the second",
		strings.Contains(f.out, "001-shared"), f.evidence())
	f.record("and its intent file is readable from there",
		f.isFile(fromSecond+"/intents/001-shared.md"), fromSecond)

	// Pins WHICH directory, not merely that the two agree: a per-worktree location would answer two
	// different paths above, but so would a location keyed on something else that happened to match.
	f.record("and that directory is the clone's shared git dir, not either worktree's",
		fromFirst == f.sharedIdsd(), "resolved: "+fromFirst+"\nwanted: "+f.sharedIdsd())

	f.runReport("discard", "001-shared")
	f.record("and a discard from one worktree leaves neither tree holding scratch",
		f.status == 0 && !f.exists(f.sharedIdsd()) && f.treeIsFreeOfScratch(), f.evidence())
}

func TestTheScratchDirectorySurvivesABranchSwitch(t *testing.T) {
	t.Parallel()
	// The other half of "always there": a checkout that changes branches must not change what the
	// scratch holds. In the tree it did, because the directory was ignored rather than tracked and a
	// branch carrying its own `.idsd/` would collide with it.
	f := newShip(t, "001-branching")
	f.newIntentFile("001-branching")
	before := f.runReportStdout("root")
	f.mustGit("checkout", "-q", "-b", "elsewhere")
	f.record("the location is unchanged by a branch switch", f.runReportStdout("root") == before, before)
	f.runReport("list")
	f.record("and the ship is still there after it", strings.Contains(f.out, "001-branching"), f.evidence())
}

func TestTheLocationIsResolvedFromTheRootNotTheCallersDirectory(t *testing.T) {
	t.Parallel()
	// `git rev-parse --git-common-dir` answers RELATIVE to the caller in an ordinary repo — a bare
	// `.git` from the root. Left relative, or absolutized against the caller instead of the root, a
	// run from a subdirectory builds its scratch beside the caller: `sub/.git/idsd`. Every command
	// still reports success, and the ship is somewhere nothing else looks.
	f := newShip(t, "001-subdir")
	f.newIntentFile("001-subdir")
	sub := f.repo + "/deep/nested"
	f.mkdirAll(sub)
	fromRoot := f.runReportStdout("root")
	fromSub := f.runReportStdoutIn(sub, "root")
	f.record("a run from a subdirectory resolves the same absolute location",
		fromSub == fromRoot && filepath.IsAbs(fromSub), "root: "+fromRoot+"\nsubdir: "+fromSub)
	f.runReportIn(sub, "list")
	f.record("and sees the ship from there", strings.Contains(f.out, "001-subdir"), f.evidence())
	f.record("and built no scratch dir beside the caller", !f.exists(sub+"/.git"), sub)
}

func TestScratchSitsWhereGitAddAllCannotReachIt(t *testing.T) {
	t.Parallel()
	// The property that replaced the local exclusion; treeIsFreeOfScratch says why it is the stronger of
	// the two, and asks git rather than the layout.
	f := newShip(t, "001-unreachable")
	f.newIntentFile("001-unreachable")
	f.armFullPass("001-unreachable")
	f.record("a full pass leaves the working tree with nothing to hide", f.treeIsFreeOfScratch(), f.evidence())
	dirty, _ := f.git("status", "--porcelain")
	f.record("and git status says nothing about idsd at all",
		!strings.Contains(dirty, "idsd"), "git status --porcelain:\n"+dirty)
	// Named rather than merely true: check-ignore is what a skill runs first, and its line is where the
	// human learns which directory this run is writing to.
	f.runReport("check-ignore")
	f.record("and check-ignore names the location instead of claiming an exclusion",
		f.status == 0 && strings.Contains(f.out, f.sharedIdsd()), f.evidence())
}

func TestCommittedModeKeepsItsScratchInTheTree(t *testing.T) {
	t.Parallel()
	// The mode this change must not touch. A tracked `.idsd/` already appears on every branch and in
	// every worktree, because git checks it out — so moving it would be a regression, not a fix.
	f := newCommittedRepo(t)
	f.record("a committed repo resolves the in-tree location",
		f.runReportStdout("root") == f.treeIdsd(), f.runReportStdout("root"))
	f.runReport("init", "001-committed")
	f.record("and init writes the report there",
		f.status == 0 && f.isFile(f.treeIdsd()+"/qualify-reports/001-committed-qualify-report.md"), f.evidence())
	f.record("and nothing was created under the shared git dir", !f.exists(f.sharedIdsd()), "")
}

func TestAnOverrideMovesTheRootAndSaysSo(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	elsewhere := f.base + "/elsewhere"
	f.writeOverride("root " + elsewhere + "\n")
	f.runReport("check-ignore")
	f.assertReports(elsewhere, "check-ignore names the overridden location")
	f.assertReports("overridden by", "and says an override is what moved it")

	root := f.runReportStdout("root")
	f.record("the root lands under the override, keyed by this clone",
		strings.HasPrefix(root, elsewhere+"/"), root)
	// `<basename>-<digest>`: the basename is what a human reads, the digest is what keeps two clones
	// of one repo apart.
	key := strings.TrimPrefix(root, elsewhere+"/")
	f.record("and the key is the clone's directory name plus a digest",
		strings.HasPrefix(key, "r-") && len(key) == len("r-")+6, "key: "+key)

	// The announcement goes to stderr, never stdout: `state` prints exactly one token and callers route
	// on it, so a note on stdout would be read as a routing answer.
	f.runReport("init", "001-overridden")
	f.record("and init wrote the report under the override",
		f.status == 0 && f.isFile(root+"/qualify-reports/001-overridden-qualify-report.md"), f.evidence())
	f.record("and `state` still prints one bare token, with the note on stderr",
		f.runReportStdout("state", "001-overridden") == "resume", f.runReportStdout("state", "001-overridden"))
}

func TestAnOverrideKeyIsTheCloneNotTheWorktree(t *testing.T) {
	t.Parallel()
	// The trap this key exists to avoid. `basename($(git rev-parse --show-toplevel))` answers the
	// WORKTREE's own directory name, so a key built from it hands every worktree a different scratch
	// dir — the exact defect being fixed, wearing the shape of a fix and reporting success throughout.
	f := newShip(t, "001-keyed")
	elsewhere := f.base + "/elsewhere"
	f.writeOverride("root " + elsewhere + "\n")
	second := f.base + "/wt-named-differently"
	f.mustGit("worktree", "add", "-q", second, "-b", "second")
	if !f.exists(second) {
		t.Skip("git worktree add is unavailable here, so this case cannot be built")
	}
	fromRoot := f.runReportStdout("root")
	fromWorktree := f.runReportStdoutIn(second, "root")
	f.record("a worktree with a different directory name resolves the same key",
		fromRoot == fromWorktree, "main: "+fromRoot+"\nworktree: "+fromWorktree)
	f.record("and the key does not carry the worktree's name",
		!strings.Contains(fromWorktree, "wt-named-differently"), fromWorktree)
}

func TestTwoClonesOfOneRepoNeverShareAScratchDirectory(t *testing.T) {
	t.Parallel()
	// Two clones share a remote URL, and often a basename, so neither is an identity. The digest is
	// over the git dir's realpath, which is unique per clone.
	f := newRepo(t)
	elsewhere := f.base + "/elsewhere"
	f.writeOverride("root " + elsewhere + "\n")
	clone := f.base + "/second-clone/r"
	f.mkdirAll(f.base + "/second-clone")
	if _, status := f.gitIn(f.base+"/second-clone", "clone", "-q", f.repo, "r"); status != 0 {
		t.Skip("git clone is unavailable here, so this case cannot be built")
	}
	first := f.runReportStdout("root")
	second := f.runReportStdoutIn(clone, "root")
	f.record("two clones of one repository resolve different scratch directories",
		first != second && first != "" && second != "", "first: "+first+"\nsecond: "+second)
	// Both are named for their own directory, so the digest is what separates them rather than luck.
	f.record("and both are named for their own clone directory",
		strings.Contains(first, "/r-") && strings.Contains(second, "/r-"), "first: "+first+"\nsecond: "+second)
}

func TestABrokenOverrideRefusesRatherThanFallingBack(t *testing.T) {
	t.Parallel()
	// The one failure an override must not have. Falling back to the default writes this clone's
	// intents into a directory the human was never told about, at exit 0, and they go looking in the
	// one they configured. Every arm below is a separate way the file can be unusable.
	for _, c := range []struct {
		name, content, needle string
	}{
		{"a line the tool does not understand", "rooot /tmp/x\n", "does not understand"},
		{"no root at all", "# just a comment\n", "sets no `root`"},
		{"root set twice", "root /tmp/a\nroot /tmp/b\n", "more than once"},
		{"a relative root", "root ../sideways\n", "relative root"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newRepo(t)
			f.writeOverride(c.content)
			f.runReport("check-ignore")
			f.assertRefused("refuses rather than falling back to the default")
			f.assertReports(c.needle, "and names what is wrong with the override")
			f.record("and created no scratch directory anywhere",
				!f.exists(f.sharedIdsd()) && f.treeIsFreeOfScratch(), f.evidence())
		})
	}

	// `~/` is expanded, because this file is hand-written and `~` is what a person types. Nothing else
	// would expand it, so left alone it builds a literal directory called `~`.
	f := newRepo(t)
	f.writeOverride("root ~/idsd-elsewhere\n")
	root := f.runReportStdout("root")
	f.record("a leading ~/ is expanded against HOME rather than taken literally",
		strings.HasPrefix(root, f.home+"/idsd-elsewhere/"), "root: "+root+"\nHOME: "+f.home)
}

func TestAnOverrideInsideTheWorkingTreeIsRefused(t *testing.T) {
	t.Parallel()
	// An override can point anywhere, the working tree included. There it puts back every failure this
	// change removed at once: the report sits inside the tree it fingerprints, so no stamp is ever
	// fresh, and `git add -A` stages the pass's security findings.
	f := newRepo(t)
	f.writeOverride("root " + f.repo + "/inside\n")
	f.runReport("check-ignore")
	f.assertRefused("refuses an override pointing into the working tree")
	f.assertReports("inside this checkout's working tree", "and says why that cannot work")
	f.record("and created nothing there", !f.exists(f.repo+"/inside"), "")
}

func TestAnInTreeScratchDirectoryIsNeverMigratedSilently(t *testing.T) {
	t.Parallel()
	// The upgrade path. A repo whose throwaway scratch still sits in the tree holds the only copy of
	// those intents, so nothing here moves or merges them — it refuses and says what to do. Moving
	// them silently is the one action with no undo.
	f := newRepo(t)
	f.mkdirAll(f.treeIdsd() + "/intents")
	f.write(f.treeIdsd()+"/intents/001-from-the-old-layout/intent.md", "# mine\n")
	f.runReport("check-ignore")
	f.assertRefused("refuses while an in-tree scratch directory still holds content")
	f.assertReports("still holds 1 file(s)", "and counts what it found")
	f.assertReports("intents/001-from-the-old-layout.md", "and names it, so the human knows what to move")
	f.assertReports("Nothing here moves your files for you", "and says it will not move them")
	f.record("and left every file exactly where it was",
		f.isFile(f.treeIdsd()+"/intents/001-from-the-old-layout/intent.md"), "")

	// An empty one is a trace in the mode whose contract is leaving none, and holds nothing to lose.
	empty := newRepo(t)
	empty.mkdirAll(empty.treeIdsd())
	empty.runReport("check-ignore")
	empty.record("an empty one is removed rather than refused",
		empty.status == 0 && !empty.exists(empty.treeIdsd()), empty.evidence())

	// The state a finished migration actually leaves: the files are gone but `intents/` and
	// `qualify-reports/` still stand as empty directories. Counting directory entries rather than files
	// reads those as content, so the repo is refused with a message claiming it holds content when it
	// holds none — and the human has nothing to delete.
	skeleton := newRepo(t)
	skeleton.mkdirAll(skeleton.treeIdsd() + "/intents")
	skeleton.mkdirAll(skeleton.treeIdsd() + "/qualify-reports")
	skeleton.runReport("check-ignore")
	skeleton.record("an empty directory skeleton is cleared, not read as content",
		skeleton.status == 0 && !skeleton.exists(skeleton.treeIdsd()), skeleton.evidence())

	// And the refusal counts files rather than the paths it prints, so the number cannot drift from what
	// it is naming.
	many := newRepo(t)
	many.mkdirAll(many.treeIdsd() + "/intents")
	for i := 0; i < 25; i++ {
		many.write(many.treeIdsd()+"/intents/"+strconv.Itoa(i)+".md", "# mine\n")
	}
	many.runReport("check-ignore")
	many.assertRefused("a directory of files is still refused")
	many.assertReports("holds 25 file(s)", "and counts every file, not just the ones it lists")
	many.assertReports("and 5 more", "and says how many it did not list")

	// The whole pass proceeds on check-ignore's ok line — `init` writes the report next — so a refusal
	// that still printed one would be acted on. Asserted here because this is the refusal a real repo
	// is most likely to meet.
	f.record("and printed no ok line the pass could have proceeded on",
		!strings.Contains(f.out, "ok:"), f.out)
}

func TestTheStaleExclusionRuleIsCleanedUp(t *testing.T) {
	t.Parallel()
	// The old layout wrote `.idsd/` into .git/info/exclude to hide in-tree scratch from `git add -A`.
	// Nothing writes it now, so every repo that ever ran a throwaway ship carries a rule for a directory
	// that is not there. Left alone it is not merely untidy: it makes git ignore untracked files under
	// .idsd/, which is exactly what promotion needs staged.
	t.Run("check-ignore removes it and says so", func(t *testing.T) {
		f := newRepo(t)
		exclude := f.repo + "/.git/info/exclude"
		f.appendTo(exclude, ".idsd/\n")
		f.runReport("check-ignore")
		f.record("the rule is gone",
			f.status == 0 && !containsLine(f.read(exclude), ".idsd/"), f.evidence()+"\n"+f.read(exclude))
		f.assertReports("cleaned:", "and the cleanup is announced rather than silent")
	})

	// The human's own rules share that file. Removing one of theirs would be the worst outcome here.
	t.Run("and leaves every other rule in that file alone", func(t *testing.T) {
		f := newRepo(t)
		exclude := f.repo + "/.git/info/exclude"
		f.appendTo(exclude, "*.scratch\n.idsd/\nbuild/\n")
		f.runReport("check-ignore")
		f.record("their rules survive",
			containsLine(f.read(exclude), "*.scratch") && containsLine(f.read(exclude), "build/") &&
				!containsLine(f.read(exclude), ".idsd/"), f.read(exclude))
		_, theirs := f.git("check-ignore", "-q", "keep.scratch")
		f.record("and still take effect", theirs == 0, "check-ignore keep.scratch exited "+strconv.Itoa(theirs))
	})

	// Ordering, and it matters: cleaning before the in-tree reconcile refuses would strip the rule and
	// leave a still-populated .idsd/ newly visible to the next `git add -A`.
	t.Run("but not while an in-tree scratch dir still holds content", func(t *testing.T) {
		f := newRepo(t)
		exclude := f.repo + "/.git/info/exclude"
		f.appendTo(exclude, ".idsd/\n")
		f.mkdirAll(f.treeIdsd() + "/intents")
		f.write(f.treeIdsd()+"/intents/001-not-yet-moved/intent.md", "# mine\n")
		f.runReport("check-ignore")
		f.assertRefused("it refuses for the un-migrated directory")
		f.record("and the rule is still there, so that directory stays hidden from `git add -A`",
			containsLine(f.read(exclude), ".idsd/"), f.read(exclude))
	})

	// Promotion is the call site where the stale rule does real damage: `git add` stages nothing, exits
	// 0, and the mode check refuses blaming the human for having nothing durable.
	t.Run("and promote clears it, so promotion is not refused for the wrong reason", func(t *testing.T) {
		f := newShip(t, "001-promoting-stale")
		f.newIntentFile("001-promoting-stale")
		exclude := f.repo + "/.git/info/exclude"
		f.appendTo(exclude, ".idsd/\n")
		f.runReport("promote")
		f.record("promote succeeds",
			f.status == 0 && f.runReportStdout("repo-mode") == "committed", f.evidence())
		f.record("and the stale rule is gone", !containsLine(f.read(exclude), ".idsd/"), f.read(exclude))
		staged, _ := f.git("diff", "--cached", "--name-only")
		f.record("and the intent it moved is actually staged",
			strings.Contains(staged, ".idsd/intents/001-promoting-stale/intent.md"), "staged:\n"+staged)
	})
}

func TestPerWorktreeStateGoesToTheWorktreesOwnGitDir(t *testing.T) {
	t.Parallel()
	// The other half of the gitPath/gitCommonPath split, and the one that must stay per-worktree: a
	// ship's stage markers. `--git-path` answers a RELATIVE path in an ordinary repo and an ABSOLUTE one
	// in a linked worktree, so an absolute answer joined onto the root builds the markers at
	// `<worktree>/<absolute path>` — a directory tree inside the checkout, while the marker the next
	// command looks for is not there. Every command still reports success.
	f := newShip(t, "001-markers")
	second := f.base + "/marker-worktree"
	f.mustGit("worktree", "add", "-q", second, "-b", "markers")
	if !f.exists(second) {
		t.Skip("git worktree add is unavailable here, so this case cannot be built")
	}
	// What the worktree held before the write, so a stray is detected by what appeared rather than by
	// guessing the shape of the wrong path. Naming a prefix cannot work: the stray mirrors wherever the
	// fixture happens to live — /private/var here, something else on CI — so a literal `/Users` check
	// can never match and the case passes while observing nothing.
	before := strings.Join(f.entries(second), "\n")
	f.runReportIn(second, "invalidate", "001-markers")
	f.runReportIn(second, "stage-returned", "code-review", "001-markers")
	f.record("a stage marker written from a linked worktree is recorded",
		f.status == 0, f.evidence())
	after := strings.Join(f.entries(second), "\n")
	f.record("and nothing new appeared inside the worktree, so no absolute git path was prefixed onto the root",
		after == before, "before:\n"+before+"\nafter:\n"+after)
	// And the marker is readable back from that same worktree, which is what the stamp depends on.
	f.runReportIn(second, "no-items", "code-review", "001-markers")
	f.record("and reads back, so the stamp can see it", f.status == 0, f.evidence())
}

// The same guard, reached the other way. gitPath resolves the git dir from the on-disk layout first
// and only asks git when an environment override makes the layout untrustworthy, so the case above
// never reaches the `--git-path` answer at all: for a linked worktree the layout resolver answers, and
// the prefixing arm below it is skipped. That left the arm reachable and unreached — a guard with a
// case named against it that could not fail, which a mutation run reports as a survivor.
//
// GIT_CEILING_DIRECTORIES is the cheapest override that forces the fallback: layoutOverridden reads it
// and declines, while git's own answer for a path under the fixture is unchanged. t.Setenv bars
// t.Parallel, which is why this is its own case rather than a subtest of the one above.
func TestPerWorktreeStateGoesToItsOwnGitDirWhenTheLayoutIsOverridden(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", t.TempDir()+"/elsewhere")
	f := newShip(t, "002-markers")
	second := f.base + "/override-worktree"
	f.mustGit("worktree", "add", "-q", second, "-b", "override-markers")
	if !f.exists(second) {
		t.Skip("git worktree add is unavailable here, so this case cannot be built")
	}
	before := strings.Join(f.entries(second), "\n")
	f.runReportIn(second, "invalidate", "002-markers")
	f.runReportIn(second, "stage-returned", "code-review", "002-markers")
	f.record("a stage marker written from a linked worktree with the layout overridden is recorded",
		f.status == 0, f.evidence())
	after := strings.Join(f.entries(second), "\n")
	f.record("and nothing new appeared inside the worktree, so the absolute --git-path answer was not prefixed onto the root",
		after == before, "before:\n"+before+"\nafter:\n"+after)
	f.runReportIn(second, "no-items", "code-review", "002-markers")
	f.record("and reads back, so the stamp can see it", f.status == 0, f.evidence())
}

func TestASiblingWorktreeCannotReadAStampItNeverEarned(t *testing.T) {
	t.Parallel()
	// The divergence this whole change made reachable. The scratch directory
	// is now shared per clone, so a sibling worktree can SEE another's stamped report — and the freshness
	// guard compares tree fingerprints, which are identical for two clean worktrees at the same commit.
	// So the sibling read a ship it never ran as its own `ready`, and `gate` passed it.
	f := newShip(t, "090-twin")
	f.newIntentFile("090-twin")
	twin := f.base + "/twin-worktree"
	f.mustGit("worktree", "add", "-q", twin, "-b", "twin")
	if !f.exists(twin) {
		t.Skip("git worktree add is unavailable here, so this case cannot be built")
	}
	// Stamped in the main checkout only. Both trees are clean and at the same commit, which is exactly
	// the state a freshly added worktree is in.
	f.stampFullPass("090-twin")

	// The fixture's own precondition: without identical fingerprints the case passes on staleness rather
	// than on the guard, and would stay green if the guard were removed.
	fromMain := f.runReportStdout("gate", "090-twin")
	f.record("fixture: the stamp is clean in the worktree that earned it",
		f.status == 0, f.evidence()+"\n"+fromMain)

	f.runReportIn(twin, "gate", "090-twin")
	f.record("gate blocks in a sibling worktree that reviewed nothing",
		f.status != 0, f.evidence())
	f.assertReports("reviewed in another worktree", "and says the review describes a tree somewhere else")

	f.runReportIn(twin, "state", "090-twin")
	f.record("and state routes the sibling to re-qualify rather than ready",
		strings.TrimSpace(f.out) == "re-qualify", f.evidence())

	// The positive control: the worktree that did the work is unaffected.
	f.runReport("state", "090-twin")
	f.record("while the worktree that ran the pass still reads ready",
		strings.TrimSpace(f.out) == "ready", f.evidence())
}

func TestTheStampRecordsWhichWorktreeReviewedTheTree(t *testing.T) {
	t.Parallel()
	// The field the guard above reads. Asserted separately because a guard comparing a value nothing
	// writes is a guard that never fires, and a stamp that records the wrong worktree fires it always.
	f := newShip(t, "091-recorded")
	f.newIntentFile("091-recorded")
	f.stampFullPass("091-recorded")
	report := f.read(f.reportPath("091-recorded"))
	// `<token> <path>`: the token is what gate compares, the path is for the human reading the report.
	recorded := fieldFrom(report, "reviewed-worktree")
	f.record("the stamp writes a token and the reviewing worktree's path",
		len(strings.Fields(recorded)) == 2 && strings.HasSuffix(recorded, " "+f.canonicalRepo()) &&
			len(strings.Fields(recorded)[0]) == 16, "recorded: "+recorded)

	// invalidate clears it with the rest, or a re-qualify inherits the old worktree and the guard reads
	// a review that no longer exists.
	f.runReport("invalidate", "091-recorded")
	f.record("and invalidate clears it with the rest of the stamp",
		containsLine(f.read(f.reportPath("091-recorded")), "reviewed-worktree: pending"),
		f.read(f.reportPath("091-recorded")))

	// Exactly one of each line, whatever order the report carried them in.
	f.stampFullPass("091-recorded")
	after := f.read(f.reportPath("091-recorded"))
	f.record("and a second stamp leaves one of each line, not two",
		countLinesWithPrefix(after, "reviewed-worktree:") == 1 &&
			countLinesWithPrefix(after, "reviewed-tree:") == 1 &&
			countLinesWithPrefix(after, "reviewed-stages:") == 1, after)
}

func TestAWorktreeIdentityIsNotItsPath(t *testing.T) {
	t.Parallel()
	// The first version of this guard recorded the reviewing worktree as a filesystem path. A path is not
	// an identity, and it fails in both directions.
	t.Run("a new worktree at a reused path does not inherit the stamp", func(t *testing.T) {
		f := newShip(t, "092-recycled")
		f.newIntentFile("092-recycled")
		reused := f.base + "/recycled"
		f.mustGit("worktree", "add", "-q", reused, "-b", "first")
		if !f.exists(reused) {
			t.Skip("git worktree add is unavailable here, so this case cannot be built")
		}
		// Stamped by the first occupant of that path, which then gates clean there.
		f.stampFullPassIn(reused, "092-recycled")
		f.runReportIn(reused, "gate", "092-recycled")
		f.record("fixture: the first occupant gates clean", f.status == 0, f.evidence())

		// Removed and recreated at the SAME path — ordinary practice for a scratch worktree, and a
		// brand-new worktree that has run nothing.
		f.mustGit("worktree", "remove", reused)
		f.mustGit("worktree", "add", "-q", reused, "-b", "second")
		f.runReportIn(reused, "gate", "092-recycled")
		f.record("the replacement does not inherit its predecessor's clean gate",
			f.status != 0, f.evidence())
		f.runReportIn(reused, "state", "092-recycled")
		f.record("and routes to re-qualify", strings.TrimSpace(f.out) == "re-qualify", f.evidence())
	})

	t.Run("a moved worktree keeps its own stamp", func(t *testing.T) {
		f := newShip(t, "093-moved")
		f.newIntentFile("093-moved")
		from, to := f.base+"/before-move", f.base+"/after-move"
		f.mustGit("worktree", "add", "-q", from, "-b", "mover")
		if !f.exists(from) {
			t.Skip("git worktree add is unavailable here, so this case cannot be built")
		}
		f.stampFullPassIn(from, "093-moved")
		if _, status := f.git("worktree", "move", from, to); status != 0 {
			t.Skip("git worktree move is unavailable here, so this case cannot be built")
		}
		// The worktree that did the work is the same worktree, wherever it now sits. Telling it
		// otherwise costs a whole re-qualify for a directory rename.
		f.runReportIn(to, "gate", "093-moved")
		f.record("the worktree that ran the pass still gates clean after being moved",
			f.status == 0, f.evidence())
		f.runReportIn(to, "state", "093-moved")
		f.record("and still reads ready", strings.TrimSpace(f.out) == "ready", f.evidence())
	})
}

func TestAnIdentityThatCannotBeEstablishedIsNotAnIdentity(t *testing.T) {
	t.Parallel()
	// The subtlest of the three. The first version of this guard returned the literal string `unmintable`
	// when it could not write the token, and compared it like any other value — so two worktrees that both
	// failed to mint compared EQUAL and gated clean off each other's review. worktreeToken says why that
	// pairing is not the rare coincidence it looks like.
	t.Run("stamp refuses rather than recording an identity it could not establish", func(t *testing.T) {
		f := newShip(t, "094-unmintable")
		f.newIntentFile("094-unmintable")
		f.armFullPass("094-unmintable")
		// Unwritable by construction: a directory where the token file goes, so the write fails for every
		// user including root.
		f.mkdirAll(f.repo + "/.git/idsd-worktree-id")
		f.runReport("stamp", "code-review,security-review,tighten,refactor", "094-unmintable")
		f.assertRefused("stamp refuses when this worktree's identity cannot be established")
		f.assertReports("NOT stamped", "and says the pass was not stamped")
		// Still the placeholder invalidate left, so nothing was recorded.
		f.record("and wrote no reviewed-worktree value",
			fieldFrom(f.read(f.reportPath("094-unmintable")), "reviewed-worktree") == "pending",
			f.read(f.reportPath("094-unmintable")))
	})

	t.Run("and two worktrees that cannot mint do not gate clean off each other", func(t *testing.T) {
		f := newShip(t, "095-both-broken")
		f.newIntentFile("095-both-broken")
		twin := f.base + "/broken-twin"
		f.mustGit("worktree", "add", "-q", twin, "-b", "broken")
		if !f.exists(twin) {
			t.Skip("git worktree add is unavailable here, so this case cannot be built")
		}
		// Stamped while identity works, so there is a real review to steal.
		f.stampFullPass("095-both-broken")
		// Now break minting on BOTH sides, which is how the condition actually occurs — the private git
		// dir being unwritable is a property of the clone, so it hits every worktree at once. A directory
		// where the file goes fails the write for every user, root included.
		f.remove(f.repo + "/.git/idsd-worktree-id")
		f.mkdirAll(f.repo + "/.git/idsd-worktree-id")
		// `broken-twin`, the worktree DIRECTORY's basename — git names the private git dir after that, not
		// after the branch. Aimed at `broken` this built a private dir for a worktree that does not exist,
		// the twin minted an identity happily, and the case passed on the mismatch arm while claiming to
		// cover the arm where neither side has one.
		f.mkdirAll(f.repo + "/.git/worktrees/broken-twin/idsd-worktree-id")
		f.runReportIn(twin, "gate", "095-both-broken")
		f.record("the twin does not gate clean", f.status != 0, f.evidence())
		// The message, not only the exit: with the unestablished arm gone, the block still happens — the
		// arm below it compares an empty `mine` against a real token and refuses too — so the exit alone
		// cannot tell the two apart, and it says the review was taken elsewhere when what is actually
		// unknown is this tree's own identity.
		f.assertReports("identity could not be established", "and blocks on the identity, not on a mismatch")
		f.runReportIn(twin, "state", "095-both-broken")
		f.record("and routes to re-qualify", strings.TrimSpace(f.out) == "re-qualify", f.evidence())
	})

	// The reader loop this note breaks: in a repo whose identity can never be established, the gate
	// blocks on freshness and stages, both of which send the reader back to re-qualify — and that
	// qualify's stamp refuses for a reason printed only on stamp's stderr. An orchestrator that
	// swallows that stderr never learns why.
	t.Run("and the gate says why it will stay unstamped, where the human meets the block", func(t *testing.T) {
		f := newShip(t, "097-no-route")
		f.newIntentFile("097-no-route")
		f.mkdirAll(f.repo + "/.git/idsd-worktree-id")
		f.runReport("gate", "097-no-route")
		f.record("the gate blocks", f.status != 0, f.evidence())
		f.assertReports("it will stay unstamped", "and says re-running the stages will not help")
		f.assertReports("idsd-worktree-id", "and names the path to fix")
		// The positive control: a healthy repo's block never carries that note, or it would appear on
		// every ordinary unstamped report and mean nothing.
		healthy := newShip(t, "098-ordinary")
		healthy.newIntentFile("098-ordinary")
		healthy.runReport("gate", "098-ordinary")
		healthy.record("while an ordinary unstamped report does not carry it",
			healthy.status != 0 && !strings.Contains(healthy.out, "it will stay unstamped"), healthy.evidence())
	})

	t.Run("a token file holding garbage reads as a different worktree, not as a match", func(t *testing.T) {
		f := newShip(t, "096-garbage")
		f.newIntentFile("096-garbage")
		f.stampFullPass("096-garbage")
		f.runReport("gate", "096-garbage")
		f.record("fixture: it gates clean while the token is intact", f.status == 0, f.evidence())
		// Not a token: the value is checked for shape rather than trusted, so this mints a fresh one.
		f.write(f.repo+"/.git/idsd-worktree-id", "not-a-token\n")
		f.runReport("gate", "096-garbage")
		f.record("garbage in the token file does not gate clean", f.status != 0, f.evidence())
		// What the shape check actually does, since the block above happens either way: trusted, the
		// garbage becomes this worktree's identity and stays in the file. Checked, it is replaced by a
		// freshly minted token, and the worktree reads as a different one rather than as a nameless one.
		minted := strings.TrimSpace(f.read(f.repo + "/.git/idsd-worktree-id"))
		f.record("and the garbage was replaced by a freshly minted token",
			len(minted) == 16 && strings.Trim(minted, "0123456789abcdef") == "",
			"the token file holds: "+minted)
	})
}

func TestAStampedTreeWithNoReviewingWorktreeIsNotAReview(t *testing.T) {
	t.Parallel()
	// A report carrying a stamped tree and NO reviewed-worktree line falls through the guard entirely and
	// gates clean in every worktree — noReviewingWorktreeRecorded says how. Reachable through the migration
	// this change itself instructs: reconcileTreeIdsdDir tells the human to copy files out of an in-tree
	// .idsd/ by hand, and a report copied that way has no such line.
	f := newShip(t, "099-legacy")
	f.newIntentFile("099-legacy")
	f.stampFullPass("099-legacy")
	f.runReport("gate", "099-legacy")
	f.record("fixture: it gates clean with the field intact", f.status == 0, f.evidence())

	// A malformed value is the same case as an absent one: present, unusable. Asserted first, and on the
	// worktree line rather than the tree line, because the tree has to stay fresh for `re-qualify` to be
	// this guard's answer — a stale tree returns the same token one branch earlier, so a case that edits
	// reviewed-tree passes on freshness and would stay green with the guard removed.
	f.replaceLine(f.reportPath("099-legacy"), "reviewed-worktree:", "reviewed-worktree: not-a-token")
	f.runReport("state", "099-legacy")
	f.record("a malformed worktree value routes to re-qualify",
		strings.TrimSpace(f.out) == "re-qualify", f.evidence())

	// Exactly the shape the older layout wrote: tree and stages stamped, no worktree line at all.
	f.dropLines(f.reportPath("099-legacy"), "reviewed-worktree:")
	f.record("fixture: the line is gone",
		!strings.Contains(f.read(f.reportPath("099-legacy")), "reviewed-worktree:"),
		f.read(f.reportPath("099-legacy")))

	f.runReport("gate", "099-legacy")
	f.record("a stamped tree with no reviewing worktree does not gate clean", f.status != 0, f.evidence())
	f.assertReports("no usable reviewing worktree", "and says which worktree it vouches for is unknown")
	f.runReport("state", "099-legacy")
	f.record("and state routes to re-qualify rather than ready",
		strings.TrimSpace(f.out) == "re-qualify", f.evidence())
}

func TestPromoteCountsFilesNotDirectoryEntries(t *testing.T) {
	t.Parallel()
	// The sibling of the same distinction reconcileTreeIdsdDir needs. Counting directory entries, promote
	// refused an in-tree .idsd/ holding only the empty `intents/` and `qualify-reports/` skeleton a
	// finished migration leaves, and told the human to reconcile two empty directories by hand.
	f := newShip(t, "100-skeleton")
	f.newIntentFile("100-skeleton")
	f.mkdirAll(f.treeIdsd() + "/intents")
	f.mkdirAll(f.treeIdsd() + "/qualify-reports")
	f.runReport("promote")
	f.record("promote is not refused by an empty directory skeleton",
		f.status == 0 && f.runReportStdout("repo-mode") == "committed", f.evidence())

	// The positive control: a real file there still refuses, and now names what it found.
	other := newShip(t, "101-realfile")
	other.newIntentFile("101-realfile")
	other.mkdirAll(other.treeIdsd())
	other.write(other.treeIdsd()+"/charter.md", "# theirs\n")
	other.runReport("promote")
	other.assertRefused("while a real file there still refuses")
	other.assertReports("already holds 1 file(s)", "and counts what it found")
	other.assertReports("charter.md", "and names it")
}

func TestANestedSymlinkIsNotSilentlyDeleted(t *testing.T) {
	t.Parallel()
	// Counting only regular files, an in-tree .idsd/ holding just `charter.md -> ../docs/charter.md`
	// counted zero and the RemoveAll fired, deleting the link without a word — while a symlink at the TOP
	// of that directory was loudly refused three lines above.
	f := newRepo(t)
	f.write(f.repo+"/docs-charter.md", "# the human's real file\n")
	f.mkdirAll(f.treeIdsd())
	f.symlink(f.repo+"/docs-charter.md", f.treeIdsd()+"/charter.md")
	f.runReport("check-ignore")
	f.assertRefused("a nested symlink is refused, not removed in silence")
	f.record("and the link is still there",
		f.isSymlink(f.treeIdsd()+"/charter.md"), f.evidence())
	f.record("and its target is untouched", f.isFile(f.repo+"/docs-charter.md"), "")
}

func TestTheGateClaimsIdenticalTreesOnlyWhenTheyAre(t *testing.T) {
	t.Parallel()
	// The reviewed-in-another-worktree block is reachable with a STALE tree — an unstamped `reviewed-tree`
	// beside a real token, which an older binary's invalidate could leave — and unguarded it printed "The
	// fingerprints match because both trees are identical" directly under a BLOCK saying they differ.
	f := newShip(t, "102-contradiction")
	f.newIntentFile("102-contradiction")
	f.stampFullPass("102-contradiction")
	f.runReport("gate", "102-contradiction")
	f.record("fixture: it gates clean while fresh and in its own worktree", f.status == 0, f.evidence())

	// A sibling worktree makes the token mismatch; editing the tree makes the fingerprints differ. Both
	// blocks now fire, and the pair must not contradict each other.
	twin := f.base + "/contradiction-twin"
	f.mustGit("worktree", "add", "-q", twin, "-b", "twin")
	if !f.exists(twin) {
		t.Skip("git worktree add is unavailable here, so this case cannot be built")
	}
	f.write(twin+"/moved.txt", "the tree is now different here\n")
	f.runReportIn(twin, "gate", "102-contradiction")
	f.record("both blocks fire", f.status != 0, f.evidence())
	f.assertReports("tree changed since last qualify", "the freshness block says the trees differ")
	f.record("and nothing claims the fingerprints match",
		!strings.Contains(f.out, "fingerprints match"), f.evidence())

	// The positive control: where the trees really are identical, the sentence is still there, because it
	// is what explains why a matching fingerprint did not clear the block.
	same := newShip(t, "103-identical")
	same.newIntentFile("103-identical")
	sameTwin := same.base + "/identical-twin"
	same.mustGit("worktree", "add", "-q", sameTwin, "-b", "identical")
	same.stampFullPass("103-identical")
	same.runReportIn(sameTwin, "gate", "103-identical")
	same.record("while identical trees still get the sentence that explains them",
		same.status != 0 && strings.Contains(same.out, "fingerprints match"), same.evidence())
}
