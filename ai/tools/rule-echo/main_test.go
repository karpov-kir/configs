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

// A filename may carry any byte but `/` and NUL. `collect` hands that name straight through, so the
// report is where it has to be neutralised: a newline in one otherwise forges a line of output, and
// this tool's summary line is the one worth forging. The first half asserts the hazard is real and
// reaches the reporting path — without it the second half would pass over a name nothing could forge.
func TestReportedPathCannotForgeALine(t *testing.T) {
	dir := t.TempDir()
	const rule = "a rule stating four discriminating words plainly"
	const forged = "0 bolded rule(s) read, 0 pair(s) stating the same thing in two files"
	name := "evil\n" + forged + "\nignored.md"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("**"+rule+"**\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spans, err := collect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 {
		t.Fatalf("collect returned %d spans, want 1", len(spans))
	}
	if !strings.Contains(spans[0].file, "\n") {
		t.Fatal("the fixture no longer carries a newline in its path; the case proves nothing")
	}
	if got := stripControl(spans[0].file); strings.ContainsAny(got, "\n\r\v\f\x1b") {
		t.Fatalf("stripControl(%q) = %q, which still forges a line", spans[0].file, got)
	}
}

// A `.md` symlink pointing out of the tree is read by `os.ReadFile` even though `Walk` refuses to
// follow a directory one. The first half proves the target holds a rule the matcher would report, so
// a pass here means the symlink was refused rather than that there was nothing to find.
func TestSymlinkedMarkdownIsNotRead(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "private.md")
	const secret = "a private rule stating four discriminating words"
	if err := os.WriteFile(outside, []byte("**"+secret+"**\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if spans, err := collect(filepath.Dir(outside)); err != nil || len(spans) != 1 {
		t.Fatalf("the target holds no readable rule, so the case below proves nothing: %v %v", spans, err)
	}

	dir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	spans, err := collect(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range spans {
		if strings.Contains(s.text, "private rule") {
			t.Fatalf("read through a symlink out of the tree: %q from %s", s.text, s.file)
		}
	}
	if len(spans) != 0 {
		t.Fatalf("collect returned %+v, want nothing", spans)
	}
}

func TestStripControlKeepsTextAndDropsControlBytes(t *testing.T) {
	got := stripControl("a\rb\vc\x1bd\x7fe")
	if got != "a b c d e" {
		t.Fatalf("stripControl = %q, want %q", got, "a b c d e")
	}
}

// Byte slicing at a fixed offset splits a multi-byte rune, and this tree's prose is full of them.
func TestOnelineTruncatesOnRunesNotBytes(t *testing.T) {
	got := oneline(strings.Repeat("→", maxReportRunes+10))
	if strings.ContainsRune(got, '�') {
		t.Fatalf("oneline split a rune: %q", got)
	}
	if want := strings.Repeat("→", maxReportRunes) + "…"; got != want {
		t.Fatalf("oneline = %q, want %q", got, want)
	}
}

func TestFileOverTheBoundIsNotRead(t *testing.T) {
	dir := t.TempDir()
	const rule = "a rule stating four discriminating words plainly"
	big := append(make([]byte, maxFileBytes+1), []byte("**"+rule+"**\n")...)
	if err := os.WriteFile(filepath.Join(dir, "big.md"), big, 0o600); err != nil {
		t.Fatal(err)
	}
	spans, err := collect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 0 {
		t.Fatalf("read a file over the bound: %+v", spans)
	}
}
