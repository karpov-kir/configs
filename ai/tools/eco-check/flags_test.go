package ecocheck_test

// The suite over flags.go: an instruction file names `<script> --<flag>` and the script's own usage
// line has to name that flag. Every presence case here is written so the finding disappears when the
// usage line grows the flag, and every absence case sits on a fixture that raises a finding of the
// same kind for a neighbouring reason — an absence asserted over a tree that produces nothing at all
// passes against a scan that never ran.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	"kk-flavor/tools/shell"
)

// The half of the flag finding that names the flag and the script, which is what a case asserting
// "this one was reported" and one asserting "and that one was not" both turn on. The kind and the
// located `<file>:<line>` in front of it are pinned whole by TestAFlagFindingNamesTheCallSiteItWasReadFrom;
// repeating them in every needle would tie a case about the separator to the fixture's own line count.
func flagFinding(flag string) string { return " — " + flag + " is passed to " }

const (
	undocumentedFlag = ecocheck.FlagUsageDoesNotName
	noUsageLine      = ecocheck.FlagCallSitesNotChecked
	flagsAtTheBound  = ecocheck.FlagScanAtItsBound
)

// A script whose header states the given usage line, under the skills tree so the fixture's own walk
// reaches it.
// The suite toy.sh names, so the test-position scan stays quiet and a case reading the whole output is
// not reading that scan's finding by mistake. Every fixture here needs it, including the ones that
// build toy.sh themselves rather than through newFlagScript.
func (f *fixture) newFlagSuiteScript() {
	f.t.Helper()
	f.newScript("toy-test.sh", "#!/usr/bin/env bash\n# usage: toy-test.sh\n# untested: it is the suite\ntrue")
}

func newFlagScript(t *testing.T, usage string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("toy.sh", "#!/usr/bin/env bash\n"+usage+"\n# tested by: toy-test.sh\ntrue")
	f.newFlagSuiteScript()
	return f
}

// One instruction file naming commands, each line of it a backticked span. Line N of the file holds
// the Nth command, which is what the located half of a finding names.
func (f *fixture) newCallSites(lines ...string) {
	f.t.Helper()
	f.newCallSitesIn("kk-flavor/standards/how.md", lines...)
}

func (f *fixture) newCallSitesIn(path string, lines ...string) {
	f.t.Helper()
	var doc strings.Builder
	for _, line := range lines {
		fmt.Fprintf(&doc, "- run `%s` first\n", line)
	}
	full := f.root + "/" + path
	f.mkdirAll(filepath.Dir(full))
	f.write(full, doc.String())
}

