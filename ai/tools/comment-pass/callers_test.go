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
	sites := strings.Join(callSites(dir, "ledger.ts", "post"), "")
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
	sites := callSites(dir, "ledger.ts", "post")
	if len(sites) == 0 || !strings.Contains(sites[0], "z.ts") {
		t.Fatalf("the call did not come first:\n%s", strings.Join(sites, ""))
	}
}
