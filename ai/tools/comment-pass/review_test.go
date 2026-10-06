package commentpass

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The review's inputs, each pinned where it found the pass wrong.

// A reply the gate refused is not kept, so the next run asks again. A change of model asks again too.
func TestARefusedReplyIsAskedAgainAndAModelChangeAsksAgain(t *testing.T) {
	dir, base := fixture(t)
	t.Setenv("TEST_STATE", t.TempDir())
	calls := 0
	refused := "c1 rewrite: r\n// **bold** words\np1 skip: none\n"
	for round := 1; round <= 2; round++ {
		if code, text := run(t, dir, base, refused, &calls); code != 1 || calls != round {
			t.Fatalf("round %d: exit %d, %d call(s)\n%s", round, code, calls, text)
		}
	}
	good := "c1 keep: fine\np1 skip: fine\n"
	run(t, dir, base, good, &calls)
	before := calls
	if code, _ := run(t, dir, base, good, &calls, "--model=another"); code != 0 || calls != before+1 {
		t.Fatalf("a model change: exit %d, %d call(s)", code, calls-before)
	}
}

// A text line that is no comment refuses the reply, so code never reaches the file.
func TestAReplyCarryingCodeIsRefused(t *testing.T) {
	m := material{places: []place{{id: "p1", line: 1}}, indents: map[string]int{}}
	if _, err := parseReply("p1 add: r\nexport const y = 2;\n", m); err == nil {
		t.Fatal("code in a reply parsed")
	}
	if _, err := parseReply("p1 add: r\n/**\n Returns the book.\n */\n", m); err != nil {
		t.Fatalf("a /* */ block refused: %v", err)
	}
}

// A bracket inside a string ends no span, a decorator sits between a comment and its class, a Go
// package doc is a header, and a Python def's indented body is its span.
func TestTheMaterialReadsStringsDecoratorsPackageDocsAndPython(t *testing.T) {
	lines := []string{"// The book's opening mark.", `const open = "{";`, "", "export function post() {", "  write();", "}"}
	if got := declarationSpan(lines, 2, false); len(got) != 1 {
		t.Fatalf("a string's brace opened a span: %v", got)
	}
	if m := findMaterial("f.ts", lines, map[int]bool{5: true}); len(m.candidates) != 0 {
		t.Fatalf("a change to post offered the constant's comment: %+v", m.candidates)
	}
	decorated := []string{"// Serves the ledger.", "@Injectable()", "export class Ledger {", "  x = 1;", "}"}
	if m := findMaterial("f.ts", decorated, map[int]bool{3: true}); len(m.candidates) != 1 || len(m.places) != 0 {
		t.Fatalf("a decorated class: %+v", m)
	}
	goDoc := []string{"// Package ledger posts entries.", "package ledger", "", "func Post() {}"}
	if m := findMaterial("ledger.go", goDoc, map[int]bool{1: true}); len(m.candidates) != 1 || !m.candidates[0].isHeader {
		t.Fatalf("a package doc: %+v", m.candidates)
	}
	python := []string{"# Posts each entry.", "def post(entries):", "    for e in entries:", "        write(e)", "", "def close():", "    pass"}
	if got := declarationSpan(python, 2, false); len(got) != 4 {
		t.Fatalf("a def's span: %v", got)
	}
	if m := findMaterial("l.py", python, map[int]bool{4: true, 7: true}); len(m.candidates) != 1 || len(m.places) != 1 {
		t.Fatalf("python material: %+v", m)
	}
}

// A renamed and edited file is read from its old path, so its unchanged lines are context.
func TestARenamedFileIsReadAsRenamed(t *testing.T) {
	dir, base := fixture(t)
	if out, err := exec.Command("git", "-C", dir, "mv", "ledger.ts", "books.ts").CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v %s", err, out)
	}
	calls := 0
	code, text := run(t, dir, base, "", &calls, "--list")
	if code != 0 || !strings.Contains(text, "books.ts: 1 comment(s), 1 place(s)") {
		t.Fatalf("exit %d:\n%s", code, text)
	}
}

// A CRLF file is written back with CRLF line ends throughout.
func TestACRLFFileKeepsItsLineEnds(t *testing.T) {
	dir, base := fixture(t)
	body, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
	write(t, dir, "ledger.ts", strings.ReplaceAll(string(body), "\n", "\r\n"))
	calls := 0
	_, listed := run(t, dir, base, "", &calls, "--list")
	var reply strings.Builder
	for _, line := range strings.Split(listed, "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) > 0 && strings.HasPrefix(fields[0], "c"):
			reply.WriteString(fields[0] + " keep: fine\n")
		case len(fields) > 0 && fields[0] == "p1":
			reply.WriteString("p1 add: r\n// Reopens the book.\n")
		case len(fields) > 0 && strings.HasPrefix(fields[0], "p"):
			reply.WriteString(fields[0] + " skip: fine\n")
		}
	}
	if code, text := run(t, dir, base, reply.String(), &calls); code != 0 {
		t.Fatalf("exit %d\n%s", code, text)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
	if strings.Count(string(got), "\n") != strings.Count(string(got), "\r\n") {
		t.Fatalf("mixed line ends:\n%q", got)
	}
}

// Ordinary comments are no markdown, and a tab takes four columns.
func TestTheGateReadsMarkdownAndTabsAsTheyPrint(t *testing.T) {
	for _, body := range []string{"Forwards **kwargs and **opts to the poster.", "- 1 is returned for an empty book.", "* @param book the book"} {
		if reMarkdown.MatchString(body) {
			t.Errorf("%q reads as markdown", body)
		}
	}
	if !reMarkdown.MatchString("A **closed** book takes no posting.") {
		t.Error("bold reads as no markdown")
	}
	if columns("\t\t"+strings.Repeat("x", 78)) <= 80 {
		t.Error("two tabs and 78 runes fit 80 columns")
	}
	if widthFor("ledger.go", 120) != otherWidth || widthFor("ledger.ts", 120) != 120 {
		t.Error("the width ignores the file's kind")
	}
}
