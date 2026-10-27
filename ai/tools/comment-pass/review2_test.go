package commentpass

import (
	"strings"
	"testing"
)

// The second review's inputs, each pinned where it found the pass wrong.

// A reply leaving a block open, or carrying a marker the file does not comment with, is refused. A
// blank line inside a comment is dropped.
func TestAReplyIsReadInTheFilesCommentSyntax(t *testing.T) {
	ts := material{places: []place{{id: "p1", line: 1}}, indents: map[string]int{}}
	for _, reply := range []string{"p1 add: r\n/**\n * x\nconst x = 1;\n", "p1 add: r\n# hash\n", "p1 add: r\n* stray star\n"} {
		if _, err := parseReply(reply, ts); err == nil {
			t.Errorf("%q parsed in a TypeScript file", reply)
		}
	}
	py := material{places: []place{{id: "p1", line: 1}}, indents: map[string]int{}, hashComments: true}
	if _, err := parseReply("p1 add: r\n// slashes\n", py); err == nil {
		t.Error("a // comment parsed in a Python file")
	}
	d, err := parseReply("p1 add: r\n// a\n\n// b\n", ts)
	if err != nil || len(d[0].text) != 2 {
		t.Fatalf("a blank line inside a comment: %v %q", err, d)
	}
}

// An indent counts in the columns it prints, a tab at four.
func TestAnIndentCountsItsColumns(t *testing.T) {
	if columns("\t\t\t") != 12 {
		t.Fatalf("three tabs print %d columns", columns("\t\t\t"))
	}
}

// An arrow's body on the next line, a Python signature over several lines and a Rust lifetime each
// keep the body inside the declaration's span.
func TestTheSpanReachesTheBody(t *testing.T) {
	arrow := []string{"export const f = (x) =>", "  x * 2;", "", "export const g = 1;"}
	if got := declarationSpan(arrow, 1, false); len(got) < 2 {
		t.Errorf("an arrow's span: %v", got)
	}
	python := []string{"def f(", "    a,", "):", "    return a", "", "def g():", "    pass"}
	if got := declarationSpan(python, 1, false); len(got) < 4 || got[3] != 4 || len(got) > 5 {
		t.Errorf("a multi-line Python signature's span: %v", got)
	}
	rust := []string{"fn f<'a>(x: &'a str) {", "    x.len();", "}"}
	if got := declarationSpan(rust, 1, true); len(got) != 3 {
		t.Errorf("a Rust function with a lifetime: %v", got)
	}
}

// A local variable and a callback passed in a call are no places to add a comment. The enclosing
// method and class stay places.
func TestLocalsAndCallbacksAreNoPlaces(t *testing.T) {
	lines := []string{"export class Book {", "  close(): void {", "    const doubled = value * 3;", "    useEffect(() => {",
		"      run();", "    });", "  }", "}"}
	m := findMaterial("f.ts", lines, map[int]bool{3: true, 5: true})
	var at []int
	for _, p := range m.places {
		at = append(at, p.line)
	}
	if len(at) != 2 || at[0] != 1 || at[1] != 2 {
		t.Fatalf("places at %v, want the class and the method", at)
	}
}

// A comment added to a decorated declaration goes above its decorators.
func TestAnAddedCommentGoesAboveTheDecorators(t *testing.T) {
	lines := []string{"@app.route('/')", "def index():", "    return 1"}
	m := material{places: []place{{id: "p1", line: 2}}, indents: map[string]int{}, code: lines}
	out := applyDecisions(lines, m, []decision{{id: "p1", verb: "add", text: []string{"# Serves the book."}}})
	if strings.Join(out, "\n") != "# Serves the book.\n@app.route('/')\ndef index():\n    return 1" {
		t.Fatalf("got:\n%s", strings.Join(out, "\n"))
	}
}

// A removed block leaves no blank line at the file's top and no doubled gap. Inside code, the blank
// line after it stays.
func TestARemovedBlockTakesTheGapItLeaves(t *testing.T) {
	remove := []decision{{id: "c1", verb: "remove"}}
	for _, tc := range []struct {
		lines       []string
		first, last int
		want        string
	}{
		{[]string{"/**", " * The book.", " */", "", "import x from 'x';"}, 1, 3, "import x from 'x';"},
		{[]string{"a();", "", "// Old.", "", "b();"}, 3, 3, "a();\n\nb();"},
		{[]string{"a();", "// Old.", "", "b();"}, 2, 2, "a();\n\nb();"},
	} {
		m := material{candidates: []candidate{{id: "c1", first: tc.first, last: tc.last}}, indents: map[string]int{}, code: tc.lines}
		if got := strings.Join(applyDecisions(tc.lines, m, remove), "\n"); got != tc.want {
			t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
		}
	}
}

