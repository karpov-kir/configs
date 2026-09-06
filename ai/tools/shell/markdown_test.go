package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// The audience marker, read through the same block walk as every other declaration: a skill that
// declares itself maintainer-only is one ai/bootstrap.sh leaves unmounted under
// --skip-maintainer-skills, and one eco-check's mount scan then expects to find no mount for.
func TestTheMaintainerMarkerIsReadOnlyOutOfFrontmatter(t *testing.T) {
	marked := []string{"---", "name: kk-reduce", "audience: maintainer", "---", "body"}
	if !IsMaintainerAudience(marked) {
		t.Error("a declared marker went unread, so bootstrap would mount a skill the scan expects unmounted")
	}
	// Spacing and case, the way the opt-out marker beside it is read.
	if !IsMaintainerAudience([]string{"---", "Audience:   Maintainer  ", "---"}) {
		t.Error("the marker is case- and space-insensitive, and one spelling of it was refused")
	}
	if IsMaintainerAudience([]string{"---", "audience: everyone", "---"}) {
		t.Error("another audience answered as the maintainer one")
	}
	if IsMaintainerAudience([]string{"---", "name: kk-build", "---", "audience: maintainer"}) {
		t.Error("a body line answered as a declaration — that skill would silently stop being installed")
	}
	if IsMaintainerAudience([]string{"---", "audience: maintainer"}) {
		t.Error("an unterminated block is not frontmatter, and the loader cannot read one either")
	}
	if IsMaintainerAudience(nil) {
		t.Error("no lines declared a marker")
	}
}

// ai/bootstrap.sh reads this same marker, in awk, on a machine that has no Go binary yet — so it
// cannot call in here and the pattern is written twice. Drift between them is silent where it hurts
// most: on the maintainer's own install every skill is mounted, so the scan stays quiet, and only an
// external install sees the skills bootstrap left out being reported as missing mounts.
func TestTheScriptAndThisPackageSpellTheMarkerTheSameWay(t *testing.T) {
	pattern := maintainerAudience.String()
	// The control. Without it a renamed or moved script leaves the assertion below comparing the
	// pattern against nothing, which every empty file "contains".
	script, err := os.ReadFile(filepath.Join("..", "..", "bootstrap.sh"))
	if err != nil || len(script) == 0 {
		t.Fatalf("reading ai/bootstrap.sh, which is the other reader of this marker: %v", err)
	}
	if !strings.Contains(string(script), "/"+pattern+"/") {
		t.Errorf("ai/bootstrap.sh does not match on /%s/, so the script and this package disagree about "+
			"which skills the audience marker covers. Whichever one is right, both have to say it.", pattern)
	}
}
