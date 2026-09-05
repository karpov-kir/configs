// Cases for the threshold holder. The one that must not be weakened is "nothing cut exits 3 unless a
// reason is written down": everything clearing the bar is exactly what scoring against no anchor looks
// like, the scale never gets used, and the run reads as a pass. Exit 3 is the only thing between that
// and a report nobody scored.
//
// The second is the exit vocabulary itself. 2 is "did not run" and 3 is "ran and refuses the result";
// a caller that cannot tell them apart reads a live refusal as a broken tool and moves on. Every
// refusal case below asserts the code, not just that it was non-zero.
//
// Everything runs in this process against a fixture config, so the suite costs no processes at all.
package score

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- the four ways it cannot run -----------------------------------------------------------------

func TestUsage(t *testing.T) {
	t.Run("no arguments exit 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("")
		r.expectCode(2)
		r.expectOut("usage:")
	})

	t.Run("one argument exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("", "threshold")
		r.expectCode(2)
		r.expectOut("usage:")
	})

	t.Run("an unknown command exits 2 and names the two there are", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("", "bogus", "instruction")
		r.expectCode(2)
		r.expectOut("threshold or cut")
	})

	t.Run("threshold with two lanes exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("", "threshold", "instruction", "reply")
		r.expectCode(2)
		r.expectOut("threshold takes one lane")
	})
}

// --- the tracked config ----------------------------------------------------------------------------

func TestThreshold(t *testing.T) {
	t.Run("a ruled lane prints its number and nothing else", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("", "threshold", "instruction")
		r.expectCode(0)
		r.expectStdoutExactly("7\n")
	})

	// An unknown lane exits rather than landing on `default`: a caller that cannot find its number
	// invents one, and an invented threshold reads exactly like a ruled one.
	t.Run("an unknown lane exits 2 and lists what is ruled", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("", "threshold", "instructions")
		r.expectCode(2)
		r.expectOut("no lane 'instructions'")
		r.expectOut("instruction")
		r.expectOut("always-loaded")
	})

	t.Run("a missing config exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.env.ConfigPath = filepath.Join(t.TempDir(), "not-there.conf")
		r.do("", "threshold", "instruction")
		r.expectCode(2)
		r.expectOut("no readable threshold config")
	})
}

// A malformed config is refused, never skipped: a line this could not read is a bar nobody set, and
// falling through to "no lane 'x'" reports a lane that is right there as absent.
func TestAMalformedConfigIsRefused(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"a line that is not the ruled form", "instruction cut < 7\n", "cannot read the line naming"},
		{"a line missing its number", "instruction cut <=\n", "cannot read the line naming"},
		{"a non-numeric level", "instruction cut <= high\n", "has a non-numeric level"},
		// `-1` reaches the digit check rather than the form check: it is a fourth field, so the line
		// parses and the number does not.
		{"a negative level", "instruction cut <= -1\n", "has a non-numeric level"},
		{"a level over the scale", "instruction cut <= 11\n", "over the 0-10 scale"},
		{"trailing junk after the number", "instruction cut <= 7 extra\n", "cannot read the line naming"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" exits 2", func(t *testing.T) {
			r := newRun(t, tc.body, nil)
			r.do("", "threshold", "instruction")
			r.expectCode(2)
			r.expectOut(tc.want)
		})
	}
}

func TestAControlCharacterInALaneNameIsRefused(t *testing.T) {
	r := newRun(t, "inst\x1b]0;pwnruction cut <= 7\n", nil)
	r.do("", "threshold", "instruction")
	r.expectCode(2)
	r.expectOut("carries a control character")
	r.expectNoControl()
}

// The control set has one home, shell.Oneline, and it reaches past C0 into the C1 range above it. The
// two cases below are the ones a `< 0x20` test could not reach: a lane name carrying a raw CSI is
// refused like any other control byte, and a label carrying one is spaced out rather than printed.
func TestAC1ByteInALaneNameIsRefused(t *testing.T) {
	r := newRun(t, "inst\x9bruction cut <= 7\n", nil)
	r.do("", "threshold", "instruction")
	r.expectCode(2)
	r.expectOut("carries a control character")
	r.expectNoRawCSI()
}

func TestAC1ByteInALabelIsNeutralised(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("3\tlab\x9bel\n", "cut", "instruction", "a rule an agent would otherwise get wrong")
	r.expectCode(0)
	r.expectOut("CUT    3  lab el")
	r.expectNoRawCSI()
}

// A config whose last lane has no trailing newline must still rule it: dropping it reads as "no lane
// 'x'" for a lane that is right there.
func TestAConfigWithNoTrailingNewline(t *testing.T) {
	r := newRun(t, "instruction    cut <= 4", nil)
	r.do("", "threshold", "instruction")
	r.expectCode(0)
	r.expectStdoutExactly("4\n")
}