// Each block is bounded on its own, at four prose lines whatever its kind. A four-line note above a
// summary is the shape the page asks of a declaration that needs both, and passes. A three-line doc
// block, a summary with its outside fact, passes. A five-line note or doc block does not.
func TestTheGateBoundsEachBlockOnItsOwn(t *testing.T) {
	m := material{candidates: []candidate{{id: "c1"}}, indents: map[string]int{}}
	note := []string{"// One.", "// Two.", "// Three.", "// Four."}
	summary := []string{"/**", " * Returns the posting, or undefined", " * when the book is closed.", " */"}
	gate := func(text []string) []string {
		return gateFindings(m, []decision{{id: "c1", verb: "rewrite", text: text}}, 120)
	}
	if found := gate(append(append([]string{}, note...), summary...)); len(found) != 0 {
		t.Errorf("a four-line note above a two-line summary was refused: %v", found)
	}
	if found := gate(append(append([]string{}, note...), "// Five.")); len(found) != 1 {
		t.Errorf("a five-line note passed: %v", found)
	}
	if found := gate([]string{"/**", " * One.", " * Two.", " * Three.", " */"}); len(found) != 0 {
		t.Errorf("a three-line doc block was refused: %v", found)
	}
	if found := gate([]string{"/**", " * One.", " * Two.", " * Three.", " * Four.", " * Five.", " */"}); len(found) != 1 {
		t.Errorf("a five-line doc block passed: %v", found)
	}
	if found := gate([]string{"/** Returns the posting. */"}); len(found) != 0 {
		t.Errorf("a one-line summary was refused: %v", found)
	}
}

// A block that a blank line parts from its declaration is that declaration's comment, and a rewrite
// puts it directly above. A file header above imports keeps its blank line.
func TestARewrittenBlockGoesDirectlyAboveItsDeclaration(t *testing.T) {
	rewrite := []decision{{id: "c1", verb: "rewrite", text: []string{"// Lists the keys in reading order."}}}
	for _, tc := range []struct {
		name   string
		lines  []string
		header bool
		want   string
	}{
		{"line 1 above a declaration", []string{"// Old.", "", "const KEYS = ['a'];"}, false,
			"// Lists the keys in reading order.\nconst KEYS = ['a'];"},
		{"mid-file above a function", []string{"import x from 'x';", "", "// Old.", "", "export function f() {", "}"}, false,
			"import x from 'x';\n\n// Lists the keys in reading order.\nexport function f() {\n}"},
		{"a file header above imports", []string{"// Old.", "", "import x from 'x';", "", "const KEYS = ['a'];"}, true,
			"// Lists the keys in reading order.\n\nimport x from 'x';\n\nconst KEYS = ['a'];"},
	} {
		changed := map[int]bool{}
		for n := range tc.lines {
			changed[n+1] = true
		}
		m := findMaterial("f.ts", tc.lines, changed)
		if len(m.candidates) != 1 || m.candidates[0].isHeader != tc.header {
			t.Fatalf("%s: material %+v", tc.name, m.candidates)
		}
		if got := strings.Join(applyDecisions(tc.lines, m, rewrite), "\n"); got != tc.want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", tc.name, got, tc.want)
		}
	}
}

// A Rust attribute is code, so a rewritten doc comment above it, or a line comment above an attribute
// inside a body, leaves the attribute standing.
func TestARewriteLeavesARustAttributeStanding(t *testing.T) {
	lines := []string{
		"/// Takes a screenshot.",
		"#[tauri::command]",
		"pub fn take_screenshot() -> Result<(), String> {",
		"    // Windows needs the handle first.",
		`    #[cfg(target_os = "windows")]`,
		"    let handle = window_handle();",
		"    capture_all()",
		"}",
	}
	m := findMaterial("lib.rs", lines, map[int]bool{6: true, 7: true})
	if len(m.candidates) != 2 || m.candidates[0].last != 1 || m.candidates[0].decl != 3 || m.candidates[1].last != 4 || m.candidates[1].decl != 6 || len(m.places) != 0 {
		t.Fatalf("material %+v, places %+v", m.candidates, m.places)
	}
	out := applyDecisions(lines, m, []decision{
		{id: "c1", verb: "rewrite", text: []string{"/// Takes a screenshot of every screen."}},
		{id: "c2", verb: "rewrite", text: []string{"// Windows asks for the handle before the capture."}},
	})
	want := append([]string{"/// Takes a screenshot of every screen.", lines[1], lines[2],
		"    // Windows asks for the handle before the capture."}, lines[4:]...)
	if got := strings.Join(out, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s", got)
	}
}

