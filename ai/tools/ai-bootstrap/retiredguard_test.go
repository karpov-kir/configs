package aibootstrap_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The retired private-name guard is undone on a machine that ran it: the hooks path it set is unset and
// its empty list removed. A list its owner filled is kept.
func TestTheOwnerRunRemovesTheRetiredPrivateNameGuard(t *testing.T) {
	f := newFixture(t)
	checkout := filepath.Dir(f.repo)
	for _, args := range [][]string{{"init", "-q"}, {"config", "core.hooksPath", "ai/hooks"}} {
		if out, err := exec.Command("git", append([]string{"-C", checkout}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	list := filepath.Join(f.home, ".config", "kk-flavor", "private-names.txt")
	f.MkdirAll(filepath.Dir(list))
	if err := os.WriteFile(list, []byte("# Private names, one per line.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.ExpectCode(f.install("--agent=claude", "--owner"), 0)
	if set, _ := exec.Command("git", "-C", checkout, "config", "--get", "core.hooksPath").Output(); strings.TrimSpace(string(set)) != "" {
		t.Fatalf("core.hooksPath is still %q", set)
	}
	if _, err := os.Stat(list); !os.IsNotExist(err) {
		t.Fatalf("the empty list stands: %v", err)
	}
	// A filled list is its owner's, and stays.
	if err := os.WriteFile(list, []byte("# list\nLedgerVault\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
	if _, err := os.Stat(list); err != nil {
		t.Fatalf("a filled list was removed: %v", err)
	}
}
