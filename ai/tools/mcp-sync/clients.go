package mcpsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"configs/ai/tools/mcp"
)

// The live registry these write is the human's, so almost every case passes a fake in this seam's
// place. The exception is codex_cli_test.go. It drives this for real against a CODEX_HOME of its own,
// and it skips where the CLI is absent.

// NewCLIClient is the production half of the Client seam: each agent's own command-line interface,
// found on PATH and driven as a process.
func NewCLIClient(stdout, stderr io.Writer) NewClient {
	return func(agent string) (Client, error) {
		if _, err := exec.LookPath(agent); err != nil {
			return nil, fmt.Errorf("%s CLI not found on PATH", agent)
		}
		run := func(args []string) error {
			command := exec.Command(agent, args...)
			command.Stdout, command.Stderr = stdout, stderr
			return command.Run()
		}
		if agent == codexAgent {
			return codexClient{run: run}, nil
		}
		return claudeClient{run: run}, nil
	}
}

const (
	claudeAgent = "claude"
	codexAgent  = "codex"
)

type claudeClient struct {
	run func(args []string) error
}

// Claude stores a server's configuration as the JSON it was declared with, so it preserves every
// field and this arm accepts every document.
func (c claudeClient) AcceptsDocument(document *mcp.Document) error {
	return nil
}

// The removal is allowed to fail, because the usual reason is that the server was never registered.
// Between the two calls the server is gone, which is what Register's own failure message is about. A
// failed add leaves an entry the human still has in their file and no longer has in their client.

// Register removes the entry and then adds it. `claude mcp add-json` refuses a name that is already
// registered, and this tool exists to update an entry as well as create one.
func (c claudeClient) Register(server mcp.Server) error {
	name := server.Name
	config, err := compact(server.Config)
	if err != nil {
		return fmt.Errorf("'%s' is not JSON Claude can be handed. No later entries were synced", name)
	}
	_ = c.run([]string{"mcp", "remove", "-s", "user", "--", name})
	if err = c.run([]string{"mcp", "add-json", "-s", "user", "--", name, config}); err != nil {
		return fmt.Errorf("re-adding '%s' failed — it was removed first, so it is now UNREGISTERED.\n"+
			"       Fix its entry and re-run this script. No later entries were synced", name)
	}
	return nil
}

// One argument, so the entry goes across whole. The declaration's own spacing and line breaks are no
// part of what it means to a client. A multi-line argument is one more thing between the file and the
// registry.
func compact(config json.RawMessage) (string, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, config); err != nil {
		return "", err
	}
	return out.String(), nil
}
