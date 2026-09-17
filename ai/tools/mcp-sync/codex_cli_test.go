package mcpsync

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The one case in this package that runs a real client, and the only one that could not be anything
// else: what it asks is whether CODEX ITSELF keeps an argument whole. Every other case here drives the
// same command line through a recorder, which says what this tool hands over and nothing about what
// the other side does with it.
//
// It is skipped where the CLI is absent — which is every CI runner and most machines. A skip is not a
// pass: the reason below names what went unmeasured, and `go test -v` prints it. What stays measured
// without the CLI is the command line itself, in codex_test.go.
//
// The registry is a CODEX_HOME of this test's own. The real one is the human's live configuration, and
// a case that writes it has stopped being a test.
func TestARealCodexKeepsEveryArgumentBoundary(t *testing.T) {
	if _, err := exec.LookPath(codexAgent); err != nil {
		t.Skipf("the codex CLI is not on PATH, so whether a real Codex preserves an argument boundary, an "+
			"environment value and an HTTP URL was NOT measured by this run: %v", err)
	}
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)

	dir := newCheckout(t, "ai", launcherExecutable)
	write(t, filepath.Join(dir, publicFile), `{"mcpServers":{
		"literal":{"type":"stdio","command":"/bin/echo","args":["a b","$(literal)","a\nb"],"env":{"VALUE":"x=y z"}},
		"streamed":{"type":"http","url":"http://127.0.0.1:9/mcp"}}}`, 0o644)

	var out, said bytes.Buffer
	if code := Run("mcp-sync.sh", []string{"--agent=codex"}, dir, NewCLIClient(&out, &said), &out, &said); code != exitSynced {
		t.Fatalf("syncing to a real Codex exited %d\n%s", code, said.String())
	}

	registered := codexEntry(t, "literal")
	if got := string(registered.Transport.Args); got != `["a b","$(literal)","a\nb"]` {
		t.Errorf("Codex holds the arguments as %s, and the declaration wrote [\"a b\",\"$(literal)\",\"a\\nb\"]. "+
			"A boundary lost on the way in is a server launched with arguments the human never wrote.", got)
	}
	if got := registered.Transport.Env["VALUE"]; got != "x=y z" {
		t.Errorf("Codex holds VALUE as %q, want %q", got, "x=y z")
	}
	if got := codexEntry(t, "streamed").Transport.URL; got != "http://127.0.0.1:9/mcp" {
		t.Errorf("Codex holds the URL as %q, want %q", got, "http://127.0.0.1:9/mcp")
	}
}

type codexRegistration struct {
	Transport struct {
		Args json.RawMessage   `json:"args"`
		Env  map[string]string `json:"env"`
		URL  string            `json:"url"`
	} `json:"transport"`
}

func codexEntry(t *testing.T, name string) codexRegistration {
	t.Helper()
	command := exec.Command(codexAgent, "mcp", "get", name, "--json")
	command.Env = os.Environ()
	out, err := command.Output()
	if err != nil {
		t.Fatalf("asking Codex what it registered for %s: %v", name, err)
	}
	var entry codexRegistration
	if err = json.Unmarshal(out, &entry); err != nil {
		t.Fatalf("reading what Codex answered for %s: %v\n%s", name, err, out)
	}
	return entry
}
