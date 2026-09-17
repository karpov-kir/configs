// Cases for the voice-check detector.
//
// Keep the rule that a path argument is refused with exit 2 and never scanned. `git diff <path>` is
// legal and diffs against the index, so a path quietly accepted scans the wrong change set and exits
// 0. A caller cannot tell that from a clean tree.
package voicecheck

import (
	"strings"
	"testing"
)

// A bare `*` opens a dereference or a multiplication, so it is code. A tool that counted it as a
// comment continuation would flag dense arithmetic as dense prose.
func TestAStarThatIsNotAComment(t *testing.T) {
	r := newRepo(t)
	r.write("math.c", "int x;\n")
	r.commit("base")
	r.write("math.c", "*ptr = 1;\n*q = 2;\n*r = 3;\n*s = 4;\n*t = 5;\n*u = 6;\n")
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
}

// A new file is the commonest place a new comment lands, so a voice scan that read only the tracked
// diff would report clean over the change most worth reading. The density mode walks the untracked
// half with no revisions named, and this holds the register check to the same.
func TestTheVoiceScanReadsAnUntrackedFileWithNoRevisionsNamed(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	r.write("fresh.go", "// Counted across the whole ledger.\npackage fixture\n")
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("fresh.go")
	r.expectStdoutHas("no-subject")
}

// With revisions named the caller asked about two commits, so a file in neither of them falls outside
// the question. A finding from it would belong to no part of the named range.
func TestTheVoiceScanLeavesTheUntrackedHalfOutWhenRevisionsAreNamed(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	r.write("keep.go", "// Reads the ledger.\npackage fixture\n")
	r.commit("second")
	r.write("fresh.go", "// Counted across the whole ledger.\npackage fixture\n")
	r.run("HEAD~1", "HEAD")
	r.expectStdoutLacks("fresh.go")
}

// A fixture is a test's material, and it is routinely in another language. A scan that counts it as
// the repository's own source measures the fixture through the repository. A handful of TypeScript
// fixtures moves a Go repository's comment rate on its own.

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

func TestAThresholdThatDoesNotParseRefuses(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"a byte cap that is not a whole number", "DENSITY_MAX_FILE_BYTES", "big"},
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
		if cfg.MaxFileBytes != 262144 {
			t.Errorf("defaults are %+v, wanted cap 262144", cfg)
		}
	})
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

	t.Run("DENSITY_MAX_FILE_BYTES moves the untracked byte cap", func(t *testing.T) {
		cfg, err := ConfigFromEnv(only("DENSITY_MAX_FILE_BYTES", "32"))
		if err != nil {
			t.Fatalf("32 was refused: %v", err)
		}
		if cfg.MaxFileBytes != 32 {
			t.Errorf("the byte cap is %d, wanted the 32 that was asked for", cfg.MaxFileBytes)
		}
		r := newRepo(t)
		r.write("big.go", housey(1))
		r.runWith(cfg)
		r.expectCode(0)
		r.expectStdoutLacks("big.go")
		r.expectStderrHas("declined unread")
	})
}

// The clean run needs the note most. A silent report reads as a change set with no cut to make.

// The classifiers the density report and the register scan both read. They are tested directly here.
// The report that used to exercise them was the per-file outlier mode, and that mode is gone. The line
// arrives trimmed, which is what every caller passes.
func TestWhatCountsAsAComment(t *testing.T) {
	for _, line := range []string{"// line", "/* block", "* star", "*/", "# hash"} {
		if !isComment(line) {
			t.Errorf("%q is a comment form this repository writes and was not counted", line)
		}
	}
	for _, line := range []string{"real := 1", "", "x := a * b", "code() // trailing"} {
		if isComment(line) {
			t.Errorf("%q is code and was counted as a comment", line)
		}
	}
}

func TestWhatIsNotThisRepositorysSource(t *testing.T) {
	for _, file := range []string{"a.md", "b.markdown", "c.txt", "d.json", "e.lock", "pnpm-lock.yaml", "f.MD"} {
		if !isProseOrData(file) {
			t.Errorf("%q is prose or data and was read as source", file)
		}
	}
	for _, file := range []string{"testdata/a.go", "pkg/testdata/deep/b.go", "testdata/c.ts"} {
		if !isFixture(file) {
			t.Errorf("%q sits under testdata and was read as source", file)
		}
	}
	for _, file := range []string{"counted.go", "pkg/real.ts", "cmd/main.go"} {
		if notThisRepositorysSource(file) {
			t.Errorf("%q is this repository's source and was skipped", file)
		}
	}
}
