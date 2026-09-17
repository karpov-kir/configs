package mcpsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/mcp"
)

// Nothing here runs a client CLI. The seam is `Client`, and what the suite drives is the REAL
// claudeClient and codexClient with their one process call replaced by a recorder — so the argument
// lists asserted below are the ones a machine would be handed, not a re-statement of them.
//
// The real CLIs are out of reach on purpose as well as in practice: `claude mcp add-json` writes the
// human's live registry, and no CI runner has either binary. The one case that genuinely needs a real
// Codex is in codex_cli_test.go, where it skips with its reason when the CLI is absent.

// calls records one client's process invocations, and can be told to fail one of them.
type calls struct {
	lines  [][]string
	failOn string
}

func (c *calls) run(args []string) error {
	c.lines = append(c.lines, args)
	for _, arg := range args {
		if c.failOn != "" && arg == c.failOn {
			return errors.New("the CLI refused it")
		}
	}
	return nil
}

func (c *calls) flat() string {
	var out []string
	for _, line := range c.lines {
		out = append(out, strings.Join(line, " "))
	}
	return strings.Join(out, "\n")
}

// The declaration this repository actually ships, in the shape a checkout has it: two stdio servers
// launched through the wrapper.
const declaration = `// a comment owning its whole line
{
  "mcpServers": {
    "playwright": {
      "type": "stdio",
      "command": "@CONFIGS@/mcp-env.sh",
      "args": ["mise", "exec", "node@lts", "--", "npx", "-y", "@playwright/mcp"]
    },
    "chrome-devtools": {
      "type": "stdio",
      "command": "@CONFIGS@/mcp-env.sh",
      "args": ["npx", "-y", "chrome-devtools-mcp"]
    }
  }
}
`

// How a fixture's launcher is left. Absent and present-but-not-executable are two ways to reach one
// refusal, and both are checkouts a human really has: a partial copy, and a file that lost its mode
// bit through an archive.
type launcherState int

const (
	launcherExecutable launcherState = iota
	launcherAbsent
	launcherUnexecutable
)

// A checkout of this repository's `ai/` directory: the declaration, and the wrapper beside it.
//
// `name` may carry a quote, because that is the injection case. Everything lands under t.TempDir(),
// and newCheckout refuses a name that would leave it — these fixtures are the shape of the script
// that once followed a live symlink out of a sandbox and overwrote real config files in this
// checkout, and the containment is asserted before the first write rather than looked for after.
func newCheckout(t *testing.T, name string, launcher launcherState) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("building the checkout fixture at %s: %v", dir, err)
	}
	refuseOutside(t, root, dir)
	write(t, filepath.Join(dir, publicFile), declaration, 0o644)
	switch launcher {
	case launcherExecutable:
		write(t, filepath.Join(dir, launcherName), "#!/bin/sh\nexec env -i \"$@\"\n", 0o755)
	case launcherUnexecutable:
		write(t, filepath.Join(dir, launcherName), "#!/bin/sh\nexec env -i \"$@\"\n", 0o644)
	}
	return dir
}

func write(t *testing.T, file, text string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(file, []byte(text), mode); err != nil {
		t.Fatalf("writing the fixture %s: %v", file, err)
	}
}

// The physical parent, so a symlink anywhere in the path cannot route a write out of the sandbox.
func refuseOutside(t *testing.T, root, path string) {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolving the fixture path %s: %v", path, err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolving the sandbox root %s: %v", root, err)
	}
	if real != realRoot && !strings.HasPrefix(real, realRoot+string(os.PathSeparator)) {
		t.Fatalf("the fixture %s resolves to %s, outside this case's sandbox %s — a case allowed to run "+
			"from there writes into whatever the path really names", path, real, realRoot)
	}
}

// One run, with the client's process boundary recorded.
type run struct {
	code   int
	stdout string
	stderr string
	claude *calls
	codex  *calls
}

func sync(t *testing.T, dir string, args ...string) run {
	t.Helper()
	claude, codex := &calls{}, &calls{}
	return syncWith(t, dir, claude, codex, args...)
}

func syncWith(t *testing.T, dir string, claude, codex *calls, args ...string) run {
	t.Helper()
	var out, said bytes.Buffer
	newClient := func(agent string) (Client, error) {
		if agent == codexAgent {
			return codexClient{run: codex.run}, nil
		}
		return claudeClient{run: claude.run}, nil
	}
	code := Run("mcp-sync.sh", args, dir, newClient, &out, &said)
	return run{code: code, stdout: out.String(), stderr: said.String(), claude: claude, codex: codex}
}

