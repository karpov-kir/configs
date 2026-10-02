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

[ plugins.'docs@openai' ] # quoted another way
enabled = true

[features] # the owner's
js_repl = true

[mcp_servers.chrome-devtools]
command = "npx"

[mcp_servers.chrome-devtools.env]
KEY = "v"

[mcp_servers]
inline-one = { command = "c" }
`

// An owner Codex install declares the light worker and its layer, which switches off each plugin and MCP
// server the config declares. It turns hooks on in the owner's [features] table and puts the guard in
// hooks.json. A second run leaves both files as they stand, and an uninstall restores the owner's text.
func TestCodexGetsTheLightWorkerAndTheGuardAndUninstallRestoresTheConfig(t *testing.T) {
	f := newFixture(t)
	config := f.codexHome + "/config.toml"
	f.Write(config, ownersCodexConfig)
	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
	body, _ := os.ReadFile(config)
	for _, want := range []string{"[features] # the owner's\nhooks = true # kk-flavor: the dispatch guard\njs_repl = true",
		"[agents.light-worker]", `config_file = "kk-flavor-light-worker.toml"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the config lacks %q:\n%s", want, body)
		}
	}
	hooks, _ := os.ReadFile(f.codexHome + "/hooks.json")
	if !strings.Contains(string(hooks), `"matcher": "Agent"`) || !strings.Contains(string(hooks), f.home+"/.kk-flavor/scripts/agent-guard-hook.sh") {
		t.Errorf("hooks.json lacks the guard:\n%s", hooks)
	}
	layer, _ := os.ReadFile(f.codexHome + "/kk-flavor-light-worker.toml")
	for _, want := range []string{"[plugins.\"browser@openai-bundled\"]\nenabled = false", "[plugins.'docs@openai']\nenabled = false",
		"[mcp_servers.chrome-devtools]\nenabled = false", "[mcp_servers.inline-one]\nenabled = false"} {
		if !strings.Contains(string(layer), want) {
			t.Errorf("the layer lacks %q:\n%s", want, layer)
		}
	}
	if strings.Contains(string(layer), "chrome-devtools.env") {
		t.Errorf("the layer switched off a server's subtable:\n%s", layer)
	}
	if backups, _ := filepath.Glob(config + ".kk-flavor-backup-*"); len(backups) != 1 {
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

// A config however it ends comes back with the same bytes, and one bootstrap created goes again.
func TestCodexConfigComesBackAsItEnded(t *testing.T) {
	for _, owners := range []string{`model = "gpt"`, "model = \"gpt\"\n\n\n", "[features]\nhooks = true\n", "model = \"gpt\"\n[features]\n",
		"model = \"gpt\"\n[features]", ""} {
		f := newFixture(t)
		config := f.codexHome + "/config.toml"
		if owners != "" {
			f.Write(config, owners)
		}
		f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
		first, _ := os.ReadFile(config)
		f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
		if again, _ := os.ReadFile(config); string(again) != string(first) {
			t.Errorf("%q: a second run rewrote the config", owners)
		}
		f.ExpectCode(f.install("--agent=codex", "--owner", "--uninstall"), 0)
		restored, err := os.ReadFile(config)
		if owners == "" {
			if err == nil {
				t.Errorf("a config bootstrap created was left: %q", restored)
			}
			continue
		}
		if string(restored) != owners {
			t.Errorf("%q came back as %q", owners, restored)
		}
	}
}

// A config of a shape the run does not edit is refused and left as it was. The shapes are hooks the
// owner switched off, features set outside a table, two [features] headers, a light worker the owner
// declared, a multi-line string and Windows line endings.
func TestCodexConfigsOfAnotherShapeAreRefusedAndLeftAlone(t *testing.T) {
	for _, owners := range []string{
		"[features]\nhooks = false\n",
		"\"features\".hooks = false\n",
		"[ \"features\" ]\nhooks = false\n",
		"\"agents\" = { max_threads = 3 }\n",
		"[features]\nx = [\n  [1],\n]\nhooks = false\n",
		"features.hooks = false\n",
		"features = { js_repl = true }\n",
		"[features]\n[features]\n",
		"[agents.light-worker]\ndescription = \"mine\"\n",
		"agents = { max_threads = 3 }\n",
		"developer_instructions = \"\"\"\n[features]\n\"\"\"\n",
		"model = \"gpt\"\r\n",
	} {
		f := newFixture(t)
		config := f.codexHome + "/config.toml"
		f.Write(config, owners)
		f.ExpectCode(f.install("--agent=codex", "--owner"), 1)
		if body, _ := os.ReadFile(config); string(body) != owners {
			t.Errorf("%q was changed to %q", owners, body)
		}
	}
	// A key added above the region of a file bootstrap created is the owner's, and it survives both a
	// second run and the uninstall.
	g := newFixture(t)
	created := g.codexHome + "/config.toml"
	g.ExpectCode(g.install("--agent=codex", "--owner"), 0)
	g.Write(created, "model = \"gpt\"\n"+g.Read(created))
	g.ExpectCode(g.install("--agent=codex", "--owner"), 0)
	g.ExpectCode(g.install("--agent=codex", "--owner", "--uninstall"), 0)
	if body, _ := os.ReadFile(created); string(body) != "model = \"gpt\"\n" {
		t.Errorf("the owner's key above a created region came back as %q", body)
	}
	// A line Codex appended inside the region is the owner's, and the uninstall leaves the region.
	f := newFixture(t)
	config := f.codexHome + "/config.toml"
	f.Write(config, "model = \"gpt\"\n")
	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)
	f.appendTo(config, "[projects.\"/p\"]\ntrust_level = \"trusted\"\n")
	f.ExpectCode(f.install("--agent=codex", "--owner", "--uninstall"), 1)
	if body, _ := os.ReadFile(config); !strings.Contains(string(body), "[projects.\"/p\"]") {
		t.Errorf("the uninstall dropped a table Codex appended:\n%s", body)
	}
}

// A Claude run and a non-owner Codex run leave Codex's config alone.
func TestOnlyAnOwnerCodexRunTouchesCodexConfig(t *testing.T) {
	for _, args := range [][]string{{"--agent=claude", "--owner"}, {"--agent=codex"}} {
		g := newFixture(t)
		g.Write(g.codexHome+"/config.toml", ownersCodexConfig)
		g.install(args...)
		if body, _ := os.ReadFile(g.codexHome + "/config.toml"); string(body) != ownersCodexConfig {
			t.Errorf("%v changed Codex's config:\n%s", args, body)
		}
	}
}
