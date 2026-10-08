package commentpass

import (
	"fmt"
	"strings"
	"testing"
)

// The call carries who uses the file's exported material: each site with the comment on its enclosing
// declaration and the lines around it. An import, a unit test and a comment are no callers, a name
// shows at most three sites, and a declaration nothing calls brings no section.
func TestTheCallCarriesTheCallersOfItsExportedMaterial(t *testing.T) {
	dir, _ := fixture(t)
	write(t, dir, "multi.ts", "import {\n  CLOSE_HOUR,\n  post,\n} from './ledger';\n")
	write(t, dir, "close.ts", "import { post } from './ledger';\n\n"+
		"// Closes the day, so the morning report reads a settled book.\n"+
		"export function closeDay(entries: Entry[]): void {\n  audit(entries);\n  post(entries);\n}\n")
	write(t, dir, "ledger.test.ts", "post([]);\n")
	write(t, dir, "notes.ts", "// post is the only writer.\nexport const n = 1;\n")
	for i := 1; i <= 3; i++ {
		write(t, dir, fmt.Sprintf("z%d.ts", i), "export function run(): void {\n  post([]);\n}\n")
	}
	lines := strings.Split(strings.TrimSuffix(headLedger, "\n"), "\n")
	m := findMaterial("ledger.ts", lines, map[int]bool{3: true, 9: true})
	got := callersSection(dir, "ledger.ts", lines, m)
	for _, want := range []string{
		"post is used in close.ts at line 6, inside line 4: export function closeDay(entries: Entry[]): void {",
		"// Closes the day, so the morning report reads a settled book.",
		"     6    post(entries);",
		"as context only",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Count(got, "post is used in") != sitesPerName {
		t.Errorf("%d sites for post, want %d:\n%s", strings.Count(got, "post is used in"), sitesPerName, got)
	}
	for _, not := range []string{"ledger.test.ts", "notes.ts", "multi.ts", "close.ts at line 1", "reopen is used"} {
		if strings.Contains(got, not) {
			t.Errorf("%q in:\n%s", not, got)
		}
	}
	if section := callersSection(dir, "ledger.ts", lines, material{}); section != "" {
		t.Errorf("no material, yet a section:\n%s", section)
	}
}

// A caller's site goes into the prompt as context, after the material and before the grammar.
func TestTheCallersSectionSitsBeforeTheGrammar(t *testing.T) {
	lines := strings.Split(headLedger, "\n")
	m := findMaterial("ledger.ts", lines, map[int]bool{3: true})
	prompt := userPrompt("ledger.ts", lines, m, nil, 120, "\nCallers of this file's exported declarations, as context only.\n")
	callers, grammar := strings.Index(prompt, "Callers of this file"), strings.Index(prompt, "Reply with one line per id")
	if callers < 0 || grammar < callers {
		t.Fatalf("prompt:\n%s", prompt)
	}
}

// Exported names are read from TypeScript and Go declarations, and an unexported one is none.
func TestExportedNames(t *testing.T) {
	for line, want := range map[string]string{
		"export async function claimsKey(a: A): Promise<boolean> {": "claimsKey",
		"export const enum Mode {":                                  "Mode",
		"export enum Mode {":                                        "Mode",
		"function local(): void {":                                  "",
	} {
		if got := exportedName("a.ts", line); got != want {
			t.Errorf("%q: %q, want %q", line, got, want)
		}
	}
	for line, want := range map[string]string{"func Post(e Entry) {": "Post", "func (b *Book) Close() {": "Close", "type Ledger struct {": "Ledger", "func post() {": ""} {
		if got := exportedName("a.go", line); got != want {
			t.Errorf("%q: %q, want %q", line, got, want)
		}
	}
}

// The review's inputs: the declaration around a call nested in an if, a call after a default import, a
// path with a colon, a name inside a string, a CRLF caller, and a site at the file's end.
func TestCallSitesReadAsTheyAre(t *testing.T) {
	nested := []string{"export function b() {", "  const helper = () => 1;", "  if (ok) {", "    post(x);", "  }", "}"}
	if got := enclosingDeclaration(nested, 4); got != 1 {
		t.Errorf("the declaration around a call in an if is line %d, want 1", got)
	}
	if inImport([]string{"import post from './ledger'", "post([])"}, 2) {
		t.Error("a call after a default import read as an import")
	}
	if !inImport([]string{"import {", "  post,", "} from './ledger';"}, 2) {
		t.Error("a name in an open import list read as a call")
	}

	dir, _ := fixture(t)
	write(t, dir, "a:b.ts", "export function x(): void {\n  post(2);\n}\n")
	write(t, dir, "api.ts", "export function send(url: string): void {\n  fetch(url, { method: \"post\" });\n}\n")
	write(t, dir, "crlf.ts", "/** Closes. */\r\nexport function c(): void {\r\n  post(3);\r\n}\r\n")
	direct, further := callSites(dir, "ledger.ts", "post")
	sites := strings.Join(append(direct, further...), "")
	for _, want := range []string{"post is used in a:b.ts at line 2", "post is used in crlf.ts at line 3"} {
		if !strings.Contains(sites, want) {
			t.Errorf("missing %q in:\n%s", want, sites)
		}
	}
	if strings.Contains(sites, "api.ts") {
		t.Errorf("a name in a string read as a use:\n%s", sites)
	}
	if strings.Contains(sites, "\r") {
		t.Errorf("a carriage return reached the prompt:\n%q", sites)
	}
	if strings.Contains(sites, "     5  \n") {
		t.Errorf("a line past the file's end:\n%s", sites)
	}
}

// A call ranks ahead of a reference, whatever the file names.
func TestACallRanksAheadOfAReference(t *testing.T) {
	dir, _ := fixture(t)
	write(t, dir, "a.ts", "export const handlers = [post];\n")
	write(t, dir, "z.ts", "export function run(): void {\n  post([]);\n}\n")
	sites, _ := callSites(dir, "ledger.ts", "post")
	if len(sites) == 0 || !strings.Contains(sites[0], "z.ts") {
		t.Fatalf("the call did not come first:\n%s", strings.Join(sites, ""))
	}
}

// A site's purpose is often stated one call further out, so the section follows the declaration
// around each first-level site to its own callers, once each. A first-level site shows that
// declaration's whole body where it is short, and a comment just above a window is shown whole.
func TestTheSectionReachesOneCallFurtherOut(t *testing.T) {
	dir, _ := fixture(t)
	write(t, dir, "close.ts", "export function closeDay(entries: Entry[]): void {\n  audit(entries);\n  post(entries);\n"+
		"  post(entries.slice(1));\n  a();\n  b();\n  c();\n  d();\n  // The whole job is to settle first.\n  settle();\n}\n")
	write(t, dir, "night.ts", "export function runNight(): void {\n  open();\n  // The morning report reads a settled book,\n"+
		"  // so the night closes the day first.\n  tidy();\n  check();\n  if (ready) {\n    closeDay(all);\n  }\n}\n")
	direct, further := callSites(dir, "ledger.ts", "post")
	sites := strings.Join(append(direct, further...), "")
	for _, want := range []string{
		"The whole job is to settle first.",
		"closeDay is used in night.ts at line 8 (which runs post at close.ts line 3)",
		"// The morning report reads a settled book,",
	} {
		if !strings.Contains(sites, want) {
			t.Errorf("missing %q in:\n%s", want, sites)
		}
	}
	if n := strings.Count(sites, "closeDay is used in"); n != 1 {
		t.Errorf("closeDay's callers shown %d times, want once:\n%s", n, sites)
	}
}

// A call chain and a local holding a plain value enclose nothing a reader would name.
func TestACallChainEnclosesNothing(t *testing.T) {
	lines := []string{"function newCellTests() {", "  const name = format(cell);", "  list().forEach(cell => {", "    post(cell);", "  });", "}"}
	if got := enclosingDeclaration(lines, 4); got != 1 {
		t.Errorf("the declaration around the call is line %d, want 1", got)
	}
}

// The review's inputs: a long comment run above a window, a wrapper of the same name, a whole body's
// own comment, and a method, which no second level follows.
func TestTheSectionStaysBoundedAndFollowsOnlyWhatItShould(t *testing.T) {
	dir, _ := fixture(t)
	write(t, dir, "big.ts", strings.Repeat("// old line\n", 400)+"post(y);\n")
	write(t, dir, "wrap.ts", "export function post(x) {\n  return api.post(x);\n}\n")
	write(t, dir, "day.ts", "// Settles the day.\nexport function settleDay() {\n  post(x);\n}\n")
	write(t, dir, "book.ts", "export class Book {\n  close() {\n    post(this);\n  }\n}\n")
	write(t, dir, "shelf.ts", "export function shelve(b: Book): void {\n  b.close();\n}\n")
	direct, further := callSites(dir, "ledger.ts", "post")
	all := strings.Join(append(direct, further...), "")
	if n := strings.Count(all, "// old line"); n > linesAbove+maxEdgeLines {
		t.Errorf("%d comment lines at a window, over %d", n, linesAbove+maxEdgeLines)
	}
	if strings.Contains(all, "(which runs post at wrap.ts") {
		t.Errorf("a same-named wrapper was followed:\n%s", all)
	}
	if n := strings.Count(all, "Settles the day."); n > 1 {
		t.Errorf("a whole body's comment shown %d times:\n%s", n, all)
	}
	if strings.Contains(all, "close is used in") {
		t.Errorf("a method was followed to a second level:\n%s", all)
	}
}