func TestAnUnknownArgumentStopsTheSyncRatherThanBeingIgnored(t *testing.T) {
	t.Parallel()
	result := sync(t, newCheckout(t, "ai", launcherExecutable), "--not-an-argument")

	if result.code != exitBadUsage {
		t.Errorf("exit %d, want %d. Every run of this writes a live registry, so an argument it does not "+
			"understand has to stop it: `mcp-sync.sh --help` once performed a real registration.",
			result.code, exitBadUsage)
	}
	for _, needle := range []string{"unknown argument --not-an-argument", "Nothing was synced", usage("mcp-sync.sh")} {
		if !strings.Contains(result.stderr, needle) {
			t.Errorf("the refusal does not say %q — a human reading it cannot tell which door the run came "+
				"out of\n%s", needle, result.stderr)
		}
	}
	// The control the exit code cannot give: a refusal that reached the CLI first is not a refusal.
	if len(result.claude.lines) > 0 {
		t.Errorf("the client was called before the argument was refused:\n%s", result.claude.flat())
	}
}

func TestTheSyncRequiresAnExplicitAgent(t *testing.T) {
	t.Parallel()
	dir := newCheckout(t, "ai", launcherExecutable)
	for _, scenario := range []struct {
		name string
		args []string
	}{
		{name: "none named", args: nil},
		{name: "one this tool does not know", args: []string{"--agent=unknown"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			result := sync(t, dir, scenario.args...)
			if result.code != exitBadUsage {
				t.Errorf("exit %d, want %d — the client is never inferred, because the wrong one is a live "+
					"registry rewritten", result.code, exitBadUsage)
			}
			if len(result.claude.lines)+len(result.codex.lines) > 0 {
				t.Errorf("a client was reached without one being selected:\n%s%s",
					result.claude.flat(), result.codex.flat())
			}
		})
	}
}

func TestHelpPrintsTheGrammarAndSyncsNothing(t *testing.T) {
	t.Parallel()
	dir := newCheckout(t, "ai", launcherExecutable)
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			result := sync(t, dir, flag)
			if result.code != exitSynced {
				t.Errorf("%s exited %d, want 0", flag, result.code)
			}
			if len(result.claude.lines) > 0 {
				t.Errorf("%s registered something:\n%s", flag, result.claude.flat())
			}
			// Every agent the parser accepts, named by the help. Read from the same list the parser reads,
			// so a client added to this tool cannot be one the help quietly stops mentioning.
			for _, agent := range agents {
				if !strings.Contains(result.stdout, agent) {
					t.Errorf("%s does not name the %q client, which this tool accepts — the help would then "+
						"be documenting a tool that no longer exists\n%s", flag, agent, result.stdout)
				}
			}
			// Once. The shell arm this replaces printed the header and then a usage line beside it, which
			// put the line out twice the moment the header grew one of its own.
			if count := strings.Count(result.stdout, usage("mcp-sync.sh")); count != 1 {
				t.Errorf("%s printed the usage line %d times, want 1 — a second copy drifts from the first "+
					"as soon as either is edited\n%s", flag, count, result.stdout)
			}
		})
	}
}

func TestACheckoutThatSubstitutesAndHasTheWrapperSyncsEveryServer(t *testing.T) {
	t.Parallel()
	dir := newCheckout(t, "ai", launcherExecutable)
	result := sync(t, dir, "--agent=claude")

	if result.code != exitSynced {
		t.Fatalf("exit %d, want 0\n%s", result.code, result.stderr)
	}
	// The control that makes the two refusals below mean something: on a checkout that is fine, the
	// same code registers. Without it, both refusals pass on any fixture broken enough to stop early.
	if len(result.claude.lines) == 0 {
		t.Fatal("a sound checkout registered nothing, so every refusal case below would pass against a " +
			"tool that never reaches a client at all")
	}
	for _, name := range []string{"playwright", "chrome-devtools"} {
		if !strings.Contains(result.stdout, "synced: "+name+" ("+publicFile+")") {
			t.Errorf("%s was not reported as synced, and the file it came from has to be named — two are "+
				"read in one run\n%s", name, result.stdout)
		}
	}
	for _, command := range registeredCommands(t, result.claude) {
		if command != filepath.Join(dir, launcherName) {
			t.Errorf("a server was registered to run %q, which is not the wrapper in this checkout. What "+
				"the CLI is handed is a literal string it expands nothing in, so a command spelled any other "+
				"way is a server that registers and never starts.", command)
		}
	}
}

