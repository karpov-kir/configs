package modelpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandEmitsRequestedOriginAndPinsPolicy(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "models.json")
	origin := filepath.Join(dir, "origin.json")
	if err := os.WriteFile(config, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(origin, []byte(`{"client":"codex","model":"original","effort":"high"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errors strings.Builder
	args := []string{"--config", config, "--client", "codex", "--role", "implement", "--origin", origin, "--transport", "cli"}
	if code := Run(Command{Args: args, Stdout: &out, Stderr: &errors}); code != 0 {
		t.Fatalf("command=%d: %s", code, errors.String())
	}
	var decision Decision
	if err := json.Unmarshal([]byte(out.String()), &decision); err != nil {
		t.Fatal(err)
	}
	if decision.Requested.Model != "original" || decision.Requested.Effort != "high" || strings.Contains(out.String(), "observed") {
		t.Fatalf("wrong requested evidence: %s", out.String())
	}
	for _, suffix := range [][]string{{"--policy-digest", decision.PolicyDigest}, {"--policy-digest", "stale"}} {
		out.Reset()
		errors.Reset()
		code := Run(Command{Args: append(append([]string{}, args...), suffix...), Stdout: &out, Stderr: &errors})
		if suffix[1] == "stale" {
			if code != 2 || out.Len() != 0 || !strings.Contains(errors.String(), "digest changed") {
				t.Fatalf("stale policy accepted: %d %s %s", code, out.String(), errors.String())
			}
		} else if code != 0 {
			t.Fatalf("pinned policy rejected: %s", errors.String())
		}
	}
}

func TestCommandCannotUseGlobalDefaultsForMissingOrigin(t *testing.T) {
	var out, errors strings.Builder
	code := Run(Command{Args: []string{"--config", "../../kk-flavor/models.json", "--client", "codex", "--role", "implement", "--transport", "cli"}, Stdout: &out, Stderr: &errors})
	if code != 2 || out.Len() != 0 || !strings.Contains(errors.String(), "original task") {
		t.Fatalf("missing origin result=%d %s %s", code, out.String(), errors.String())
	}
}

func TestEveryProductionRoleKeepsItsAssignment(t *testing.T) {
	policy, err := Load("../../kk-flavor/models.json")
	if err != nil {
		t.Fatal(err)
	}
	for name := range policy.content.Roles {
		for _, client := range []string{"codex", "claude"} {
			decision, err := policy.Resolve(Request{Client: client, Role: name, Transport: "cli", Origin: &Origin{Client: client, Model: "original", Effort: "high"}})
			if err != nil {
				t.Fatalf("%s/%s: %v", client, name, err)
			}
			model, effort := "original", "high"
			if name == "judge" {
				model, effort = "haiku", ""
				if client == "codex" {
					model, effort = "gpt-5.4-mini", "low"
				}
			}
			if decision.Requested.Model != model || decision.Requested.Effort != effort {
				t.Fatalf("production role %s/%s changed: %+v", client, name, decision)
			}
		}
	}
}

func TestInstalledPolicyFollowsTheInvokedMount(t *testing.T) {
	dir := t.TempDir()
	scripts := filepath.Join(dir, "owned", "kk-flavor", "scripts")
	if err := os.MkdirAll(scripts, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scripts, "model-policy.sh")
	if err := os.WriteFile(script, []byte("script"), 0700); err != nil {
		t.Fatal(err)
	}
	mount := filepath.Join(dir, "mount")
	if err := os.Symlink(script, mount); err != nil {
		t.Fatal(err)
	}
	got, err := InstalledPath(mount)
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(canonical, "owned", "kk-flavor", "models.json")
	if err != nil || got != want {
		t.Fatalf("mounted policy=%q, %v; want %q", got, err, want)
	}
	if _, err := InstalledPath(filepath.Join(dir, "unrelated")); err == nil {
		t.Fatal("guessed a policy from an unrelated location")
	}
}

func TestOriginRejectsCaseFoldedAndDuplicateIdentity(t *testing.T) {
	for _, raw := range []string{
		`{"client":"codex","Client":"claude","model":"original","effort":"high"}`,
		`{"client":"codex","model":"original","model":"other","effort":"high"}`,
		`{"client":"codex","model":"original","effort":"turbo"}`,
		`{"client":"codex","model":"original","effort":"high","observed":true}`,
	} {
		if _, err := ParseOrigin([]byte(raw)); err == nil {
			t.Fatalf("invalid origin accepted: %s", raw)
		}
	}
}
