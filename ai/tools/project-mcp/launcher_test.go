package projectmcp

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/mcp"
)

// The single case in this package that starts a process. What a project entry holds is a program for
// `sh`, and asserting the text of a program is agreeing with the code that wrote it. Whether it
// survives a home directory full of characters `sh` treats as syntax is a question only `sh` answers.

// The home here is deliberately hostile: a space, a `$value`, a quote and a non-ASCII character. A
// home directory is a name from outside this system, and the launcher reaches the wrapper through
// `$HOME/.kk-flavor` on every machine that clones the project.
func TestTheLauncherReachesTheWrapperThroughAHomeFullOfShellSyntax(t *testing.T) {
	t.Parallel()
	p := newProject(t, claudeAgent)
	home := filepath.Join(p.root, `home $value "quote é`)
	checkout := filepath.Join(home, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, "kk-flavor"), 0o755); err != nil {
		t.Fatalf("building the home fixture: %v", err)
	}
	// `.kk-flavor` is a symlink into whichever checkout installed it. The launcher resolves it
	// physically before walking up to the wrapper, because the kernel reads `..` from the link's target.
	if err := os.Symlink(filepath.Join(checkout, "kk-flavor"), filepath.Join(home, ".kk-flavor")); err != nil {
		t.Fatalf("linking .kk-flavor: %v", err)
	}
	// A wrapper that prints its arguments instead of launching a server, so what arrives is readable.
	if err := os.WriteFile(filepath.Join(checkout, "mcp-env.sh"),
		[]byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatalf("writing the wrapper fixture: %v", err)
	}

	if outcome := p.run(); outcome.code != exitDone {
		t.Fatalf("the install exited %d\n%s", outcome.code, outcome.stderr)
	}
	entry := installedEntry(t, p, fixtureServers[0])

	command := exec.Command(entry.Command, entry.Args...)
	command.Env = append(os.Environ(), "HOME="+home)
	out, err := command.Output()
	if err != nil {
		t.Fatalf("running what the project file registered: %v\nThe entry is the whole deliverable, and a "+
			"home directory holding shell syntax is what breaks one.", err)
	}

	want := strings.Join(declaredArgs(t, p, fixtureServers[0]), "\n") + "\n"
	if string(out) != want {
		t.Errorf("the wrapper was reached with\n  %q\nand the declaration says\n  %q\nAn argument lost or "+
			"split here is a server launched with something the human never wrote.", out, want)
	}
}

type installedServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func installedEntry(t *testing.T, p *project, name string) installedServer {
	t.Helper()
	var config struct {
		Servers map[string]installedServer `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(p.read()), &config); err != nil {
		t.Fatalf("reading the project config: %v", err)
	}
	entry, held := config.Servers[name]
	if !held {
		t.Fatalf("%s is not in the project config, so this case would run nothing", name)
	}
	return entry
}

// An earlier version took the arguments from readPublicServers, and the expectation moved with the
// mapping. The `$0` word was dropped, `sh` swallowed the first argument, and the case agreed with the
// mapping about which arguments were left.

// declaredArgs reads the server's own arguments out of the declaration file, so the expectation
// follows the declaration instead of pinning a copy of it. It NEVER reads readPublicServers, whose
// answer is what this case is measuring.
func declaredArgs(t *testing.T, p *project, name string) []string {
	t.Helper()
	document, err := mcp.ReadDocument(filepath.Join(p.configs, "mcp.jsonc"), mcp.ConfigsToken)
	if err != nil {
		t.Fatalf("reading the declaration fixture: %v", err)
	}
	for _, server := range document.Servers {
		if server.Name != name {
			continue
		}
		var declared struct {
			Args []string `json:"args"`
		}
		if err = json.Unmarshal(server.Config, &declared); err != nil {
			t.Fatalf("reading %s's arguments: %v", name, err)
		}
		return declared.Args
	}
	t.Fatalf("%s is not in the declaration fixture", name)
	return nil
}
