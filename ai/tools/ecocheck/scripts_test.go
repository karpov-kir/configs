package ecocheck_test

// Script test position: every script states the -test.sh covering it or why it has none.

import (
	"fmt"
	"strings"
	"testing"
)

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
		f.reports("names more suites than the scan reads")
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
		f.write(f.root+"/skills/buried.sh", buried.String())
		f.chmod(f.root+"/skills/buried.sh", 0o755)
		f.reports(noPosition)
	})

	// The suite list is built from filenames the reviewed tree chose. A newline in one splits a
	// basename in two, the tail reads as a suite that exists, and a header naming an absent suite then
	// passes. The control case comes first: without the hostile file, the finding must be there to
	// lose.
	t.Run("reports a named suite that is absent (control for the case below)", func(t *testing.T) {
		newGhostSuiteScript(t, false).reports(missingTest)
	})

	t.Run("a newline in a filename cannot forge the suite that satisfies a header", func(t *testing.T) {
		newGhostSuiteScript(t, true).reports(missingTest)
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

func newCoveredScript(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("covered.sh", "#!/usr/bin/env bash\n# A change here needs a case in covered-test.sh beside it.\ntrue")
	f.newScript("covered-test.sh", "#!/usr/bin/env bash\ntrue")
	return f
}

func newGhostSuiteScript(t *testing.T, forgeTheSuite bool) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("tool.sh", "#!/usr/bin/env bash\n# a change here needs a case in ghost-test.sh\ntrue")
	if forgeTheSuite {
		f.newFileWithNewlineName(f.root+"/skills/x\nghost-test.sh", "not a suite", "the forged-suite-name case")
	}
	return f
}

func newDashSuiteScript(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("dash.sh", "#!/usr/bin/env bash\n# a change here needs a case in --test.sh\ntrue")
	return f
}
