package commentpass

import (
	"strings"
	"testing"
)

// A line inside a string that spans lines is the string's content, never a comment, in each
// language's own multi-line string.
func TestALineInsideAStringIsNoComment(t *testing.T) {
	for _, tc := range []struct {
		path  string
		lines []string
	}{
		{"conf.sh", []string{"cat > conf <<EOF", "# listen 80", "EOF", "run"}},
		{"conf.sh", []string{"cat > conf <<-'END'", "\t# listen 80", "\tEND", "run"}},
		{"help.py", []string{`HELP = """`, "# not a comment", `"""`, "x = 1"}},
		{"help.py", []string{"HELP = r'''", "# not a comment", "'''", "x = 1"}},
		{"help.ts", []string{"export const help = `", "* run", "// stop", "`;"}},
		{"help.go", []string{"var help = `", "// run", "`"}},
		{"help.js", []string{"const help = `", "* run", "`;"}},
		{"Help.java", []string{`String help = """`, "// run", `""";`}},
		{"Help.kt", []string{`val help = """`, "// run", `"""`}},
		{"help.rs", []string{`let help = "first`, "// run", `last";`}},
		{"help.rs", []string{`let help = r#"first`, `// run "quoted"`, `last"#;`}},
		{"conf.sh", []string{"cat <<'END-OF-TEXT'", "# inside", "END-OF-TEXT"}},
		{"help.swift", []string{`let help = #"""`, `// run \""" still`, `"""#`}},
		{"help.ts", []string{"const re = /x/; const help = `", "// run", "`;"}},
	} {
		if m := findMaterial(tc.path, tc.lines, map[int]bool{1: true, 2: true, 3: true}); len(m.candidates) != 0 {
			t.Errorf("%s %q: candidates %+v", tc.path, tc.lines, m.candidates)
		}
	}
}

// Quotes in a comment, a Rust character literal and a lifetime open no string, nor does a shift, a
// regex literal or a string that never closes, so the comments after them are still found.
func TestQuotesThatOpenNoString(t *testing.T) {
	for _, tc := range []struct {
		path  string
		lines []string
	}{
		{"a.sh", []string{"mask=$(( 1 << bit ))", "a=1", "# Runs it.", "run() { :; }"}},
		{"a.sh", []string{`echo "use: cmd << file"`, "a=1", "# Runs it.", "run() { :; }"}},
		{"a.ts", []string{"const parts = s.split(/`/);", "const a = 1;", "// Runs it.", "export function run() {}"}},
		{"a.kt", []string{`val dir = """C:\"""`, "val a = 1", "// Runs it.", "fun run() {}"}},
		{"a.swift", []string{`let s = #"a \"""# + "b"`, "let a = 1", "// Runs it.", "func run() {}"}},
		{"a.ts", []string{"const parts = s.split(/`/);", "// Runs it.", "export function run() {}", "const more = t.split(/`/);"}},
		{"a.sh", []string{"mask=$(( (1 << bits) - 1 ))", "cat <<EOF", "# in heredoc", "EOF", "# Runs it.", "f() { :; }"}},
		{"a.sh", []string{"let x=1<<2", "cat <<EOF", "# in heredoc", "EOF", "# Runs it.", "f() { :; }"}},
		{"a.ts", []string{"const f = (s) => /`/.test(s);", "const t = `", "// in template", "`;", "// Runs it.", "export function run() {}"}},
		{"a.ts", []string{"const avg = sum", "  / n; /* note", " * more */", "// Runs it.", "export function run() {}"}},
		{"a.ts", []string{"// don't wait", "const a = 1;", "// Runs it.", "export function run() {}"}},
		{"a.ts", []string{"/* it's `odd */", "const a = 1;", "// Runs it.", "export function run() {}"}},
		{"a.rs", []string{`let q = '"';`, "fn f<'a>(x: &'a str) {}", "// Runs it.", "pub fn run() {}"}},
		{"a.py", []string{"# don't wait", "a = 1", "# Runs it.", "def run(): pass"}},
		{"a.sh", []string{`echo "${#args}" # it's fine`, "a=1", "# Runs it.", "run() { :; }"}},
	} {
		// The comment is the one opening on `// Runs` or `# Runs`, on the declaration under it.
		first := 0
		for n, line := range tc.lines {
			if strings.Contains(line, " Runs it.") {
				first = n + 1
			}
		}
		m := findMaterial(tc.path, tc.lines, map[int]bool{first + 1: true})
		if len(m.candidates) != 1 || m.candidates[0].first != first {
			t.Errorf("%s %q: candidates %+v", tc.path, tc.lines, m.candidates)
		}
	}
}