func TestFlagCallSites(t *testing.T) {
	t.Run("fires on a flag an instruction file names and the usage line does not", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --agent=claude")
		f.reports(flagFinding("--agent"))
	})

	// The undocumented flag beside it is this case's control: asserting only that `--gate` goes
	// unreported passes just as well against a scan that reported nothing at all.
	t.Run("accepts one that usage line does name", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --gate --agent=claude")
		output := f.run()
		f.found(output, flagFinding("--agent"))
		f.absent(output, flagFinding("--gate"))
	})

	// `--agent=claude` at the call site and `--agent=claude|codex` in the usage line are one flag. A
	// scan comparing the whole token reports every flag that takes a value.
	t.Run("matches a flag on its name and not on the value beside it", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh --agent=claude|codex [<root>]")
		f.newCallSites("toy.sh --agent=codex --nope")
		output := f.run()
		f.found(output, flagFinding("--nope"))
		f.absent(output, flagFinding("--agent"))
	})

	t.Run("reads a usage grammar wrapped across comment lines", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]\n#          [--agent=claude|codex]")
		f.newCallSites("toy.sh --agent=claude --nope")
		output := f.run()
		f.found(output, flagFinding("--nope"))
		f.absent(output, flagFinding("--agent"))
	})

	// The line under a usage block at the header's OWN indent is commentary, not the grammar. Read as
	// part of it, every flag the header mentions in passing counts as documented and the scan stops
	// asking the only question it has.
	t.Run("does not take a flag from the prose line beside the usage block", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]\n# Run --help for the rest.")
		f.newCallSites("toy.sh --help")
		f.reports(flagFinding("--help"))
	})

	// The `   #` commentary ai/tools/stub_usage_test.go cuts off the stub's line is cut here too, so
	// the two halves of the chain read the same usage line.
	t.Run("and none from the commentary after the usage line's own second marker", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]   # --full is the old name for it")
		f.newCallSites("toy.sh --full")
		f.reports(flagFinding("--full"))
	})

	// Lowercase, the spelling subcommands.go and tool-stub-test.sh already hold every script to. A
	// capitalised `Usage:` is a script with no usage line, which is the other finding and not a pass.
	t.Run("reads a capitalised Usage: as no usage line at all", func(t *testing.T) {
		f := newFlagScript(t, "#   Usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --gate")
		f.reports(noUsageLine)
	})

	t.Run("reports a script that states no usage line rather than the flag it could not check", func(t *testing.T) {
		f := newFlagScript(t, "# no grammar here")
		f.newCallSites("toy.sh --gate")
		output := f.run()
		f.found(output, noUsageLine+f.root+"/kk-flavor/standards/how.md:1 — "+f.root+"/kk-flavor/skills/toy.sh")
		f.absent(output, undocumentedFlag)
	})

	// A `# usage:` inside a function body is not the script's documented usage line. leadingCommentBlock
	// is what stops the read; without it a script could document a flag anywhere in its own source.
	t.Run("does not read a usage line written past the leading comment block", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("toy.sh", "#!/usr/bin/env bash\n# tested by: toy-test.sh\ntrue\n#   usage: toy.sh [--gate]")
		f.newFlagSuiteScript()
		f.newCallSites("toy.sh --gate")
		f.reports(noUsageLine)
	})
}

func TestWhereAFlagCallSiteIsRead(t *testing.T) {
	t.Run("in a fenced block, which is what a session copies", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.write(f.root+"/kk-flavor/standards/how.md", "```sh\ntoy.sh --agent=claude\n```\n")
		f.reports(flagFinding("--agent"))
	})

	t.Run("and behind the path an instruction file writes in front of the script", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("~/.kk-flavor/skills/toy.sh --agent=claude")
		f.reports(flagFinding("--agent"))
	})

	// Prose is not a command. A sentence naming the script and, later, a flag of something else would
	// otherwise be read as one call site — so the presence half of this case runs on the same tree.
	t.Run("and never in prose outside a span, which names no command", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.write(f.root+"/kk-flavor/standards/how.md",
			"Run toy.sh --loose when ready, or `toy.sh --tight` instead.\n")
		output := f.run()
		f.found(output, flagFinding("--tight"))
		f.absent(output, flagFinding("--loose"))
	})

	// `toy.sh --agent | grep --color` is two commands, and `--color` is grep's. Read as one, the scan
	// reports toy.sh for a flag nothing ever passed it — and the flag of whatever the tree pipes into
	// becomes a defect in the script upstream of the pipe.
	t.Run("and stops at the separator that starts the next command", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --agent=claude | grep --color")
		output := f.run()
		f.found(output, flagFinding("--agent"))
		f.absent(output, flagFinding("--color"))
	})
}

