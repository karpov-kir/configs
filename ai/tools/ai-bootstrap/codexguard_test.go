package aibootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ownersCodexConfig = `model = "gpt"

[plugins."browser@openai-bundled"]
enabled = true

[features]
js_repl = true

[mcp_servers.chrome-devtools]
command = "npx"

[mcp_servers.chrome-devtools.env]
KEY = "v"
`

// An owner Codex install declares the light worker and its layer, which switches off each plugin and MCP
// server the config declares. It turns hooks on in the owner's [features] table and adds the guard. A
// second run leaves the file as it stands, and an uninstall restores the owner's text.
func TestCodexGetsTheLightWorkerAndTheGuardAndUninstallRestoresTheConfig(t *testing.T) {
	f := newFixture(t)
	config := f.codexHome + "/config.toml"
	f.Write(config, ownersCodexConfig)
	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
	body, _ := os.ReadFile(config)
	for _, want := range []string{"[features]\nhooks = true # kk-flavor: the dispatch guard\njs_repl = true",
		"[agents.light-worker]", `config_file = "kk-flavor-light-worker.toml"`, `matcher = "spawn_agent"`,
		f.home + "/.kk-flavor/scripts/agent-guard-hook.sh"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the config lacks %q:\n%s", want, body)
		}
	}
	layer, _ := os.ReadFile(f.codexHome + "/kk-flavor-light-worker.toml")
	for _, want := range []string{"[plugins.\"browser@openai-bundled\"]\nenabled = false", "[mcp_servers.chrome-devtools]\nenabled = false"} {
		if !strings.Contains(string(layer), want) {
			t.Errorf("the layer lacks %q:\n%s", want, layer)
		}
	}
	if strings.Contains(string(layer), "chrome-devtools.env") {
		t.Errorf("the layer switched off a server's subtable:\n%s", layer)
	}
	backups, _ := filepath.Glob(config + ".kk-flavor-backup-*")
	if len(backups) != 1 {
		t.Errorf("%d backups, want the one written before the first change", len(backups))
	}
	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
	if again, _ := os.ReadFile(config); string(again) != string(body) {
		t.Errorf("a second run changed the config:\n%s", again)
	}
	f.ExpectCode(f.install("--agent=codex", "--owner", "--uninstall"), 0)
	if restored, _ := os.ReadFile(config); string(restored) != ownersCodexConfig {
		t.Errorf("the uninstall left:\n%s", restored)
	}
}

// Hooks the owner switched off stay off, and the guard is refused. A Claude run and a non-owner Codex
// run leave the config alone.
func TestCodexHooksTheOwnerSwitchedOffStayOff(t *testing.T) {
	f := newFixture(t)
	config := f.codexHome + "/config.toml"
	off := "[features]\nhooks = false\n"
	f.Write(config, off)
	f.ExpectCode(f.install("--agent=codex", "--owner"), 1)
	if body, _ := os.ReadFile(config); string(body) != off {
		t.Errorf("hooks switched off were changed:\n%s", body)
	}
	for _, args := range [][]string{{"--agent=claude", "--owner"}, {"--agent=codex"}} {
		g := newFixture(t)
		g.Write(g.codexHome+"/config.toml", ownersCodexConfig)
		g.install(args...)
		if body, _ := os.ReadFile(g.codexHome + "/config.toml"); string(body) != ownersCodexConfig {
			t.Errorf("%v changed Codex's config:\n%s", args, body)
		}
	}
}
