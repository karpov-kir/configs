// Cases for the comment-density detector. The one that must not be weakened is "a path argument is
// refused with exit 2, never scanned": `git diff <path>` is legal and diffs against the index, so a
// path that is quietly accepted scans the wrong change set and exits 0 — indistinguishable from a
// clean tree, and the whole point of the tool is lost silently.
//
// Every case runs in this process against a throwaway repository copied from one seed, so the suite
// costs the seed's six git processes plus two per scan rather than a process per case.
//
// ONE case from the shell suite is deliberately gone, replaced by its opposite: the shell skipped an
// untracked file whose name held a newline, because such a name wrote a second line into the text
// stream it built for awk and forged a file header. Nothing here builds that stream, so the name
// cannot corrupt anything and the file is scanned like any other. TestANewlineInAPathIsNoLongerAHazard
// holds that, and it is the case to read first if this is ever ported back.
package density

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var seedRepo string

func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	base, err := os.MkdirTemp("", "density-seed")
	if err != nil {
		panic("density tests: no temp dir, so nothing was tested: " + err.Error())
	}
	defer os.RemoveAll(base)
	os.Setenv("HOME", filepath.Join(base, "home"))
	os.MkdirAll(os.Getenv("HOME"), 0o755)

	seedRepo = filepath.Join(base, "seed")
	if err := buildSeed(seedRepo); err != nil {
		// Panic rather than skip: every fixture is a copy of this, so carrying on would report passes
		// over repositories that do not exist.
		panic("density tests: could not build the seed repository, so nothing was tested: " + err.Error())
	}
	os.Exit(m.Run())
}

func buildSeed(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := git(dir, args...); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		return err
	}
	if err := git(dir, "add", "seed.txt"); err != nil {
		return err
	}
	return git(dir, "commit", "-qm", "base")
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// --- fixtures --------------------------------------------------------------------------------

type repo struct {
	t      *testing.T
	dir    string
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := copyTree(seedRepo, dir); err != nil {
		t.Fatalf("could not build a fixture repo: %v — stopping, since every case reads one", err)
	}
	return &repo{t: t, dir: dir}
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
}

// write puts a file in the working tree, creating parents.
func (r *repo) write(name, body string) {
	r.t.Helper()
	full := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("could not create the parent for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		r.t.Fatalf("could not write the fixture %s: %v", name, err)
	}
}

// commit tracks and commits everything currently in the tree, so later writes read as changes.
func (r *repo) commit(message string) {
	r.t.Helper()
	if err := git(r.dir, "add", "-A"); err != nil {
		r.t.Fatalf("could not stage the fixture: %v", err)
	}
	if err := git(r.dir, "commit", "-qm", message); err != nil {
		r.t.Fatalf("could not commit the fixture: %v", err)
	}
}

func (r *repo) run(args ...string) {
	r.runWith(Config{MaxRatio: defaultMaxRatio, MinLines: defaultMinLines, MaxFileBytes: defaultMaxFileBytes}, args...)
}

func (r *repo) runWith(cfg Config, args ...string) {
	r.stdout.Reset()
	r.stderr.Reset()
	r.code = Run("comment-density.sh", args, r.dir, cfg, &r.stdout, &r.stderr)
}

func (r *repo) expectCode(want int) {
	r.t.Helper()
	if r.code != want {
		r.t.Errorf("exit %d, wanted %d\nstdout: %s\nstderr: %s", r.code, want, r.stdout.String(), r.stderr.String())
	}
}

func (r *repo) expectStdoutHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stdout.String(), want) {
		r.t.Errorf("wanted %q on stdout, got: %s", want, r.stdout.String())
	}
}

func (r *repo) expectStdoutLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stdout.String(), unwanted) {
		r.t.Errorf("%q appears on stdout: %s", unwanted, r.stdout.String())
	}
}

// A refused run must leave nothing on stdout: anything there is what a caller capturing the report
// reads as a finding.
func (r *repo) expectNoStdout() {
	r.t.Helper()
	if r.stdout.Len() != 0 {
		r.t.Errorf("expected nothing on stdout, got: %s", r.stdout.String())
	}
}

func (r *repo) expectStderrHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stderr.String(), want) {
		r.t.Errorf("wanted %q on stderr, got: %s", want, r.stderr.String())
	}
}

func (r *repo) expectStderrLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stderr.String(), unwanted) {
		r.t.Errorf("%q appears on stderr: %s", unwanted, r.stderr.String())
	}
}

// heavy builds a file of n comment lines and m code lines.
func heavy(comments, code int) string {
	var b strings.Builder
	for i := 0; i < comments; i++ {
		fmt.Fprintf(&b, "// comment %d\n", i)
	}
	for i := 0; i < code; i++ {
		fmt.Fprintf(&b, "x := %d\n", i)
	}
	return b.String()
}

// --- an unchanged tree -------------------------------------------------------------------------

