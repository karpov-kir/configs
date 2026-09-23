package voicecheck

import (
	"strings"
	"testing"
)

func TestAuthoredWordsLeaveOutWhatTheAuthorDidNotWrite(t *testing.T) {
	template := map[string]bool{"## Summary": true, "Closes the ticket named below.": true,
		"- [ ] I have run the tests": true}
	body := strings.Join([]string{
		"## Summary",
		"<!-- Describe the change. -->",
		"Closes the ticket named below.",
		"- [x] I have run the tests",
		"Stack, base first: #12 ← #13 ← **#14 (this one)**",
		"Posting now reads the book's own claim first.",
		"```",
		"go test ./...",
		"```",
		"🤖 Generated with [Claude Code](https://claude.com/claude-code)",
	}, "\n")
	if got := AuthoredWords(body, template); got != 8 {
		t.Fatalf("%d authored words, want the eight of the one sentence the author wrote", got)
	}
}

func TestABodyOverItsBandIsOneFindingWithTheMedian(t *testing.T) {
	bands := map[string]band{KindPRBody: {words: 5, median: 3}}
	found := KindFindings("-", "one two three four five six", KindPRBody, bands, nil)
	if len(found) != 1 || found[0].Check != checkOverBand || !strings.Contains(found[0].Text, "median is 3") {
		t.Fatalf("found %v", found)
	}
	if found := KindFindings("-", "one two three", KindPRBody, bands, nil); len(found) != 0 {
		t.Fatalf("a body inside its band reports %v", found)
	}
}

func TestATestsSectionNarratingWhatRanIsAFinding(t *testing.T) {
	bands := map[string]band{KindPRBody: {words: 500, median: 48}}
	body := strings.Join([]string{
		"The poster reads the book first.",
		"## Tests",
		"Ran `npm run test` and all tests pass.",
		"CI is green.",
		"Drove a checkout against the staging ledger and read the posted row.",
		"## Notes",
		"All tests pass on the old branch too.",
	}, "\n")
	var lines []int
	for _, f := range KindFindings("-", body, KindPRBody, bands, nil) {
		if f.Check == checkTestsNarration {
			lines = append(lines, f.Line)
		}
	}
	if len(lines) != 2 || lines[0] != 3 || lines[1] != 4 {
		t.Fatalf("narration found on lines %v, want 3 and 4: the manual drive and the line outside the section stay", lines)
	}
}

func TestABandLineNeedsItsFiguresAndAReason(t *testing.T) {
	if _, b, err := parseBandLine("pr-body 90 40 # counted over thirty PRs", 1); err != nil || b.words != 90 || b.median != 40 {
		t.Fatalf("a well-formed band reads as %+v, %v", b, err)
	}
	for _, bad := range []string{"pr-body 90 40", "pr-body ninety 40 # r", "slack 90 40 # r", "pr-body 90 # r"} {
		if _, _, err := parseBandLine(bad, 1); err == nil {
			t.Errorf("band %q is accepted", bad)
		}
	}
}
