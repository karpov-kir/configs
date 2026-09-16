package ecocheck_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

// The fixture for the `--` in scripts.go's parse call, whose own comment says what a root opening
// with a dash costs when `bash -n` reads it as an option. Built at all because the root arrives as a
// literal argument, so its leading byte is the caller's to choose.
//
// The root has to be relative, because an absolute one always opens on `/`. That is what the chdir is
// for, and why this case cannot be built on the fixture every other case here uses.
func newDashLeadingRoot(t *testing.T) (root string, output string) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH, so nothing here parses a script at all")
	}
	base := t.TempDir()
	root = "-r"
	for _, dir := range []string{base + "/" + root + "/kk-flavor/standards", base + "/" + root + "/kk-flavor/skills"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(base+"/"+root+"/kk-flavor/inject.md", []byte("# Flavor\n"), 0o644); err != nil {
		t.Fatalf("write inject.md: %v", err)
	}
	// Broken on purpose: a parse that really opened the file has something to report, and one that
	// never got past bash's option handling has only bash's usage.
	script := base + "/" + root + "/kk-flavor/skills/broken.sh"
	if err := os.WriteFile(script, []byte("if then\n"), 0o755); err != nil {
		t.Fatalf("write broken.sh: %v", err)
	}
	t.Chdir(base)

	return root, runChecker(t, "--agent=claude", root)
}

func TestAScriptUnderADashLeadingRootIsParsedAndNotReadAsAnOption(t *testing.T) {
	t.Run("reports the script's own syntax error", func(t *testing.T) {
		root, output := newDashLeadingRoot(t)
		needle := ecocheck.SyntaxError + root + "/kk-flavor/skills/broken.sh: line 1: syntax error"
		if !strings.Contains(output, needle) {
			t.Errorf("expected a finding containing %q\n%s", needle, output)
		}
	})

	t.Run("and does not report bash refusing the path as an option", func(t *testing.T) {
		_, output := newDashLeadingRoot(t)
		if strings.Contains(output, "invalid option") {
			t.Errorf("bash was handed the path as an option and never opened the file\n%s", output)
		}
	})
}

func TestScriptTestPosition(t *testing.T) {
	t.Run("fires on a script naming neither a test nor an untested reason", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("lonely.sh", "#!/usr/bin/env bash\n# Does a thing.\ntrue")
		f.reports(noPosition)
	})

	t.Run("fires on a header naming a -test.sh that is not in the tree", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("claims.sh", "#!/usr/bin/env bash\n# A change here needs a case in claims-test.sh beside it.\ntrue")
		f.reports(missingTest)
	})

	t.Run("accepts a header whose named test exists", func(t *testing.T) {
		newCoveredScript(t).doesNotReport(missingTest)
	})

	t.Run("a named existing test is a declared position", func(t *testing.T) {
		newCoveredScript(t).doesNotReport(noPosition)
	})

	t.Run("accepts an explicit untested: declaration with a reason", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("waived.sh", "#!/usr/bin/env bash\n# untested: a four-line wrapper whose only failure mode is the exec bit.\ntrue")
		f.doesNotReport(noPosition)
	})

	t.Run("a bare untested: with no reason does not clear the check", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("bare.sh", "#!/usr/bin/env bash\n# untested:\ntrue")
		f.reports(noPosition)
	})

	// The harness is exempt: asking a test file to name its own test makes every one of them a finding.
	t.Run("asks nothing of -test.sh and -mutate.sh themselves", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("harness-test.sh", "#!/usr/bin/env bash\ntrue")
		f.newScript("harness-mutate.sh", "#!/usr/bin/env bash\ntrue")
		f.doesNotReport(noPosition)
	})

	// Header-scoped on purpose: a suite a script merely mentions in its body would read as coverage.
	t.Run("a -test.sh named below the header does not count as declared", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("body.sh", "#!/usr/bin/env bash\n# Does a thing.\nset -u\n# see also body-test.sh\ntrue")
		f.reports(noPosition)
	})

	// The cap that keeps a crafted header from turning one scan into thousands of whole-tree walks. It
	// has to *report*, never quietly read less than it looks like it read.
	t.Run("reports a header naming more suites than it reads", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("greedy.sh", "#!/usr/bin/env bash\n"+
			"# see n1-test.sh n2-test.sh n3-test.sh n4-test.sh n5-test.sh n6-test.sh\n"+
			"# and n7-test.sh n8-test.sh n9-test.sh n10-test.sh n11-test.sh n12-test.sh\ntrue")
		f.reports(ecocheck.ScriptNamesTooManySuites)
	})

	// The bound on the header read. A declaration past 200 lines is not seen, which is correct, and it
	// still has to be *reported* rather than pass as declared.
	t.Run("a declaration past the header bound does not clear the check", func(t *testing.T) {
		f := newRoot(t)
		var buried strings.Builder
		buried.WriteString("#!/usr/bin/env bash\n")
		for line := 1; line <= 205; line++ {
			fmt.Fprintf(&buried, "# padding %d\n", line)
		}
		buried.WriteString("# untested: this reason sits past the 200-line bound and cannot clear the check\n")
		buried.WriteString("true\n")
		f.write(f.root+"/kk-flavor/skills/buried.sh", buried.String())
		f.chmod(f.root+"/kk-flavor/skills/buried.sh", 0o755)
		f.reports(noPosition)
	})

	// The suite list is built from filenames the reviewed tree chose. A newline in one splits a
	// basename in two, the tail reads as a suite that exists, and a header naming an absent suite then
	// passes. The control case comes first: without the hostile file, the finding must be there to
	// lose.
	t.Run("reports a named suite that is absent (control for the case below)", func(t *testing.T) {
		newScriptNamingAnAbsentSuite(t).reports(missingTest)
	})

	t.Run("a newline in a filename cannot forge the suite that satisfies a header", func(t *testing.T) {
		newScriptWhoseSuiteAFilenameForges(t).reports(missingTest)
	})

	t.Run("a suite name starting with a dash is still checked", func(t *testing.T) {
		newDashSuiteScript(t).reports(missingTest)
	})

	t.Run("and grep never dumps its usage into the findings", func(t *testing.T) {
		newDashSuiteScript(t).doesNotReport("unrecognized option")
	})

	t.Run("nor its usage banner", func(t *testing.T) {
		newDashSuiteScript(t).doesNotReport("Usage: grep")
	})
}