// A multi-line decorator whose template holds an apostrophe or a bracket ends where its own brackets
// close, so the comment above it sits on the class.
func TestADecoratorsTemplateCountsNoBracket(t *testing.T) {
	lines := []string{"// The login form.", "@Component({", "  selector: 'login',", "  template: `",
		"    <p>(Don't have an account?)</p>", "  `,", "})", "export class Login {", "}"}
	m := findMaterial("a.ts", lines, map[int]bool{8: true})
	if len(m.candidates) != 1 || m.candidates[0].decl != 8 || len(m.places) != 0 {
		t.Fatalf("material %+v, places %+v", m.candidates, m.places)
	}
	unbalanced := []string{"// The menu.", "@Component({", "  template: `", "    <p>1) open</p>", "  `,", "})", "export class Menu {", "}"}
	if m := findMaterial("a.ts", unbalanced, map[int]bool{7: true}); len(m.candidates) != 1 || m.candidates[0].decl != 7 || len(m.places) != 0 {
		t.Fatalf("an unbalanced bracket: material %+v, places %+v", m.candidates, m.places)
	}
}

// An edit that would drop a line of a string is refused, though the line reads as a comment.
func TestAnEditThatDropsAStringsLineIsRefused(t *testing.T) {
	before := []string{"# Writes the config.", "cat > conf <<EOF", "# listen 80", "EOF"}
	if found := codeChanges(before, []string{"# Writes it.", "cat > conf <<EOF", "EOF"}, "conf.sh"); len(found) == 0 ||
		!strings.Contains(found[0], "# listen 80") {
		t.Fatalf("a dropped heredoc line passed: %v", found)
	}
}

// A string still open at the file's end reads the file as it is, and says where the string opened.
func TestAStringThatNeverClosesIsNamed(t *testing.T) {
	lines := []string{"// Sets a.", "const a = 1;", "const t = `", "// in template", "export function run() {}"}
	view, open := codeView(lines, "a.ts")
	if open != 3 || view[0] != "// Sets a." || view[3] != "" {
		t.Fatalf("open at %d, view %q", open, view)
	}
	if m := findMaterial("a.ts", lines, map[int]bool{2: true, 4: true, 5: true}); len(m.candidates) != 1 || len(m.places) != 0 {
		t.Fatalf("material %+v, places %+v", m.candidates, m.places)
	}
	if found := codeChanges(lines, append(append([]string{}, lines[:3]...), lines[4]), "a.ts"); len(found) == 0 {
		t.Fatal("a removal past the unclosed string passed")
	}
	if _, open := codeView([]string{"const t = `a`;"}, "a.ts"); open != 0 {
		t.Fatalf("a closed string named at %d", open)
	}
}

