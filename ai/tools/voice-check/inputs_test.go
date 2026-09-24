package voicecheck

import (
	"errors"
	"os"
	"strings"
	"testing"

	"configs/ai/tools/diffscan"
	"configs/ai/tools/shell"
)

func TestARevisionIsNotAPath(t *testing.T) {
	t.Run("a directory path exits 2 and is named as a path", func(t *testing.T) {
		r := newRepo(t)
		r.write("subdir/file.go", "x\n")
		r.commit("add subdir")
		r.run("subdir")
		r.expectCode(2)
		r.expectStderrHas("'subdir' is a path, not a git-diff revision")
		r.expectStderrHas("the scan did NOT run")
		// The grammar goes with an argument refusal, so the caller is told what this tool does take.
		r.expectStderrHas(usage)
		// Not the message a scan git rejected prints — two different failures must not read alike.
		r.expectStderrLacks("git rejected these arguments")
		r.expectNoStdout()
	})

	t.Run("an option exits 2 and is named as an option", func(t *testing.T) {
		r := newRepo(t)
		r.run("--output=/dev/null")
		r.expectCode(2)
		r.expectStderrHas("is an option, not a git-diff revision")
		r.expectStderrHas(usage)
		r.expectNoStdout()
	})

	// `--bar` reaches the same gate through its own arm, so an option behind it must be refused the
	// same way and with the same grammar.
	t.Run("an option after --bar exits 2 with the grammar", func(t *testing.T) {
		r := newRepo(t)
		r.run("--density", "--output=/dev/null")
		r.expectCode(2)
		r.expectStderrHas("is an option, not a git-diff revision")
		r.expectStderrHas(usage)
		r.expectNoStdout()
	})

	t.Run("a revision git cannot resolve exits 2 as git's rejection", func(t *testing.T) {
		r := newRepo(t)
		// What git answers for a name it holds no object under. The answer is stated here, because a diff
		// git refuses is the only way a caller ever hears about an unresolvable revision.
		r.fake.Fail["Patch"] = errors.New("git diff no-such-rev --: fatal: bad revision 'no-such-rev'")
		r.run("no-such-rev")
		r.expectCode(2)
		r.expectStderrHas("git rejected these arguments")
		r.expectStderrHas("Not a clean result")
		r.expectStderrLacks("is a path, not a git-diff revision")
		// A revision this repository does not carry is a sound invocation. Answering it with the grammar
		// would send the caller to fix an argument that was already the right shape.
		r.expectStderrLacks(usage)
		r.expectNoStdout()
	})

	// Both a real revision and a real filename. It is read as the revision and the scan runs. Every git
	// invocation behind this one ends with `--`, and the ambiguity git would otherwise refuse never
	// reaches it.

	// This case wanted that refusal until the port put the `--` there. A refusal hands the branch under
	// review a way to switch the scan off for everyone reading it, by committing a file called HEAD.
	// A revision is what the argument was always going to mean here.
	t.Run("an argument that is both a revision and a filename is read as the revision", func(t *testing.T) {
		// Real git, because git's own refusal is what the `--` averts, and no fake can be made to give it.
		r := newRealRepo(t)
		r.write("HEAD", "ambiguous\n")
		r.commit("add a file called HEAD")
		r.run("HEAD")
		r.expectCode(exitClean)
		r.expectStderrLacks("git rejected these arguments")
	})

	t.Run("a pathspec after -- is scanned rather than refused", func(t *testing.T) {
		r := newRepo(t)
		r.write("kept.go", "x := 1\n")
		r.commit("base")
		r.changed("kept.go", housey(1))
		r.run("HEAD", "--", "kept.go")
		r.expectCode(1)
		r.expectStdoutHas("kept.go")
	})

	t.Run("a pathspec after -- that selects nothing exits 0", func(t *testing.T) {
		r := newRepo(t)
		r.write("kept.go", "x := 1\n")
		r.commit("base")
		// git narrows the diff to the pathspec, and this pathspec matches no file the change touched.
		// The scan is answered with an empty diff, however heavy the tree beside it is.
		r.write("kept.go", housey(1))
		r.run("HEAD", "--", "no-such-path")
		r.expectCode(0)
		r.expectNoStdout()
	})
}

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
	r.changed("real.go", "++ b/decoy.go\n"+heavy(8, 1))
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
	// The path arrives bare, and never C-quoted. That is what the config key core.quotePath, set false,
	// buys, and repo/exec_test.go holds it against a real git for every listing the port takes.
	r.changed("café.go", housey(1))
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("café.go")
}

