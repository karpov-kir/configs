package projectmcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"

	"configs/ai/tools/mcp"
)

// `$HOME/.kk-flavor` is a path every install of this flavor has, so the entry reads the same on every
// machine that clones the project. That is what makes it committable.

// `cd -P` and `pwd -P` resolve the link, because `.kk-flavor` is a symlink into whichever checkout
// installed it. The kernel reads `..` from the link's target, so the link's own path reaches the
// wrong directory.

// The program a project entry runs instead of naming this checkout.
const launcher = `bucket=$(CDPATH= cd -P "$HOME/.kk-flavor" && pwd -P) || exit; exec "$bucket/../mcp-env.sh" "$@"`

// `sh -c <program> <name> <argument>…` assigns the word after the program to `$0` and the rest to
// `$@`. A missing name shifts every argument down one, and the server loses its last.

// The name `sh` gives the launcher as `$0`. It is NOT one of the server's arguments.
const launcherName = "kk-flavor-mcp"

// A server name that can be a TOML table key and a JSON key without quoting or escaping either.
var portableServerName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// projectServer is one public server as a project file states it: always `sh` running the launcher,
// with the declaration's own arguments after it.
type projectServer struct {
	Name string
	Args []string
}

// Only the committed `mcp.jsonc` is read here. A project file is shared with everyone who clones the
// project, and the private half exists for servers a committed file must leave out.

// Every server has to map exactly, or the run stops. A declaration that grew a field, an `env`, a
// second transport or a command other than the launcher, has no portable project form this tool can
// infer. A guess writes a project file naming something other than what the human declared.

// readPublicServers maps the public declaration to what a project file may hold.
func readPublicServers(configsDir string) ([]projectServer, error) {
	file := filepath.Join(configsDir, "mcp.jsonc")
	// The document is read with the token left in place, because a project entry carries no checkout
	// path. The token stays as written, and portableArgs holds the command against it literally.
	document, err := mcp.ReadDocument(file, mcp.ConfigsToken)
	if err != nil {
		return nil, err
	}
	servers := make([]projectServer, 0, len(document.Servers))
	for _, server := range document.Servers {
		args, err := portableArgs(server)
		if err != nil {
			return nil, fmt.Errorf("public server %s needs an explicit portable project mapping: %w",
				server.Name, err)
		}
		servers = append(servers, projectServer{
			Name: server.Name,
			Args: append([]string{"-c", launcher, launcherName}, args...),
		})
	}
	return servers, nil
}

func portableArgs(server mcp.Server) ([]string, error) {
	if !portableServerName.MatchString(server.Name) {
		return nil, fmt.Errorf("the name is not portable")
	}
	var declared struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(server.Config, &declared); err != nil {
		return nil, err
	}
	if declared.Type != mcp.Stdio || declared.Command != mcp.ConfigsToken+"/mcp-env.sh" || declared.Args == nil {
		return nil, fmt.Errorf("the transport has no portable form")
	}
	fields, err := mcp.ParseObject(server.Config)
	if err != nil {
		return nil, err
	}
	for _, key := range fields.Keys() {
		if key != "type" && key != "command" && key != "args" {
			return nil, fmt.Errorf("%q has no portable form", key)
		}
	}
	for _, arg := range declared.Args {
		if containsNUL(arg) {
			return nil, fmt.Errorf("an argument carries a NUL")
		}
	}
	return declared.Args, nil
}

func containsNUL(text string) bool {
	for _, char := range text {
		if char == 0 {
			return true
		}
	}
	return false
}

// claudeEntry builds what a Claude project entry holds for one server, the same three fields for
// every server. They go in the order a reader expects to meet them, and a map answers in another.
func claudeEntry(server projectServer) (json.RawMessage, error) {
	entry := mcp.NewObject()
	for _, field := range []struct {
		key   string
		value any
	}{{"type", mcp.Stdio}, {"command", "sh"}, {"args", server.Args}} {
		if err := entry.SetValue(field.key, field.value); err != nil {
			return nil, err
		}
	}
	return entry.MarshalJSON()
}
