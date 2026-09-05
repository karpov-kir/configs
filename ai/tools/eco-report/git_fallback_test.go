package ecoreport_test

// The paths that ask git, which nothing else here reaches.
//
// layout.go answers the repository's shape by reading the filesystem, and every caller in this package
// takes that answer when it comes. It comes for every fixture the rest of the suite builds, so the
// `rev-parse` fallback under it — and the three guards that make its answers usable — ran in no case
// at all. Mutation reported exactly that: disabling `--git-common-dir`, the absolutize-against-the-root
// step, and gitPath's absolute-answer guard each left the whole suite green.
//
// The lever is GIT_CEILING_DIRECTORIES, pointed somewhere that is no ancestor of the fixture. layout.go
// refuses on any of its four environment names, so the callers fall through to git; and a ceiling that
// is not an ancestor changes nothing about what git itself answers, so what these cases compare is the
// two code paths and not two different repositories.
//
// That the lever works is not assumed here: TestTheResolverDeclinesWhenTheEnvironmentOverridesTheLayout
// in layout_test.go drives layoutRoot, layoutGitDir and layoutCommonDir with each of the four names set
// and requires all three to decline.
//
// Not parallel, and it cannot be: t.Setenv and t.Parallel are mutually exclusive.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTheGitFallbackResolvesWhatTheLayoutReaderWould(t *testing.T) {
	// No ancestor of any fixture, so git's own behaviour is untouched and only layout.go reacts.
	t.Setenv("GIT_CEILING_DIRECTORIES", "/nonexistent-ceiling-for-the-fallback-cases")

	t.Run("the shared git dir, not the worktree's own", func(t *testing.T) {
		f := newShip(t, "001-fallback-shared")
		f.newIntentFile("001-fallback-shared")
		second := f.base + "/fallback-second"
		f.mustGit("worktree", "add", "-q", second, "-b", "fallback-second")
		if !f.exists(second) {
			t.Skip("git worktree add is unavailable here, so this case cannot be built")
		}
		fromFirst := f.runReportStdout("root")
		fromSecond := f.runReportStdoutIn(second, "root")
		f.record("both worktrees resolve one location through git",
			fromFirst == fromSecond && fromFirst != "", "first: "+fromFirst+"\nsecond: "+fromSecond)
		// Which directory, not merely that the two agree: `--git-path .` also answers the same string
		// from one worktree asked twice, and would pass a case that only compared them to each other.
		f.record("and it is the clone's shared git dir",
			fromFirst == f.sharedIdsd(), "resolved: "+fromFirst+"\nwanted: "+f.sharedIdsd())
	})

	t.Run("absolutized against the root, not the caller", func(t *testing.T) {
		f := newShip(t, "001-fallback-subdir")
		f.newIntentFile("001-fallback-subdir")
		sub := f.repo + "/deep/nested"
		f.mkdirAll(sub)
		fromRoot := f.runReportStdout("root")
		fromSub := f.runReportStdoutIn(sub, "root")
		// `--git-common-dir` answers a bare `.git` from an ordinary repo's root. Left as it arrives it is
		// relative, and the location a caller gets depends on where they were standing.
		f.record("a run from a subdirectory resolves the same absolute location through git",
			fromSub == fromRoot && filepath.IsAbs(fromSub), "root: "+fromRoot+"\nsubdir: "+fromSub)
		f.record("and built nothing beside the caller", !f.exists(sub+"/.git"), sub)
	})

	t.Run("an absolute per-worktree git path is not prefixed onto the root", func(t *testing.T) {
		f := newShip(t, "001-fallback-markers")
		second := f.base + "/fallback-markers"
		f.mustGit("worktree", "add", "-q", second, "-b", "fallback-markers")
		if !f.exists(second) {
			t.Skip("git worktree add is unavailable here, so this case cannot be built")
		}
		// `--git-path` answers absolute in a linked worktree. Joined onto the root anyway it builds a
		// directory tree inside the checkout, and the marker the next command looks for is not there —
		// while every command still reports success. So the observation is what APPEARED in the
		// worktree, never the shape of the wrong path, which mirrors wherever the fixture happens to live.
		before := strings.Join(f.entries(second), "\n")
		f.runReportIn(second, "invalidate", "001-fallback-markers")
		f.runReportIn(second, "stage-returned", "code-review", "001-fallback-markers")
		f.record("a stage marker written from a linked worktree is recorded", f.status == 0, f.evidence())
		after := strings.Join(f.entries(second), "\n")
		f.record("and nothing new appeared inside the worktree", after == before,
			"before:\n"+before+"\nafter:\n"+after)
		f.runReportIn(second, "no-items", "code-review", "001-fallback-markers")
		f.record("and it reads back, so the stamp can see it", f.status == 0, f.evidence())
	})
}
