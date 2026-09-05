package density

// What reaches the scanner rather than what it does with a line once it has one: the arguments
// it refuses, the diff text it parses, the untracked arm, and the path shapes that have hidden a
// file from a scan.

import (
	"strings"
	"testing"

	"kk-flavor/tools/diffscan"
	"kk-flavor/tools/shell"
)

// --- the arguments it refuses --------------------------------------------------------------------

func TestARevisionIsNotAPath(t *testing.T) {
	t.Run("a directory path exits 2 and is named as a path", func(t *testing.T) {
		r := newRepo(t)
		r.write("subdir/file.go", "x\n")
		r.commit("add subdir")
		r.run("subdir")
		r.expectCode(2)
		r.expectStderrHas("is a path, not a git-diff revision")
		r.expectStderrHas("the scan did NOT run")
		// Not the message a scan git rejected prints — two different failures must not read alike.
		r.expectStderrLacks("git rejected these arguments")
		r.expectNoStdout()
	})

	t.Run("a file path exits 2 and the refusal names the file", func(t *testing.T) {
		r := newRepo(t)
		r.write("seen.go", "x\n")
		r.commit("add seen")
		r.run("seen.go")
		r.expectCode(2)
		r.expectStderrHas("'seen.go' is a path")
	})

	t.Run("an option exits 2 and is named as an option", func(t *testing.T) {
		r := newRepo(t)
		r.run("--output=/dev/null")
		r.expectCode(2)
		r.expectStderrHas("is an option, not a git-diff revision")
		r.expectNoStdout()
	})

	t.Run("a revision git cannot resolve exits 2 as git's rejection", func(t *testing.T) {
		r := newRepo(t)
		r.run("no-such-rev")
		r.expectCode(2)
		r.expectStderrHas("git rejected these arguments")
		r.expectStderrHas("Not a clean result")
		// And not the path refusal: the argument was never a path.
		r.expectStderrLacks("is a path, not a git-diff revision")
		r.expectNoStdout()
	})

	// Both a real revision and a real filename. git itself refuses the ambiguity, and this must arrive
	// as git's rejection rather than as a path refusal that never consulted git.
	t.Run("an argument that is both a revision and a filename exits 2 as git's rejection", func(t *testing.T) {
		r := newRepo(t)
		r.write("HEAD", "ambiguous\n")
		r.commit("add a file called HEAD")
		r.run("HEAD")
		r.expectCode(2)
		r.expectStderrHas("git rejected these arguments")
	})

	t.Run("a pathspec after -- is scanned rather than refused", func(t *testing.T) {
		r := newRepo(t)
		r.write("kept.go", "x := 1\n")
		r.commit("base")
		r.write("kept.go", heavy(6, 1))
		r.run("HEAD", "--", "kept.go")
		r.expectCode(1)
		r.expectStdoutHas("kept.go")
	})

	t.Run("a pathspec after -- that selects nothing exits 0", func(t *testing.T) {
		r := newRepo(t)
		r.write("kept.go", "x := 1\n")
		r.commit("base")
		r.write("kept.go", heavy(6, 1))
		r.run("HEAD", "--", "no-such-path")
		r.expectCode(0)
		r.expectNoStdout()
	})
}

// --- the diff shape it parses ---------------------------------------------------------------------

// `diff --git` is the anchor, never `+++` alone: an added line reading `+++ b/other.go` would reassign
// the file and every added line after it would be counted against a file that is not in the change.
func TestAnAddedLineShapedLikeADiffHeader(t *testing.T) {
	r := newRepo(t)
	r.write("real.go", "package fixture\n")
	r.commit("base")
	// TWO plus signs, not three: the diff prefixes every added line with one, so a source line of
	// `++ b/decoy.go` is what arrives as `+++ b/decoy.go` and can be mistaken for a real file header.
	// Written with three, the line arrives as `++++ ` and matches nothing — a fixture that exercises
	// the anchor is the only one that can fail when the anchor is removed.
	r.write("real.go", "++ b/decoy.go\n"+heavy(8, 1))
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("real.go")
	r.expectStdoutLacks("decoy.go")
}

// core.quotePath=false, or the path arrives C-quoted and the `b/` test fails, dropping the file.
func TestANonASCIIPathIsStillAssigned(t *testing.T) {
	r := newRepo(t)
	r.write("café.go", "package fixture\n")
	r.commit("base")
	r.write("café.go", heavy(8, 1))
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("café.go")
}

// --text, or one `* -diff` in the branch author's .gitattributes collapses the body to
// "Binary files … differ" and the scan exits 0 over a real outlier.
func TestADiffAttributeDoesNotSuppressTheScan(t *testing.T) {
	r := newRepo(t)
	r.write("attr.go", "package fixture\n")
	r.write(".gitattributes", "* -diff\n")
	r.commit("base")
	r.write("attr.go", heavy(8, 1))
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("attr.go")
}

// --- the untracked arm -----------------------------------------------------------------------------

