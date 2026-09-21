// This file holds the shipped `ai/mcp.jsonc` to what the two tools that read it depend on.
//
// It lives in this package with the other cases that read the shipped checkout, and away from the
// `mcpsync` and `projectmcp` packages, for the reason shipped_tree_test.go gives.
//
// Each tool's own cases stay beside it and run over a declaration they build. What they cannot say is
// anything about the file this repository actually ships.
package tools_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"configs/ai/tools/mcp"
	projectmcp "configs/ai/tools/project-mcp"
	"configs/ai/tools/repo/repotest"
)

// This repository's own `ai/`, and the committed half of the declaration in it.
const (
	shippedConfigsDir  = repoRoot + "/ai"
	shippedDeclaration = shippedConfigsDir + "/mcp.jsonc"
)

// The public servers that declaration ships, as a literal this file states. Every other case here
// derives its expectation from the file under test, so they stay green as the declaration empties
// out. This literal is what goes red on that edit.
var shippedServers = []string{"chrome-devtools", "playwright"}

// The wrapper every project entry reaches, named once per entry in both file formats. This is the
// single word of `projectmcp`'s launcher program that identifies an entry, and the program's shape is
// that package's subject.
const launcherProgram = "mcp-env.sh"

func TestTheShippedDeclarationParsesOnlyAfterItsCommentsAreStripped(t *testing.T) {
	t.Parallel()
	text := readFile(t, shippedDeclaration)
	if _, err := mcp.ParseDocument(shippedDeclaration, mcp.StripComments(text)); err != nil {
		t.Errorf("%s does not parse after stripping: %v\nNothing would sync.", shippedDeclaration, err)
	}
	// The control behind the ParseDocument assertion. Over a file carrying no comments that assertion
	// would be a tautology, so this check proves the file carries them.
	var value any
	if err := json.Unmarshal([]byte(text), &value); err == nil {
		t.Errorf("%s parses as plain JSON, so the case above no longer says anything about the comment "+
			"stripping — the fixture it measures has lost its comments", shippedDeclaration)
	}
}

// End to end over the file that ships. What `mcpsync` hands a client has to name a command that is
// really there, or every stdio server registers and none of them starts.
func TestEveryStdioServerTheShippedDeclarationRegistersNamesARunnableCommand(t *testing.T) {
	t.Parallel()
	configsDir, err := filepath.Abs(shippedConfigsDir)
	if err != nil {
		t.Fatalf("resolving %s: %v", shippedConfigsDir, err)
	}
	stripped := mcp.StripComments(readFile(t, shippedDeclaration))
	substituted := stdioCommands(t, mcp.SubstituteConfigsDir(stripped, configsDir))
	// The same commands with the token left in, which is the control. The substitution is what makes
	// them resolve, and the loop over `substituted` would otherwise pass whether it happened or not.
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

func TestTheShippedDeclarationNamesExactlyThePublicServersPinnedHere(t *testing.T) {
	t.Parallel()
	document, err := mcp.ReadDocument(shippedDeclaration, mcp.ConfigsToken)
	if err != nil {
		t.Fatalf("reading %s: %v", shippedDeclaration, err)
	}
	var named []string
	for _, server := range document.Servers {
		named = append(named, server.Name)
	}
	sort.Strings(named)
	if strings.Join(named, " ") != strings.Join(shippedServers, " ") {
		t.Errorf("the public declaration names %v; this file pins %v — a server added or dropped here is "+
			"one every project this flavor configures gains or loses", named, shippedServers)
	}
}

// projectmcp refuses a declaration it cannot map to a portable project entry: an added field, a
// second transport, a command that is not the launcher. That refusal is only ever asked about a
// declaration, so this case is the only place saying whether the declaration this repository ships
// still has a project form.
func TestTheShippedDeclarationStillMapsIntoAProjectFile(t *testing.T) {
	t.Parallel()
	configsDir, err := filepath.Abs(shippedConfigsDir)
	if err != nil {
		t.Fatalf("resolving %s: %v", shippedConfigsDir, err)
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			project, home := t.TempDir(), t.TempDir()
			var out, said bytes.Buffer
			code := projectmcp.Run("project-mcp.sh", []string{"--agent=" + agent, project},
				configsDir, home, repotest.New(project), &out, &said)
			if code != 0 {
				t.Fatalf("exit %d over this repository's own declaration, so it has no portable project "+
					"form and `project-mcp.sh` now refuses every project\n%s", code, said.String())
			}
			// Entries counted by the launcher each one runs, which both file formats spell the same way.
			// A count of the server NAMES would pass on a renamed server, since one name is a prefix of
			// the other often enough. The names are the subject of
			// TestTheShippedDeclarationNamesExactlyThePublicServersPinnedHere.
			written := projectConfig(t, project)
			if entries := strings.Count(written, launcherProgram); entries != len(shippedServers) {
				t.Errorf("the project config holds %d server entries and the declaration names %d, so a "+
					"server the human declared reaches no project at all\n%s",
					entries, len(shippedServers), written)
			}
		})
	}
}

// Every stdio server's command, in document order.
func stdioCommands(t *testing.T, text string) []string {
	t.Helper()
	document, err := mcp.ParseDocument(shippedDeclaration, text)
	if err != nil {
		t.Fatalf("reading %s: %v", shippedDeclaration, err)
	}
	var commands []string
	for _, server := range document.Servers {
		if server.Transport() == mcp.Stdio {
			commands = append(commands, server.Command())
		}
	}
	return commands
}

// Everything the run left in the project, as one text. A walk finds them, so this case reads whatever
// each client's file is called. A file naming `.mcp.json` and `.codex/config.toml` would hold a
// second copy of `projectmcp`'s table.
func projectConfig(t *testing.T, project string) string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found = append(found, string(text))
		return nil
	})
	if err != nil {
		t.Fatalf("reading the project: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("the run wrote nothing into the project, so the loop below asserts nothing at all")
	}
	return strings.Join(found, "\n")
}

// Executable by its mode bits. An `access` check asks whether THIS process may run it, and root is
// allowed to execute a file with no x bit at all. Such a check would accept a command no other user
// can start.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