// A finding that names only the script sends its reader to grep every instruction file for the call.
// The citation scan prints `<file>:<line> -> …` in the same output, and these hold this scan to it.
func TestAFlagFindingNamesTheCallSiteItWasReadFrom(t *testing.T) {
	// Whole, so the sentence is pinned and not just the pieces: the located file, the line the call
	// actually sits on rather than the first line of the file, and the script it was checked against.
	t.Run("names the instruction file, the line and the script, in one sentence", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --gate", "toy.sh --agent=claude")
		f.reports(undocumentedFlag + f.root + "/kk-flavor/standards/how.md:2 — --agent is passed to " +
			f.root + "/kk-flavor/skills/toy.sh, whose usage line does not name it")
	})

	// Most of this tree's call sites are fenced rather than backticked — every one in ai/README.md is —
	// so the locator is pinned on the branch that carries it, not only on the one beside it.
	t.Run("and does the same for a call site inside a fenced block", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.write(f.root+"/kk-flavor/standards/how.md",
			"Prose first.\n\n```sh\ntoy.sh --gate\ntoy.sh --agent=claude\n```\n")
		f.reports(undocumentedFlag + f.root + "/kk-flavor/standards/how.md:5 — --agent is passed to " +
			f.root + "/kk-flavor/skills/toy.sh, whose usage line does not name it")
	})

	// The same pair in two files is one defect and one fix, so it is one finding, located at the first
	// file the walk reached. Reported once per call site instead, a single flag named in every skill
	// spends its whole rank on copies of one sentence.
	t.Run("and reports one pair once, at the first file that names it", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSitesIn("kk-flavor/standards/how.md", "toy.sh --agent=claude")
		f.newCallSitesIn("kk-flavor/standards/zz-later.md", "toy.sh --agent=claude")
		count, output := f.countLinesStartingWith(undocumentedFlag)
		if count != 1 {
			t.Fatalf("want the pair reported once, got %d:\n%s", count, output)
		}
		f.found(output, undocumentedFlag+f.root+"/kk-flavor/standards/how.md:1")
	})

	// A script with no usage line is one defect too, whatever it is passed. Its finding is the one
	// place a flood could come from, because every call site reaching such a script hits the same case.
	t.Run("and reports a script that states no usage line once, not once per flag", func(t *testing.T) {
		f := newFlagScript(t, "# no grammar here")
		f.newCallSites("toy.sh --gate", "toy.sh --agent=claude", "toy.sh --loud")
		count, output := f.countLinesStartingWith(noUsageLine)
		if count != 1 {
			t.Fatalf("want the script reported once, got %d:\n%s", count, output)
		}
		f.found(output, noUsageLine+f.root+"/kk-flavor/standards/how.md:1")
	})
}

// The two call sites this scan has no authority to check, and why neither is silence: another scan
// already names each. Both cases carry a real finding on the same tree, so they cannot pass over a
// scan that produced nothing.
func TestAFlagCallSiteWithNoOneScriptBehindIt(t *testing.T) {
	t.Run("says nothing about a script this tree does not hold", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --agent=claude", "nosuch.sh --agent=claude")
		output := f.run()
		f.found(output, flagFinding("--agent"))
		f.absent(output, "nosuch.sh")
	})

	// Two scripts of one basename: subcommands.go already names both files, and a flag finding reached
	// through the basename could not say which of them it was about.
	t.Run("and nothing about a basename two scripts answer to", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newScript("nested/toy.sh", "#!/usr/bin/env bash\n#   usage: toy.sh [--gate]\n# tested by: toy-test.sh\ntrue")
		f.newCallSites("toy.sh --agent=claude")
		output := f.run()
		f.absent(output, flagFinding("--agent"))
		f.found(output, ecocheck.SubcommandCallSitesNotChecked)
	})
}