func TestAnUnchangedTree(t *testing.T) {
	r := newRepo(t)
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
	r.expectStderrHas("0 file(s) reached the scan")
	// The denominator is what tells "nothing was comment-heavy" from "nothing was read".
	r.expectStderrHas("says nothing about the change set")

	r.run()
	r.expectCode(0)
}

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

// --- the ratio, and both sides of both bars ------------------------------------------------------

func TestTheRatioAndItsFloors(t *testing.T) {
	t.Run("a comment-heavy file exits 1 and prints its counts and ratio", func(t *testing.T) {
		r := newRepo(t)
		// A base sharing no line with heavy()'s output: `x := 0` would be matched as context and the
		// fixture would claim an added line the diff does not carry.
		r.write("dense.go", "package fixture\n")
		r.commit("base")
		r.write("dense.go", heavy(8, 2))
		r.run("HEAD")
		r.expectCode(1)
		r.expectStdoutHas("dense.go: 8 comment / 2 code added lines (0.80)")
	})

	// The floor, both sides. Four is under it and five reaches it — `>=`, not `>`.
	t.Run("four added comment lines are under the floor", func(t *testing.T) {
		r := newRepo(t)
		r.write("few.go", "package fixture\n")
		r.commit("base")
		r.write("few.go", heavy(4, 0))
		r.run("HEAD")
		r.expectCode(0)
		r.expectNoStdout()
		// The file was read all the same, so this run is not one that read nothing.
		r.expectStderrHas("1 file(s) reached the scan, 1 with countable added lines")
		r.expectStderrLacks("says nothing about the change set")
	})

	t.Run("five added comment lines reach the floor", func(t *testing.T) {
		r := newRepo(t)
		r.write("five.go", "package fixture\n")
		r.commit("base")
		r.write("five.go", heavy(5, 0))
		r.run("HEAD")
		r.expectCode(1)
		r.expectStdoutHas("five.go")
	})

	// The bar, both sides. Exactly at it is not an outlier — `>`, not `>=`.
	t.Run("a ratio exactly at the bar is not an outlier", func(t *testing.T) {
		r := newRepo(t)
		r.write("edge.go", "package fixture\n")
		r.commit("base")
		// 6 comments of 20 lines is 0.30 exactly.
		r.write("edge.go", heavy(6, 14))
		r.run("HEAD")
		r.expectCode(0)
		r.expectNoStdout()
	})

	t.Run("a ratio above the bar is", func(t *testing.T) {
		r := newRepo(t)
		r.write("over.go", "package fixture\n")
		r.commit("base")
		// 7 of 20 is 0.35.
		r.write("over.go", heavy(7, 13))
		r.run("HEAD")
		r.expectCode(1)
		r.expectStdoutHas("(0.35)")
	})

	t.Run("raising the ratio clears the outlier", func(t *testing.T) {
		r := newRepo(t)
		r.write("over.go", "package fixture\n")
		r.commit("base")
		r.write("over.go", heavy(7, 13))
		r.runWith(Config{MaxRatio: 0.9, MinLines: defaultMinLines, MaxFileBytes: defaultMaxFileBytes}, "HEAD")
		r.expectCode(0)
	})

	t.Run("raising the floor clears it too", func(t *testing.T) {
		r := newRepo(t)
		r.write("over.go", "package fixture\n")
		r.commit("base")
		r.write("over.go", heavy(7, 13))
		r.runWith(Config{MaxRatio: defaultMaxRatio, MinLines: 100, MaxFileBytes: defaultMaxFileBytes}, "HEAD")
		r.expectCode(0)
	})

	// Blank added lines are neither comment nor code, so they must not dilute the ratio into a pass.
	t.Run("blank added lines do not dilute the ratio", func(t *testing.T) {
		r := newRepo(t)
		r.write("blanks.go", "package fixture\n")
		r.commit("base")
		r.write("blanks.go", "// a\n// b\n// c\n// d\n// e\n\n\n\n\n\n\n\n\n\n\ny := 1\n")
		r.run("HEAD")
		r.expectCode(1)
		// Five comments, one code — the ten blanks counted as neither.
		r.expectStdoutHas("5 comment / 1 code added lines (0.83)")
	})
}

// --- what counts as a comment --------------------------------------------------------------------

func TestTheCommentForms(t *testing.T) {
	r := newRepo(t)
	r.write("forms.go", "package fixture\n")
	r.commit("base")
	r.write("forms.go", "// line\n/* block\n * star\n */\n# hash\n   // indented\nreal := 1\n")
	r.run("HEAD")
	r.expectCode(1)
	// Six comment forms, and the code line the only one counted as code.
	r.expectStdoutHas("6 comment / 1 code added lines")
}

// A bare `*` opening a dereference or a multiplication is code, not a comment continuation. Counting
// it as a comment flags dense arithmetic as dense prose.
func TestAStarThatIsNotAComment(t *testing.T) {
	r := newRepo(t)
	r.write("math.c", "int x;\n")
	r.commit("base")
	r.write("math.c", "*ptr = 1;\n*q = 2;\n*r = 3;\n*s = 4;\n*t = 5;\n*u = 6;\n")
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
}

