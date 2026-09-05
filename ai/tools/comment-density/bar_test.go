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

// Goes through Run with `--bar` in front, the way the command arrives, so every case here also covers
// the dispatch.
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

// newRepoWithLeanBaseline commits two files at one comment line in ten, so a comment-heavy file a case
// writes afterwards is over every bar the mode holds.
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
	// git reads `...b` as `HEAD...b`, and the diff over it succeeds before baseRevision is ever asked.
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
	r.expectStdoutHas("(2 file(s) outside this change)")
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
	r.expectStdoutHas("(3 file(s) outside this change)")
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
	r.expectStderrHas("1 changed source file(s), 1 read, 0 skipped unread; 2 file(s) outside the change in the baseline.")
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
	r.expectStdoutHas("(1 file(s) outside this change)")
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
	r.expectStdoutHas("(2 file(s) outside this change)")
	r.expectStdoutHas("(4 comment / 1 code)")
}

func TestBarHoldsOnlyNewFilesToThePerFileCeiling(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("a.go", strings.Repeat("// more\n", 6)+"code()\n")
	r.write("heavy.go", "// a\n// b\n// c\ncode()\n")

	r.runBar()
	r.expectCode(exitFound)
	r.expectStdoutHas("heavy.go: 75% against a 10% ceiling")
	r.expectStdoutLacks("a.go:")
}

func TestBarWithRevisionsJudgesOnlyThatDiff(t *testing.T) {
	r := newRepoWithLeanBaseline(t)
	r.write("d.go", "// a\n// b\n// c\n"+strings.Repeat("code()\n", 8))
	r.commit("d lands")
	r.write("untracked.go", "// a\n// b\n// c\ncode()\n")

	for _, args := range [][]string{{"HEAD~1", "HEAD"}, {"HEAD~1..HEAD"}, {"HEAD~1...HEAD"}} {
		r.runBar(args...)
		r.expectCode(exitFound)
		r.expectStdoutHas("(2 file(s) outside this change)")
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
	r.expectStderrHas("2 changed source file(s), 1 read, 1 skipped unread; 2 file(s) outside the change in the baseline.")
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
