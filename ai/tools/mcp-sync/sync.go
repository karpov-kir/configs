// The client is never inferred. Every run of this writes a live registry, so the selector is
// mandatory, and an argument this tool fails to understand stops it. A `--help` run that expected
// usage text once performed a real registration.

// It adds and updates, and it prunes no server removed from a file.

// Every message names the caller by the `self` Run is given, because the stub execs this binary with
// `-a "$0"`. A constant in its place would name this binary. `self` is a parameter, so the suite
// drives the real messages: under `go test`, os.Args[0] holds the test binary's own name.

// Sync the MCP servers declared in `ai/mcp.jsonc`, and `ai/mcp.private.jsonc` when present, into the
// selected client's user scope.
//
//	usage: mcp-sync.sh --agent=claude|codex
package mcpsync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"configs/ai/tools/mcp"
)

// Exit codes on the tools' shared vocabulary. 1 is "this machine or this declaration is in a state I
// cannot sync", 2 is "I failed to understand what you asked, so I wrote no entry".
const (
	exitSynced   = 0
	exitRefused  = 1
	exitBadUsage = 2
)

// The clients this tool knows how to register with. Written once: the usage line, the selector
// refusal and the argument parser all read it, so a client added here cannot be one the help fails to
// name.
var agents = []string{claudeAgent, codexAgent}

// The wrapper every stdio server in these files is launched through. It is what stands between an
// unpinned `npx` package and every credential exported in the shell that started the client. Servers
// registered without it each fail to start, so its absence stops the sync.
const launcherName = "mcp-env.sh"

const publicFile = "mcp.jsonc"

const privateFile = "mcp.private.jsonc"

// Client is one agent's user-scope registry: the seam the suite drives instead of a CLI that writes
// the live one. The two phases are separate so the whole document is refused before registration
// starts. A private file naming a field Codex cannot preserve must not land after the public entries
// have been rewritten.
type Client interface {
	// AcceptsDocument refuses a declaration this client cannot register faithfully. Its error is the
	// whole complaint, naming the file, because only the client knows what it cannot represent.
	AcceptsDocument(document *mcp.Document) error
	// Register adds or updates one server. Its error is the whole complaint too: what a failed
	// registration leaves behind differs per client, and only Claude's arm has removed the entry first.
	Register(server mcp.Server) error
}

// NewClient is the seam a caller swaps. Production passes NewCLIClient, which finds the agent's CLI on
// PATH. The suite passes a function returning a fake that records what it was asked to register.
type NewClient func(agent string) (Client, error)

// `configsDir` is the directory holding the declaration files and the launcher, which is the tool's
// own directory. The caller resolves it from the path the stub was invoked by, so the suite can drive
// many fixtures without moving a shared process.

// Run executes one invocation and returns its exit code.
func Run(self string, args []string, configsDir string, newClient NewClient, stdout, stderr io.Writer) int {
	agent := ""
	for _, arg := range args {
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Fprint(stdout, help(self))
			return exitSynced
		case strings.HasPrefix(arg, "--agent=") && isKnownAgent(strings.TrimPrefix(arg, "--agent=")):
			agent = strings.TrimPrefix(arg, "--agent=")
		default:
			fmt.Fprintf(stderr, "%s: unknown argument %s. Nothing was synced.\n", self, arg)
			fmt.Fprintln(stderr, usage(self))
			return exitBadUsage
		}
	}
	if agent == "" {
		fmt.Fprintf(stderr, "%s: select --agent=%s. Nothing was synced.\n", self, selector())
		fmt.Fprintln(stderr, usage(self))
		return exitBadUsage
	}

	client, err := newClient(agent)
	if err != nil {
		return refuse(stderr, "%s", err)
	}
	documents, code := readDocuments(configsDir, client, stderr)
	if code != exitSynced {
		return code
	}
	for _, document := range documents {
		for _, server := range document.Servers {
			if err = client.Register(server); err != nil {
				// This never goes through refuse(). Earlier entries have been registered by here, and
				// refuse()'s closing claim of an untouched registry would be false. The client's own error
				// says where it stopped and what that left behind.
				fmt.Fprintf(stderr, "error: %s.\n", err)
				return exitRefused
			}
			fmt.Fprintf(stdout, "synced: %s (%s)\n", server.Name, filepath.Base(document.Source))
		}
	}
	return exitSynced
}

// Every declaration this run will sync, read and refused as a set. Registration waits until all of
// them are in hand, so a refusal in the private file leaves the public entries as they were.
func readDocuments(configsDir string, client Client, stderr io.Writer) ([]*mcp.Document, int) {
	public := filepath.Join(configsDir, publicFile)
	if _, err := os.Stat(public); err != nil {
		return nil, refuse(stderr, "%s not found", public)
	}
	if !mcp.ConfigsDirIsSubstitutable(configsDir) {
		fmt.Fprintf(stderr, "error: %s contains a quote or a backslash, so %s cannot be substituted into\n",
			configsDir, mcp.ConfigsToken)
		fmt.Fprint(stderr, "       valid JSON naming it. Nothing was synced — move the checkout somewhere without one.\n")
		return nil, exitRefused
	}
	if !isExecutable(filepath.Join(configsDir, launcherName)) {
		fmt.Fprintf(stderr, "error: %s is missing or not executable, and every stdio server here is\n",
			filepath.Join(configsDir, launcherName))
		fmt.Fprint(stderr, "       launched through it. Registering them anyway would leave each one failing to start,\n")
		fmt.Fprint(stderr, "       so nothing was synced.\n")
		return nil, exitRefused
	}

	var documents []*mcp.Document
	for _, file := range []string{public, filepath.Join(configsDir, privateFile)} {
		if _, err := os.Stat(file); err != nil {
			continue
		}
		document, err := mcp.ReadDocument(file, configsDir)
		if err != nil {
			return nil, refuse(stderr, "%s", err)
		}
		if err = client.AcceptsDocument(document); err != nil {
			return nil, refuse(stderr, "%s", err)
		}
		documents = append(documents, document)
	}
	return documents, exitSynced
}

// Every refusal that stops the sync before it starts says so beside the reason. A caller reading only
// stderr has to be told that its registry is untouched.
func refuse(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "error: %s. Nothing was synced.\n", fmt.Sprintf(format, args...))
	return exitRefused
}

func isKnownAgent(agent string) bool {
	for _, known := range agents {
		if agent == known {
			return true
		}
	}
	return false
}

func selector() string {
	return strings.Join(agents, "|")
}

func usage(self string) string {
	return fmt.Sprintf("usage: %s --agent=%s", self, selector())
}

// What `--help` prints. The grammar is generated from the agent list, so a client added to this tool
// cannot be one the help fails to name.
func help(self string) string {
	return fmt.Sprintf(`Sync the MCP servers declared in %s — and %s (gitignored, same shape),
when present — into the selected client's user scope. The client is never inferred.
Edit either file, then re-run. It adds and updates but does not prune servers removed from a file.
Codex accepts stdio command/args/env and streamable HTTP URLs; unsupported fields are refused.
  %s
`, publicFile, privateFile, usage(self))
}

// isExecutable reads the mode bits. Root may execute a file carrying no x bit, so an `access` check
// accepts the very fixture the refusal exists for.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
