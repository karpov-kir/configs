package tools_test

// The standards this module's tools quote back to an agent, held to what the tools actually hand over.
//
// It lives here rather than beside the tool because `kk-flavor/standards/` is outside the module: Go
// keys its test cache on the module, so a package that reads a standard answers `ok (cached)` over one
// that has changed under it. This package is allowed to read the checkout, and the gate runs it with
// the out-of-module inputs it carries.

import (
	"os"
	"strings"
	"testing"

	ecoreport "kk-flavor/tools/eco-report"
)

const recordsStandardSource = repoRoot + "/ai/kk-flavor/standards/records.md"

// `ecoreport.JudgeCommand` and the standard's eviction rule are the same command written in two files,
// and nothing but this holds them together: an agent runs what the note printed, while a reader
// checking the rule reads what the standard wrote. Two judges, or two provider defaults, and they
// disagree about what evicting an entry means with nothing to say which one the record was pruned by.
func TestTheStandardQuotesTheJudgeCommandTheNotesHandOver(t *testing.T) {
	raw, err := os.ReadFile(recordsStandardSource)
	if err != nil {
		t.Fatal(err)
	}
	command, _, _ := strings.Cut(ecoreport.JudgeCommand, "  #")
	if !strings.Contains(string(raw), command) {
		t.Fatalf("%s does not carry %q, which judgeRung hands an agent", recordsStandardSource, command)
	}
}
