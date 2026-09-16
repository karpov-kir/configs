package ecoreport

import (
	"os"
	"strings"
	"testing"
)

// This checkout's copy of the standard, reached the way `harness_test.go` reaches the rest of
// kk-flavor and for the reason stated there.
const recordsStandardSource = "../../kk-flavor/standards/records.md"

// `judgeCommand` and the standard's eviction rule are the same command written in two files, and
// nothing but this holds them together: an agent runs what the note printed, while a reader checking
// the rule reads what the standard wrote. Two judges, or two provider defaults, and they disagree
// about what evicting an entry means with nothing to say which one the record was pruned by.
func TestTheStandardQuotesTheJudgeCommandTheNotesHandOver(t *testing.T) {
	raw, err := os.ReadFile(recordsStandardSource)
	if err != nil {
		t.Fatal(err)
	}
	command, _, _ := strings.Cut(judgeCommand, "  #")
	if !strings.Contains(string(raw), command) {
		t.Fatalf("%s does not carry %q, which judgeRung hands an agent", recordsStandardSource, command)
	}
}
