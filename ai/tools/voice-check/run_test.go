// Cases for the command: its flags, the paths it reads, and what it prints.
package voicecheck

import (
	"os"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

// A bare path is read with the prose profile, which is the one profile with the bold check.
func TestABarePathIsReadAsProse(t *testing.T) {
	r := newRepo(t)
	r.write("body.md", "The **one** rule here.\n")
	r.run("body.md")
	r.expectCode(1)
	r.expectStdoutHas("body.md:1: bold")
	r.expectStderrHas("prose profile")
}

// `--` ends the flags, so a path that starts with `-` is read rather than refused as an option.
func TestAPathAfterTheSeparatorIsReadEvenWhenItLooksLikeAFlag(t *testing.T) {
	r := newRepo(t)
	r.write("--odd.md", housey(1))
	r.run("--", "--odd.md")
	r.expectCode(1)
	r.expectStdoutHas("--odd.md:1: contrast")
}

func TestAnUnknownOptionIsRefusedWithTheGrammar(t *testing.T) {
	r := newRepo(t)
	r.write("body.md", housey(1))
	r.run("--source", "body.md")
	r.expectCode(2)
	r.expectStderrHas(`no option "--source"`)
	r.expectStderrHas(usage)
	r.expectNoStdout()
}

// A path that cannot be read is a sound invocation the tool could not carry out, so the refusal names
// the path and leaves the grammar out.
func TestAPathThatCannotBeReadRefusesWithoutTheGrammar(t *testing.T) {
	r := newRepo(t)
	r.write("dir/body.md", housey(1))
	r.run("dir")
	r.expectCode(2)
	r.expectStderrHas("cannot read dir")
	if strings.Contains(r.stderr.String(), usage) {
		t.Errorf("the refusal printed the grammar for an argument that had the right shape: %s", r.stderr.String())
	}
	r.expectNoStdout()
}

func TestNoPathRefusesTheRun(t *testing.T) {
	r := newRepo(t)
	r.run()
	r.expectCode(2)
	r.expectStderrHas("needs a path")
	r.expectNoStdout()
}

func TestAnUnknownProfileRefusesTheRun(t *testing.T) {
	r := newRepo(t)
	r.run("--profile=comment", "body.md")
	r.expectCode(2)
	r.expectStderrHas(`no profile "comment"`)
	r.expectNoStdout()
}

func TestAKindIsRefusedOutsideTheProseProfile(t *testing.T) {
	r := newRepo(t)
	r.write("body.md", housey(1))
	r.run("--profile=instruction", "--kind=pr-body", "body.md")
	r.expectCode(2)
	r.expectStderrHas("--kind reads a PR body or a ticket")
	r.expectNoStdout()
}

// A kind reads the template off the repository's top level, and the lines it shares with the template
// go unread.
func TestAKindLeavesTheTemplateUnread(t *testing.T) {
	r := newRepo(t)
	r.write(".github/PULL_REQUEST_TEMPLATE.md", "**Summary** of the change\n")
	r.write("body.md", "**Summary** of the change\n\nThe reader climbs rather than walks.\n")
	r.run("--kind=pr-body", "body.md")
	r.expectCode(1)
	r.expectStdoutLacks("bold")
	r.expectStdoutHas("body.md:3: contrast")
}

func TestAThresholdThatDoesNotParseRefuses(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"a byte cap that is not a whole number", "DENSITY_MAX_FILE_BYTES", "big"},
		{"a negative byte cap, which refuses every file", "DENSITY_MAX_FILE_BYTES", "-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			_, err := LoadConfig(func(key string) (string, bool) {
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
		cfg, err := LoadConfig(func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatalf("an empty environment was refused: %v", err)
		}
		if cfg.MaxFileBytes != 262144 {
			t.Errorf("defaults are %+v, wanted cap 262144", cfg)
		}
	})
}

// A file over the byte cap refuses the run. Read in part or skipped, it would leave a clean report
// over text the scan never saw.
func TestAFileOverTheByteCapRefusesTheRun(t *testing.T) {
	cfg, err := LoadConfig(func(key string) (string, bool) {
		if key == "DENSITY_MAX_FILE_BYTES" {
			return "32", true
		}
		return "", false
	})
	if err != nil || cfg.MaxFileBytes != 32 {
		t.Fatalf("got %+v %v, want the 32 that was asked for", cfg, err)
	}
	r := newRepo(t)
	r.write("big.md", housey(1))
	r.runWith(cfg, "big.md")
	r.expectCode(2)
	r.expectStderrHas("big.md is over DENSITY_MAX_FILE_BYTES")
	r.expectNoStdout()
}

// Findings are printed in one order whatever order the paths were named in, so a reader comparing two
// runs reads one list.
func TestTheReportIsOrderedByFileThenLineThenCheck(t *testing.T) {
	r := newRepo(t)
	r.write("z.md", housey(1))
	r.write("a.md", housey(1))
	r.run("z.md", "a.md")
	r.expectCode(1)
	if out := r.stdout.String(); strings.Index(out, "a.md") > strings.Index(out, "z.md") {
		t.Errorf("z.md was reported before a.md:\n%s", out)
	}
}

func TestANewlineInAPathIsEchoedOnOneLine(t *testing.T) {
	r := newRepo(t)
	name := "odd\nname.md"
	r.write(name, housey(1))
	r.run(name)
	r.expectCode(1)
	r.expectStdoutHas("odd name.md")
	if lines := strings.Count(strings.TrimRight(r.stdout.String(), "\n"), "\n") + 1; lines != 1 {
		t.Errorf("the report is %d lines over one finding, wanted 1: %q", lines, r.stdout.String())
	}
}

// A path long enough to be cut says it was cut. Unmarked, a name truncated at the bound is a shorter
// different name, and a caller grepping the report for the file it changed finds nothing.
func TestAnOverlongPathIsCutAndSaysSo(t *testing.T) {
	r := newRepo(t)
	name := strings.Repeat("d", maxPathBytes) + "/over.md"
	r.write(name, housey(1))
	r.run(name)
	r.expectCode(1)
	r.expectStdoutHas(shell.CutMarker + ":1:")
	r.expectStdoutLacks("over.md")
	for _, line := range strings.Split(strings.TrimRight(r.stdout.String(), "\n"), "\n") {
		reported, _, _ := strings.Cut(line, ":")
		if len(reported) > maxPathBytes {
			t.Errorf("the reported path is %d bytes, over the %d-byte bound", len(reported), maxPathBytes)
		}
	}
}

// No case may read the configuration of whoever runs the suite. The values TestMain saved are what
// the assertion runs against, so the case names exactly what a leak reached.
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
