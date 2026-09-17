package projectmcp

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/mcp"
)

// The one case in this package that starts a process, and the only one that could not be anything
// else: what a project entry holds is a program for `sh`, and asserting the text of a program is
// agreeing with the code that wrote it. Whether it survives a home directory full of characters `sh`
// treats as syntax is a question only `sh` answers.
//
// The home here is deliberately hostile — a space, a `$value`, a quote and a non-ASCII character —
// because a home directory is a name from outside this system, and the launcher reaches the wrapper
// through `$HOME/.kk-flavor` on every machine that clones the project.
func TestTheLauncherReachesTheWrapperThroughAHomeFullOfShellSyntax(t *testing.T) {
	t.Parallel()
	p := newProject(t, claudeAgent)
	home := filepath.Join(p.root, `home $value "quote é`)
	checkout := filepath.Join(home, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, "kk-flavor"), 0o755); err != nil {
		t.Fatalf("building the home fixture: %v", err)
	}
	// `.kk-flavor` is a symlink into whichever checkout installed it, which is why the launcher resolves
	// it physically before walking up to the wrapper: `..` is read from the link's target, not its name.
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
	entry := installedEntry(t, p, shippedServers[0])

	command := exec.Command(entry.Command, entry.Args...)
	command.Env = append(os.Environ(), "HOME="+home)
	out, err := command.Output()
	if err != nil {
		t.Fatalf("running what the project file registered: %v\nThe entry is the whole deliverable, and a "+
			"home directory holding shell syntax is what breaks one.", err)
	}

	want := strings.Join(declaredArgs(t, shippedServers[0]), "\n") + "\n"
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

// What the shipped declaration says the server's own arguments are — read out of the file rather than
// written out here, so the expectation follows the declaration instead of pinning a copy of it.
//
// Read from the declaration and NEVER from readPublicServers, whose answer is what this case is
// measuring. Taken from there, the expectation moved with the mapping: dropping the `$0` word made
// `sh` swallow the first argument, and the case agreed with the mapping about which arguments were
// left.
func declaredArgs(t *testing.T, name string) []string {
	t.Helper()
	document, err := mcp.ReadDocument(filepath.Join(shippedConfigsDir, "mcp.jsonc"), mcp.ConfigsToken)
	if err != nil {
		t.Fatalf("reading this repository's own declaration: %v", err)
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
	t.Fatalf("%s is not in this repository's own declaration", name)
	return nil
}
