package ecocheck

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