// --text, or one `* -diff` in the branch author's .gitattributes collapses the body to
// "Binary files … differ" and the scan exits 0 over a real outlier.
func TestADiffAttributeDoesNotSuppressTheScan(t *testing.T) {
	// Real git, because the attribute is the subject. What it does to a diff body is git's behaviour,
	// and a fake stating it would be agreeing with itself.
	r := newRealRepo(t)
	r.write("attr.go", "package fixture\n")
	r.write(".gitattributes", "* -diff\n")
	r.commit("base")
	r.write("attr.go", housey(1))
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("attr.go")
}

func TestANewlineInAPathIsNoLongerAHazard(t *testing.T) {
	r := newRepo(t)
	name := "odd\nname.go"
	r.write(name, housey(1))
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("odd name.go")
	r.expectStdoutLacks("odd\nname.go")
	if lines := strings.Count(strings.TrimRight(r.stdout.String(), "\n"), "\n") + 1; lines != 1 {
		t.Errorf("the report is %d lines over one outlier, wanted 1: %q", lines, r.stdout.String())
	}
	r.expectStderrHas("over 1 file(s)")
}

// A path long enough to be cut says it was cut. Unmarked, a name truncated at the bound is a shorter
// different name, and a caller grepping the report for the file it changed finds nothing and reads
// that as "not an outlier".
func TestAnOverlongPathIsCutAndSaysSo(t *testing.T) {
	r := newRepo(t)
	name := strings.Repeat("d", maxPathBytes) + "/over.go"
	r.write(name, housey(1))
	r.run()
	r.expectCode(1)
	r.expectStdoutHas(shell.CutMarker + ":1:")
	r.expectStdoutLacks("over.go")
	for _, line := range strings.Split(strings.TrimRight(r.stdout.String(), "\n"), "\n") {
		reported, _, _ := strings.Cut(line, ":")
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
	r.write(name, housey(1))
	// The C-quoted field git really prints for this path, which is the form the scan has to unquote.
	r.addedUnder(`"b/tab\there.go"`, strings.Split(strings.TrimSuffix(housey(1), "\n"), "\n")...)
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("tab here.go")
	r.expectStderrHas("over 1 file(s)")
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
	r.changed("a.go", strings.Repeat("x", 70000)+"\n")
	r.changed("z.go", housey(1))

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

// No case may read the configuration of whoever runs the suite.

// Pinning HOME alone was not enough. The tool takes its machine override from
// $XDG_CONFIG_HOME/kk-flavor/ and only falls back to $HOME/.config. On a machine exporting that
// variable, every case that set neither read the owner's own file.

// Three did go red on one laptop, over a `domain` keyword a newer build of this tool had written
// there. That failure was about the owner's config, and never about the code under test. It
// reproduced nowhere else.

// The values TestMain saved are what the assertion runs against, and never a temp-directory prefix, so
// the case names exactly what a leak reached.
func TestNoCaseCanReachTheConfigurationOfWhoeverRunsTheSuite(t *testing.T) {
	for _, pin := range []struct{ name, now, machine string }{
		{"HOME", os.Getenv("HOME"), machineHome},
		{"XDG_CONFIG_HOME", os.Getenv("XDG_CONFIG_HOME"), machineConfigHome},
	} {
		if pin.now == "" {
			t.Errorf("%s is unset, so a case falls back to this machine's own", pin.name)
			continue
		}
		if pin.machine != "" && pin.now == pin.machine {
			t.Errorf("%s is still %s, which is this machine's own", pin.name, pin.now)
		}
	}
}

// `--source` reads whole files, and a revision range names which. Writer I of run 10 passed a range,
// the scan read it as a path, and the writer ran the check without `--source` instead.
func TestSourceOverARevisionReadsEveryBlockOfTheFilesItTouches(t *testing.T) {
	r := newRepo(t)
	r.write("src/ledger.ts", "// Posts a book.\nexport function post(): void {}\n")
	r.commit("a clean file")
	// The block already standing carries the finding, and the change touches only the code.
	r.write("src/ledger.ts", "// Posts a book; the ledger reads it.\nexport function post(): void {}\n")
	r.commit("a block with a finding")
	r.write("src/ledger.ts", "// Posts a book; the ledger reads it.\nexport function post(): number { return 0; }\n")
	r.commit("a code change under it")
	r.run("--source", "HEAD~1..HEAD")
	r.expectCode(1)
	r.expectStdoutHas("src/ledger.ts:1: semicolon")
}
