package voicecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func authoredWords(lines []string) int {
	return len(strings.Fields(strings.Join(lines, " ")))
}

// A template filled with thirty words of the author's own reads as those thirty words. The template's
// headings, its ticked checklist, its comments, the stack map and a generated line belong to the
// template and the tooling.
func TestAuthoredLinesAreTheAuthorsAlone(t *testing.T) {
	template := map[string]bool{"## Summary": true, "## Tests": true, "- [ ] I have run the tests": true,
		"Closes the ticket named below.": true}
	thirty := "Posting reads the book's claim before the probe's, because the probe reported a format years after the book took it, and the ledger team trusts the book as its source."
	if n := len(strings.Fields(thirty)); n != 30 {
		t.Fatalf("the fixture's own sentence has %d words", n)
	}
	body := strings.Join([]string{
		"## Summary",
		"<!-- Describe the change.",
		"     Keep it short. -->",
		"Closes the ticket named below.",
		"Stack, base first: #12 ← #13 ← **#14 (this one)**",
		thirty,
		"## Tests",
		"- [x] I have run the tests",
		"```",
		"go test ./...",
		"```",
		"🤖 Generated with [Claude Code](https://claude.com/claude-code)",
	}, "\n")
	lines := AuthoredLines(body, template)
	if got := authoredWords(lines); got != 30 {
		t.Fatalf("%d authored words, want the thirty the author wrote:\n%q", got, lines)
	}
	if lines[5] != thirty {
		t.Fatalf("the authored sentence moved off its line: %q", lines)
	}
}

// Each kind reads its own template off the repository: the PR template for a PR body, the issue
// templates for a ticket.
func TestAKindReadsItsOwnTemplate(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		".github/PULL_REQUEST_TEMPLATE.md": "## What changed\n",
		".github/ISSUE_TEMPLATE/bug.md":    "## Steps to reproduce\n",
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if pr := templateLines(root, KindPRBody); !pr["## What changed"] || pr["## Steps to reproduce"] {
		t.Fatalf("the PR body read %v", pr)
	}
	if ticket := templateLines(root, KindTicket); !ticket["## Steps to reproduce"] || ticket["## What changed"] {
		t.Fatalf("the ticket read %v", ticket)
	}
}

func TestATestsSectionNarratingWhatRanIsAFinding(t *testing.T) {
	raw := strings.Split(strings.Join([]string{
		"The poster reads the book first.",
		"## Tests",
		"Ran `npm run test` and all tests pass.",
		"CI is green.",
		"Drove a checkout against the staging ledger and read the posted row.",
		"## Notes",
		"All tests pass on the old branch too.",
	}, "\n"), "\n")
	var lines []int
	for _, f := range KindFindings("-", raw, AuthoredLines(strings.Join(raw, "\n"), nil)) {
		lines = append(lines, f.Line)
	}
	if len(lines) != 2 || lines[0] != 3 || lines[1] != 4 {
		t.Fatalf("narration found on lines %v, want 3 and 4: the manual drive and the line outside the section stay", lines)
	}
}
