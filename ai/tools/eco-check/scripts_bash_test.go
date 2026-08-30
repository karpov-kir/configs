package ecocheck

// Half of what this package promises is that every script still parses, and it delivers that by
// forking bash. With no bash to fork, the parse loop ran zero times and added nothing — byte for byte
// the output a tree of clean scripts produces, under exit 0. Nothing anywhere said the question had
// not been asked.
//
// Driven through the checker rather than through Run, because the seam is the point: every machine
// this suite runs on has a bash, so the case cannot be built by taking one away.

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// A checkout holding one script, whose body the case chooses.
func newCheckerOverAScript(t *testing.T, body string) *checker {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{root + "/kk-flavor", root + "/skills"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(root+"/skills/broken.sh", []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	c, ok := newChecker(root)
	if !ok {
		t.Fatal("the fixture is not a checkout")
	}
	return c
}

func (c *checker) reported(needle string) bool {
	return strings.Contains(strings.Join(c.findings, "\n"), needle)
}

func TestNoBashToParseWithIsRefusedNotReportedAsClean(t *testing.T) {
	// The control, and it runs first: with the real lookup this script's syntax error is found. That
	// is what makes the silence below a scan that could not run rather than a fixture holding nothing.
	t.Run("finds the syntax error while a bash is there to find it", func(t *testing.T) {
		c := newCheckerOverAScript(t, "if then\n")
		if len(c.bashBinaries()) == 0 {
			t.Skip("no bash on this machine, so the control cannot be established")
		}
		c.scanScriptsParse()
		if !c.reported("syntax: ") {
			t.Fatalf("the fixture raised no syntax error, so the cases below prove nothing: %v", c.findings)
		}
		if len(c.unrunnable) != 0 {
			t.Errorf("a scan that did parse reported itself unrunnable: %v", c.unrunnable)
		}
	})

	// The defect. Zero binaries produced zero findings, which is what a clean tree produces.
	t.Run("says NO script was parsed when there is no bash to parse with", func(t *testing.T) {
		c := newCheckerOverAScript(t, "if then\n")
		c.bashBinaries = func() []string { return nil }
		c.scanScriptsParse()

		if c.reported("syntax: ") {
			t.Fatalf("a parse happened without a binary, so this case is not measuring the seam: %v", c.findings)
		}
		if len(c.unrunnable) == 0 {
			t.Fatal("zero parses were reported as a parse scan that found nothing")
		}
		if !strings.Contains(strings.Join(c.unrunnable, "\n"), "NO script was parsed") {
			t.Errorf("the refusal does not say what did not happen: %v", c.unrunnable)
		}
	})

	// And it has to leave through the exit. A reason on stderr that 0 rides out on is a reason nobody
	// reads: this runs as a gate, and callers read the code.
	t.Run("and the check exits 2 rather than on its finding count", func(t *testing.T) {
		c := newCheckerOverAScript(t, "if then\n")
		c.bashBinaries = func() []string { return nil }
		c.scanScriptsParse()

		var out, errOut bytes.Buffer
		if got := c.exitCode(&out, &errOut); got != 2 {
			t.Errorf("exitCode = %d, want 2 — 0 and 1 both say the tree was checked\n%s", got, out.String())
		}
		if !strings.Contains(errOut.String(), "NO script was parsed") {
			t.Errorf("stderr does not name what did not happen:\n%s", errOut.String())
		}
	})

	// The other direction: the refusal must not fire on a tree that was parsed, or every real run
	// exits 2 and the code stops meaning anything.
	t.Run("while a tree it could parse exits on its findings as before", func(t *testing.T) {
		c := newCheckerOverAScript(t, "if then\n")
		if len(c.bashBinaries()) == 0 {
			t.Skip("no bash on this machine, so there is nothing to parse with")
		}
		c.scanScriptsParse()

		var out, errOut bytes.Buffer
		if got := c.exitCode(&out, &errOut); got != 1 {
			t.Errorf("exitCode = %d, want 1 — the tree was checked and has a finding\n%s", got, out.String())
		}
	})
}
