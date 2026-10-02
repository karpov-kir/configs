package commentstrip

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

// The movedRules entry holds only while the move changed no word. The test rebuilds the rules as
// they stood before it: code-style.md as it was, with comments.md's text back under its heading, and
// the brief citing the old file. Those sum to the old key, and the files now sum to the new one.
func TestMovedRulesRebuildTheRulesBeforeTheMove(t *testing.T) {
	read := func(path string) string {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	sum := func(parts ...string) string {
		h := sha256.Sum256([]byte(strings.Join(parts, "")))
		return hex.EncodeToString(h[:])[:12]
	}
	comments, brief := read("../../kk-flavor/standards/comments.md"), read("../../kk-flavor/workers/comment-writer.md")
	if len(movedRules) != 1 {
		t.Fatalf("%d moved-rules entries, want the one the move made", len(movedRules))
	}
	for old, now := range movedRules {
		if sum(comments, brief) != now {
			t.Skipf("the rules changed after the move, so the entry no longer applies and can go")
		}
		heading := "**Layer:** craft\n\n# Comments\n\n"
		if !strings.HasPrefix(comments, heading) {
			t.Fatalf("comments.md does not open on its heading")
		}
		before := strings.Replace(read("testdata/code-style-before-move.txt"), "<<comments.md>>\n", strings.TrimPrefix(comments, heading), 1)
		cited := strings.ReplaceAll(brief, "`~/.kk-flavor/standards/comments.md`", "`~/.kk-flavor/standards/code-style.md` → **Comments**")
		if got := sum(before, cited); got != old {
			t.Errorf("the rules before the move sum to %s, and the entry says %s", got, old)
		}
		if !sameRules(old, now) || sameRules(old, "") || sameRules("", now) {
			t.Error("sameRules does not read the entry")
		}
	}
}

// A block archived under rules that only moved stands under the rules after the move, and reopens once
// the entry is gone. Run 21's 85 blocks read 85 kept with it and 0 without.
func TestABlockUnderMovedRulesStandsOnlyThroughTheEntry(t *testing.T) {
	rulesHome(t, "rules before the move ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	archiveWritten(t, f, archive)
	before := rulesSum()
	rulesHome(t, "rules after the move ")
	lines := shell.SplitLines(keptSource)
	for _, tc := range []struct {
		moved map[string]string
		kept  bool
	}{
		{map[string]string{before: rulesSum()}, true},
		{map[string]string{}, false},
		{map[string]string{"000000000000": rulesSum()}, false},
	} {
		saved := movedRules
		movedRules = tc.moved
		got := KeepVerdicts(archive, f.path, lines)
		movedRules = saved
		if len(got) != 1 || (got[0].Run == "run11") != tc.kept || !tc.kept && got[0].Reopened != "the rules changed since run11" {
			t.Errorf("entries %v: got %+v, want kept %v", tc.moved, got, tc.kept)
		}
	}
}
