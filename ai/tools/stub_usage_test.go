// A stub's header documents a usage line and the binary behind it prints one when it refuses a bad
// invocation. Nothing else compares the two: each binary's line is asserted in its own suite and each
// stub's is only grepped for its lowercase prefix by tool-stub-test.sh, so the two flag lists could
// drift apart with both suites green.
//
// Each binary's line is taken by driving the refusal rather than by reading the source for a literal:
// a test that greps the constant out of the package would agree with the code however wrong the
// printed text is.
package tools_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	repokey "kk-flavor/tools/repo-key"
)

// A stub whose binary prints a usage line, with the invocation that makes it print one. Two roots is
// the cheapest refusal that is not also a resolution failure, so the line under test is the usage one
// rather than a message about the tree.
type usagePrinter struct {
	stub        string
	run         func(args []string, out, errOut io.Writer) int
	refusedArgs []string
}

func (p usagePrinter) base() string {
	return filepath.Base(p.stub)
}

var usagePrinters = []usagePrinter{
	{"../kk-flavor/skills/kk-ecosystem/scripts/check.sh", ecocheck.Run, []string{"--agent=claude", "one", "two"}},
	{"../kk-flavor/scripts/repo-key.sh", repokey.Run, []string{"one", "two"}},
}

func TestEveryStubDocumentsTheUsageItsBinaryPrints(t *testing.T) {
	for _, printer := range usagePrinters {
		printed := refusedUsage(t, printer)
		if printed == "" {
			t.Errorf("%s printed no usage line when refused, so this case would pass against any stub at all", printer.base())
			continue
		}
		if documented := documentedUsage(t, printer.stub); documented != printed {
			t.Errorf("%s documents a usage line its binary does not print\n  stub: %q\nbinary: %q\n"+
				"one of the two grew a flag the other did not", printer.base(), documented, printed)
		}
	}
}

// What a tool says when it refuses an invocation it cannot parse, with the tool's own name-prefix cut
// off so what is left is comparable with a stub header.
func refusedUsage(t *testing.T, printer usagePrinter) string {
	t.Helper()
	var output bytes.Buffer
	if status := printer.run(printer.refusedArgs, &output, &output); status != 2 {
		t.Fatalf("expected exit 2 from a refused invocation of %s, got %d\n%s", printer.base(), status, output.String())
	}
	for _, line := range strings.Split(output.String(), "\n") {
		if after, found := strings.CutPrefix(line, printer.base()+": "); found && strings.HasPrefix(after, "usage: ") {
			return after
		}
	}
	return ""
}

// The usage line a stub's header states, with the comment marker and the trailing prose stripped. The
// stub writes it as `#   usage: <line>   # <what the argument means>`, so the run of spaces before the
// second marker is the boundary — a single space cannot be one, since the usage text holds those.
func documentedUsage(t *testing.T, stub string) string {
	t.Helper()
	body, err := os.ReadFile(stub)
	if err != nil {
		t.Fatalf("read %s: %v", stub, err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimLeft(strings.TrimPrefix(strings.TrimSpace(line), "#"), " ")
		if !strings.HasPrefix(trimmed, "usage: ") {
			continue
		}
		if cut := strings.Index(trimmed, "   #"); cut >= 0 {
			trimmed = trimmed[:cut]
		}
		return strings.TrimRight(trimmed, " ")
	}
	t.Fatalf("%s states no usage line, so the stub documents nothing to compare", stub)
	return ""
}
