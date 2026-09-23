package tools_test

// The standards this module's tools quote back to an agent say what the tools actually hand over.

// It lives here, and not beside the tool, for the reason shipped_tree_test.go's cases do.

import (
	"os"
	"strings"
	"testing"

	ecoreport "configs/ai/tools/eco-report"
)

const recordsStandardSource = repoRoot + "/ai/kk-flavor/standards/records.md"

// `ecoreport.JudgeCommand` and the standard's eviction rule are one command written in two files, and
// this case is the only thing holding them together. An agent runs what the note printed, and a reader
// checking the rule reads what the standard wrote. Drift to two judges or two provider defaults, and
// they disagree about evicting an entry, with the record silent on which judge pruned it.
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
