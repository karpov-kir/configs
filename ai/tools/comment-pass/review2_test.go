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
	m := material{places: []place{{id: "p1", line: 2}}, indents: map[string]int{}}
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
		m := material{candidates: []candidate{{id: "c1", first: tc.first, last: tc.last}}, indents: map[string]int{}}
		if got := strings.Join(applyDecisions(tc.lines, m, remove), "\n"); got != tc.want {
			t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
		}
	}
}

// Each block is bounded on its own. A four-line note above a two-line summary is the shape the page asks
// of a declaration that needs both, and passes. A five-line note, or a doc block over two lines, does not.
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
	if found := gate([]string{"/**", " * One.", " * Two.", " * Three.", " */"}); len(found) != 1 {
		t.Errorf("a three-line summary passed: %v", found)
	}
	if found := gate([]string{"/** Returns the posting. */"}); len(found) != 0 {
		t.Errorf("a one-line summary was refused: %v", found)
	}
}
