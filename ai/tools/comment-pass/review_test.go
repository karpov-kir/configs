package commentpass

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The review's inputs, each pinned where it found the pass wrong.

// A reply the gate refused is kept, so the next run refuses it again without asking again: a second
// ask would be a re-roll. A change of model asks again.
func TestARefusedReplyIsNotAskedAgainAndAModelChangeAsksAgain(t *testing.T) {
	dir, base := fixture(t)
	t.Setenv("TEST_STATE", t.TempDir())
	calls := 0
	refused := "c1 rewrite: r\n// **bold** words\np1 skip: none\n"
	for round := 1; round <= 2; round++ {
		if code, text := run(t, dir, base, refused, &calls); code != 1 || calls != 1 {
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
}

// TestARewriteThatOnlyRewrapsIsKept checks that a rewrite with the same words in the same form, with
// only its line breaks moved, keeps the comment and leaves the file as it was.
func TestARewriteThatOnlyRewrapsIsKept(t *testing.T) {
	dir, base := fixture(t)
	calls := 0
	before, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
	code, text := run(t, dir, base, "c1 rewrite: r\n// Posts\n//   entries.\np1 skip: fine\n", &calls)
	after, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
	if code != 0 || string(after) != string(before) || !strings.Contains(text, "1 kept, 0 rewritten") {
		t.Fatalf("exit %d\n%s\n%s", code, text, after)
	}
}

// TestMovedCodeOffersNoComment checks that code the change only moved to another file offers none of
// its comments, and that code the change edited where it moved to does.
func TestMovedCodeOffersNoComment(t *testing.T) {
	post := "// Posts entries.\nexport function post(): void {\n  write();\n}\n"
	dir, rev := repository(t, "ledger.ts", post+"\nexport const A = 1;\n", "export const A = 1;\n")
	moved := "// Reopens the book.\nexport function reopen(): void {\n  open();\n}\n\n" + post
	write(t, dir, "posting.ts", moved)
	if out, err := exec.Command("git", "-C", dir, "add", "-N", "posting.ts").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	calls := 0
	code, text := run(t, dir, rev, "", &calls, "--list")
	if code != 0 || strings.Contains(text, "lines 6-6") || !strings.Contains(text, "lines 1-1") {
		t.Fatalf("exit %d:\n%s", code, text)
	}
	write(t, dir, "posting.ts", strings.Replace(moved, "  write();", "  write(true);", 1))
	code, text = run(t, dir, rev, "", &calls, "--list")
	if code != 0 || !strings.Contains(text, "lines 6-6") {
		t.Fatalf("exit %d:\n%s", code, text)
	}
}

// TestADocCommentIsNeverRemoved checks that a doc comment on its declaration is rewritten or kept,
// never removed: the call says so, and a reply that removes one refuses the file. It also checks that
// a line comment a blank line parts from its declaration is no doc.
func TestADocCommentIsNeverRemoved(t *testing.T) {
	lines := []string{"/** Mounts the requests screen. */", "const show = (): void => mount();", "",
		"// The book closes at midnight.", "", "export const CLOSE = 0;"}
	m := findMaterial("view.ts", lines, map[int]bool{2: true, 6: true})
	if len(m.candidates) != 2 || !m.candidates[0].isDoc || m.candidates[1].isDoc {
		t.Fatalf("candidates: %+v", m.candidates)
	}
	if prompt := userPrompt("view.ts", lines, m, nil, 100, ""); !strings.Contains(prompt, "a doc comment that takes keep or rewrite") {
		t.Fatalf("prompt:\n%s", prompt)
	}
	if _, err := parseReply("c1 remove: restates the name\nc2 remove: r\n", m); err == nil || !strings.Contains(err.Error(), "c1 with remove") {
		t.Fatalf("a removed doc comment parsed: %v", err)
	}
	if _, err := parseReply("c1 keep: r\nc2 remove: r\n", m); err != nil {
		t.Fatalf("a removed line comment refused: %v", err)
	}
	goLines := []string{"package ledger", "", "// Post writes the entry.", "func Post() {}"}
	if gm := findMaterial("ledger.go", goLines, map[int]bool{4: true}); len(gm.candidates) != 1 || !gm.candidates[0].isDoc {
		t.Fatalf("Go candidates: %+v", gm.candidates)
	}
}

// TestABareBlockLineTakesItsStar checks that a block line a reply leaves without its `*` takes the
// marker, so the line prints inside the block.
func TestABareBlockLineTakesItsStar(t *testing.T) {
	m := material{candidates: []candidate{{id: "c1"}}, indents: map[string]int{}}
	got, err := parseReply("c1 rewrite: r\n/** Returns the body; a refusal throws\nthe server's text. */\n", m)
	if err != nil || len(got) != 1 || strings.Join(got[0].text, "\n") != "/** Returns the body; a refusal throws\n* the server's text. */" {
		t.Fatalf("%v %q", err, got)
	}
}

// TestALongCommentIsNotKept checks that a comment over the bound of lines is never kept as it is:
// the call says it takes rewrite or remove, a reply that keeps it refuses the file, and a run that
// cuts it prints the text it had.
func TestALongCommentIsNotKept(t *testing.T) {
	long := "// One.\n// Two.\n// Three.\n// Four.\n// Five.\n"
	dir, base := repository(t, "ledger.ts", long+"export function post(): void {}\n", long+"export function post(): number {\n  return 1;\n}\n")
	calls := 0
	code, text := run(t, dir, base, "c1 keep: every fact matters\n", &calls)
	if code != 1 || !strings.Contains(text, "c1 with keep") {
		t.Fatalf("a kept long comment passed, exit %d:\n%s", code, text)
	}
	lines := strings.Split(long+"export function post(): number {", "\n")
	if prompt := userPrompt("ledger.ts", lines, findMaterial("ledger.ts", lines, map[int]bool{6: true}), nil, 100, ""); !strings.Contains(prompt, "takes rewrite or remove") {
		t.Fatalf("prompt:\n%s", prompt)
	}
	code, text = run(t, dir, base, "c1 rewrite: r\n// Posts one entry.\n", &calls)
	if code != 0 || !strings.Contains(text, "was cut. It read:\n// One.") {
		t.Fatalf("exit %d:\n%s", code, text)
	}
}

// TestALongCommentInTheSameWordsIsRefused checks that a long comment given back in the same words
// stays a rewrite, and that the gate refuses it.
func TestALongCommentInTheSameWordsIsRefused(t *testing.T) {
	long := "// One.\n// Two.\n// Three.\n// Four.\n// Five.\n"
	dir, base := repository(t, "ledger.ts", long+"export function post(): void {}\n", long+"export function post(): number {\n  return 1;\n}\n")
	calls := 0
	code, text := run(t, dir, base, "c1 rewrite: r\n"+long, &calls)
	if code != 1 || !strings.Contains(text, "over 4") {
		t.Fatalf("exit %d:\n%s", code, text)
	}
}

// TestAGoDocOpensOnItsName checks that a comment written in a Go file is `//` lines, and that a doc
// comment opens on the declared name, an article allowed.
func TestAGoDocOpensOnItsName(t *testing.T) {
	lines := []string{"package ledger", "", "// Writes the entry.", "func (l *Ledger) Post() {}", "", "type Book struct{}"}
	m := findMaterial("ledger.go", lines, map[int]bool{4: true, 6: true})
	for _, tc := range []struct {
		text []string
		id   string
		want string
	}{
		{[]string{"/** Post writes the entry. */"}, "c1", "opens on /*"},
		{[]string{"// Writes the entry."}, "c1", "opens on the name Post"},
		{[]string{"// Post writes the entry."}, "c1", ""},
		{[]string{"// A Book holds entries."}, "p1", ""},
		{[]string{"// Holds entries."}, "p1", "opens on the name Book"},
	} {
		found := strings.Join(gateFindings(m, []decision{{id: tc.id, verb: "rewrite", text: tc.text}}, noWidth), "\n")
		if tc.want == "" && found != "" || tc.want != "" && !strings.Contains(found, tc.want) {
			t.Errorf("%q: %q", tc.text, found)
		}
	}
}

// TestAGoLocalTakesNoDoc checks that a comment on a local in a Go body is no doc comment: the pass
// may remove it, and a rewrite need not open on the local's name.
func TestAGoLocalTakesNoDoc(t *testing.T) {
	lines := []string{"package ledger", "", "func Post() {", "\t// Count only the open ones.", "\tvar n int", "\t_ = n", "}"}
	m := findMaterial("ledger.go", lines, map[int]bool{4: true, 5: true})
	if len(m.candidates) != 1 || m.candidates[0].isDoc {
		t.Fatalf("candidates: %+v", m.candidates)
	}
	if found := gateFindings(m, []decision{{id: "c1", verb: "rewrite", text: []string{"\t// Counts the open ones."}}}, noWidth); len(found) != 0 {
		t.Fatalf("a local's comment refused: %q", found)
	}
}

// TestAGoBlankVarTakesAnyOpening checks that a doc comment on `var _` need not open on `_`, which
// names nothing.
func TestAGoBlankVarTakesAnyOpening(t *testing.T) {
	lines := []string{"package ledger", "", "// Checks that Book is a Stringer.", "var _ fmt.Stringer = Book{}"}
	m := findMaterial("ledger.go", lines, map[int]bool{4: true})
	if found := gateFindings(m, []decision{{id: "c1", verb: "rewrite", text: []string{"// Book must stay a Stringer."}}}, noWidth); len(found) != 0 {
		t.Fatalf("a blank var's comment refused: %q", found)
	}
}