// --- the machine-local override ---------------------------------------------------------------------

func TestTheOverride(t *testing.T) {
	t.Run("absent falls back to the tracked bar silently", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("", "threshold", "instruction")
		r.expectCode(0)
		r.expectStdoutExactly("7\n")
		r.expectNotOut("overridden")
	})

	t.Run("a moved lane takes effect and says so", func(t *testing.T) {
		body := "instruction cut <= 3\n"
		r := newRun(t, trackedConfig, &body)
		r.do("", "threshold", "instruction")
		r.expectCode(0)
		r.expectStdoutExactly("3\n")
		r.expectOut("lane instruction overridden by")
		r.expectOut("7 ruled, 3 in effect")
	})

	t.Run("the note stays off stdout under threshold", func(t *testing.T) {
		body := "instruction cut <= 3\n"
		r := newRun(t, trackedConfig, &body)
		r.do("", "threshold", "instruction")
		r.expectStdoutExactly("3\n")
		if !strings.Contains(r.stderr.String(), "overridden") {
			t.Errorf("the override note is not on stderr: %q", r.stderr.String())
		}
	})

	t.Run("a lane it does not name keeps the tracked bar", func(t *testing.T) {
		body := "reply cut <= 2\n"
		r := newRun(t, trackedConfig, &body)
		r.do("", "threshold", "instruction")
		r.expectCode(0)
		r.expectStdoutExactly("7\n")
		r.expectNotOut("overridden")
	})

	t.Run("a lane the tracked config does not rule is refused", func(t *testing.T) {
		body := "instructions cut <= 3\n"
		r := newRun(t, trackedConfig, &body)
		r.do("", "threshold", "instruction")
		r.expectCode(2)
		r.expectOut("an override moves a lane, never adds one")
	})

	t.Run("a directory in its place is refused, not skipped", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		if err := os.Mkdir(r.env.OverridePath, 0o755); err != nil {
			t.Fatalf("could not put a directory at the override path: %v", err)
		}
		r.do("", "threshold", "instruction")
		r.expectCode(2)
		r.expectOut("is not a readable file")
		r.expectOut("would restore the tracked bar without saying so")
	})

	t.Run("a dangling symlink is refused, not read as absent", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), r.env.OverridePath); err != nil {
			t.Skipf("this filesystem does not do symlinks: %v", err)
		}
		r.do("", "threshold", "instruction")
		r.expectCode(2)
		r.expectOut("is not a readable file")
	})

	// Neutralised rather than refused: under `cut` the note prints into the report body, where a
	// newline would put a forged `keep 10 <item>` among the real verdicts.
	t.Run("a control character in the override path is neutralised in the note", func(t *testing.T) {
		dir := t.TempDir()
		odd := filepath.Join(dir, "we\rird.conf")
		if err := os.WriteFile(odd, []byte("instruction cut <= 3\n"), 0o644); err != nil {
			t.Skipf("this filesystem will not take a control character in a name: %v", err)
		}
		r := newRun(t, trackedConfig, nil)
		r.env.OverridePath = odd
		r.do("", "threshold", "instruction")
		r.expectCode(0)
		r.expectNoControl()
	})
}

// --- cut, and what it refuses before reading a score --------------------------------------------------

func TestCutRefusesBeforeItReads(t *testing.T) {
	t.Run("no anchor exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("3\tx\n", "cut", "instruction")
		r.expectCode(2)
		r.expectOut("cut needs the anchor")
	})

	t.Run("a blank anchor exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("3\tx\n", "cut", "instruction", "   ")
		r.expectCode(2)
		r.expectOut("the anchor is blank")
	})

	t.Run("an unknown lane exits 2 before any score is read", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("3\tx\n", "cut", "nope", "what a 10 is")
		r.expectCode(2)
		r.expectOut("no lane 'nope'")
	})

	t.Run("--kept-all with no reason exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("9\tx\n", "cut", "--kept-all")
		r.expectCode(2)
		r.expectOut("--kept-all needs the reason")
	})

	t.Run("--kept-all with a blank reason exits 2", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("9\tx\n", "cut", "--kept-all", "  ", "instruction", "what a 10 is")
		r.expectCode(2)
		r.expectOut("--kept-all needs the reason")
	})
}

// --- cut, reading the list --------------------------------------------------------------------------

