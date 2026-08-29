package shell

import "testing"

// The block has to close. Walking to end-of-file and accepting on the way lets a body line answer for
// a block that never closed — a file the loader cannot read frontmatter from at all, reported as a
// clean declaration. The old reader was bounded to lines 2-10, so past line 10 it reported nothing
// and the mismatch surfaced; the bound went, and end-of-file has to take its place as the terminator.
func TestFrontmatterMustClose(t *testing.T) {
	name := func(lines []string) string { return FrontmatterName(lines) }

	closed := []string{"---", "name: alpha", "---", "body", "name: beta"}
	if got := name(closed); got != "alpha" {
		t.Errorf("closed block: got %q, want alpha", got)
	}
	// Never closed: everything after line 1 is body, whatever it looks like.
	unterminated := []string{"---", "intro", "body text", "name: beta"}
	if got := name(unterminated); got != "" {
		t.Errorf("unterminated block: got %q, want empty — a body line answered as a declaration", got)
	}
	// Far enough down that the old line-2-to-10 window would also have missed it.
	deep := []string{"---", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "name: beta"}
	if got := name(deep); got != "" {
		t.Errorf("unterminated, past line 10: got %q, want empty", got)
	}
	// A file that does not open with a delimiter has no frontmatter at all.
	if got := name([]string{"# Title", "name: beta"}); got != "" {
		t.Errorf("no opening delimiter: got %q, want empty", got)
	}
	// An empty block closes immediately and declares nothing.
	if got := name([]string{"---", "---", "name: beta"}); got != "" {
		t.Errorf("empty block: got %q, want empty", got)
	}
	if got := name(nil); got != "" {
		t.Errorf("no lines: got %q, want empty", got)
	}
}