// Every scan that reads a usage line anchors on a lowercase `usage:`, so a header writing `Usage:`
// and nothing lowercase is in none of them. Today the flag scan notices such a line only by accident
// — only where some instruction file happens to pass that script a flag — so the rule is remembered
// rather than enforced. These cases are what enforces it.
func TestAUsageLineNoScanCanRead(t *testing.T) {
	unread := ecocheck.ScriptUsageSpellingUnread

	t.Run("fires on a header stating Usage: and nothing lowercase", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("shout.sh", "#!/usr/bin/env bash\n# Usage: shout.sh [--gate]\n# untested: a fixture.\ntrue")
		f.reports(unread + f.root + "/kk-flavor/skills/shout.sh writes 'Usage:'")
	})

	t.Run("and on USAGE: as well", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("louder.sh", "#!/usr/bin/env bash\n# USAGE: louder.sh [--gate]\n# untested: a fixture.\ntrue")
		f.reports(unread + f.root + "/kk-flavor/skills/louder.sh writes 'USAGE:'")
	})

	// Without this the cases above pass against a scan that reports every script it walks.
	t.Run("stays silent on a header stating the lowercase line (control for the cases above)", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("quiet.sh", "#!/usr/bin/env bash\n# usage: quiet.sh [--gate]\n# untested: a fixture.\ntrue")
		f.doesNotReport(unread)
	})

	// The lowercase line is what those scans read, so a header carrying both is documented to them
	// and there is nothing here to report — whichever of the two is written first.
	t.Run("and on a header carrying both spellings", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("both.sh", "#!/usr/bin/env bash\n# Usage: both.sh [--gate]\n# usage: both.sh [--gate]\n# untested: a fixture.\ntrue")
		f.doesNotReport(unread)
	})

	// A script documenting nothing at all is the other finding, not this one. Saying "its spelling is
	// wrong" of a header with no usage line in it names a defect that is not the one there is.
	t.Run("asks nothing of a header stating no usage line at all", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("bare.sh", "#!/usr/bin/env bash\n# Does a thing.\n# untested: a fixture.\ntrue")
		f.doesNotReport(unread)
	})

	// Header-scoped, the way every other reader of a usage line is: a `Usage:` in the body is not a
	// header the scans were ever going to read.
	t.Run("does not read a Usage: below the header", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("body.sh", "#!/usr/bin/env bash\n# usage: body.sh\n# untested: a fixture.\nset -u\n# Usage: body.sh --wrong\ntrue")
		f.doesNotReport(unread)
	})

	// The test-position scan exempts the harness; this one does not. A -test.sh header is as invisible
	// to those scans as any other file's.
	t.Run("holds a -test.sh to the same spelling", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("harness-test.sh", "#!/usr/bin/env bash\n# Usage: harness-test.sh\ntrue")
		f.reports(unread + f.root + "/kk-flavor/skills/harness-test.sh")
	})
}

