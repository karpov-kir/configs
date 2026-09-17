// Sync the MCP servers declared in `ai/mcp.jsonc` — and `ai/mcp.private.jsonc` when present — into
// the selected client's user scope.
//
//	usage: mcp-sync.sh --agent=claude|codex
//
// The client is never inferred: every run of this writes a live registry, so the selector is
// mandatory and an argument this tool does not understand stops it rather than being ignored. `bash
// mcp-sync.sh --help`, run expecting usage text, once performed a real registration instead.
//
// It adds and updates but does not prune servers removed from a file.
//
// Every message names the caller by the `self` Run is given rather than by a constant, because the
// stub execs this binary with `-a "$0"`. It is a parameter and not a read of os.Args[0] so the suite
// drives the real messages: under `go test` that global holds the test binary's own name.
package mcpsync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"configs/ai/tools/mcp"
)

// Exit codes on the tools' shared vocabulary. 1 is "this machine or this declaration is not in a
// state I can sync", 2 is "I did not understand what you asked, so I did nothing".
const (
	exitSynced   = 0
	exitRefused  = 1
	exitBadUsage = 2
)

// The clients this tool knows how to register with. Written once: the usage line, the selector
// refusal and the argument parser all read it, so a client added here cannot be one the help fails to
// name.
var agents = []string{claudeAgent, codexAgent}

// The wrapper every stdio server in these files is launched through, and the only thing standing
// between an unpinned `npx` package and every credential exported in the shell that started the
// client. Registering servers without it leaves each one failing to start, so its absence stops the
// sync.
const launcherName = "mcp-env.sh"

const publicFile = "mcp.jsonc"

const privateFile = "mcp.private.jsonc"

// Client is one agent's user-scope registry: the seam the suite drives instead of a CLI that writes
// the live one. The two phases are separate because the whole document is refused before anything is
// registered — a private file naming a field Codex cannot preserve must not land after the public
// entries have already been rewritten.
type Client interface {
	// AcceptsDocument refuses a declaration this client cannot register faithfully. Its error is the
	// whole complaint, naming the file, because only the client knows what it cannot represent.
	AcceptsDocument(document *mcp.Document) error
	// Register adds or updates one server. Its error is the whole complaint too: what a failed
	// registration leaves behind differs per client, and only Claude's arm has removed the entry first.
	Register(server mcp.Server) error
}

// NewClient is the seam a caller swaps. Production passes NewCLIClient, which finds the agent's CLI on
// PATH; the suite passes one returning a fake that records what it was asked to register.
type NewClient func(agent string) (Client, error)

// Run executes one invocation and returns its exit code. `configsDir` is the directory holding the
// declaration files and the launcher — the tool's own directory, resolved by the caller from the path
// the stub was invoked by, so the suite can drive many fixtures without moving a shared process.
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
				// Never through refuse(): by here, earlier entries have been registered, and "Nothing was
				// synced" would be a false claim about a registry this run has already changed. What the
				// client's own error says instead is where it stopped and what that left behind.
				fmt.Fprintf(stderr, "error: %s.\n", err)
				return exitRefused
			}
			fmt.Fprintf(stdout, "synced: %s (%s)\n", server.Name, filepath.Base(document.Source))
		}
	}
	return exitSynced
}

// Every declaration this run will sync, read and refused as a set. Nothing is registered until all of
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

// Every refusal that stops the sync before it starts says so in the same breath as the reason: a
// caller reading only stderr has to be told that its registry is untouched.
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

// What `--help` prints. The grammar is generated from the agent list rather than written out, so a
// client added to this tool cannot be one the help fails to name.
func help(self string) string {
	return fmt.Sprintf(`Sync the MCP servers declared in %s — and %s (gitignored, same shape),
when present — into the selected client's user scope. The client is never inferred.
Edit either file, then re-run. It adds and updates but does not prune servers removed from a file.
Codex accepts stdio command/args/env and streamable HTTP URLs; unsupported fields are refused.
  %s
`, publicFile, privateFile, usage(self))
}

// Executable by its mode bits, never by asking whether this process may run it: root is allowed to
// execute a file with no x bit at all, so an `access` check would accept the very fixture the refusal
// exists for.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
