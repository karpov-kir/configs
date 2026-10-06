package commentpass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A line past its allowance wraps at word boundaries under its own marker, and a one-line block
// opens into a block. A line that fits, and a word longer than the allowance, stay as they were.
func TestAWideLineWrapsUnderItsOwnMarker(t *testing.T) {
	for _, tc := range []struct {
		line      string
		allowance int
		want      []string
	}{
		{"// one two three four", 12, []string{"// one two", "// three", "// four"}},
		{"# one two three four", 11, []string{"# one two", "# three", "# four"}},
		{"* one two three four", 11, []string{"* one two", "* three", "* four"}},
		{"/** one two three */", 12, []string{"/**", "* one two", "* three", "*/"}},
		{"/* one two three */", 12, []string{"/*", "* one two", "* three", "*/"}},
		{"// fits", 12, []string{"// fits"}},
		{"// " + strings.Repeat("w", 20), 12, []string{"// " + strings.Repeat("w", 20)}},
	} {
		if got := wrapLine(tc.line, tc.allowance); strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%q at %d: got %q, want %q", tc.line, tc.allowance, got, tc.want)
		}
		for _, line := range wrapLine(tc.line, tc.allowance) {
			if placedColumns(line) > tc.allowance && !strings.Contains(line, strings.Repeat("w", 20)) {
				t.Errorf("%q wraps to %q, wider than %d", tc.line, line, tc.allowance)
			}
		}
	}
}

// A block's `*` line lands one column in, so the gate counts that column.
func TestTheGateCountsTheColumnBeforeAStar(t *testing.T) {
	m := material{candidates: []candidate{{id: "c1"}}, indents: map[string]int{"c1": 0}}
	line := "* " + strings.Repeat("x", 78)
	if found := gateFindings(m, []decision{{id: "c1", verb: "rewrite", text: []string{"/**", line, "*/"}}}, 80); len(found) != 1 {
		t.Fatalf("an 81-column star line passed the gate at 80: %v", found)
	}
}

// A wide comment in a reply is written wrapped within the fixture's width of 120, and the run passes.
func TestAWideCommentIsWrittenWrapped(t *testing.T) {
	dir, base := fixture(t)
	calls := 0
	long := "// " + strings.TrimSpace(strings.Repeat("posts the entry ", 14))
	code, text := run(t, dir, base, "c1 rewrite: r\n"+long+"\np1 skip: fine\n", &calls)
	got, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
	if code != 0 || strings.Count(string(got), "posts") != 14 || strings.Count(string(got), "\n// ") < 2 {
		t.Fatalf("exit %d\n%s\n%s", code, text, string(got))
	}
	for _, line := range strings.Split(string(got), "\n") {
		if columns(line) > 120 {
			t.Errorf("a written line is %d columns: %s", columns(line), line)
		}
	}
}