// A header writes its suite as a basename, and the scan has to reach a file from it. Two lanes
// carrying one `-test.sh` name weld into a name that answers for both, and a header naming it was
// then satisfied by a suite in the other lane that never sees this script — the same defect as naming
// a suite that does not exist, one step subtler, because the phase that runs it does find something.
func TestANamedSuiteResolvesToAFileAndNotToABasename(t *testing.T) {
	// The script naming the suite sits beside neither carrier, so only the basename connects them.
	newSuiteNameOneLaneCarries := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.newScript("one/scripts/shared-test.sh", "#!/usr/bin/env bash\ntrue")
		f.newScript("three/scripts/tool.sh", "#!/usr/bin/env bash\n# a change here needs a case in shared-test.sh\ntrue")
		return f
	}

	newSuiteNameTwoLanesCarry := func(t *testing.T) *fixture {
		f := newSuiteNameOneLaneCarries(t)
		f.newScript("two/scripts/shared-test.sh", "#!/usr/bin/env bash\ntrue")
		return f
	}

	// Without this the case below passes on a scan that calls every named suite ambiguous.
	t.Run("accepts a suite name only one file answers to (control for the case below)", func(t *testing.T) {
		newSuiteNameOneLaneCarries(t).doesNotReport(welded)
	})

	t.Run("reports one two files answer to rather than picking either", func(t *testing.T) {
		newSuiteNameTwoLanesCarry(t).reports(welded)
	})

	// Not reported as missing: the name does answer to files, and a reader sent to write a suite that
	// is already there twice would look for a defect that is not the one there is.
	t.Run("and does not call that name missing", func(t *testing.T) {
		newSuiteNameTwoLanesCarry(t).doesNotReport(missingTest)
	})

	// The sibling is what "a case in <suite> beside it" names, so the tree answers which file was
	// meant and there is nothing left to report — even while another lane carries the same name.
	t.Run("resolves a shared name through the suite sitting beside the script", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("one/scripts/shared-test.sh", "#!/usr/bin/env bash\ntrue")
		f.newScript("two/scripts/shared-test.sh", "#!/usr/bin/env bash\ntrue")
		f.newScript("two/scripts/tool.sh", "#!/usr/bin/env bash\n# a change here needs a case in shared-test.sh\ntrue")
		f.doesNotReport(welded)
	})
}

// Every finding of this scan names the script by path. Named by basename, two lanes' findings were
// byte-identical, and identical findings collapse in the sort: one of the two scripts went unmentioned
// by the check that had just found it.
func TestATestPositionFindingNamesTheScriptByPath(t *testing.T) {
	newTwoScriptsUnderOneName := func(t *testing.T) *fixture {
		f := newRoot(t)
		for _, lane := range []string{"one", "two"} {
			f.newScript(lane+"/scripts/claims.sh",
				"#!/usr/bin/env bash\n# a change here needs a case in claims-test.sh beside it.\ntrue")
		}
		return f
	}

	t.Run("reports both scripts, not one of them twice", func(t *testing.T) {
		f := newTwoScriptsUnderOneName(t)
		if count, output := f.countLinesStartingWith(missingTest + ":"); count != 2 {
			t.Errorf("expected one finding per script, got %d\n%s", count, indent(output))
		}
	})

	t.Run("and names a path a reader can open", func(t *testing.T) {
		f := newTwoScriptsUnderOneName(t)
		f.reports(missingTest + ": " + f.root + "/kk-flavor/skills/one/scripts/claims.sh names claims-test.sh")
	})
}

// `bash -n` quotes the script's path and its own text back, and the path is a filename the reviewed
// tree chose. That is the one message built from bytes this checker did not write, and it went through
// a hand-rolled control-byte range rather than the definition every other message uses.
func TestParseErrorsCarryNoControlByte(t *testing.T) {
	newEscapedScriptName := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.newScript("ev\x1b[2Kil.sh", "if then")
		return f
	}

	assertNoControlByteEscapes(t, "the syntax error", ecocheck.SyntaxError, newEscapedScriptName)
}

