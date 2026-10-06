package voicecheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo/repotest"
)

// writeList writes a private-name list and returns a config that reads it.
func writeList(t *testing.T, body string) Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private-names.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := baseConfig()
	cfg.PrivateNames = path
	return cfg
}

// The list lives outside every repository, and a test can name its own.
func TestThePrivateNameListLivesUnderTheConfigDirectory(t *testing.T) {
	env := map[string]string{"HOME": "/home/owner"}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	if got := PrivateNamesPath(lookup); got != "/home/owner/.config/kk-flavor/private-names.txt" {
		t.Fatalf("path %q", got)
	}
	env["XDG_CONFIG_HOME"] = "/cfg"
	if got := PrivateNamesPath(lookup); got != "/cfg/kk-flavor/private-names.txt" {
		t.Fatalf("path %q", got)
	}
	env[privateNamesEnv] = "/tmp/list.txt"
	if got := PrivateNamesPath(lookup); got != "/tmp/list.txt" {
		t.Fatalf("path %q", got)
	}
}

// A plain entry matches as a whole word in any case, a `re:` entry as written, and a comment or a
// blank line is no entry. A finding names the entry's line and never the words it matched.
func TestAPrivateNameIsFoundAndNeverQuoted(t *testing.T) {
	names, err := loadPrivateNames(writeList(t, PrivateNamesHeader+"\nLedgerVault\nre:acme-[0-9]+\n").PrivateNames)
	if err != nil || len(names) != 2 {
		t.Fatalf("loaded %d entries: %v", len(names), err)
	}
	for text, want := range map[string]int{"the ledgervault book": 1, "LedgerVaults": 0, "posted by acme-42": 1, "acme-x": 0} {
		if got := len(names.hits(text)); got != want {
			t.Errorf("%q: %d hit(s), want %d", text, got, want)
		}
	}
	s := scanner{profile: ProfileComment, private: names}
	found := s.scanSource("f.ts", []string{"// The LedgerVault book closes at midnight.", "export const CLOSE = 0;"}, nil, nil)
	if !hasCheck(found, checkPrivateName) {
		t.Fatalf("no private-name finding: %v", found)
	}
	for _, f := range found {
		if strings.Contains(strings.ToLower(f.Text), "ledgervault") {
			t.Fatalf("a finding quotes the name: %+v", f)
		}
	}
}

// The branch mode reads every line a range adds, code as much as comments, and every commit message,
// and reports each by its place and the entry's line.
func TestTheBranchModeReadsAddedLinesAndCommitMessages(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.ts"), []byte("export const A = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.ts"), []byte("export const A = 1;\nconst vault = 'LedgerVault';\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "Read the ledgervault table")
	cfg := writeList(t, "# list\nLedgerVault\n")
	var out, errOut strings.Builder
	code := Run("voice-check.sh", []string{"--private-names", "HEAD~1...HEAD"}, dir, repotest.New(dir), cfg, &out, &errOut)
	text := out.String()
	if code != 1 || !strings.Contains(text, "a.ts:2: private-name: the entry on line 2 of the private-name list") ||
		!strings.Contains(text, ": private-name: the entry on line 2") || strings.Count(text, "private-name:") != 2 {
		t.Fatalf("exit %d:\n%s%s", code, text, errOut.String())
	}
	if strings.Contains(strings.ToLower(text+errOut.String()), "ledgervault") {
		t.Fatalf("the report quotes the name:\n%s%s", text, errOut.String())
	}
	message := filepath.Join(dir, "MSG")
	if err := os.WriteFile(message, []byte("Fix the book\n\nNo private word here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run("voice-check.sh", []string{"--private-names", "--message=" + message}, dir, repotest.New(dir), cfg, &out, &errOut); code != 0 {
		t.Fatalf("a clean message exits %d", code)
	}
	if err := os.WriteFile(message, []byte("Read the LEDGERVAULT table\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run("voice-check.sh", []string{"--private-names", "--message=" + message}, dir, repotest.New(dir), cfg, &out, &errOut); code != 1 {
		t.Fatalf("a message holding a name exits %d", code)
	}
}

// No list is an empty one, and an entry that does not compile refuses the scan.
func TestAnEmptyListPassesAndABrokenEntryRefuses(t *testing.T) {
	dir := t.TempDir()
	var out, errOut strings.Builder
	cfg := baseConfig()
	cfg.PrivateNames = filepath.Join(dir, "absent.txt")
	if code := Run("voice-check.sh", []string{"--private-names", "HEAD"}, dir, repotest.New(dir), cfg, &out, &errOut); code != 0 ||
		!strings.Contains(errOut.String(), "nothing was matched") {
		t.Fatalf("no list exits %d: %s", code, errOut.String())
	}
	if code := Run("voice-check.sh", []string{"--private-names", "HEAD"}, dir, repotest.New(dir), writeList(t, "re:(\n"), &out, &errOut); code != 2 {
		t.Fatalf("a broken entry exits %d", code)
	}
}
