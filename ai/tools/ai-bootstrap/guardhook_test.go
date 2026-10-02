package aibootstrap_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// guardHooks is every PreToolUse command in Claude's settings, and the settings' other keys.
func guardHooks(t *testing.T, path string) (commands []string, keys []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatalf("%s is not JSON: %v", path, err)
	}
	for key := range settings {
		keys = append(keys, key)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks["PreToolUse"].([]any)
	for _, group := range groups {
		entries, _ := group.(map[string]any)["hooks"].([]any)
		for _, entry := range entries {
			commands = append(commands, entry.(map[string]any)["command"].(string))
		}
	}
	return commands, keys
}

// An owner install adds the dispatch guard beside the hooks and settings already there, once however
// often it runs, and an uninstall takes out the guard alone.
func TestTheOwnerInstallAddsTheDispatchGuardAndUninstallTakesItOut(t *testing.T) {
	f := newFixture(t)
	settings := f.home + "/.claude/settings.json"
	f.Write(settings, `{"model":"opus","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"rtk hook claude"}]}]}}`)
	guard := f.home + "/.kk-flavor/scripts/agent-guard.sh"
	for range 2 {
		f.ExpectCode(f.install("--agent=claude", "--owner"), 0)
	}
	commands, keys := guardHooks(t, settings)
	if strings.Join(commands, ",") != "rtk hook claude,"+guard || !strings.Contains(strings.Join(keys, ","), "model") {
		t.Fatalf("settings after two installs: hooks %v, keys %v", commands, keys)
	}
	f.ExpectCode(f.install("--agent=claude", "--owner", "--uninstall"), 0)
	if commands, _ := guardHooks(t, settings); strings.Join(commands, ",") != "rtk hook claude" {
		t.Fatalf("the uninstall left hooks %v", commands)
	}
}

// Settings the run cannot read are refused and left as they were. A Codex run and a non-owner run add
// no hook.
func TestTheDispatchGuardLeavesUnreadableSettingsAndOtherInstallsAlone(t *testing.T) {
	f := newFixture(t)
	settings := f.home + "/.claude/settings.json"
	f.Write(settings, "not json\n")
	f.ExpectCode(f.install("--agent=claude", "--owner"), 1)
	if body, _ := os.ReadFile(settings); string(body) != "not json\n" {
		t.Fatalf("unreadable settings were rewritten: %q", body)
	}
	for _, args := range [][]string{{"--agent=codex", "--owner"}, {"--agent=claude"}} {
		g := newFixture(t)
		g.install(args...)
		if _, err := os.Stat(g.home + "/.claude/settings.json"); err == nil {
			t.Errorf("%v wrote Claude's settings", args)
		}
	}
}