// --- what is not counted at all -------------------------------------------------------------------

func TestProseDataAndLockfilesAreNotCounted(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	for _, name := range []string{"a.md", "b.markdown", "c.txt", "d.json", "e.lock", "pnpm-lock.yaml", "f.MD"} {
		r.write(name, heavy(9, 0))
	}
	r.run()
	r.expectCode(0)
	r.expectNoStdout()
	// The denominator separates reached from countable: seven files were opened and none of them held
	// a line this tool counts.
	r.expectStderrHas("7 file(s) reached the scan, 0 with countable added lines")
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
		r.runWith(Config{MaxRatio: defaultMaxRatio, MinLines: defaultMinLines, MaxFileBytes: 32})
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

// The shell skipped an untracked file whose name held a newline, and announced the skip: such a name
// wrote a second line into the pseudo-diff it built for awk, which is the one file header a caller
// could forge. Nothing here builds that stream — the file is opened by name and its lines never become
// diff text — so the hazard does not exist and the file is scanned like any other.
//
// This is the case that replaces the shell's, and it asserts the opposite. A scan that silently
// declined a file was a hole in the denominator; now there is nothing to decline.
func TestANewlineInAPathIsNoLongerAHazard(t *testing.T) {
	r := newRepo(t)
	name := "odd\nname.go"
	r.write(name, heavy(8, 1))
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("odd\nname.go")
	r.expectStderrHas("1 file(s) reached the scan")
	r.expectStderrHas("0 untracked file(s) skipped unread")
}

// --- ranges, caps and the index ---------------------------------------------------------------------

func TestATwoRevisionRangeIsScanned(t *testing.T) {
	r := newRepo(t)
	r.write("ranged.go", "package fixture\n")
	r.commit("base")
	r.write("ranged.go", heavy(8, 1))
	r.commit("dense")
	r.run("HEAD~1..HEAD")
	r.expectCode(1)
	r.expectStdoutHas("ranged.go")
}

// A suppressed outlier is announced, never dropped — which holds only while the cap on the loop and
// the one in the announcement stay the same number.
func TestPastTheDisplayCap(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	for i := 0; i < maxShown+1; i++ {
		r.write(fmt.Sprintf("f%03d.go", i), heavy(8, 1))
	}
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("… and 1 further outlier(s), not shown")
	shown := strings.Count(r.stdout.String(), " comment / ")
	if shown != maxShown {
		t.Errorf("printed %d outliers above the announcement, wanted exactly the cap %d", shown, maxShown)
	}
}

func TestAStagedChangeIsStillReported(t *testing.T) {
	r := newRepo(t)
	r.write("staged.go", "package fixture\n")
	r.commit("base")
	r.write("staged.go", heavy(8, 1))
	if err := git(r.dir, "add", "staged.go"); err != nil {
		t.Fatalf("could not stage the fixture: %v", err)
	}
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("staged.go")
}

// --- the thresholds themselves ----------------------------------------------------------------------

// A threshold that does not parse is a scan that did not run, never one against the default: the shell
// handed the raw string to awk, where `ratio > "junk"` is a string comparison, and the scan reported a
// result against a bar nobody chose.
func TestAThresholdThatDoesNotParseRefuses(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"a ratio that is not a number", "COMMENT_MAX_RATIO", "junk"},
		{"a floor that is not a whole number", "COMMENT_MIN_LINES", "2.5"},
		{"a byte cap that is not a whole number", "DENSITY_MAX_FILE_BYTES", "big"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			_, err := ConfigFromEnv(func(key string) (string, bool) {
				if key == tc.key {
					return tc.value, true
				}
				return "", false
			})
			if err == nil {
				t.Fatalf("%s=%q was accepted, so a scan would run against a threshold nobody chose", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), "the scan did NOT run") {
				t.Errorf("the refusal does not say the scan did not run: %v", err)
			}
		})
	}

	t.Run("an unset environment gives the documented defaults", func(t *testing.T) {
		cfg, err := ConfigFromEnv(func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatalf("an empty environment was refused: %v", err)
		}
		if cfg.MaxRatio != 0.3 || cfg.MinLines != 5 || cfg.MaxFileBytes != 262144 {
			t.Errorf("defaults are %+v, wanted ratio 0.3, floor 5, cap 262144", cfg)
		}
	})
}

// Two runs over one tree print one report. The shell iterated an awk hash, whose order is unspecified,
// so a diff of two reports was unreadable and the display cap took a different 200 each time.
func TestTheReportIsOrdered(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	for _, name := range []string{"zeta.go", "alpha.go", "mid.go"} {
		r.write(name, heavy(8, 1))
	}
	r.run()
	r.expectCode(1)
	got := r.stdout.String()
	first := strings.Index(got, "alpha.go")
	second := strings.Index(got, "mid.go")
	third := strings.Index(got, "zeta.go")
	if !(first < second && second < third) {
		t.Errorf("the report is not in path order:\n%s", got)
	}
}
