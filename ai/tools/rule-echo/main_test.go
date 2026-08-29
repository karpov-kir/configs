package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
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
// A fence nobody closed toggles the scan off for every line below it, so the rules there are never
// read and the summary reports the shortfall as nothing to find — the quiet this tool exists to
// remove, reached through a typo. The first half proves the rule is one the matcher collects when the
// fence is closed, so the second half failing means the fence swallowed it rather than that there was
// nothing to swallow.
func TestAnUnclosedFenceDoesNotSilenceTheRestOfTheFile(t *testing.T) {
	const rule = "a rule stating four discriminating words plainly"

	closed, _ := spansFrom(t, fenceMarker+"\nsample\n"+fenceMarker+"\n**"+rule+"**\n")
	if len(closed) != 1 || closed[0].text != rule {
		t.Fatalf("with the fence closed, collect returned %+v", closed)
	}

	var open []span
	var doc string
	stderr := captureStderr(t, func() {
		open, doc = spansFrom(t, fenceMarker+"\nsample\n**"+rule+"**\n")
	})
	if len(open) != 1 || open[0].text != rule {
		t.Fatalf("an unclosed fence swallowed the rule below it: %+v", open)
	}
	// Reading the file with fencing off is a narrowing of the tool's own rules, and a narrowing that
	// says nothing is indistinguishable from a clean read.
	if !strings.Contains(stderr, "unclosed fence") {
		t.Fatalf("the narrowing was not announced; stderr was %q", stderr)
	}
	// The whole use of the line is that a reader can open the file it names, so the path has to
	// arrive whole. The rule bound is sized for a report line and would cut a long path.
	if !strings.Contains(stderr, doc) {
		t.Fatalf("the announcement does not name %s in full:\n%s", doc, stderr)
	}
}

// Returns the document's path alongside its spans, because a case asserting what a message names
// needs the name the tool was handed.
func spansFrom(t *testing.T, body string) ([]span, string) {
	t.Helper()
	doc := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(doc, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	spans, err := collect(filepath.Dir(doc))
	if err != nil {
		t.Fatal(err)
	}
	return spans, doc
}

// `collect` names what it narrowed on os.Stderr, and that announcement is half of what the
// unclosed-fence case asserts, so the stream is captured rather than left to the terminal. No case
// here calls t.Parallel, so swapping a process-global for the duration is safe.
func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	saved := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = write
	drained := make(chan string, 1)
	go func() {
		var buf strings.Builder
		io.Copy(&buf, read)
		drained <- buf.String()
	}()
	// Deferred, because t.Fatal inside run() unwinds through Goexit: left swapped, every later case in
	// this package would write into a closed pipe.
	defer func() { os.Stderr = saved }()
	run()
	write.Close()
	return <-drained
}

// Both rows are real spans out of this tree, and both cleared the old match. The first has been
// adjudicated three separate times and accepted every time, because two consumers naming the same
// dependency at their own point of use is what `One home` asks for, not what it bars. The second is a
// genuine restatement and has to stay one, or the test below would pass by silencing everything.
func TestAPairSharingOnlyACitedNameIsNotARestatement(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want verdict
	}{
		{
			"the same dependency declared by two consumers",
			"The pass is `~/.claude/skills/kk-qualify/SKILL.md`",
			"The pass is `~/.claude/skills/kk-qualify/SKILL.md`",
			sharedName,
		},
		{
			"the same rule reworded in two files",
			"A licence you received goes into every spawn prompt you build, verbatim",
			"The licence goes in every spawn prompt's emphasis slot",
			restatement,
		},
		{
			// The blind spot the split could open: prose that states a rule and happens to name a file.
			// It stays a restatement because what the two share survives the name being removed — five
			// words of agreement that owe the path nothing.
			"a rule that names a file but agrees beyond it",
			"Never edit a generated file by hand, regenerate it from `~/.claude/skills/kk-qualify/SKILL.md`",
			"Never edit a generated file by hand, regenerate it instead",
			restatement,
		},
		{"two rules sharing nothing", "the first rule about fences", "an unrelated rule about ledgers", unrelated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, shared, beyond := classify(spanOf(c.a), spanOf(c.b))
			if got != c.want {
				t.Fatalf("classify = %v, want %v (%d shared, %d beyond the name)", got, c.want, shared, beyond)
			}
		})
	}
}

