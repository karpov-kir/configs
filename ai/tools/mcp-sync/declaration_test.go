package mcpsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kk-flavor/tools/mcp"
)

// This repository's own `ai/` directory, two levels above this package. The cases below are about the
// files that actually ship, so they read them rather than a fixture — a fixture would agree with
// itself while the committed declaration stopped parsing.
//
// `ai/mcp.jsonc` and `ai/mcp-env.sh` are outside this Go module, so the gate keys its `gotest` unit on
// both by name (`ai/tools/gate/units.go`). Without that, editing either one leaves this suite
// answering `(cached)` over a file it never read.
const shippedConfigsDir = "../.."

func shippedDeclaration(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(shippedConfigsDir, publicFile))
	if err != nil {
		t.Fatalf("reading this repository's own %s: %v", publicFile, err)
	}
	return string(text)
}

func TestTheShippedDeclarationParsesOnlyAfterItsCommentsAreStripped(t *testing.T) {
	t.Parallel()
	text := shippedDeclaration(t)
	if _, err := mcp.ParseDocument(publicFile, mcp.StripComments(text)); err != nil {
		t.Errorf("this repository's own %s does not parse after stripping: %v\nNothing would sync.",
			publicFile, err)
	}
	// The control that makes the line above a measurement rather than a tautology: the file really does
	// carry comments, so the stripping is doing something.
	var value any
	if err := json.Unmarshal([]byte(text), &value); err == nil {
		t.Errorf("this repository's own %s parses as plain JSON, so the case above no longer says anything "+
			"about the comment stripping — the fixture it measures has lost its comments", publicFile)
	}
}

// End to end over the file that ships: what this tool hands a client has to name a command that is
// really there, or every stdio server registers and none of them starts.
func TestEveryStdioServerTheShippedDeclarationRegistersNamesARunnableCommand(t *testing.T) {
	t.Parallel()
	configsDir, err := filepath.Abs(shippedConfigsDir)
	if err != nil {
		t.Fatalf("resolving %s: %v", shippedConfigsDir, err)
	}
	stripped := mcp.StripComments(shippedDeclaration(t))
	substituted := stdioCommands(t, mcp.SubstituteConfigsDir(stripped, configsDir))
	// The same commands with the token left in, which is the control: without the substitution none of
	// them resolves, so the loop below would otherwise pass whether it happened or not.
	declared := stdioCommands(t, stripped)

	if len(substituted) == 0 {
		t.Fatal("this repository's own declaration names no stdio server, so the loop below asserts " +
			"nothing at all")
	}
	for index, command := range substituted {
		if !isExecutable(command) {
			t.Errorf("a server would be registered to run %q, which is not an executable on this machine. "+
				"It would register cleanly and fail at launch, with nothing saying so until the next session.",
				command)
		}
		if isExecutable(declared[index]) {
			t.Errorf("%q resolves with the token still in it, so the line above would pass whether the "+
				"substitution happened or not", declared[index])
		}
	}
}

// Every stdio server's command, in document order.
func stdioCommands(t *testing.T, text string) []string {
	t.Helper()
	document, err := mcp.ParseDocument(publicFile, text)
	if err != nil {
		t.Fatalf("reading this repository's own %s: %v", publicFile, err)
	}
	var commands []string
	for _, server := range document.Servers {
		if server.Transport() == mcp.Stdio {
			commands = append(commands, server.Command())
		}
	}
	return commands
}