func TestUntrackedFiles(t *testing.T) {
	t.Run("scanned when no revision is given, not when one is", func(t *testing.T) {
		r := newRepo(t)
		r.write("fresh.go", heavy(8, 1))
		r.run()
		r.expectCode(1)
		r.expectStdoutHas("fresh.go")

		// With revisions the caller named two commits, and a file in neither is not what they asked
		// about.
		r.run("HEAD")
		r.expectCode(0)
		r.expectStdoutLacks("fresh.go")
	})

	t.Run("one over the byte cap is skipped, and the skip is counted rather than silent", func(t *testing.T) {
		r := newRepo(t)
		r.write("big.go", heavy(400, 1))
		cfg := baseConfig()
		cfg.MaxFileBytes = 32
		r.runWith(cfg)
		r.expectCode(0)
		r.expectStdoutLacks("big.go")
		r.expectStderrHas("1 untracked file(s) skipped unread")
	})

	t.Run("a binary one is skipped and reaches the denominator", func(t *testing.T) {
		r := newRepo(t)
		r.write("blob.bin", "// comment\x00"+heavy(8, 0))
		r.run()
		r.expectCode(0)
		r.expectStdoutLacks("blob.bin")
		r.expectStderrHas("1 untracked file(s) skipped unread")
	})

	// Two files are two files. In the shell these shared one text stream, so a missing final newline on
	// the first fused its last line to the second's header and neither was counted as itself.
	t.Run("two are counted apart when the first has no final newline", func(t *testing.T) {
		r := newRepo(t)
		r.write("first.go", strings.TrimSuffix(heavy(8, 1), "\n"))
		r.write("second.go", heavy(9, 1))
		r.run()
		r.expectCode(1)
		r.expectStdoutHas("first.go: 8 comment / 1 code")
		r.expectStdoutHas("second.go: 9 comment / 1 code")
	})
}

// A newline in a path corrupts nothing: the file is opened by name and its lines never become diff
// text, so it is scanned like any other.
//
// It is still one report line per outlier. The name reaches the report through shell.Oneline, so the
// newline arrives as a space: a report where the count of lines and the count of outliers disagree is
// unreadable to the caller ranking them, and the second half of a split name reads as a record of its
// own.
func TestANewlineInAPathIsNoLongerAHazard(t *testing.T) {
	r := newRepo(t)
	name := "odd\nname.go"
	r.write(name, heavy(8, 1))
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("odd name.go: 8 comment / 1 code")
	r.expectStdoutLacks("odd\nname.go")
	if lines := strings.Count(strings.TrimRight(r.stdout.String(), "\n"), "\n") + 1; lines != 1 {
		t.Errorf("the report is %d lines over one outlier, wanted 1: %q", lines, r.stdout.String())
	}
	r.expectStderrHas("1 file(s) reached the scan")
	r.expectStderrHas("0 untracked file(s) skipped unread")
}

// A path long enough to be cut says it was cut. Unmarked, a name truncated at the bound is a shorter
// different name, and a caller grepping the report for the file it changed finds nothing and reads
// that as "not an outlier".
func TestAnOverlongPathIsCutAndSaysSo(t *testing.T) {
	r := newRepo(t)
	name := strings.Repeat("d", maxPathBytes) + "/over.go"
	r.write(name, heavy(8, 1))
	r.run()
	r.expectCode(1)
	r.expectStdoutHas(shell.CutMarker + ": 8 comment / 1 code")
	r.expectStdoutLacks("over.go")
	for _, line := range strings.Split(strings.TrimRight(r.stdout.String(), "\n"), "\n") {
		reported, _, _ := strings.Cut(line, ": ")
		if len(reported) > maxPathBytes {
			t.Errorf("the reported path is %d bytes, over the %d-byte bound", len(reported), maxPathBytes)
		}
	}
}

// git C-quotes a path holding a control character even under core.quotePath=false, so the `b/` test
// misses the header and the file is never assigned. Unassigned, every added line in it is dropped —
// while `diff --git` has already counted the file as reached, so the run reports a denominator it did
// not cover and a name nobody can read becomes a way to hide a file from the scan.
func TestATrackedPathWithAControlCharacterIsStillAssigned(t *testing.T) {
	r := newRepo(t)
	name := "tab\there.go"
	r.write(name, "package fixture\n")
	r.commit("base")
	r.write(name, heavy(8, 1))
	r.run("HEAD")
	r.expectCode(1)
	// Through Oneline on the way out, like every other path in the report.
	r.expectStdoutHas("tab here.go: 8 comment / 1 code")
	r.expectStderrHas("1 file(s) reached the scan, 1 with countable added lines")
}

// A diff line past the cap ends the read where it stands, and every file after it in the diff goes
// unscanned. Report that as a clean 0 and the run has covered part of a change set and answered for
// all of it, so it refuses instead. The cap is dropped to the scanner's own starting buffer for the
// case; at the real 16MB the fixture would have to be 16MB.
func TestADiffLinePastTheCapRefusesRatherThanReportingClean(t *testing.T) {
	r := newRepo(t)
	r.write("a.go", "package fixture\n")
	r.write("z.go", "package fixture\n")
	r.commit("base")
	// a.go sorts first, so the long line lands ahead of the outlier and hides it.
	r.write("a.go", strings.Repeat("x", 70000)+"\n")
	r.write("z.go", heavy(8, 1))

	realCap := diffscan.MaxDiffLineBytes
	diffscan.MaxDiffLineBytes = 64 * 1024
	r.run("HEAD")
	diffscan.MaxDiffLineBytes = realCap
	r.expectCode(2)
	r.expectStderrHas("the scan did NOT run over all of it")
	r.expectStderrHas("Not a clean result")
	r.expectNoStdout()

	// The negative control for the assertions above: the same tree under the real cap reaches the
	// outlier the long line was hiding, so the refusal was the cap firing and not a clean fixture.
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("z.go")
}
