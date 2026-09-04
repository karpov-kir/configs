// A stub's header documents a usage line and the binary behind it prints one when it refuses a bad
// invocation. Nothing else compares the two: the binary's is asserted in eco-check's own suite and the
// stub's is only grepped for its lowercase prefix by tool-stub-test.sh, so the flag list in each could
// drift apart with both suites green.
//
// The binary's line is taken by driving the refusal rather than by reading the source for a literal:
// a test that greps the constant out of eco-check.go would agree with the code however wrong the
// printed text is.
package tools_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

func TestTheStubDocumentsTheUsageItsBinaryPrints(t *testing.T) {
	printed := refusedUsage(t)
	if printed == "" {
		t.Fatal("eco-check printed no usage line when refused, so this case would pass against any stub at all")
	}
	documented := documentedUsage(t, "../skills/kk-ecosystem/scripts/check.sh")
	if documented != printed {
		t.Errorf("check.sh documents a usage line its binary does not print\n stub: %q\nbinary: %q\n"+
			"one of the two grew a flag the other did not", documented, printed)
	}
}

// What the tool says when it refuses an invocation it cannot parse. Two roots is the cheapest refusal
// that is not also a root-resolution failure, so the line under test is the usage one.
func refusedUsage(t *testing.T) string {
	t.Helper()
	var output bytes.Buffer
	if status := ecocheck.Run([]string{"one", "two"}, &output, &output); status != 2 {
		t.Fatalf("expected exit 2 from a refused invocation, got %d\n%s", status, output.String())
	}
	for _, line := range strings.Split(output.String(), "\n") {
		if _, found := strings.CutPrefix(line, "check.sh: usage: "); found {
			return strings.TrimPrefix(line, "check.sh: ")
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