// Claude's arm removes before it adds, because `add-json` refuses a name already registered and this
// tool exists to update an entry as well as create one.
func TestClaudeEntriesAreRemovedBeforeTheyAreAdded(t *testing.T) {
	t.Parallel()
	result := sync(t, newCheckout(t, "ai", launcherExecutable), "--agent=claude")
	if result.code != exitSynced {
		t.Fatalf("exit %d, want 0\n%s", result.code, result.stderr)
	}
	if len(result.claude.lines) != 4 {
		t.Fatalf("two servers produced %d calls, want 4 — one remove and one add each\n%s",
			len(result.claude.lines), result.claude.flat())
	}
	for index, want := range []string{"remove", "add-json", "remove", "add-json"} {
		if result.claude.lines[index][1] != want {
			t.Errorf("call %d is %q, want %q. Added without removing first, an update is refused by the CLI "+
				"and the entry the human edited never reaches their client.",
				index, result.claude.lines[index][1], want)
		}
	}
}

// What a failed re-add leaves behind is not "nothing happened": the entry was removed first, so the
// human's client no longer has a server their file still declares.
func TestAFailedClaudeReAddSaysTheServerIsNowUnregistered(t *testing.T) {
	t.Parallel()
	claude := &calls{failOn: "playwright"}
	result := syncWith(t, newCheckout(t, "ai", launcherExecutable), claude, &calls{}, "--agent=claude")

	if result.code != exitRefused {
		t.Errorf("exit %d, want %d", result.code, exitRefused)
	}
	for _, needle := range []string{"UNREGISTERED", "playwright", "No later entries were synced"} {
		if !strings.Contains(result.stderr, needle) {
			t.Errorf("the refusal does not say %q, and the state it leaves is exactly what the human has to "+
				"be told\n%s", needle, result.stderr)
		}
	}
}

// A failure part-way is not "nothing happened": the entries before it are registered, and a refusal
// claiming otherwise sends the human looking for a registry that has already moved.
func TestAFailurePartWayThroughDoesNotClaimNothingWasSynced(t *testing.T) {
	t.Parallel()
	claude := &calls{failOn: "chrome-devtools"}
	result := syncWith(t, newCheckout(t, "ai", launcherExecutable), claude, &calls{}, "--agent=claude")

	if result.code != exitRefused {
		t.Fatalf("exit %d, want %d", result.code, exitRefused)
	}
	if !strings.Contains(result.stdout, "synced: playwright") {
		t.Fatalf("no entry was registered before the failure, so this case measures nothing\n%s", result.stdout)
	}
	if strings.Contains(result.stderr, "Nothing was synced") {
		t.Errorf("the refusal says nothing was synced, and one server already had been\n%s", result.stderr)
	}
	if !strings.Contains(result.stderr, "No later entries were synced") {
		t.Errorf("the refusal does not say where it stopped\n%s", result.stderr)
	}
}

func TestACheckoutThatCannotBeSubstitutedIntoJSONIsRefusedBeforeTheClient(t *testing.T) {
	t.Parallel()
	result := sync(t, newCheckout(t, `qu"oted`, launcherExecutable), "--agent=claude")

	if result.code != exitRefused {
		t.Errorf("exit %d, want %d", result.code, exitRefused)
	}
	for _, needle := range []string{"contains a quote or a backslash", "Nothing was synced"} {
		if !strings.Contains(result.stderr, needle) {
			t.Errorf("the refusal does not say %q\n%s", needle, result.stderr)
		}
	}
	if len(result.claude.lines) > 0 {
		t.Errorf("the injection reached the client:\n%s", result.claude.flat())
	}
}

func TestACheckoutWithoutARunnableWrapperIsRefused(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name     string
		launcher launcherState
	}{
		{name: "the wrapper is missing", launcher: launcherAbsent},
		// Present but not executable is the same refusal: the CLI would take the entry happily and the
		// server would fail at launch, with nothing saying so until the next session started it.
		{name: "the wrapper is not executable", launcher: launcherUnexecutable},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			result := sync(t, newCheckout(t, "ai", scenario.launcher), "--agent=claude")
			if result.code != exitRefused {
				t.Errorf("exit %d, want %d", result.code, exitRefused)
			}
			if !strings.Contains(result.stderr, "mcp-env.sh is missing or not executable") {
				t.Errorf("the refusal does not name the wrapper it could not find\n%s", result.stderr)
			}
			if len(result.claude.lines) > 0 {
				t.Errorf("servers were registered pointing at a wrapper that will not run:\n%s",
					result.claude.flat())
			}
		})
	}
}

func TestADeclarationThatIsNotThereStopsTheSync(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, filepath.Join(dir, launcherName), "#!/bin/sh\n", 0o755)
	result := sync(t, dir, "--agent=claude")

	if result.code != exitRefused {
		t.Errorf("exit %d, want %d", result.code, exitRefused)
	}
	if !strings.Contains(result.stderr, publicFile) || !strings.Contains(result.stderr, "not found") {
		t.Errorf("the refusal does not name the file it could not read\n%s", result.stderr)
	}
}