// The gate keeps each line a tool reads as code, and passes a rewrite of prose that only looks like
// code: floor division, a starless doc tag.
func TestTheGateReadsWhatAToolReadsAndOnlyThat(t *testing.T) {
	// A Python continuation opening on `//` is floor division, no comment.
	py := []string{"def f(total, count):", "    return (", "        total", "        // count", "    )"}
	if m := findMaterial("a.py", py, map[int]bool{4: true}); len(m.candidates) != 0 {
		t.Errorf("floor division read as a comment: %+v", m.candidates)
	}
	for _, tc := range []struct {
		path  string
		lines []string
	}{
		{"a.sh", []string{"# Runs the thing.", "# shellcheck disable=SC2086", "run $args"}},
		{"a.py", []string{"# Explain.", "# pragma: no cover", "def f(): pass"}},
		{"a.swift", []string{"// Explain.", "// swiftlint:disable:next force_cast", "func f() {}"}},
		{"a.py", []string{"# Explain.", "# -*- coding: utf-8 -*-", "x = 1"}},
		{"a.go", []string{"// Explain.", "// #nosec G104", "f()"}},
		{"a.go", []string{"// Explain.", "// +build linux", "package a"}},
		{"a.ts", []string{"/**", " * Explain.", " * @jest-environment jsdom", " */", "x();"}},
	} {
		after := append([]string{}, tc.lines[2:]...)
		if tc.lines[0] == "/**" {
			after = []string{"/** Explained. */", "x();"}
		}
		if found := codeChanges(tc.lines, after, tc.path); len(found) == 0 {
			t.Errorf("%s %q: dropping the directive passed", tc.path, tc.lines)
		}
	}
	// A Javadoc tag line with no star is prose, and a rewrite of its block passes.
	java := []string{"/**", " Returns foo.", " @param x the x", " */", "int foo(int x) {}"}
	if found := codeChanges(java, []string{"/** Returns foo for x. */", "int foo(int x) {}"}, "A.java"); len(found) != 0 {
		t.Errorf("a starless tag line refused: %v", found)
	}
	// Code after a block's close is code.
	if found := codeChanges([]string{"/* x */ run();"}, []string{"/* y */"}, "a.ts"); len(found) == 0 {
		t.Error("code after a closed block comment dropped")
	}
}

// A decorator ending in a block comment is still a decorator, and a marker closing a region stays
// where it is when a comment is added below it.
func TestPlacementPastATrailingCommentAndAClosingMarker(t *testing.T) {
	member := []string{"class A {", "  /** The label. */", "  @Input() /* required */", "  label: string;", "}"}
	if m := findMaterial("a.ts", member, map[int]bool{4: true}); len(m.candidates) != 1 || m.candidates[0].decl != 4 || len(m.places) != 1 || m.places[0].line != 1 {
		t.Errorf("material %+v, places %+v", m.candidates, m.places)
	}
	region := []string{"// #region helpers", "function a() {}", "// #endregion", "function b() {}"}
	if got := aboveDecorators(region, 4, false); got != 4 {
		t.Errorf("a comment for b climbs to line %d", got)
	}
}

// A directive a tool reads only at the file's top takes no comment above it, a comment above a closing
// marker sits on the declaration under it, and an `@` line inside a `/* */` part of a merged block is
// prose.
func TestDirectivesThatStayWhereTheyAre(t *testing.T) {
	cookie := []string{"#!/usr/bin/env python3", "# -*- coding: latin-1 -*-", "def f():", "    pass"}
	if got := aboveDecorators(cookie, 3, true); got != 3 {
		t.Errorf("a comment climbs over the coding cookie to line %d", got)
	}
	if got := aboveDecorators([]string{"/** @jest-environment jsdom */", "export function f() {}"}, 2, false); got != 2 {
		t.Errorf("a comment climbs over a jest docblock to line %d", got)
	}
	closer := []string{"// Comment for b.", "// eslint-enable no-console", "function b() {}"}
	if m := findMaterial("a.ts", closer, map[int]bool{3: true}); len(m.candidates) != 1 || m.candidates[0].decl != 3 || len(m.places) != 0 {
		t.Errorf("material %+v, places %+v", m.candidates, m.places)
	}
	merged := []string{"// note", "/*", `@Deprecated("x")`, "*/", "fun f() {}"}
	if found := codeChanges(merged, []string{"fun f() {}"}, "A.kt"); len(found) != 0 {
		t.Errorf("removing commented-out code refused: %v", found)
	}
}
