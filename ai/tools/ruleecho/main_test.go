package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoldSpans(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"one rule", `- **Rules an agent reads** — any doc.`, []string{"Rules an agent reads"}},
		{"two rules", `**First rule here** and **second rule here**.`, []string{"First rule here", "second rule here"}},
		{
			// The regression. `**lossy**` is under the length floor, so pairing by proximity left its
			// closing delimiter free to open a span over the prose running up to the next rule.
			"short bold between two rules",
			`- **Rules an agent reads**: **lossy**, whole rules included. ecosystem.md is the bar. **Score what survives them**, one per rule.`,
			[]string{"Rules an agent reads", "lossy", "Score what survives them"},
		},
		{"unterminated tail", `**A real rule here** and **an unclosed one`, []string{"A real rule here"}},
		{"no bold", `plain prose with no delimiters`, nil},
		{"empty bold", `****`, []string{""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, s := range boldSpans(c.line) {
				got = append(got, s.text)
			}
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Fatalf("boldSpans(%q) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}

// The offset each span reports has to land on its own opening `**`, or the citation filter reads the
// wrong text and a cross-reference is reported as a restatement.
func TestBoldSpanStartLandsOnItsDelimiter(t *testing.T) {
	line := `see file.md → **One home**, then **the second rule stated**.`
	for _, s := range boldSpans(line) {
		if !strings.HasPrefix(line[s.start:], "**"+s.text+"**") {
			t.Fatalf("span %q reports start %d, which is %q", s.text, s.start, line[s.start:])
		}
	}
}

func TestCollectSkipsFencesAndCitations(t *testing.T) {
	dir := t.TempDir()
	// Every span here clears keyOf's four-word floor, so each one is dropped by the rule under test
	// rather than by the floor — the difference between an assertion and a vacuous pass.
	const kept = "a rule stating four discriminating words plainly"
	body := "# doc\n" +
		"file.md → **a cited section naming another file entirely**\n" +
		"**" + kept + "**\n" +
		"```\n**a fenced rule with several discriminating words**\n```\n"
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	spans, err := collect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].text != kept {
		t.Fatalf("collect returned %+v", spans)
	}
}
