package density

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (r *repo) runBar(args ...string) {
	r.runBarIn(r.dir, baseConfig(), args...)
}

func (r *repo) runBarIn(cwd string, cfg Config, args ...string) {
	r.stdout.Reset()
	r.stderr.Reset()
	r.code = Run("comment-density.sh", append([]string{"--bar"}, args...), cwd, cfg, &r.stdout, &r.stderr)
}

func TestBarIsAModeOnlyAsTheFirstArgument(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.run("HEAD", "--bar")
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("'--bar' is an option, not a git-diff revision")
	r.expectNoStdout()
}

func newRepoWithLeanBaseline(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	lean := strings.Repeat("code()\n", 9) + "// one\n"
	r.write("a.go", lean)
	r.write("b.go", lean)
	r.commit("baseline")
	return r
}

func TestStatsOfEndsABlockAtABlankLine(t *testing.T) {
	counted := statsOf("// one\n\n// two\ncode()\n")
	if counted.blocks != 2 {
		t.Fatalf("a blank line between two comments left %d block(s), want 2", counted.blocks)
	}
	if counted.comments != 2 || counted.code != 1 {
		t.Fatalf("got %d comment / %d code, want 2 / 1", counted.comments, counted.code)
	}
}

func TestStatsOfEndsABlockAtCode(t *testing.T) {
	counted := statsOf("// one\ncode()\n// two\n")
	if counted.blocks != 2 {
		t.Fatalf("code between two comments left %d block(s), want 2", counted.blocks)
	}
}

func TestStatsOfCountsABlockEndingTheFile(t *testing.T) {
	counted := statsOf("code()\n// trailing")
	if counted.blocks != 1 || counted.comments != 1 {
		t.Fatalf("got %d block(s) and %d comment(s), want 1 and 1", counted.blocks, counted.comments)
	}
}

func TestStatsOfCountsDereferenceAsCode(t *testing.T) {
	counted := statsOf("*ptr = 1\n")
	if counted.code != 1 || counted.comments != 0 {
		t.Fatalf("got %d comment / %d code, want 0 / 1", counted.comments, counted.code)
	}
}

func TestStatsOfLongBlockBoundary(t *testing.T) {
	four := statsOf("// a\n// b\n// c\n// d\ncode()\n")
	if four.longBlocks != 0 {
		t.Fatalf("a 4-line block counted as long")
	}
	five := statsOf("// a\n// b\n// c\n// d\n// e\ncode()\n")
	if five.longBlocks != 1 {
		t.Fatalf("a 5-line block did not count as long")
	}
}

func TestRatioAndBlockMeansOfAnEmptyFileAreZero(t *testing.T) {
	var empty stats
	if empty.ratio() != 0 || empty.meanBlock() != 0 || empty.longShare() != 0 {
		t.Fatalf("an empty file answered non-zero: %v", empty)
	}
}

func TestCutToRatioIsZeroWhenAlreadyUnder(t *testing.T) {
	if cut := cutToRatio(stats{comments: 1, code: 99}, stats{comments: 9, code: 91}); cut != 0 {
		t.Fatalf("a set already under the bar was told to cut %d", cut)
	}
}

// 100 code lines at 9 comments per 91 code allow 9 comment lines, so a set holding 50 cuts 41. The wrong
// answer is 50 minus 9% of 150: the share of the total, where the bar wants the code that stays.
func TestCutToRatioLeavesTheAllowanceAgainstTheCodeThatStays(t *testing.T) {
	if cut := cutToRatio(stats{comments: 50, code: 100}, stats{comments: 9, code: 91}); cut != 41 {
		t.Fatalf("cut %d, want 41", cut)
	}
}

// At one comment per two code lines, 4 code lines allow exactly 2. Through ratio() as a float that is
// 1.999…, which truncates to 1 and asks for one cut more than the bar needs.
func TestCutToRatioUsesTheBaselinesOwnCounts(t *testing.T) {
	if cut := cutToRatio(stats{comments: 3, code: 4}, stats{comments: 1, code: 2}); cut != 1 {
		t.Fatalf("cut %d, want 1", cut)
	}
}

func TestCutToRatioCutsNothingAgainstABaselineWithoutCode(t *testing.T) {
	if cut := cutToRatio(stats{comments: 50, code: 1}, stats{comments: 5, code: 0}); cut != 0 {
		t.Fatalf("a baseline of only comments cut %d", cut)
	}
}