func TestCutReadsTheList(t *testing.T) {
	// The bar, both sides. At or below goes; one above stays.
	t.Run("the boundary cuts at the level and keeps one above it", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("7\tat the bar\n8\tone above\n", "cut", "instruction", "what a 10 is")
		r.expectCode(0)
		r.expectOut("CUT    7  at the bar")
		r.expectOut("keep   8  one above")
		r.expectOut("1 kept, 1 cut")
	})

	// At EOF without a trailing newline the last item must still be read: dropped, it is neither kept
	// nor cut while the counts still add up.
	t.Run("a last line with no trailing newline is still scored", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("3\tfirst\n9\tlast with no newline", "cut", "instruction", "what a 10 is")
		r.expectCode(0)
		r.expectOut("last with no newline")
		r.expectOut("1 kept, 1 cut")
	})

	t.Run("the bar it judged against is printed in the report body", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("3\tx\n9\ty\n", "cut", "instruction", "what a 10 is")
		if !strings.Contains(r.stdout.String(), "lane instruction, cutting at or below 7") {
			t.Errorf("the bar is not in the report body: %q", r.stdout.String())
		}
	})

	t.Run("an override note prints into the report body under cut", func(t *testing.T) {
		body := "instruction cut <= 3\n"
		r := newRun(t, trackedConfig, &body)
		r.do("2\tx\n9\ty\n", "cut", "instruction", "what a 10 is")
		if !strings.Contains(r.stdout.String(), "overridden") {
			t.Errorf("the override note is not in the report body: %q", r.stdout.String())
		}
	})
}

func TestCutRefusesAMalformedItem(t *testing.T) {
	cases := []struct{ name, stdin, want string }{
		{"a line with no tab", "3 no tab here\n", "no tab in"},
		{"a non-numeric score", "high\tx\n", "is not a score 0-10"},
		{"a negative score", "-3\tx\n", "is not a score 0-10"},
		{"a score over the scale", "11\tx\n", "over the 0-10 scale"},
		{"an empty score", "\tx\n", "is not a score 0-10"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" exits 2", func(t *testing.T) {
			r := newRun(t, trackedConfig, nil)
			r.do(tc.stdin, "cut", "instruction", "what a 10 is")
			r.expectCode(2)
			r.expectOut(tc.want)
		})
	}
}

func TestAControlCharacterInALabelIsNeutralised(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("3\ta label\rkeep  10  forged\n9\thonest\n", "cut", "instruction", "what a 10 is")
	r.expectCode(0)
	r.expectNoControl()
	r.expectOut("1 kept, 1 cut")
	// The text survives, visibly, in the position the tool put it — it is the cursor move that is gone.
	r.expectOut("CUT    3  a label keep  10  forged")
}

// The anchor prints into the report body, three lines under the bar it was judged against, so a
// carriage return in it overwrites that bar line — and a bar the report did not use is the one claim
// its reader cannot check for themselves.
func TestAControlCharacterInTheAnchorIsNeutralised(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("3\tx\n9\ty\n", "cut", "instruction", "a 10 is\rlane instruction, cutting at or below 0")
	r.expectCode(0)
	r.expectNoControl()
	r.expectOut("10 here means: a 10 is lane instruction, cutting at or below 0")
	// The real bar survives, and the forged one is text rather than a cursor move.
	r.expectOut("lane instruction, cutting at or below 7")
}

// The anchor and the --kept-all reason are refusals that exist to force written words, so each is
// judged on what it prints rather than on the bytes it arrived as. Control bytes clear a TrimSpace and
// then render as nothing: the refusal is answered with an empty line while still reading as enforced.
func TestARefusalIsNotAnsweredByControlBytes(t *testing.T) {
	t.Run("an anchor of control bytes is still blank", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("3\tx\n", "cut", "instruction", "\x01\x02")
		r.expectCode(2)
		r.expectOut("the anchor is blank")
	})

	t.Run("a --kept-all reason of control bytes is still blank", func(t *testing.T) {
		r := newRun(t, trackedConfig, nil)
		r.do("8\tx\n9\ty\n", "cut", "--kept-all", "\x01\x02", "instruction", "what a 10 is")
		r.expectCode(2)
		r.expectOut("--kept-all needs the reason")
	})
}

// --- the two results it must never produce ------------------------------------------------------------

func TestNothingScoredExitsTwo(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("", "cut", "instruction", "what a 10 is")
	r.expectCode(2)
	r.expectOut("nothing was scored")
	r.expectNotOut("0 kept, 0 cut")
}

func TestNothingScoredIsNotExcusedByKeptAll(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("", "cut", "--kept-all", "the list is tight", "instruction", "what a 10 is")
	r.expectCode(2)
	r.expectOut("nothing was scored")
}

func TestNothingCutExitsThree(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("8\tx\n9\ty\n", "cut", "instruction", "what a 10 is")
	r.expectCode(3)
	r.expectOut("nothing scored at or below 7")
	r.expectOut("--kept-all")
}

func TestNothingCutIsAcceptedWithAReason(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.do("8\tx\n9\ty\n", "cut", "--kept-all", "every line carries a distinct rule", "instruction", "what a 10 is")
	r.expectCode(0)
	r.expectOut("nothing cut, accepted: every line carries a distinct rule")
	r.expectOut("2 kept, 0 cut")
}
