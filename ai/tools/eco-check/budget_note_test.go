package ecocheck

// The census note names the imports the budget could not count, on the line that prints on a clean
// run. It is driven directly here, not through a tree: the import pattern's charset admits no control
// byte today, so no fixture can carry one into the note. The guard is there for the scan that widens
// that charset, and a guard with no case is one a later edit removes without a word.

import (
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
