package ecoreport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCharterConstraintsRemainHumanReadable(t *testing.T) {
	for _, tc := range []struct {
		name, text, finding string
	}{
		{"missing section", "# Charter\n\n## Vision\nA service.\n", "missing"},
		{"duplicate section", "# Charter\n## Constraints\n- One.\n## Constraints\n- Two.\n", "duplicate"},
		{"past cap", "# Charter\n## Constraints\n" + strings.Repeat("- A project invariant.\n", 31), "30"},
		{"counted record", "# Charter\n## Constraints\n1x | 2026-09-08 | A rule.\n", "bullet"},
		{"counted bullet", "# Charter\n## Constraints\n- 1x | 2026-09-08 | A rule.\n", "bullet"},
		{"nested rule", "# Charter\n## Constraints\n- A rule.\n  - Another rule.\n", "bullet"},
		{"paragraph", "# Charter\n## Constraints\nA long rationale belongs in decisions.\n", "bullet"},
		{"empty rule", "# Charter\n## Constraints\n- \n", "bullet"},
		{"nested section", "# Charter\n## Constraints\n### Hidden constraints\n- One.\n", "bullet"},
		{"protected empty", "# Charter\n## Constraints\n<!-- Human-owned; changes require approval. -->\n", ""},
		{"at cap", "# Charter\n## Constraints\n" + strings.Repeat("- A project invariant.\n", 30), ""},
		{"next section", "# Charter\n## Constraints\n- One.\n## Scope\nAnything here is prose.\n", ""},
		{"fenced fake heading", "# Charter\n```markdown\n## Constraints\n- Fake.\n```\n", "missing"},
		{"long fence", "# Charter\n````markdown\n```\n## Constraints\n- Fake.\n```\n````\n", "missing"},
		{"fence language is not closing", "# Charter\n```markdown\n```example\n## Constraints\n- Fake.\n```\n", "missing"},
		{"commented section", "# Charter\n<!--\n## Constraints\n## Scope\n-->\n", "missing"},
		{"commented bullet", "# Charter\n## Constraints\n- One. <!-- hidden\n## Constraints\n-->\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "charter.md")
			if err := os.WriteFile(path, []byte(tc.text), 0o600); err != nil {
				t.Fatal(err)
			}
			findings := charterLayoutFindings(path)
			if tc.finding == "" && len(findings) != 0 {
				t.Fatalf("valid charter: %v", findings)
			}
			if tc.finding != "" && !strings.Contains(strings.Join(findings, "\n"), tc.finding) {
				t.Fatalf("wanted %q, got %v", tc.finding, findings)
			}
		})
	}
}
