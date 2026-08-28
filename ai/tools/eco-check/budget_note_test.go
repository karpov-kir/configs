package ecocheck

// The census note names the imports the budget could not count, on the line that prints on a clean
// run. It is driven directly here, not through a tree: the import pattern's charset admits no control
// byte today, so no fixture can carry one into the note. The guard is there for the scan that widens
// that charset, and a guard with no case is one a later edit removes without a word.

import (
	"fmt"
	"strings"
	"testing"
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
