package modelpolicy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modelserved "configs/ai/tools/model-served"
)

// The package's cases never reach the user's served-model cache, or a real `claude auth status`.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "model-policy-cache-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CACHE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const servedPolicy = `{
  "version": 4,
  "limits": { "intents-in-flight": 1 },
  "tiers": { "codex": ["c1", "c2", "c3"], "claude": ["haiku", "sonnet", "opus"] },
  "sessions": { "s": { "codex": { "model": "c3", "effort": "low" }, "claude": { "model": "opus" } } },
  "workers": { "cheap": { "codex": { "model": "c1", "effort": "low" }, "claude": { "model": "sonnet" } } }
}`

func resolveServed(t *testing.T, account string, age time.Duration) (Decision, string) {
	t.Helper()
	dir := t.TempDir()
	config := filepath.Join(dir, "models.json")
	if err := os.WriteFile(config, []byte(servedPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, "served.json")
	probed := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	if err := modelserved.Replace(cache, "claude", "a@example.invalid (Org, team)", map[string]string{
		"sonnet": "claude-opus-5-5[1m]", "opus": "claude-opus-5-5",
	}, probed); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	status := Run(Command{Args: []string{"--config", config, "--client", "claude", "--task", "cheap"},
		Stdout: &out, Stderr: &errOut, ServedCache: cache,
		Account: func() string { return account }, Now: func() time.Time { return probed.Add(age) }})
	if status != 0 {
		t.Fatalf("status %d: %s", status, errOut.String())
	}
	var decision Decision
	if err := json.Unmarshal(out.Bytes(), &decision); err != nil {
		t.Fatal(err)
	}
	return decision, errOut.String()
}

// A worker row asking for a model its account does not serve dispatches the nearest tier above it
// that the account serves, and says so. A subagent would otherwise run on its parent's model.
func TestAWorkerDispatchesTheNearestServedTier(t *testing.T) {
	decision, said := resolveServed(t, "a@example.invalid (Org, team)", time.Hour)
	if decision.Requested.Model != "sonnet" || decision.Dispatched.Model != "opus" {
		t.Fatalf("requested %s, dispatched %s", decision.Requested.Model, decision.Dispatched.Model)
	}
	if !strings.Contains(said, "requested sonnet, dispatched opus (nearest served)") {
		t.Fatalf("said %q", said)
	}
}

// A set kept for another login, or past a day, leaves the row as written and says why.
func TestASetForAnotherLoginOrPastADayIsNotUsed(t *testing.T) {
	for name, c := range map[string]struct {
		account string
		age     time.Duration
		why     string
	}{
		"another login": {"b@example.invalid (Other, max)", time.Hour, "served set is for a@example.invalid"},
		"past a day":    {"a@example.invalid (Org, team)", 25 * time.Hour, "past a day"},
	} {
		decision, said := resolveServed(t, c.account, c.age)
		if decision.Dispatched.Model != "sonnet" || !strings.Contains(said, c.why) {
			t.Errorf("%s: dispatched %s, said %q", name, decision.Dispatched.Model, said)
		}
	}
}
