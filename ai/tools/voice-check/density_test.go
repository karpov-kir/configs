// Cases for the voice-check detector. Don't weaken "a path argument is refused with exit 2, never
// scanned": `git diff <path>` is legal and diffs against the index, so a path quietly accepted scans
// the wrong change set and exits 0 — indistinguishable from a clean tree.
package voicecheck

import (
	"fmt"
	"strings"
	"testing"
)

func TestAnUnchangedTree(t *testing.T) {
	r := newRepo(t)
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
	r.expectStderrHas("0 file(s) reached the scan")
	r.expectStderrHas("says nothing about the change set")
	r.expectStderrLacks("not a bar")

	r.run()
	r.expectCode(0)
}

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

	t.Run("four added comment lines are under the floor", func(t *testing.T) {
		r := newRepo(t)
		r.write("few.go", "package fixture\n")
		r.commit("base")
		r.write("few.go", heavy(4, 0))
		r.run("HEAD")
		r.expectCode(0)
		r.expectNoStdout()
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

	t.Run("a ratio exactly at the bar is not an outlier", func(t *testing.T) {
		r := newRepo(t)
		r.write("edge.go", "package fixture\n")
		r.commit("base")
		r.write("edge.go", heavy(6, 14))
		r.run("HEAD")
		r.expectCode(0)
		r.expectNoStdout()
	})

	t.Run("a ratio above the bar is", func(t *testing.T) {
		r := newRepo(t)
		r.write("over.go", "package fixture\n")
		r.commit("base")
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
		cfg := baseConfig()
		cfg.MaxRatio = 0.9
		r.runWith(cfg, "HEAD")
		r.expectCode(0)
	})

	t.Run("raising the floor clears it too", func(t *testing.T) {
		r := newRepo(t)
		r.write("over.go", "package fixture\n")
		r.commit("base")
		r.write("over.go", heavy(7, 13))
		cfg := baseConfig()
		cfg.MinLines = 100
		r.runWith(cfg, "HEAD")
		r.expectCode(0)
	})

	t.Run("blank added lines do not dilute the ratio", func(t *testing.T) {
		r := newRepo(t)
		r.write("blanks.go", "package fixture\n")
		r.commit("base")
		r.write("blanks.go", "// a\n// b\n// c\n// d\n// e\n\n\n\n\n\n\n\n\n\n\ny := 1\n")
		r.run("HEAD")
		r.expectCode(1)
		r.expectStdoutHas("5 comment / 1 code added lines (0.83)")
	})
}

func TestTheCommentForms(t *testing.T) {
	r := newRepo(t)
	r.write("forms.go", "package fixture\n")
	r.commit("base")
	r.write("forms.go", "// line\n/* block\n * star\n */\n# hash\n   // indented\nreal := 1\n")
	r.run("HEAD")
	r.expectCode(1)
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
	r.expectStderrHas("7 file(s) reached the scan, 0 with countable added lines")
	// Files were reached, so the empty-scan note stays silent; nothing was ranked, so the note that
	// disowns the bar must stay silent too rather than claim a ranking this run never made.
	r.expectStderrHas("ranks nothing")
	r.expectStderrLacks("nothing reached the scan")
	r.expectStderrLacks("not a bar")
}

// A new file is the commonest place a new comment lands, so a voice scan that read only the tracked
// diff would report clean over the change most worth reading. The default mode and the bar both walk
// the untracked half with no revisions named; this holds --voice to the same.
func TestTheVoiceScanReadsAnUntrackedFileWithNoRevisionsNamed(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	r.write("fresh.go", "// Counted across the whole ledger.\npackage fixture\n")
	r.run("--voice")
	r.expectCode(1)
	r.expectStdoutHas("fresh.go")
	r.expectStdoutHas("no-subject")
}

// With revisions named the caller asked about two commits, and a file in neither of them is not part
// of the question. Answering with it would report a finding the named range does not carry.
func TestTheVoiceScanLeavesTheUntrackedHalfOutWhenRevisionsAreNamed(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	r.write("keep.go", "// Reads the ledger.\npackage fixture\n")
	r.commit("second")
	r.write("fresh.go", "// Counted across the whole ledger.\npackage fixture\n")
	r.run("--voice", "HEAD~1", "HEAD")
	r.expectStdoutLacks("fresh.go")
}

// A fixture is written to be read by a test rather than to be this repository's source, and it is
// routinely in another language. Counted as the repository's, it measures the fixture through the
// repository: a handful of TypeScript fixtures moves a Go repository's comment rate on its own.
func TestAFixtureUnderTestdataIsNotThisRepositorysSource(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	for _, name := range []string{"testdata/a.go", "pkg/testdata/deep/b.go", "testdata/c.ts"} {
		r.write(name, heavy(9, 0))
	}
	r.write("counted.go", heavy(9, 0))
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("counted.go")
	r.expectStdoutLacks("testdata")
	r.expectStderrHas("4 file(s) reached the scan, 1 with countable added lines")
}

// A fixture NAMED on the command line is still read: naming one is asking for it, and the voice
// check's prose and instruction profiles are handed paths by a human.
func TestNamingAFixtureIsAskingForIt(t *testing.T) {
	if !notThisRepositorysSource("pkg/testdata/x.go") {
		t.Error("a discovered fixture was treated as this repository's source")
	}
	if isFixture("pkg/testdataish/x.go") {
		t.Error("a directory merely starting with testdata was read as Go's reserved one")
	}
	if !isFixture("testdata/x.go") || !isFixture("a/b/testdata/x.go") {
		t.Error("testdata was not matched as a path segment at every depth")
	}
}

func TestATwoRevisionRangeIsScanned(t *testing.T) {
	r := newRepo(t)
	r.write("ranged.go", "package fixture\n")
	r.commit("base")
	r.write("ranged.go", heavy(8, 1))
	r.commit("dense")
	// Untracked, comment-heavy, and in neither commit the caller named — the only thing here that can
	// show the untracked half staying out. Delete it and this case stops testing that.
	r.write("stray.go", heavy(9, 1))
	r.run("HEAD~1..HEAD")
	r.expectCode(1)
	r.expectStdoutHas("ranged.go")
	r.expectStdoutLacks("stray.go")
	r.expectStderrHas("1 file(s) reached the scan")
}

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

func TestAThresholdThatDoesNotParseRefuses(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"a ratio that is not a number", "COMMENT_MAX_RATIO", "junk"},
		{"a floor that is not a whole number", "COMMENT_MIN_LINES", "2.5"},
		{"a byte cap that is not a whole number", "DENSITY_MAX_FILE_BYTES", "big"},
		{"a ratio above the share it measures", "COMMENT_MAX_RATIO", "1.5"},
		{"a negative ratio, under which every file is an outlier", "COMMENT_MAX_RATIO", "-0.1"},
		{"a floor no file can fall under", "COMMENT_MIN_LINES", "0"},
		{"a negative byte cap, which skips every untracked file unread", "DENSITY_MAX_FILE_BYTES", "-1"},
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

func TestAThresholdOverrideTakesEffect(t *testing.T) {
	only := func(key, value string) func(string) (string, bool) {
		return func(asked string) (string, bool) {
			if asked == key {
				return value, true
			}
			return "", false
		}
	}
	// 7 comments of 20 added lines is 0.35 — over the 0.3 default, under a 0.9 override, and under a
	// floor of 100.
	dense := func(t *testing.T) *repo {
		t.Helper()
		r := newRepo(t)
		r.write("over.go", "package fixture\n")
		r.commit("base")
		r.write("over.go", heavy(7, 13))
		return r
	}

	t.Run("COMMENT_MAX_RATIO moves the ratio bar", func(t *testing.T) {
		cfg, err := ConfigFromEnv(only("COMMENT_MAX_RATIO", "0.9"))
		if err != nil {
			t.Fatalf("0.9 was refused: %v", err)
		}
		if cfg.MaxRatio != 0.9 {
			t.Errorf("the ratio bar is %v, wanted the 0.9 that was asked for", cfg.MaxRatio)
		}
		r := dense(t)
		r.runWith(cfg, "HEAD")
		r.expectCode(0)
		r.expectNoStdout()
	})

	t.Run("COMMENT_MIN_LINES moves the floor", func(t *testing.T) {
		cfg, err := ConfigFromEnv(only("COMMENT_MIN_LINES", "100"))
		if err != nil {
			t.Fatalf("100 was refused: %v", err)
		}
		if cfg.MinLines != 100 {
			t.Errorf("the floor is %d, wanted the 100 that was asked for", cfg.MinLines)
		}
		r := dense(t)
		r.runWith(cfg, "HEAD")
		r.expectCode(0)
		r.expectNoStdout()
	})

	t.Run("DENSITY_MAX_FILE_BYTES moves the untracked byte cap", func(t *testing.T) {
		cfg, err := ConfigFromEnv(only("DENSITY_MAX_FILE_BYTES", "32"))
		if err != nil {
			t.Fatalf("32 was refused: %v", err)
		}
		if cfg.MaxFileBytes != 32 {
			t.Errorf("the byte cap is %d, wanted the 32 that was asked for", cfg.MaxFileBytes)
		}
		r := newRepo(t)
		r.write("big.go", heavy(400, 1))
		r.runWith(cfg)
		r.expectCode(0)
		r.expectStdoutLacks("big.go")
		r.expectStderrHas("1 untracked file(s) skipped unread")
	})
}

// The clean run needs the note most: no outlier otherwise reads as nothing to cut.
func TestTheDefaultReportDisownsTheBar(t *testing.T) {
	t.Run("an outlier report says it is not the bar", func(t *testing.T) {
		r := newRepo(t)
		r.write("dense.go", "package fixture\n")
		r.commit("base")
		r.write("dense.go", heavy(8, 2))
		r.run("HEAD")
		r.expectCode(1)
		r.expectStderrHas("not a bar")
		r.expectStderrHas("ADDED lines")
		r.expectStderrHas("--bar")
	})

	t.Run("a clean run says it too", func(t *testing.T) {
		r := newRepo(t)
		r.write("lean.go", "package fixture\n")
		r.commit("base")
		r.write("lean.go", heavy(0, 9))
		r.run("HEAD")
		r.expectCode(0)
		r.expectStderrHas("not a bar")
	})

	// Asserting the absence alone passes over a --bar run that reported nothing at all, so the exit
	// code and the report's first line prove the bar ran before the absence means anything.
	t.Run("the bar's own report does not disown itself", func(t *testing.T) {
		r := newRepoWithLeanBaseline(t)
		r.write("dense.go", heavy(8, 2))
		r.runBar()
		r.expectCode(exitFound)
		r.expectStdoutHas("host repo:")
		r.expectStderrLacks("not a bar")
	})
}
