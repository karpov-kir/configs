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