func TestPercentileOfNothingIsZero(t *testing.T) {
	if got := percentile(nil, 0.9); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestPercentileClampsToTheLastValue(t *testing.T) {
	if got := percentile([]float64{0.1, 0.2}, 1.0); got != 0.2 {
		t.Fatalf("got %v, want 0.2", got)
	}
}

func TestPercentileLeavesTheCallersOrderAlone(t *testing.T) {
	values := []float64{0.3, 0.1, 0.2}
	percentile(values, 0.5)
	if values[0] != 0.3 {
		t.Fatalf("the caller's slice was sorted under it: %v", values)
	}
}

func TestBaseRevisionNamesWhatTheDiffComparedAgainst(t *testing.T) {
	host := hostRepo{}
	cases := map[string][]string{
		"HEAD":   nil,
		"HEAD~0": {"HEAD~0"},
		"a":      {"a", "b"},
		"x":      {"x..y"},
	}
	for want, args := range cases {
		got, err := host.baseRevision(args)
		if err != nil || got != want {
			t.Errorf("baseRevision(%v) = %q, %v; want %q", args, got, err, want)
		}
	}
	if got, err := host.baseRevision([]string{"..y"}); err != nil || got != "HEAD" {
		t.Errorf("an empty left side answered %q, %v; want HEAD", got, err)
	}
}

func TestBaseRevisionOfASymmetricRangeIsTheMergeBase(t *testing.T) {
	host := hostRepo{root: newRepoWithLeanBaseline(t).dir}
	got, err := host.baseRevision([]string{"HEAD...HEAD"})
	if err != nil {
		t.Fatalf("merge base of HEAD with itself failed: %v", err)
	}
	if len(got) != 40 {
		t.Fatalf("expected a commit hash, got %q", got)
	}
	if _, err := host.baseRevision([]string{"HEAD...no-such-branch"}); err == nil {
		t.Fatalf("a range with no merge base was answered")
	}
	if got, err := host.baseRevision([]string{"...HEAD"}); err != nil || len(got) != 40 {
		t.Fatalf("an empty left side of a symmetric range answered %q, %v; want the merge base with HEAD", got, err)
	}
}

// RefuseNonRevisions tells a caller to put paths after `--`; read as a revision, `--` becomes the base
// every later listing fails on, and the bar exits 2 printing git's ls-tree usage.
func TestBarNarrowsToThePathspecAfterADoubleDash(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("pkg/heavy.go", "// a\n// b\n// c\ncode()\n")
	r.write("other.go", "// a\n// b\n// c\n// d\ncode()\n")

	r.runBar("--", "pkg")
	r.expectCode(exitFound)
	r.expectStderrLacks("usage: git")
	r.expectStdoutHas("(3 comment / 1 code)")
	r.expectStdoutHas("pkg/heavy.go: 75% against a 10% ceiling")
	r.expectStdoutLacks("other.go")
	r.expectStdoutHas("(2 file(s) in the baseline)")
}

func TestBarPathspecIsRelativeToWhereTheCallerRan(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("pkg/heavy.go", "// a\n// b\n// c\ncode()\n")
	r.write("other.go", "// a\n// b\n// c\n// d\ncode()\n")

	r.runBar("--", "pkg/heavy.go")
	r.expectCode(exitFound)
	fromRoot := r.stdout.String()

	r.runBarIn(filepath.Join(r.dir, "pkg"), baseConfig(), "--", "heavy.go")
	r.expectCode(exitFound)
	r.expectStdoutLacks("other.go")
	if fromSubdir := r.stdout.String(); fromSubdir != fromRoot {
		t.Fatalf("a pathspec from a subdirectory reported differently:\nroot:\n%s\nsubdirectory:\n%s", fromRoot, fromSubdir)
	}
}

func TestBarPathspecWithRevisionsKeepsTheirBase(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("pkg/d.go", "// a\n// b\n// c\n"+strings.Repeat("code()\n", 8))
	r.write("e.go", "// a\n"+strings.Repeat("code()\n", 9))
	r.commit("d and e land")

	r.runBar("HEAD~1", "HEAD", "--", "pkg")
	r.expectCode(exitFound)
	r.expectStdoutHas("pkg/d.go: 27% against a 10% ceiling")
	r.expectStdoutHas("(3 comment / 8 code)")
	r.expectStdoutHas("(3 file(s) in the baseline)")
}

func TestBarRefusesOutsideARepository(t *testing.T) {
	r := newRepo(t)
	r.runBarIn(t.TempDir(), baseConfig())
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("not inside a git repository")
	r.expectNoStdout()
}

func TestBarInARepositoryWithNoCommitNamesThat(t *testing.T) {
	r := newRepo(t)
	unborn := t.TempDir()
	if err := git(unborn, "init", "-q"); err != nil {
		t.Fatalf("could not init the unborn repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unborn, "heavy.go"), []byte("// a\n// b\n// c\ncode()\n"), 0o644); err != nil {
		t.Fatalf("could not write the fixture: %v", err)
	}

	r.runBarIn(unborn, baseConfig())
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("this repository has no commit yet")
	r.expectStderrLacks("rejected these arguments")
	r.expectNoStdout()
}

func TestBarCarriesGitsOwnAccountOfABadRevision(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.runBar("no-such-revision")
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("git rejected these arguments")
	r.expectStderrHas("git said: fatal: ")
	r.expectStderrHas("no-such-revision")
	r.expectNoStdout()
}

func TestBarRefusesWithNoBaseline(t *testing.T) {
	r := newRepo(t)
	r.write("only.go", "// a\n// b\n// c\ncode()\n")
	r.runBar()
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("no rate to hold it to")
}

func TestBarIsCleanWhenTheChangeSetHoldsNoSource(t *testing.T) {
	r := newRepo(t)
	r.write("notes.md", "# notes\n")
	r.runBar()
	r.expectCode(exitClean)
	r.expectStderrHas("no source file in this change set")
}

// A set exactly at the baseline's rate is not over it: `>`, not `>=`, on both the per-file ceiling
// and the line allowance.
func TestBarIsCleanAtTheBaselinesOwnRate(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("same.go", strings.Repeat("code()\n", 9)+"// one\n")

	r.runBar()
	r.expectCode(exitClean)
	r.expectStdoutHas("host repo: 10.0% comment lines")
	r.expectStdoutHas("change set: 10.0% comment lines (1 comment / 9 code)")
	r.expectStdoutLacks("over on")
	r.expectStdoutLacks("against a")
	r.expectStderrHas("1 changed source file(s), 1 read, 0 skipped unread; 2 file(s) in the baseline.")
}

func TestBarReportsTheOverageAndRepeats(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("heavy.go", "// a\n// b\n// c\n// d\n// e\n// f\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("over on lines: cut ")
	first := r.stdout.String()

	r.runBar()
	if second := r.stdout.String(); second != first {
		t.Fatalf("a second run over the same tree reported differently:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestBarReportsLongBlocks(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("wall.go", "// a\n// b\n// c\n// d\n// e\n"+strings.Repeat("code()\n", 60))

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("over on blocks: 1 block(s) over 4 lines against 0 allowed")
	r.expectStdoutLacks("over on lines")
}

// lib/new.go is untracked and outside the subdirectory the second run starts in: `git ls-files` with no
// pathspec lists only under the directory it runs in, and the change set would lose the file there.
func TestBarRunFromASubdirectoryMatchesTheRoot(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("a.go", strings.Repeat("// more\n", 6)+"code()\n")
	r.write("pkg/heavy.go", "// a\n// b\n// c\ncode()\n")
	r.write("lib/new.go", "// x\n// y\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	fromRoot := r.stdout.String()
	r.expectStdoutHas("(2 file(s) in the baseline)")
	r.expectStdoutHas("(11 comment / 3 code)")

	r.runBarIn(filepath.Join(r.dir, "pkg"), baseConfig())
	r.expectCode(exitFound)
	r.expectStdoutHas("lib/new.go: 67% against a 10% ceiling")
	if fromSubdir := r.stdout.String(); fromSubdir != fromRoot {
		t.Fatalf("run from a subdirectory reported differently:\nroot:\n%s\nsubdirectory:\n%s", fromRoot, fromSubdir)
	}
}

func TestBarKeepsAFileTheChangeOnlyDeletedFrom(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("heavy.go", "// a\n// b\n// c\n// d\n// e\n// f\ncode()\n")
	r.commit("heavy lands")
	r.write("heavy.go", "// a\n// b\n// c\n// d\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("(3 file(s) in the baseline)")
	r.expectStdoutHas("(4 comment / 1 code)")
	// The file it cut comments from is no longer removed from the rate it is judged against.
	r.expectStdoutHas("29.6% comment lines")
}

func TestBarHoldsOnlyNewFilesToThePerFileCeiling(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("a.go", strings.Repeat("// more\n", 6)+"code()\n")
	r.write("heavy.go", "// a\n// b\n// c\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("heavy.go: 75% against a 10% ceiling")
	r.expectStdoutLacks("a.go: 86% against")
}

func TestBarWithRevisionsJudgesOnlyThatDiff(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("d.go", "// a\n// b\n// c\n"+strings.Repeat("code()\n", 8))
	r.commit("d lands")
	r.write("untracked.go", "// a\n// b\n// c\ncode()\n")

	for _, args := range [][]string{{"HEAD~1", "HEAD"}, {"HEAD~1..HEAD"}, {"HEAD~1...HEAD"}} {
		r.runBar(args...)
		r.expectCode(exitFound)
		r.expectStdoutHas("(2 file(s) in the baseline)")
		r.expectStdoutHas("d.go: 27% against a 10% ceiling")
		r.expectStdoutLacks("untracked.go")
	}
}

func TestBarPrintsItsDenominatorOnStderr(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("heavy.go", "// a\n// b\n// c\ncode()\n")
	r.write("big.go", strings.Repeat("code()\n", 20))
	cfg := baseConfig()
	cfg.MaxFileBytes = 100

	r.runBarIn(r.dir, cfg)
	r.expectCode(exitFound)
	r.expectStderrHas("2 changed source file(s), 1 read, 1 skipped unread; 2 file(s) in the baseline.")
}

func TestBarShowsAtMostMaxShownFilesOverTheCeiling(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	for i := 0; i <= maxShown; i++ {
		r.write(fmt.Sprintf("heavy%03d.go", i), "// a\n// b\n// c\ncode()\n")
	}

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("… and 1 further file(s) over the ceiling, not shown")
	if shown := strings.Count(r.stdout.String(), "against a "); shown != maxShown {
		t.Fatalf("%d per-file lines printed, want exactly %d", shown, maxShown)
	}
}

func TestBarSkipsASymlinkRatherThanFollowingIt(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("// a\n// b\n// c\ncode()\n"), 0o644); err != nil {
		t.Fatalf("could not write the target outside the repo: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(r.dir, "link.go")); err != nil {
		t.Fatalf("could not plant the symlink: %v", err)
	}

	r.runBar()
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("no changed source file could be read")
	r.expectNoStdout()
}

// A NUL byte marks the file binary. heavy.go keeps the run measured, so the skip shows as a count on
// stderr instead of a refusal.
func TestBarSkipsABinaryFileUnread(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("blob.go", "// a\n// b\n// c\x00\ncode()\n")
	r.write("heavy.go", "// a\n// b\n// c\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	r.expectStderrHas("2 changed source file(s), 1 read, 1 skipped unread;")
	r.expectStdoutHas("heavy.go: 75% against a 10% ceiling")
	r.expectStdoutLacks("blob.go")
}

// `git diff HEAD` is "ambiguous" once a file named HEAD sits in the working tree, and the branch under
// review can commit one; the listings end in `--` whether a pathspec follows or not, so the bar runs.
func TestBarRunsWithAFileNamedHEADInTheTree(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("HEAD", "// a\n// b\n// c\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	r.expectStderrLacks("ambiguous argument")
	r.expectStdoutHas("HEAD: 75% against a 10% ceiling")
}

// A baseline of only code has no comment block, so its long-block rate has no denominator.
func TestBarHoldsBlocksAgainstABaselineWithoutAny(t *testing.T) {
	r := newRepo(t)
	r.write("a.go", strings.Repeat("code()\n", 10))
	r.write("b.go", strings.Repeat("code()\n", 10))
	r.commit("code only")
	r.write("wall.go", "// a\n// b\n// c\n// d\n// e\n"+strings.Repeat("code()\n", 5))

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("host repo: 0.0% comment lines, 0.0-line mean block, 0% of blocks over 4 lines")
	r.expectStdoutHas("over on lines: cut 5 comment line(s) to reach 0.0%")
	r.expectStdoutHas("over on blocks: 1 block(s) over 4 lines against 0 allowed")
}

func TestBarRefusesAPathWhereARevisionBelongs(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.runBar("a.go")
	r.expectCode(exitDidNotRun)
	r.expectStderrHas("'a.go' is a path, not a git-diff revision")
	r.expectNoStdout()
}

// Touching a comment-heavy file used to remove it from the baseline AND add it to the numerator, so one
// edit both loosened what the change was measured against and raised what it measured. The file's
// pre-change content is the repo's, so the baseline keeps it at the content it had before this change.
func TestATouchedFileStaysInTheBaselineAtItsOldContent(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("legacy.go", heavy(40, 10))
	r.commit("legacy arrives")
	r.write("legacy.go", heavy(40, 11))
	r.runBar()
	// Two lean files plus the legacy file the change touched: three, not two.
	r.expectStdoutHas("3 file(s) in the baseline")
	// A file new to this change cannot grade itself, so it stays out.
	r.write("fresh.go", heavy(9, 1))
	r.runBar()
	r.expectStdoutHas("3 file(s) in the baseline")
}

// The overage counts a changed file whole, so a file the change inherited can carry most of it. Without
// the attribution the total reads as a debt the change ran up, and the reader who cannot see the
// composition goes looking for a friendlier denominator instead — which is how this defect recurred.
func TestTheOverageNamesTheFilesCarryingIt(t *testing.T) {
	t.Run("an inherited file carrying the mass is named and marked carried", func(t *testing.T) {
		r := newRepoWithLeanBaseline(t)
		// A baseline wide enough that the legacy file's own pre-change mass does not set the rate.
		for i := 0; i < 20; i++ {
			r.write(fmt.Sprintf("lean%d.go", i), strings.Repeat("code()\n", 50))
		}
		r.write("legacy.go", heavy(60, 20))
		r.commit("legacy arrives")
		r.write("legacy.go", heavy(60, 21))
		r.write("mine.go", heavy(3, 20))
		r.runBar()
		r.expectCode(exitFound)
		r.expectStdoutHas("over on lines:")
		r.expectStdoutHas("legacy.go: 60 comment line(s), 0% written here")
		r.expectStdoutLacks("mine.go: 3 comment line(s)")
	})

	t.Run("a file the change wrote is named without the carried mark", func(t *testing.T) {
		r := newRepoWithLeanBaseline(t)
		for i := 0; i < 20; i++ {
			r.write(fmt.Sprintf("lean%d.go", i), strings.Repeat("code()\n", 50))
		}
		r.commit("lean baseline")
		r.write("fresh.go", heavy(60, 20))
		r.runBar()
		r.expectCode(exitFound)
		r.expectStdoutHas("fresh.go: 60 comment line(s), 100% written here")
	})

	t.Run("a change set under the bar names nobody", func(t *testing.T) {
		r := newRepoWithLeanBaseline(t)
		r.write("lean.go", strings.Repeat("code()\n", 40))
		r.runBar()
		r.expectStdoutLacks("comment line(s),")
	})
}

// The overage counts changed files whole, and carried mass is reported rather than charged — so the
// overage alone is not what the change owes, and on a change that brushes a comment-heavy file it can
// exceed the whole chargeable total. A reader acting on the headline would cut a change already under
// the bar, so the chargeable figure is printed beside it.
func TestTheReportSeparatesWhatIsChargeableFromTheOverage(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	for i := 0; i < 20; i++ {
		r.write(fmt.Sprintf("lean%d.go", i), strings.Repeat("code()\n", 50))
	}
	r.write("legacy.go", heavy(60, 20))
	r.commit("legacy arrives")

	// Brush the legacy file, and write a small lean file of this change's own.
	r.write("legacy.go", heavy(60, 21))
	r.write("mine.go", strings.Repeat("code()\n", 30))
	r.runBar()

	r.expectStdoutHas("over on lines:")
	r.expectStdoutHas("legacy.go: 60 comment line(s)")
	// Everything this change actually wrote is lean, so nothing it wrote is chargeable.
	r.expectStdoutHas("nothing chargeable")
}

// Whether the change created the file is a crude stand-in for whether the change wrote its comments: a
// comment-only rewrite of an existing file reads as inherited and is charged nothing, which is the whole
// mass the bar exists to catch. Authorship is measured over the population being graded — the file's own
// comment lines — and printed as a fraction, so there is no threshold for a later pass to clear.
func TestTheReportMeasuresCommentAuthorshipPerFile(t *testing.T) {
	t.Run("a comment-only rewrite reads as this change's own", func(t *testing.T) {
		r := newRepoWithLeanBaseline(t)
		for i := 0; i < 20; i++ {
			r.write(fmt.Sprintf("lean%d.go", i), strings.Repeat("code()\n", 50))
		}
		r.write("rewritten.go", "// old one\n// old two\n"+strings.Repeat("code()\n", 8))
		r.commit("the file arrives")
		// Same code, every comment line replaced: the change wrote all of its comment mass.
		r.write("rewritten.go", heavy(40, 0)+strings.Repeat("code()\n", 8))
		r.runBar()
		r.expectStdoutHas("rewritten.go: 40 comment line(s), 100% written here")
	})

	t.Run("a file brushed without touching its comments reads as inherited", func(t *testing.T) {
		r := newRepoWithLeanBaseline(t)
		for i := 0; i < 20; i++ {
			r.write(fmt.Sprintf("lean%d.go", i), strings.Repeat("code()\n", 50))
		}
		r.write("legacy.go", heavy(60, 20))
		r.commit("legacy arrives")
		r.write("legacy.go", heavy(60, 21))
		r.runBar()
		r.expectStdoutHas("legacy.go: 60 comment line(s), 0% written here")
	})
}

// Every report names the build that produced it, so two readings taken apart can be told apart. The
// stub exports the stamp; run directly, as here, nothing does.
func TestBarNamesTheBuildThatMeasured(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("same.go", strings.Repeat("code()\n", 9)+"// one\n")
	t.Setenv("ECO_TOOL_BUILD", "deadbeefcafe")

	r.runBar()
	r.expectStdoutHas("measured by: comment-density build deadbeefcafe")
}

// An identity nobody stamped is reported, never omitted. A line that disappears when the build is
// unknown leaves its absence meaning two things — no stamp, or an older binary that never printed one
// — and the reader cannot tell which.
func TestBarNamesAnUnknownBuildRatherThanOmittingIt(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("same.go", strings.Repeat("code()\n", 9)+"// one\n")
	t.Setenv("ECO_TOOL_BUILD", "")

	r.runBar()
	r.expectStdoutHas("measured by: comment-density build unknown")
}

// Under the bar the attribution half prints nothing at all, so the identity must not ride on it: an
// absent line would read as an older binary to anyone comparing an under-bar log with an over-bar one.
func TestBarNamesTheBuildEvenWhenUnderTheBar(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("same.go", strings.Repeat("code()\n", 9)+"// one\n")
	t.Setenv("ECO_TOOL_BUILD", "underbar99")

	r.runBar()
	r.expectCode(exitClean)
	r.expectStdoutLacks("chargeable")
	r.expectStdoutHas("measured by: comment-density build underbar99")
}

// A closed range asks what its right-hand side holds, so content comes from there and not from a tree
// that has since moved. Reading the tree measures today's files under yesterday's file list and answers
// with a plausible number rather than an error, which is the harder failure to notice.
func TestBarReadsAClosedRangeAtItsOwnRevision(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("heavy.go", "// a\n// b\n// c\n// d\n// e\n// f\ncode()\n")
	r.commit("the commit this range names")
	// The tree moves on past the range, leaving the same file lean and uncommitted.
	r.write("heavy.go", strings.Repeat("code()\n", 9)+"// one\n")

	r.runBar("HEAD~1..HEAD")
	r.expectCode(exitFound)
	r.expectStdoutHas("over on lines: cut ")
}

// The other half of the same rule: a single revision means "since then", and what it measures is work
// the tree still holds. Reading a commit there would drop the uncommitted lines being asked about.
func TestBarReadsASingleRevisionFromTheWorkingTree(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("heavy.go", strings.Repeat("code()\n", 9)+"// one\n")
	r.commit("a lean file to modify")
	// Uncommitted, so only the tree holds it — which is exactly what "since HEAD" is asking about.
	r.write("heavy.go", "// a\n// b\n// c\n// d\n// e\n// f\ncode()\n")

	r.runBar("HEAD")
	r.expectCode(exitFound)
	r.expectStdoutHas("over on lines: cut ")
}

// The baseline's file list and its content have to name one revision. Taken from today's index it names
// files the revision never held — and those read as unreadable and leave without a word, so a count
// alone cannot catch it. The case that can is a file the revision HELD and the tree has since deleted:
// the revision's listing counts it, the index's does not.
func TestBarTakesItsBaselineListFromTheContentRevision(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("gone.go", strings.Repeat("code()\n", 9)+"// one\n")
	r.commit("a file the range still had")
	r.write("heavy.go", "// a\n// b\n// c\n// d\n// e\n// f\ncode()\n")
	r.commit("the commit this range names")
	if err := os.Remove(filepath.Join(r.dir, "gone.go")); err != nil {
		t.Fatalf("could not delete the fixture: %v", err)
	}
	r.commit("later work deletes it")

	r.runBar("HEAD~2..HEAD~1")
	r.expectStdoutHas("3 file(s) in the baseline")
}

func TestContentRevisionPinsOnlyClosedRanges(t *testing.T) {
	for _, row := range []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"HEAD"}, ""},
		{[]string{"origin/main"}, ""},
		{[]string{"a..b"}, "b"},
		{[]string{"a...b"}, "b"},
		{[]string{"a.."}, "HEAD"},
		{[]string{"a..."}, "HEAD"},
		{[]string{"a", "b"}, "b"},
		{[]string{"merge", "p1", "p2"}, ""},
	} {
		if got := contentRevision(row.args); got != row.want {
			t.Errorf("contentRevision(%q) = %q, want %q", row.args, got, row.want)
		}
	}
}

// The report names the checkout as well as the binary. They are different facts: a stamp hashes source,
// so a tree can hash identically to its own source and still be a commit nobody else has — which is what
// makes two readings taken apart incomparable when the mount moved between them.
func TestBarNamesTheTreeItMeasuredOn(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("same.go", strings.Repeat("code()\n", 9)+"// one\n")
	t.Setenv("ECO_TOOL_BUILD", "deadbeefcafe")
	t.Setenv("ECO_TOOL_TREE", "feedfacedead")

	r.runBar()
	r.expectStdoutHas("measured by: comment-density build deadbeefcafe, tree feedfacedead")
}

// An unnamed checkout is reported, never omitted — the same reason the build is. A line that drops the
// tree when nothing named it leaves its absence meaning either no git or an older binary.
func TestBarNamesAnUnknownTreeRatherThanOmittingIt(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("same.go", strings.Repeat("code()\n", 9)+"// one\n")
	t.Setenv("ECO_TOOL_BUILD", "deadbeefcafe")
	t.Setenv("ECO_TOOL_TREE", "")

	r.runBar()
	r.expectStdoutHas("build deadbeefcafe, tree unknown")
}

// The bar and the voice check both enforce "a block is at most four lines", so they have to mean the
// same thing by it. They did not: the bar counted every comment line, the voice check counted prose
// lines, and one tool reported a block long and clean at the same time.
func TestTheBarAndTheVoiceCheckAgreeOnBlockLength(t *testing.T) {
	// One summary sentence over a five-line doc-tag list: eight comment lines, two prose lines.
	tagged := "/**\n * Returns the book's total.\n * @param book the book\n * @param currency the currency\n" +
		" * @returns the total\n * @throws when two currencies are declared\n */\nfunc f() {}\n"
	if counted := statsOf(tagged); counted.longBlocks != 0 {
		t.Errorf("the bar reported a one-sentence summary over a tag list as %d long block(s)", counted.longBlocks)
	}
	lines := strings.Split(strings.TrimSuffix(tagged, "\n"), "\n")
	if found := (scanner{profile: ProfileComment}).scanSource("f.go", lines, nil); hasCheck(found, checkLongBlock) {
		t.Error("the voice check reported the same block long, so the two disagree")
	}

	prose := "const a = 1;\n/**\n * One.\n * Two.\n * Three.\n * Four.\n * Five.\n */\nfunc f() {}\n"
	if counted := statsOf(prose); counted.longBlocks != 1 {
		t.Errorf("the bar reported %d long block(s) over five prose lines, want 1", counted.longBlocks)
	}
	if found := (scanner{profile: ProfileComment}).scanSource("f.go", strings.Split(strings.TrimSuffix(prose, "\n"), "\n"), nil); !hasCheck(found, checkLongBlock) {
		t.Error("the voice check did not report five prose lines long, so the two disagree")
	}
}
