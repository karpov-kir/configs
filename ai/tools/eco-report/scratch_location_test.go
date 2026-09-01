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
	// The property that replaced the local exclusion. Stronger than what it replaced: an exclude entry
	// can be edited away or fail to travel, a path outside the tree cannot.
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
	f.write(f.treeIdsd()+"/intents/001-from-the-old-layout.md", "# mine\n")
	f.runReport("check-ignore")
	f.assertRefused("refuses while an in-tree scratch directory still holds content")
	f.assertReports("still holds 1 file(s)", "and counts what it found")
	f.assertReports("intents/001-from-the-old-layout.md", "and names it, so the human knows what to move")
	f.assertReports("Nothing here moves your files for you", "and says it will not move them")
	f.record("and left every file exactly where it was",
		f.isFile(f.treeIdsd()+"/intents/001-from-the-old-layout.md"), "")

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
		f.write(f.treeIdsd()+"/intents/001-not-yet-moved.md", "# mine\n")
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
			strings.Contains(staged, ".idsd/intents/001-promoting-stale.md"), "staged:\n"+staged)
	})
}

func TestPerWorktreeStateGoesToTheWorktreesOwnGitDir(t *testing.T) {
	t.Parallel()
	// The other half of the gitPath/gitCommonPath split, and the one that must stay per-worktree: a
	// ship's stage markers. `--git-path` answers a RELATIVE path in an ordinary repo and an ABSOLUTE one
	// in a linked worktree, so an absolute answer joined onto the root builds the markers at
	// `<worktree>/<absolute path>` — a directory tree inside the checkout, while the marker the next
	// command looks for is not there. Every command still reports success.
	//
	// This used to be observed through the exclusion writing, which throwaway mode no longer does.
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