// `bash -n` reads the script and nothing else, so two files holding the same bytes have the same
// answer and the second needs no process of its own. What must never follow from that is a broken
// script inheriting a clean one's silence, which is why only the clean answer is held.
//
// Every case here checks the tree twice, because the memo is held for the process and one run cannot
// observe it: the parse workers reach both copies of a script at once, and neither has stored
// anything yet. Every fixture carries a marker line of its own for the same reason — the cases share
// one memo, so a fixture reusing another's bytes would pass on the answer that case's fork left.
func TestRepeatedScriptContentIsParsedOnce(t *testing.T) {
	// The half that would be a silent hole. Both copies are reported by their own path on a run where
	// the bytes have been seen before, or a tree hides a broken script behind a clean one.
	t.Run("reports a broken script on a run that has already parsed its bytes", func(t *testing.T) {
		f := newRepeatedScript(t, "repeated-broken", "if then")
		f.reportsOnASecondRun(f.root + "/kk-flavor/skills/second.sh: line 2")
	})

	t.Run("and reports the first copy of it too", func(t *testing.T) {
		f := newRepeatedScript(t, "repeated-broken-b", "if then")
		f.reportsOnASecondRun(f.root + "/kk-flavor/skills/first.sh: line 2")
	})

	// Keyed on the whole content, not on a stand-in for it. The two scripts below are the same length
	// and differ by their last byte, so a memo keyed on anything coarser answers for both.
	t.Run("parses a script differing from a clean one by its last byte alone", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("clean.sh", "# marker: one-byte\ntrue; :")
		f.newScript("broken.sh", "# marker: one-byte\ntrue; (")
		f.reportsOnASecondRun(ecocheck.SyntaxError)
	})

	t.Run("stays quiet on two copies of a script that parses", func(t *testing.T) {
		newRepeatedScript(t, "repeated-clean", "# untested: fixture\ntrue").doesNotReportOnASecondRun(ecocheck.SyntaxError)
	})
}

// Two scripts holding the same bytes, marked so no other case's fork can answer for them.
func newRepeatedScript(t *testing.T, marker, body string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("first.sh", "# marker: "+marker+"\n"+body)
	f.newScript("second.sh", "# marker: "+marker+"\n"+body)
	return f
}

// A script is parsed under every bash `#!/usr/bin/env bash` could resolve to, because macOS still
// ships 3.2 as /bin/bash and it rejects what bash 5 accepts. The memo is per binary for that reason,
// and this is the case that says so: `|&` parses under bash 4 and later and is a syntax error before
// it, so one binary answering for the other loses the finding entirely.
func TestEachBashVersionIsAskedSeparately(t *testing.T) {
	if !refusesTheBash4Pipe(t, "/bin/bash") || refusesTheBash4Pipe(t, "bash") {
		t.Skip("this machine has no pair of bash binaries that disagree about `|&`, so nothing here separates them")
	}
	f := newRoot(t)
	f.newScript("v4.sh", "# untested: fixture\ntrue |& cat")
	f.reportsOnASecondRun(ecocheck.SyntaxError)
}

func refusesTheBash4Pipe(t *testing.T, binary string) bool {
	t.Helper()
	path := t.TempDir() + "/probe.sh"
	if err := os.WriteFile(path, []byte("true |& cat\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return exec.Command(binary, "-n", path).Run() != nil
}

func newCoveredScript(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("covered.sh", "#!/usr/bin/env bash\n# A change here needs a case in covered-test.sh beside it.\ntrue")
	f.newScript("covered-test.sh", "#!/usr/bin/env bash\ntrue")
	return f
}

func newScriptNamingAnAbsentSuite(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("tool.sh", "#!/usr/bin/env bash\n# a change here needs a case in ghost-test.sh\ntrue")
	return f
}

// The same script, with a committed filename whose second line reads as the suite it names.
func newScriptWhoseSuiteAFilenameForges(t *testing.T) *fixture {
	t.Helper()
	f := newScriptNamingAnAbsentSuite(t)
	f.newFileWithNewlineName(f.root+"/kk-flavor/skills/x\nghost-test.sh", "not a suite", "the forged-suite-name case")
	return f
}

func newDashSuiteScript(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("dash.sh", "#!/usr/bin/env bash\n# a change here needs a case in --test.sh\ntrue")
	return f
}