// An edit that would change, drop or move a code line is refused with the line shown, whatever the
// blocks were read as. An attribute's shape counts as code in every file, a Python one included.
func TestAnEditThatTouchesCodeIsRefused(t *testing.T) {
	before := []string{"/// Takes a screenshot.", "#[tauri::command]", "", "pub fn take() {}"}
	if found := codeChanges(before, []string{"/// Takes one, as `/* */` would.", "#[tauri::command]", "pub fn take() {}"}, "a.rs"); len(found) != 0 {
		t.Errorf("a rewritten comment and a dropped blank line were refused: %v", found)
	}
	for _, after := range [][]string{
		{"/// Takes one.", "pub fn take() {}"},
		{"#[tauri::command]", "/// Takes one.", "pub fn take() {}", "#[tauri::command]"},
		{"/// Takes one.", "#[tauri::command]", "pub fn take_all() {}"},
	} {
		if found := codeChanges(before, after, "a.rs"); len(found) == 0 || !strings.Contains(strings.Join(found, "\n"), "code line") {
			t.Errorf("%q passed: %v", after, found)
		}
	}
	py := []string{"# The routes.", "#[1] see the paper", "@app.route('/')", "def index(): pass"}
	if found := codeChanges(py, []string{"# The routes, after the paper.", "@app.route('/')", "def index(): pass"}, "a.py"); len(found) != 0 {
		t.Errorf("a rewritten Python comment opening on #[ was refused: %v", found)
	}
	if found := codeChanges(py, []string{"# The routes.", "def index(): pass"}, "a.py"); len(found) == 0 {
		t.Error("a dropped decorator passed in a Python file")
	}
}

// A tool directive inside a comment block is code a rewrite must not drop.
func TestARewriteThatDropsADirectiveIsRefused(t *testing.T) {
	for _, tc := range []struct{ before, after []string }{
		{[]string{"// files is the tree.", "//", "//go:embed static", "var files embed.FS"}, []string{"// New text.", "var files embed.FS"}},
		{[]string{"// The handler.", "// eslint-disable-next-line @typescript-eslint/no-explicit-any", "export function h(x: any) {}"},
			[]string{"// Handles x.", "export function h(x: any) {}"}},
	} {
		if found := codeChanges(tc.before, tc.after, "a.rs"); len(found) == 0 {
			t.Errorf("%q passed", tc.before)
		}
	}
}

// A directive ends a comment block and stays on the line it governs. A comment added there goes above
// it.
func TestADirectiveIsNoPartOfAComment(t *testing.T) {
	lines := []string{"package a", "", "// files is the tree.", "//go:embed static", "var files embed.FS"}
	m := findMaterial("a.go", lines, map[int]bool{5: true})
	if len(m.candidates) != 1 || m.candidates[0].last != 3 || m.candidates[0].decl != 5 || len(m.places) != 0 {
		t.Fatalf("material %+v, places %+v", m.candidates, m.places)
	}
	bare := []string{"// eslint-disable-next-line @typescript-eslint/no-explicit-any", "export function h(x: any) {}"}
	m = findMaterial("h.ts", bare, map[int]bool{2: true})
	out := applyDecisions(bare, m, []decision{{id: "p1", verb: "add", text: []string{"// Handles x."}}})
	if len(m.candidates) != 0 || len(m.places) != 1 || strings.Join(out, "\n") != "// Handles x.\n"+strings.Join(bare, "\n") {
		t.Fatalf("material %+v, places %+v, got:\n%s", m.candidates, m.places, strings.Join(out, "\n"))
	}
}

// Prose opening on a word a tool's directive also opens on is a comment.
func TestProseIsNoDirective(t *testing.T) {
	for _, line := range []string{"// type: the key's kind", " * type: what the entry records", "// global settings are read once", "// region of the screen",
		"// eslint reads its own, from the nearest file up.", "// global state, shared by the workers."} {
		if isDirective(line) {
			t.Errorf("%q read as a directive", line)
		}
	}
	for _, line := range []string{"# type: ignore", "// eslint-disable-next-line no-console", "//go:generate stringer", "/* global foo, bar */", "// #region Helpers", "#region Helpers",
		"/* eslint-env browser, node */", "# pylint: skip-file"} {
		if !isDirective(line) {
			t.Errorf("%q read as prose", line)
		}
	}
}

