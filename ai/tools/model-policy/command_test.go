package modelpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandEmitsRequestedSettingsAndNothingObserved(t *testing.T) {
	var out, errors strings.Builder
	args := []string{"--config", fixtureConfig(t), "--client", "claude", "--task", "reader-judge"}
	if code := Run(Command{Args: args, Stdout: &out, Stderr: &errors}); code != 0 {
		t.Fatalf("command=%d: %s", code, errors.String())
	}
	var decision Decision
	if err := json.Unmarshal([]byte(out.String()), &decision); err != nil {
		t.Fatal(err)
	}
	if decision.Requested.Model != "haiku" || decision.Rolls != 3 || strings.Contains(out.String(), "observed") {
		t.Fatalf("wrong requested evidence: %s", out.String())
	}
}

func TestCommandRefusesAnUnassignedTask(t *testing.T) {
	var out, errors strings.Builder
	args := []string{"--config", fixtureConfig(t), "--client", "claude", "--task", "kk-invented"}
	if code := Run(Command{Args: args, Stdout: &out, Stderr: &errors}); code != 2 || out.Len() != 0 {
		t.Fatalf("unassigned task result=%d %s", code, out.String())
	}
	if !strings.Contains(errors.String(), "assigns no model") {
		t.Fatalf("unhelpful refusal: %s", errors.String())
	}
}

// A row carrying an effort with no model is refused at parse, for either client. On claude no field
// carries the effort. On codex a field does, and the spawn then runs at whatever model the caller
// had. That silent inheritance is what the policy exists to remove, and it leaves an orchestrator
// outside the tier order and outside the ceiling.
func TestARowNamingAnEffortAndNoModelIsRefused(t *testing.T) {
	// One row per client covers the map: validateAssignments, the assignment validator, walks the
	// sessions and the workers with one validator, and TestPolicyRejectsMalformedDocuments breaks a
	// session row against it.
	for _, swap := range []struct{ what, from, to string }{
		{"a codex worker row", `"build/explore":{"codex":{"model":"middling","effort":"low"}`, `"build/explore":{"codex":{"effort":"low"}`},
		{"a claude worker row", `"build/explore":{"codex":{"model":"middling","effort":"low"},"claude":{"model":"sonnet"}`, `"build/explore":{"codex":{"model":"middling","effort":"low"},"claude":{"effort":"low"}`},
	} {
		raw := strings.Replace(sample, swap.from, swap.to, 1)
		if raw == sample {
			t.Fatalf("%s: the fixture edit matched nothing, so this case tests the unmodified sample", swap.what)
		}
		policy, err := Parse([]byte(raw))
		if err == nil {
			t.Errorf("%s naming no model parsed into %v", swap.what, policy.TaskNames())
			continue
		}
		// Both rows assert on the refusal sentence. validName, the name check, refuses an empty model
		// too, and it answers for this guard whenever the guard is disabled.
		if !strings.Contains(err.Error(), "names no model") {
			t.Errorf("%s was refused for the wrong reason: %v", swap.what, err)
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

// The command takes a path, so a case about it has to write a file. The shipped file is read in
// `ai/tools`. It sits outside the module, and Go's test cache cannot see it change.

// fixtureConfig writes the sample document to a temp file and returns its path.
func fixtureConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(path, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
