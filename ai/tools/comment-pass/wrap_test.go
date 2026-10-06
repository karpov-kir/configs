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

// Only prose wraps. A directive, an aligned line and a block already closed in its body stay whole.
func TestWhatIsNoProseStaysWhole(t *testing.T) {
	for _, line := range []string{
		"// eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment",
		"// @ts-expect-error the ledger's typings lag the runtime by one release, which is tracked upstream",
		"# type: ignore[attr-defined]  # the stub lacks the attribute the runtime adds at import time here",
		"x = 1  # noqa: E501 the long literal mirrors the ledger export byte for byte and cannot be split",
		"// | col a   | col b   | col c                    | col d                  | col e         |",
		"/** a */ b c d e f g h i j k l m n o p q r s t u v w x y z aa bb cc dd ee ff gg hh ii jj kk */",
	} {
		if got := wrapLine(line, 40); len(got) != 1 || got[0] != line {
			t.Errorf("%q was wrapped: %q", line, got)
		}
	}
}

// The lines of an @example and of a fence are code, and stay whole. Prose after the next tag wraps.
func TestExampleCodeStaysWhole(t *testing.T) {
	code := "* const result = compute(alpha, beta, gamma) // an example call that runs long here"
	text := []string{"/**", "* @example", code, "* @returns the posted total, which the ledger reports in minor units", "*/"}
	m := material{indents: map[string]int{"c1": 0}}
	got := rewrap(m, []decision{{id: "c1", verb: "rewrite", text: text}}, 40)[0].text
	if !strings.Contains(strings.Join(got, "\n"), code) || len(got) <= len(text) {
		t.Fatalf("got:\n%s", strings.Join(got, "\n"))
	}
	fenced := []string{"/**", "* ```", code, "* ```", "*/"}
	if got := rewrap(m, []decision{{id: "c1", verb: "rewrite", text: fenced}}, 40)[0].text; strings.Join(got, "|") != strings.Join(fenced, "|") {
		t.Fatalf("a fenced line was wrapped:\n%s", strings.Join(got, "\n"))
	}
}

// --ask asks again where a refused reply is kept, and the refusal names that way out.
func TestAskAsksAgain(t *testing.T) {
	dir, base := fixture(t)
	t.Setenv("TEST_STATE", t.TempDir())
	calls := 0
	refused := "c1 rewrite: r\n// **bold** words\np1 skip: none\n"
	_, text := run(t, dir, base, refused, &calls)
	if !strings.Contains(text, "--ask") {
		t.Fatalf("the refusal names no way out:\n%s", text)
	}
	if code, _ := run(t, dir, base, "c1 keep: fine\np1 skip: fine\n", &calls, "--ask"); code != 0 || calls != 2 {
		t.Fatalf("--ask: exit %d, %d call(s)", code, calls)
	}
}