// The private half is read in the same run, and it wins: it is where an entry carrying a credential or
// naming an internal host lives, and it exists to override the committed one.
func TestThePrivateDeclarationIsSyncedAfterThePublicOne(t *testing.T) {
	t.Parallel()
	dir := newCheckout(t, "ai", launcherExecutable)
	write(t, filepath.Join(dir, privateFile),
		`{"mcpServers":{"playwright":{"type":"stdio","command":"@CONFIGS@/mcp-env.sh","args":["updated"]}}}`, 0o644)
	result := sync(t, dir, "--agent=claude")

	if result.code != exitSynced {
		t.Fatalf("exit %d, want 0\n%s", result.code, result.stderr)
	}
	if !strings.Contains(result.stdout, "synced: playwright ("+privateFile+")") {
		t.Errorf("the private definition was not synced, or was not attributed to its own file\n%s",
			result.stdout)
	}
	last := result.claude.lines[len(result.claude.lines)-1]
	if !strings.Contains(last[len(last)-1], `"updated"`) {
		t.Errorf("the last entry handed to the client is %q, and the private definition has to be the one "+
			"that lands — it is read second for exactly that reason", last[len(last)-1])
	}
}

// Every declaration is read and refused as a set before anything is registered: a private file naming
// something Codex cannot preserve must not land after the public entries have already been rewritten.
func TestAPrivateFileCodexCannotPreserveStopsTheWholeSync(t *testing.T) {
	t.Parallel()
	dir := newCheckout(t, "ai", launcherExecutable)
	write(t, filepath.Join(dir, privateFile),
		`{"mcpServers":{"unsupported":{"type":"http","url":"https://example.invalid","headers":{"X":"v"}}}}`, 0o644)
	result := sync(t, dir, "--agent=codex")

	if result.code != exitRefused {
		t.Errorf("exit %d, want %d", result.code, exitRefused)
	}
	if !strings.Contains(result.stderr, privateFile) {
		t.Errorf("the refusal does not name the file at fault\n%s", result.stderr)
	}
	if len(result.codex.lines) > 0 {
		t.Errorf("the public entries were registered before the private file was read, so a refusal now "+
			"leaves the registry half-written:\n%s", result.codex.flat())
	}
}

func TestADeclarationCodexCannotSpellOnACommandLineIsRefused(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		text string
	}{
		{name: "a top-level key beyond mcpServers", text: `{"mcpServers":{},"other":1}`},
		{name: "a server name Codex cannot take", text: `{"mcpServers":{"a b":{"command":"/bin/echo"}}}`},
		{name: "a field Codex would drop", text: `{"mcpServers":{"one":{"command":"/bin/echo","cwd":"/tmp"}}}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			dir := newCheckout(t, "ai", launcherExecutable)
			write(t, filepath.Join(dir, publicFile), scenario.text, 0o644)
			result := sync(t, dir, "--agent=codex")
			if result.code != exitRefused {
				t.Errorf("exit %d, want %d — %s would otherwise be registered as something other than what "+
					"the file says", result.code, exitRefused, scenario.name)
			}
			if len(result.codex.lines) > 0 {
				t.Errorf("it reached the CLI anyway:\n%s", result.codex.flat())
			}
		})
	}
}

// Claude stores the entry as the JSON it was declared with, so the same file that Codex refuses is one
// Claude takes whole.
func TestClaudeTakesAnEntryCodexRefuses(t *testing.T) {
	t.Parallel()
	dir := newCheckout(t, "ai", launcherExecutable)
	write(t, filepath.Join(dir, publicFile),
		`{"mcpServers":{"one":{"type":"stdio","command":"/bin/echo","cwd":"/tmp"}}}`, 0o644)
	result := sync(t, dir, "--agent=claude")

	if result.code != exitSynced {
		t.Fatalf("exit %d, want 0 — a field Codex cannot represent is not a field Claude cannot\n%s",
			result.code, result.stderr)
	}
	if !strings.Contains(result.claude.flat(), `"cwd":"/tmp"`) {
		t.Errorf("the field was dropped on the way to the client:\n%s", result.claude.flat())
	}
}

// The commands `claude mcp add-json` was handed, read back out of the JSON.
func registeredCommands(t *testing.T, claude *calls) []string {
	t.Helper()
	var commands []string
	for _, line := range claude.lines {
		if len(line) < 2 || line[1] != "add-json" {
			continue
		}
		var entry struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(line[len(line)-1]), &entry); err != nil {
			t.Fatalf("the CLI was handed something that is not JSON: %q", line[len(line)-1])
		}
		if entry.Type == mcp.Stdio {
			commands = append(commands, entry.Command)
		}
	}
	if len(commands) == 0 {
		t.Fatal("no stdio server was registered, so the caller's assertion reads nothing")
	}
	return commands
}
