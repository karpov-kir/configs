package bloatjudge

import (
	modelpolicy "kk-flavor/tools/model-policy"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProviderSelection(t *testing.T) {
	for _, c := range []struct{ name, requested, thread, installed, want string }{
		{"explicit codex", "codex", "", "claude codex", "codex"},
		{"explicit claude", "claude", "thread", "claude codex", "claude"},
		{"missing in Codex task", "", "thread", "claude codex", ""},
		{"missing with both installed", "", "", "claude codex", ""},
		{"auto with Codex only", "auto", "", "codex", ""},
		{"auto with Claude only", "auto", "", "claude", ""},
		{"explicit missing", "codex", "", "claude", ""},
		{"explicit Claude missing", "claude", "thread", "codex", ""},
		{"missing with Claude only", "", "", "claude", ""},
		{"missing with Codex only", "", "thread", "codex", ""},
		{"neither installed", "", "", "", ""},
		{"unknown provider", "typo", "", "claude codex", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range strings.Fields(c.installed) {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir)
			t.Setenv("JUDGE_PROVIDER", c.requested)
			t.Setenv("CODEX_THREAD_ID", c.thread)
			got, err := resolveProvider()
			if c.want == "" {
				if err == nil {
					t.Fatalf("accepted unavailable or invalid provider: %s", got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("provider = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestCodexCallerUsesOnlyTheFinalMessage(t *testing.T) {
	fakeCodex(t, `while [ "$#" -gt 0 ]; do
 if [ "$1" = "--output-last-message" ]; then shift; answer="$1"; fi
 shift
done
printf 'progress, not a verdict\n'
printf 'none\n' > "$answer"`)
	got, err := CodexCaller(time.Second, testSettings())("prompt", "view")
	if err != nil || got != "none\n" {
		t.Fatalf("answer = %q, %v; want final message", got, err)
	}
}

func fakeCodex(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCodexCallerRefusesMissingFinalAnswer(t *testing.T) {
	fakeCodex(t, "echo none")
	if _, err := CodexCaller(time.Second, testSettings())("prompt", "view"); err == nil || !strings.Contains(err.Error(), "final answer") {
		t.Fatalf("missing answer accepted: %v", err)
	}
}

func TestCodexCallerRefusesFailedProcess(t *testing.T) {
	fakeCodex(t, "exit 7")
	if _, err := CodexCaller(time.Second, testSettings())("prompt", "view"); err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("failed process accepted: %v", err)
	}
}

func TestCodexCallerBoundsTheRoll(t *testing.T) {
	fakeCodex(t, "sleep 30")
	if _, err := CodexCaller(100*time.Millisecond, testSettings())("prompt", "view"); err == nil || !strings.Contains(err.Error(), "within 100ms") {
		t.Fatalf("timeout was not reported: %v", err)
	}
}

func TestCodexCallerIsolatesInputAndCleansItsDirectory(t *testing.T) {
	record := filepath.Join(t.TempDir(), "cwd")
	t.Setenv("JUDGE_TEST_CWD", record)
	fakeCodex(t, `pwd > "$JUDGE_TEST_CWD"
for arg do
 case "$arg" in
  --ignore-user-config) user_config=1;;
  --ignore-rules) rules=1;;
  project_doc_max_bytes=0) instructions=1;;
  read-only) sandbox=1;;
 esac
done
[ "$user_config$rules$instructions$sandbox" = 1111 ] || exit 9
while [ "$#" -gt 0 ]; do
 if [ "$1" = "--output-last-message" ]; then shift; answer="$1"; fi
 shift
done
cat > "$answer"`)
	got, err := CodexCaller(time.Second, testSettings())("judge", "résumé $() `command`")
	if err != nil || got != "judge\n\nrésumé $() `command`" {
		t.Fatalf("input = %q, %v", got, err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	dir := strings.TrimSpace(string(raw))
	if !strings.Contains(filepath.Base(dir), "bloat-judge-") {
		t.Fatalf("not isolated: %s", dir)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary directory survived: %s, %v", dir, err)
	}
}

func testSettings() modelpolicy.Settings {
	return modelpolicy.Settings{Model: "fixture-model", Effort: "low"}
}

func TestRetiredJudgeModelIsRejected(t *testing.T) {
	t.Setenv("JUDGE_MODEL", "chosen-model")
	if _, err := Configure(Configuration{Deadline: time.Second, PolicyPath: "missing"}); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("retired override not rejected: %v", err)
	}
}

func TestConfiguredJudgeUsesTheCentralPolicy(t *testing.T) {
	fakeCodex(t, "exit 0")
	t.Setenv("JUDGE_PROVIDER", "codex")
	t.Setenv("JUDGE_MODEL", "")
	os.Unsetenv("JUDGE_MODEL")
	configured, err := Configure(Configuration{Deadline: time.Second, PolicyPath: "../../kk-flavor/models.json"})
	if err != nil {
		t.Fatal(err)
	}
	if configured.Decision.Requested.Model != "gpt-5.6-luna" || configured.Decision.Requested.Effort != "low" {
		t.Fatalf("central judge assignment = %+v", configured.Decision)
	}
	if configured.Decision.Rolls != 3 {
		t.Fatalf("central judge roll count = %d", configured.Decision.Rolls)
	}
	for _, args := range [][]string{codexArgs("answer", testSettings()), claudeArgs("prompt", testSettings())} {
		found := false
		for i, arg := range args {
			if arg == "--model" && i+1 < len(args) && args[i+1] == "fixture-model" {
				found = true
			}
		}
		if !found {
			t.Fatalf("configured model was lost: %v", args)
		}
	}
}

func TestJudgeCacheSeparatesClientSelections(t *testing.T) {
	fakeCodex(t, "exit 0")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	identities := map[string]bool{}
	for _, client := range []string{"codex", "claude"} {
		t.Setenv("JUDGE_PROVIDER", client)
		configured, err := Configure(Configuration{Deadline: time.Second, PolicyPath: "../../kk-flavor/models.json"})
		if err != nil {
			t.Fatal(err)
		}
		if configured.CacheIdentity == "" || identities[configured.CacheIdentity] {
			t.Fatalf("judge cache did not distinguish %s selection: %q", client, configured.CacheIdentity)
		}
		identities[configured.CacheIdentity] = true
	}
}
