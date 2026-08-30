package ecocheck

// The census note names the imports the budget could not count, on the line that prints on a clean
// run. It is driven directly here, not through a tree: the import pattern's charset admits no control
// byte today, so no fixture can carry one into the note. The guard is there for the scan that widens
// that charset, and a guard with no case is one a later edit removes without a word.

import (
	"fmt"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

func TestUncountedNoteCarriesNoControlByte(t *testing.T) {
	note := uncountedNote([]string{"a\x1b[2Kb\x00.md"})

	// The control: without it the assertion below passes on an empty note.
	if !strings.Contains(note, "1 uncounted import(s)") {
		t.Fatalf("the note has to name the count it exists to carry: %q", note)
	}
	if strings.ContainsAny(note, "\x1b\x00") {
		t.Errorf("a control byte reached the line that rides a clean run: %q", note)
	}
}

// Three caps, all on the same line and all for one reason: the note rides the exit-0 path, so an
// uncapped list prints as much attacker-chosen text as the tree cares to write underneath
// `wiring: clean`. Every case here asserts the count as well, because the count is the part that must
// stay exact — trimming the naming is the whole point, and a cap that trimmed the figure too would
// make the note lie about how much it withheld.
func TestUncountedNamesAreCapped(t *testing.T) {
	t.Run("names ten and says how many it withheld", func(t *testing.T) {
		var many []string
		for i := 1; i <= 40; i++ {
			many = append(many, fmt.Sprintf("name%02d.md", i))
		}
		note := uncountedNote(many)

		if !strings.Contains(note, "40 uncounted import(s)") {
			t.Fatalf("the count stays exact whatever the naming is trimmed to: %q", note)
		}
		if !strings.Contains(note, "and 30 more") {
			t.Errorf("the note has to say how many names it withheld: %q", note)
		}
		if strings.Contains(note, "name11.md") {
			t.Errorf("an eleventh name reached the line that rides a clean run: %q", note)
		}
	})

	// Entries alone are not a bound on bytes: ten names the tree chose the length of are still as long
	// as the tree wants them.
	t.Run("bounds the naming in bytes and not only in entries", func(t *testing.T) {
		long := strings.Repeat("wide", 80) + ".md"
		note := uncountedNote([]string{long, long + "b", long + "c", long + "d", long + "e",
			long + "f", long + "g", long + "h", long + "i", long + "j"})

		if !strings.Contains(note, "10 uncounted import(s)") {
			t.Fatalf("the count stays exact whatever the naming is trimmed to: %q", note)
		}
		if len(note) > 400 {
			t.Errorf("ten tree-chosen names printed %d bytes under a clean run: %q", len(note), note)
		}
	})

	// And one name on its own is bounded too, or a single committed 8 MB filename is the whole line.
	t.Run("bounds a single name on its own", func(t *testing.T) {
		note := uncountedNote([]string{strings.Repeat("wide", 80) + ".md"})

		if !strings.Contains(note, "1 uncounted import(s)") {
			t.Fatalf("the count stays exact whatever the naming is trimmed to: %q", note)
		}
		if len(note) > 120 {
			t.Errorf("one tree-chosen name printed %d bytes under a clean run: %q", len(note), note)
		}
	})
}

// This list is what a later pass greps to find an import again, so a name cut at 60 bytes with
// nothing saying so is a name that pass will not find — and cannot tell from one the tree really
// wrote that short. The bound stays where it is; what the marker adds is that the reader knows which
// of the two they are holding.
func TestACutUncountedNameSaysThatItWasCut(t *testing.T) {
	t.Run("marks a name it cut", func(t *testing.T) {
		note := uncountedNote([]string{strings.Repeat("wide", 80) + ".md"})

		// The control: without it the assertion below passes on an empty note.
		if !strings.Contains(note, "1 uncounted import(s)") {
			t.Fatalf("the note has to name the count it exists to carry: %q", note)
		}
		if !strings.Contains(note, "w"+shell.CutMarker) {
			t.Errorf("the uncounted name was cut with nothing saying so: %q", note)
		}
	})

	// The other direction. A mark on a name that was never cut says the opposite thing and is the same
	// size of lie, so the short name has to arrive whole and bare.
	t.Run("and leaves a name that fits unmarked", func(t *testing.T) {
		note := uncountedNote([]string{"short.md"})

		if !strings.Contains(note, "short.md") {
			t.Fatalf("the note has to name the import it could not count: %q", note)
		}
		if strings.Contains(note, shell.CutMarker) {
			t.Errorf("a name that fits inside the bound was marked as cut: %q", note)
		}
	})
}

// The joined list carries its own bound, and that one bites long before the per-name bound does: ten
// names of 29 bytes sit well inside 60 apiece and still overrun 200 together. Cut there with
// nothing saying so, the last name printed is a shorter name that reads as whole — the per-name
// defect above, one level up, on the same list a later pass greps to find the import again.
//
// No name here is long enough for the per-name bound to touch, so the only marker that can appear in
// this note is the list's, and the whole note is asserted rather than the marker's presence: the same
// name is marked at other bounds elsewhere in a real report, and "a marker is in there somewhere"
// passes whatever this call site does.
func TestACutUncountedListSaysThatItWasCut(t *testing.T) {
	t.Run("marks the list where the joined bound cut it", func(t *testing.T) {
		var tenNames []string
		for _, letter := range "abcdefghij" {
			tenNames = append(tenNames, strings.Repeat(string(letter), 26)+".md")
		}

		// Six whole names and their separators fill 180 of the 197 bytes the bound leaves once the
		// marker is reserved. The seventh arrives 17 bytes long and marked; the last three not at all.
		want := " + 10 uncounted import(s): " +
			strings.Repeat("a", 26) + ".md " +
			strings.Repeat("b", 26) + ".md " +
			strings.Repeat("c", 26) + ".md " +
			strings.Repeat("d", 26) + ".md " +
			strings.Repeat("e", 26) + ".md " +
			strings.Repeat("f", 26) + ".md " +
			strings.Repeat("g", 17) + shell.CutMarker
		if note := uncountedNote(tenNames); note != want {
			t.Errorf("the uncounted list\n got %q\nwant %q", note, want)
		}
	})

	// The other direction, and the one thing the marking cut could have broken here: a list inside the
	// bound is handed back untouched, so it still ends in the separator the line below the cut trims.
	// A marker on a list that was never cut, or a separator left dangling at the end of the note,
	// would each show up as the whole value differing.
	t.Run("and leaves a list that fits whole, with no separator left dangling", func(t *testing.T) {
		want := " + 2 uncounted import(s): alpha.md beta.md"
		if note := uncountedNote([]string{"alpha.md", "beta.md"}); note != want {
			t.Errorf("the uncounted list\n got %q\nwant %q", note, want)
		}
	})
}