// A decorator on the line of the declaration it decorates is that declaration, so an added comment
// never climbs over the body above it and a comment above it is that declaration's.
func TestADecoratorOnItsDeclarationsLineIsTheDeclaration(t *testing.T) {
	swift := []string{"@objc func tapped() {", "    go()", "}", "func reload() {", "    load()", "}"}
	m := findMaterial("a.swift", swift, map[int]bool{5: true})
	out := applyDecisions(swift, m, []decision{{id: "p1", verb: "add", text: []string{"// Reloads the list."}}})
	if len(m.places) != 1 || out[3] != "// Reloads the list." {
		t.Fatalf("places %+v, got:\n%s", m.places, strings.Join(out, "\n"))
	}
	tapped := []string{"// Handles a tap.", "@objc func tapped() {", "    go()", "}", "", "func reload() {", "}"}
	if m := findMaterial("a.swift", tapped, map[int]bool{3: true}); len(m.candidates) != 1 || m.candidates[0].decl != 2 {
		t.Fatalf("material %+v", m.candidates)
	}
	rust := []string{"/// Inlined.", "#[inline] fn f() {", "}"}
	if m := findMaterial("a.rs", rust, map[int]bool{2: true}); len(m.candidates) != 1 || m.candidates[0].decl != 2 {
		t.Fatalf("material %+v", m.candidates)
	}
	kotlin := []string{"object A {", "    // Runs it.", "    @JvmStatic @Throws(IOException::class)", "    fun run() {", "    }", "}"}
	if m := findMaterial("A.kt", kotlin, map[int]bool{4: true}); len(m.candidates) != 1 || m.candidates[0].decl != 4 || len(m.places) != 0 {
		t.Fatalf("two decorators on a line: material %+v, places %+v", m.candidates, m.places)
	}
	trailing := []string{"class A {", "  // The name.", "  @Input() /* bound */ name: string;", "  other = 1;", "}"}
	if m := findMaterial("a.ts", trailing, map[int]bool{3: true}); len(m.candidates) != 1 || m.candidates[0].decl != 3 {
		t.Fatalf("a decorator before a block comment and its member: material %+v", m.candidates)
	}
	if got := aboveDecorators([]string{"@buttons[0].clicked.connect", "def on_click():"}, 2, true); got != 1 {
		t.Fatalf("an expression decorator: %d", got)
	}
	if got := aboveDecorators([]string{"/*", "  Reads it.", "  @deprecated */", "export function readAll() {}"}, 4, false); got != 4 {
		t.Fatalf("a block's closing line read as a decorator: %d", got)
	}
	py := []string{"x = (a", "     @ b)", "def f():", "    pass"}
	if got := aboveDecorators(py, 3, true); got != 3 {
		t.Fatalf("a matrix product read as a decorator: %d", got)
	}
}

// A directive inside a doc block leaves both parts on the declaration under them.
func TestADirectiveInsideABlockLeavesBothPartsOnTheDeclaration(t *testing.T) {
	lines := []string{"package a", "", "// Kind is the entry's kind.", "//go:generate stringer -type=Kind", "// It orders the ledger.", "type Kind int"}
	m := findMaterial("a.go", lines, map[int]bool{6: true})
	if len(m.candidates) != 2 || m.candidates[0].decl != 6 || m.candidates[1].decl != 6 {
		t.Fatalf("material %+v", m.candidates)
	}
}

// A directive's step over the comment under it stops at a blank line, which parts the next block.
func TestADirectivesStepStopsAtABlankLine(t *testing.T) {
	section := []string{"export const a = 1;", "", "// Helpers for the log.", "/* eslint-disable no-console */", "", "// Logs it.",
		"export function log() {", "  console.log(1);", "}"}
	if m := findMaterial("a.ts", section, map[int]bool{8: true}); len(m.candidates) != 1 || m.candidates[0].first != 6 {
		t.Fatalf("material %+v", m.candidates)
	}
}

// A line opening on `*` outside a `/* */` block is code, as a continued expression, and no comment.
func TestAStrayStarLineIsCode(t *testing.T) {
	lines := []string{"func f() int {", "	return a +", "		* b", "}"}
	if found := commentBlocks(lines, false); len(found) != 0 {
		t.Fatalf("blocks %v", found)
	}
	if got := commentBlocks([]string{"/**", " * The book.", " */", "x"}, false); len(got) != 1 || got[0].Span != 3 {
		t.Fatalf("a doc block: %v", got)
	}
}

// A Rust attribute split over several lines sits between a doc comment and its declaration, which
// takes no second comment, and an added comment goes above the whole attribute.
func TestAMultiLineAttributeIsSkipped(t *testing.T) {
	lines := []string{"/// Takes a screenshot.", "#[cfg_attr(", `    feature = "x",`, "    derive(Debug)", ")]", "pub fn take() {}"}
	m := findMaterial("lib.rs", lines, map[int]bool{6: true})
	if len(m.candidates) != 1 || m.candidates[0].decl != 6 || len(m.places) != 0 {
		t.Fatalf("material %+v, places %+v", m.candidates, m.places)
	}
	bare := lines[1:]
	m = findMaterial("lib.rs", bare, map[int]bool{5: true})
	out := applyDecisions(bare, m, []decision{{id: "p1", verb: "add", text: []string{"/// Takes one."}}})
	if len(m.places) != 1 || out[0] != "/// Takes one." {
		t.Fatalf("places %+v, got:\n%s", m.places, strings.Join(out, "\n"))
	}
}