// A pair that shares only a cited name must not fail the run: the whole point of setting it apart is
// that a reader has already answered it, and an exit 1 asks them again every time.
func TestOnlyARestatementFailsTheRun(t *testing.T) {
	naming, _, _ := classify(
		spanOf("The pass is `~/.claude/skills/kk-qualify/SKILL.md`"),
		spanOf("The pass is `~/.claude/skills/kk-qualify/SKILL.md`"))
	if naming == restatement {
		t.Fatal("a shared name is being counted as a restatement, so a clean tree would exit 1")
	}
}

func spanOf(text string) span {
	return span{text: text, key: keyOf(text)}
}

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
	if got := shell.Oneline(spans[0].file); strings.ContainsAny(got, "\n\r\v\f\x1b") {
		t.Fatalf("shell.Oneline(%q) = %q, which still forges a line", spans[0].file, got)
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

func TestReportedTextDropsEveryControlByte(t *testing.T) {
	got := shell.Oneline("a\rb\vc\x1bd\x7fe")
	if got != "a b c d e" {
		t.Fatalf("shell.Oneline = %q, want %q", got, "a b c d e")
	}
}

// Byte slicing at a fixed offset splits a multi-byte rune, and this tree's prose is full of them.
func TestAQuotedRuleTruncatesOnRunesNotBytes(t *testing.T) {
	got := quotedRule(strings.Repeat("→", maxReportRunes+10))
	if strings.ContainsRune(got, '�') {
		t.Fatalf("quotedRule split a rune: %q", got)
	}
	if want := strings.Repeat("→", maxReportRunes) + "…"; got != want {
		t.Fatalf("quotedRule = %q, want %q", got, want)
	}
}

func TestFileOverTheBoundIsNotRead(t *testing.T) {
	dir := t.TempDir()
	const rule = "a rule stating four discriminating words plainly"
	// The name is long on purpose: the refusal has to name a path the reader can open, and a path
	// under the rune bound would be reported whole by a cut and an uncut version alike, so the case
	// would pass either way.
	doc := filepath.Join(dir, strings.Repeat("long-name-segment-", 8)+"big.md")
	big := append(make([]byte, maxFileBytes+1), []byte("**"+rule+"**\n")...)
	if err := os.WriteFile(doc, big, 0o600); err != nil {
		t.Fatal(err)
	}
	if len([]rune(doc)) <= maxReportRunes {
		t.Fatalf("the fixture path is %d runes, inside the %d-rune bound; the case proves nothing",
			len([]rune(doc)), maxReportRunes)
	}

	var spans []span
	var collectErr error
	stderr := captureStderr(t, func() { spans, collectErr = collect(dir) })
	if collectErr != nil {
		t.Fatal(collectErr)
	}
	if len(spans) != 0 {
		t.Fatalf("read a file over the bound: %+v", spans)
	}
	if !strings.Contains(stderr, doc) {
		t.Fatalf("the refusal does not name %s in full:\n%s", doc, stderr)
	}
}

// The C1 range, in both spellings a terminal reads. This tool prints a repo's own bolded prose and
// its own paths, so the bytes here are chosen by the tree under review. A local copy of the control
// set read C0 and DEL only: an encoded U+009B is CSI and an encoded U+0085 is NEL, and both reached
// the terminal intact. `shell` owns which bytes are control bytes, so this holds the two to the same
// answer rather than restating the range.
func TestReportedTextNeutralisesTheC1Range(t *testing.T) {
	for _, in := range []string{"a\u0080b", "a\u0085b", "a\u009bb", "a\x9bb", "a\x85b"} {
		if got := shell.Oneline(in); got != "a b" {
			t.Errorf("shell.Oneline(%q) = %q, want %q", in, got, "a b")
		}
	}
	// Multi-byte characters survive: the range doubles as UTF-8 continuation bytes, so a rule mapping
	// by byte value would shred every CJK character and emoji a rule might carry.
	for _, in := range []string{"a\u65e5b", "a\U0001f600b", "a\u00e9b"} {
		if got := shell.Oneline(in); got != in {
			t.Errorf("shell.Oneline(%q) = %q — a real character was damaged", in, got)
		}
	}
}
