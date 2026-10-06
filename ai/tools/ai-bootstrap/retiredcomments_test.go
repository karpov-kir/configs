package aibootstrap_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A machine that ran the retired comment pipeline loses its agent link, whose source the flavor no
// longer ships, and its state moves to the Trash, where it can still be restored.
func TestTheOwnerRunRetiresTheCommentPipeline(t *testing.T) {
	f := newFixture(t)
	agents := filepath.Join(f.home, ".claude", "agents")
	f.MkdirAll(agents)
	stale := filepath.Join(agents, "comment-writer.md")
	if err := os.Symlink(filepath.Join(f.repo, "kk-flavor", "agents", "comment-writer.md"), stale); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(f.home, ".local", "state", "kk-flavor", "comments", "ledger-1")
	f.MkdirAll(archive)
	f.ExpectCode(f.install("--agent=claude", "--owner"), 0)
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("the stale agent link stands: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(archive)); !os.IsNotExist(err) {
		t.Fatalf("the archive stands: %v", err)
	}
	trashed, _ := filepath.Glob(filepath.Join(f.home, ".Trash", "*comments-retired-*", "ledger-1"))
	if len(trashed) != 1 {
		t.Fatalf("the archive did not reach the Trash: %v", trashed)
	}
}
