package aibootstrap_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	voicecheck "configs/ai/tools/voice-check"
)

// The owner's run creates the private-name list once, empty, and points a git checkout's hooks at
// ai/hooks. A second run keeps the list its owner filled.
func TestTheOwnerRunCreatesTheListAndSetsTheHooks(t *testing.T) {
	f := newFixture(t)
	checkout := filepath.Dir(f.repo)
	if out, err := exec.Command("git", "-C", checkout, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	f.ExpectCode(f.install("--agent=claude", "--owner"), 0)
	list := filepath.Join(f.home, ".config", "kk-flavor", "private-names.txt")
	f.ExpectFileBody(list, voicecheck.PrivateNamesHeader)
	set, _ := exec.Command("git", "-C", checkout, "config", "--get", "core.hooksPath").Output()
	if strings.TrimSpace(string(set)) != "ai/hooks" {
		t.Fatalf("core.hooksPath is %q", set)
	}
	filled := voicecheck.PrivateNamesHeader + "LedgerVault\n"
	if err := os.WriteFile(list, []byte(filled), 0o600); err != nil {
		t.Fatal(err)
	}
	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
	f.ExpectFileBody(list, filled)
}

// The commit-msg hook refuses a message holding an entry, and names the entry's line, not its text.
func TestTheCommitMessageHookRefusesAPrivateName(t *testing.T) {
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside the configs checkout")
	}
	dir := t.TempDir()
	list := filepath.Join(dir, "private-names.txt")
	message := filepath.Join(dir, "MSG")
	if err := os.WriteFile(list, []byte("# list\nLedgerVault\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(strings.TrimSpace(string(top)), "ai", "hooks", "commit-msg")
	for body, refused := range map[string]bool{"Read the ledgervault table\n": true, "Fix the book\n": false} {
		if err := os.WriteFile(message, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(hook, message)
		cmd.Env = append(os.Environ(), "KK_PRIVATE_NAMES="+list)
		out, err := cmd.CombinedOutput()
		if (err != nil) != refused || (refused && !strings.Contains(string(out), "the entry on line 2 of the private-name list")) ||
			strings.Contains(strings.ToLower(string(out)), "ledgervault") {
			t.Errorf("%q: refused %v, want %v:\n%s", body, err != nil, refused, out)
		}
	}
}
