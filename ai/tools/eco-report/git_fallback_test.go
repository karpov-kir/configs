package ecoreport_test

// The path that asks the repository, which no other case here reaches.
//
// layout.go answers the repository's shape by reading the filesystem, and every caller in this package
// takes that answer when it comes. It comes for every fixture the rest of the suite builds, so the
// port underneath it ran in no case. Mutation reported exactly that: disabling the CommonDir call, the
// absolute-answer guard in gitPath, the helper, and the absolutize-against-the-root step each left the
// suite green. Two of the three now live in `ai/tools/repo`'s adapter, which makes every answer absolute,
// and `repo/exec_test.go` holds them to real git. What is left here is that this package asks at all,
// which is this one case: the same behaviour over the layout reader is pinned in scratch_location_test.go.
//
// The lever is GIT_CEILING_DIRECTORIES, pointed somewhere that is no ancestor of the fixture. layout.go
// refuses on any of its four environment names, so the callers fall through to git; and a ceiling that
// is not an ancestor changes nothing about what git itself answers, so what this case compares is the
// two code paths and not two different repositories.
//
// The lever itself is proven in layout_test.go: TestTheResolverDeclinesWhenTheEnvironmentOverridesTheLayout
// drives layoutRoot, layoutGitDir and layoutCommonDir with each of the four names set and requires all
// three to decline.
//
// Not parallel, and it cannot be: t.Setenv and t.Parallel are mutually exclusive.

import (
	"testing"
)

func TestTheGitFallbackResolvesTheSharedGitDirAndNotTheWorktreesOwn(t *testing.T) {
	// No ancestor of any fixture, so git's own behaviour is untouched and only layout.go reacts.
	t.Setenv("GIT_CEILING_DIRECTORIES", "/nonexistent-ceiling-for-the-fallback-cases")

	f := newShip(t, "001-fallback-shared")
	f.newIntentFile("001-fallback-shared")
	second := f.newLinkedWorktree("fallback-second")
	fromFirst := f.runReportStdout("root")
	fromSecond := f.runReportStdoutIn(second, "root")
	f.record("both worktrees resolve one location through git",
		fromFirst == fromSecond && fromFirst != "", "first: "+fromFirst+"\nsecond: "+fromSecond)
	// Which directory, not merely that the two agree: `--git-path .` also answers the same string
	// from one worktree asked twice, and would pass a case that only compared them to each other.
	f.record("and it is the clone's shared git dir",
		fromFirst == f.sharedIdsd(), "resolved: "+fromFirst+"\nwanted: "+f.sharedIdsd())
}