func TestTheFlagScanStaysWithinItsBounds(t *testing.T) {
	// Past the bound the rest are reported and not checked. Silence here would be a call site that
	// went unchecked reading exactly like one that passed.
	t.Run("says how many call sites it withheld past its bound", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		var sites []string
		for i := range ecocheck.FlagCallSiteCap + 44 {
			sites = append(sites, fmt.Sprintf("toy.sh --f%04d", i))
		}
		f.newCallSites(sites...)
		f.reports(fmt.Sprintf(flagsAtTheBound+" %d-call-site bound: 44 more were NOT checked", ecocheck.FlagCallSiteCap))
	})

	// The count is of call sites and not of the distinct pairs among them, because past the bound
	// nothing enters the dedupe map — a map that keeps growing with the tree is what the bound exists
	// to stop. Five call sites nobody checked is what "5 more were NOT checked" has to say.
	t.Run("and counts every call site it withheld, not the pairs among them", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		var sites []string
		for i := range ecocheck.FlagCallSiteCap {
			sites = append(sites, fmt.Sprintf("toy.sh --f%04d", i))
		}
		for range 5 {
			sites = append(sites, "toy.sh --over-the-bound")
		}
		f.newCallSites(sites...)
		f.reports(fmt.Sprintf(flagsAtTheBound+" %d-call-site bound: 5 more were NOT checked", ecocheck.FlagCallSiteCap))
	})

	// The flag name is the instruction file's own text and nothing bounds its length. Uncut, its tail
	// takes the printer's 500-byte bound and the sentence naming the defect goes with it. Marked,
	// because an unmarked cut leaves a shorter wrong flag name reading as a whole one.
	t.Run("and marks a flag name the instruction file made too long to print", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --" + strings.Repeat("a", 400))
		f.reports(shell.CutMarker + " is passed to " + f.root + "/kk-flavor/skills/toy.sh, whose usage line")
	})

	// Without this the case above would pass on a checker that marked every flag name, cut or not.
	t.Run("and leaves one that fits whole, with no mark on it", func(t *testing.T) {
		f := newFlagScript(t, "#   usage: toy.sh [--gate]")
		f.newCallSites("toy.sh --agent=claude")
		f.doesNotReport(shell.CutMarker + " is passed to ")
	})
}

// readLines refuses a file over the read bound by naming it and handing back no lines and no error.
// A scan that reads only the error then describes a file it never opened, and the run says both "it was
// NOT checked" and "it states no usage line" about one script. The second is a claim, and no run may
// make one about content it refused to read.
func TestAScriptTheFlagScanCouldNotRead(t *testing.T) {
	t.Run("is not reported as one that states no usage line", func(t *testing.T) {
		f := newRoot(t)
		f.newFlagSuiteScript()
		f.newScript("toy.sh", "#!/usr/bin/env bash\n#   usage: toy.sh [--gate]\n# tested by: toy-test.sh\ntrue")
		f.writeOversize(f.root + "/kk-flavor/skills/toy.sh")
		f.newCallSites("toy.sh --agent=claude")
		output := f.run()
		// The control: the refusal itself is reported, so the two absences below are absences from a
		// run that did reach this script rather than from one that never saw it.
		f.found(output, ecocheck.FileTooLargeToScan+f.root+"/kk-flavor/skills/toy.sh")
		f.absent(output, noUsageLine)
		f.absent(output, undocumentedFlag)
	})
}

// The path half of a finding is the tree's own text, and a committed directory name carries whatever
// bytes its author chose. The control is not optional: without it the second half passes on a run that
// raised no finding at all.
func TestAFlagFindingCarriesNoControlByte(t *testing.T) {
	assertNoControlByteEscapes(t, "a flag under a directory named with an escape", undocumentedFlag,
		func(t *testing.T) *fixture {
			f := newRoot(t)
			f.newScript("we\x1b[2Kird/toy.sh", "#!/usr/bin/env bash\n#   usage: toy.sh [--gate]\n# tested by: toy-test.sh\ntrue")
			f.newFlagSuiteScript()
			f.newCallSites("toy.sh --agent=claude")
			return f
		})

	// The located half is the reviewed tree's text as much as the script path is, and it is the half
	// a call site's own author chose the directory of.
	assertNoControlByteEscapes(t, "a call site in a directory named with an escape", undocumentedFlag,
		func(t *testing.T) *fixture {
			f := newFlagScript(t, "#   usage: toy.sh [--gate]")
			f.newCallSitesIn("kk-flavor/standards/we\x1b[2Kird/how.md", "toy.sh --agent=claude")
			return f
		})
}
