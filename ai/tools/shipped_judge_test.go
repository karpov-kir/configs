// The judge configured from the shipped policy. `bloat-judge` is the one task a tool resolves on its
// own, so what the central file assigns it is the whole of its cost control, and these are the cases
// that read that file rather than a fixture.
//
// Here rather than in `ai/tools/bloat-judge/` for the reason shipped_tree_test.go's cases are.
// Everything the judge does with a decision once it has one is that package's own suite's, against a
// fixture.
package tools_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	bloatjudge "configs/ai/tools/bloat-judge"
)

func TestConfiguredJudgeUsesTheCentralPolicy(t *testing.T) {
	fakeCodex(t)
	t.Setenv("JUDGE_PROVIDER", "codex")
	t.Setenv("JUDGE_MODEL", "")
	os.Unsetenv("JUDGE_MODEL")
	configured, err := bloatjudge.Configure(bloatjudge.Configuration{Deadline: time.Second, PolicyPath: shippedPolicyPath})
	if err != nil {
		t.Fatal(err)
	}
	if configured.Decision.Requested.Model != "gpt-5.6-luna" || configured.Decision.Requested.Effort != "low" {
		t.Fatalf("central judge assignment = %+v", configured.Decision)
	}
	if configured.Decision.Rolls != 3 {
		t.Fatalf("central judge roll count = %d", configured.Decision.Rolls)
	}
}

func TestJudgeCacheSeparatesClientSelections(t *testing.T) {
	fakeCodex(t)
	fakeClient(t, "claude")
	identities := map[string]bool{}
	for _, client := range []string{"codex", "claude"} {
		t.Setenv("JUDGE_PROVIDER", client)
		configured, err := bloatjudge.Configure(bloatjudge.Configuration{Deadline: time.Second, PolicyPath: shippedPolicyPath})
		if err != nil {
			t.Fatal(err)
		}
		if configured.CacheIdentity == "" || identities[configured.CacheIdentity] {
			t.Fatalf("judge cache did not distinguish %s selection: %q", client, configured.CacheIdentity)
		}
		identities[configured.CacheIdentity] = true
	}
}

// A CLI that exits without answering. Configure only looks for one on PATH, and neither case here
// takes a roll — a real client would price a model against somebody's account to prove which name the
// policy handed over.
func fakeCodex(t *testing.T) { fakeClient(t, "codex") }

func fakeClient(t *testing.T, name string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
