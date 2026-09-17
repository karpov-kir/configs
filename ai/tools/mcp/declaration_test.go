package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestServersComeBackInTheOrderTheFileWroteThem(t *testing.T) {
	t.Parallel()
	document, err := ParseDocument("fixture.jsonc",
		`{"mcpServers":{"zebra":{"command":"z"},"alpha":{"command":"a"},"middle":{"command":"m"}}}`)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	var names []string
	for _, server := range document.Servers {
		names = append(names, server.Name)
	}
	if got := strings.Join(names, ","); got != "zebra,alpha,middle" {
		t.Errorf("servers came back as %q, want \"zebra,alpha,middle\". A project's Codex region is compared "+
			"byte for byte with what a reinstall would write, so a reordering turns an untouched region into "+
			"one this family refuses as edited.", got)
	}
}

func TestADeclarationWithoutAServersObjectIsRefusedByName(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		text string
	}{
		{name: "not JSON at all", text: "INVALID PRIVATE SECRET"},
		{name: "a JSON array", text: `["mcpServers"]`},
		{name: "an object with no mcpServers", text: `{"other": {}}`},
		{name: "mcpServers as an array", text: `{"mcpServers": []}`},
		{name: "empty", text: ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDocument("fixture.jsonc", scenario.text)
			if err == nil {
				t.Fatalf("%s was accepted as a declaration, so a malformed file would be synced as an empty "+
					"one — servers silently missing rather than a refusal", scenario.name)
			}
			if !strings.Contains(err.Error(), "fixture.jsonc") {
				t.Errorf("the refusal does not name the file at fault: %q. Two files are read in one run, "+
					"and the human has to know which to open.", err)
			}
		})
	}
}

func TestAnEntrysConfigurationSurvivesVerbatim(t *testing.T) {
	t.Parallel()
	document, err := ParseDocument("fixture.jsonc",
		`{"mcpServers":{"one":{"type":"stdio","command":"/bin/echo","args":["a b"],"unmodelled":{"x":1}}}}`)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	var back map[string]any
	if err = json.Unmarshal(document.Servers[0].Config, &back); err != nil {
		t.Fatalf("the entry did not come back as JSON: %v", err)
	}
	if _, held := back["unmodelled"]; !held {
		t.Errorf("a field this package does not model was dropped on the way through. The refusals in " +
			"mcp-sync exist to REFUSE an unrepresentable field, which needs the field to still be there.")
	}
}

func TestATransportDefaultsToStdioTheWayBothClientsDo(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		config string
		want   string
	}{
		{config: `{"command":"x"}`, want: Stdio},
		{config: `{"type":"stdio","command":"x"}`, want: Stdio},
		{config: `{"type":"http","url":"https://example.invalid"}`, want: "http"},
	} {
		t.Run(scenario.config, func(t *testing.T) {
			t.Parallel()
			server := Server{Name: "one", Config: json.RawMessage(scenario.config)}
			if got := server.Transport(); got != scenario.want {
				t.Errorf("%s: transport = %q, want %q", scenario.config, got, scenario.want)
			}
		})
	}
}
