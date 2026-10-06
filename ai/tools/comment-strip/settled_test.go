package commentstrip

import (
	"strings"
	"testing"
)

// The lane's decisions are its own record. Run 20's lane kept an interface and run 26's turned it into
// a type alias, with no record between the two. A lane that keeps one declaration and rewrites another
// settles both. The next lane's edit to either is refused. A record lapses when another hand changes
// its code.
func TestTheLaneSettlesWhatItKeepsAndEdits(t *testing.T) {
	archive := t.TempDir()
	received := strings.Split("// A ledger keeps each book apart.\nexport interface Book {\n  id: string;\n}\n\n"+
		"export function post(book: Book): void {\n  write(book);\n}\n", "\n")
	left := strings.Split("// A ledger keeps each book apart.\nexport interface Book {\n  id: string;\n}\n\n"+
		"export function post(book: Book): void {\n  write(book.id);\n}\n", "\n")
	if edits, err := Settle(archive, "run26", "ledger.ts", received, left, []int{1}); err != nil || len(edits) != 0 {
		t.Fatalf("the first lane: %v %v", edits, err)
	}
	held := SettledHolding(archive, "ledger.ts", left)
	if len(held) != 2 || !strings.Contains(strings.Join(held, "\n"), "export interface Book {") ||
		!strings.Contains(strings.Join(held, "\n"), "export function post(book: Book): void {") {
		t.Fatalf("settled %q, want the kept interface and the edited function", held)
	}
	// The next lane turns the interface into a type alias.
	alias := strings.Split("// A ledger keeps each book apart.\nexport type Book = {\n  id: string;\n};\n\n"+
		"export function post(book: Book): void {\n  write(book.id);\n}\n", "\n")
	edits, err := Settle(archive, "run27", "ledger.ts", left, alias, nil)
	if err != nil || len(edits) != 1 || !strings.Contains(edits[0], "the lane edited `export interface Book {`, which run26 settled") {
		t.Fatalf("the next lane's edit: %v %v", edits, err)
	}
	// Another hand changes the function before the lane reads it, and its record lapses.
	changed := strings.Split(strings.Join(left, "\n")+"export const VERSION = 2;\n", "\n")
	changed[6] = "  write(book.id, VERSION);"
	if edits, err := Settle(archive, "run28", "ledger.ts", changed, changed, nil); err != nil || len(edits) != 0 {
		t.Fatalf("a lane that edits nothing: %v %v", edits, err)
	}
	if held := SettledHolding(archive, "ledger.ts", changed); len(held) != 1 || !strings.Contains(held[0], "export interface Book {") {
		t.Fatalf("after another hand changed the function, settled %q", held)
	}
}

// A member the lane kept is settled and refused like a top declaration. A record that lapsed leaves
// the file even where the lane rules on no other declaration.
func TestASettledMemberIsHeldAndALapsedOneLeaves(t *testing.T) {
	archive := t.TempDir()
	file := strings.Split("export class Book {\n  // A ledger totals a book once per close.\n  total(): number {\n    return 1;\n  }\n}\n", "\n")
	if _, err := Settle(archive, "run26", "ledger.ts", file, file, []int{2}); err != nil {
		t.Fatal(err)
	}
	edited := append([]string{}, file...)
	edited[3] = "    return 2;"
	if edits, _ := Settle(archive, "run27", "ledger.ts", file, edited, nil); len(edits) != 1 || !strings.Contains(edits[0], "`total(): number {`") {
		t.Fatalf("a later lane's edit to the member: %v", edits)
	}
	// Another hand changes the member before the lane, and its record leaves the file.
	if edits, err := Settle(archive, "run28", "ledger.ts", edited, edited, nil); err != nil || len(edits) != 0 {
		t.Fatalf("a lane that edits nothing: %v %v", edits, err)
	}
	if held := SettledHolding(archive, "ledger.ts", edited); len(held) != 0 {
		t.Fatalf("a lapsed record still lists %q", held)
	}
}
