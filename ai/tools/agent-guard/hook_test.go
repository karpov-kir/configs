package agentguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The hook passes the guard's own refusal and runs the dispatch on every other exit. The resolver exits
// 2 when it cannot build the guard, and a hook passing that through would refuse every dispatch.
func TestTheHookPassesOnlyTheGuardsOwnRefusal(t *testing.T) {
	hook, err := os.ReadFile("../../kk-flavor/scripts/agent-guard-hook.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, guard string
		want        int
	}{
		{"the guard refuses", "echo '" + RefusalPrefix + " no need named' >&2; exit 2", 2},
		{"the guard lets it run", "exit 0", 0},
		{"the resolver cannot build it", "echo 'agent-guard.sh: agent-guard did not build, so it did NOT run' >&2; exit 2", 0},
		{"the guard is gone", "exit 127", 0},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "agent-guard-hook.sh"), hook, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "agent-guard.sh"), []byte("#!/usr/bin/env bash\ncat >/dev/null\n"+tc.guard+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(filepath.Join(dir, "agent-guard-hook.sh"))
		cmd.Stdin = strings.NewReader(`{"tool_name":"Agent"}`)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()
		got := 0
		if exit, ok := err.(*exec.ExitError); ok {
			got = exit.ExitCode()
		}
		if got != tc.want {
			t.Errorf("%s: exit %d, want %d (%s)", tc.name, got, tc.want, stderr.String())
		}
	}
}
