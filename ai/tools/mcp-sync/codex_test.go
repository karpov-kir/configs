package mcpsync

import (
	"encoding/json"
	"strings"
	"testing"

	"configs/ai/tools/mcp"
)

func server(config string) mcp.Server {
	return mcp.Server{Name: "one", Config: json.RawMessage(config)}
}

// Every transport field Codex can be handed, and every shape of one it cannot. The table is
// exhaustive. A row decides whether its declaration reaches a registry unchanged or draws a refusal.
// A missing row leaves a field accepted by accident.
func TestCodexTakesWhatItCanPreserveAndRefusesWhatItCannot(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name   string
		config string
		want   bool
	}{
		{name: "stdio with arguments and an environment", want: true,
			config: `{"type":"stdio","command":"/bin/echo","args":["a b","$(literal)"],"env":{"VALUE":"x=y"}}`},
		{name: "streamable HTTP", want: true, config: `{"type":"http","url":"http://127.0.0.1:9/mcp"}`},
		{name: "stdio by default, with no type at all", want: true, config: `{"command":"/bin/echo"}`},
		{name: "a transport Codex does not have", want: false,
			config: `{"type":"sse","url":"https://example.invalid"}`},
		{name: "HTTP headers, which the command line cannot carry", want: false,
			config: `{"type":"http","url":"https://example.invalid","headers":{"X":"value"}}`},
		{name: "a URL that is not one", want: false, config: `{"type":"http","url":"not a url"}`},
		{name: "an argument that is not a string", want: false, config: `{"command":"echo","args":[1]}`},
		{name: "an environment value that is not a string", want: false, config: `{"command":"echo","env":{"X":2}}`},
		{name: "a working directory, which Codex would drop", want: false, config: `{"command":"echo","cwd":"/tmp"}`},
		{name: "an argument carrying a NUL", want: false, config: `{"command":"echo","args":["a\u0000b"]}`},
		{name: "an environment name no shell could export", want: false,
			config: `{"command":"echo","env":{"1BAD":"x"}}`},
		{name: "no command at all", want: false, config: `{"type":"stdio"}`},
		{name: "an empty command", want: false, config: `{"type":"stdio","command":""}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			_, err := codexArgs(server(scenario.config))
			if (err == nil) != scenario.want {
				t.Errorf("%s: accepted = %v, want %v (%v)\nA declaration wrongly accepted here registers as "+
					"something other than what the file says; one wrongly refused stops a sync that was fine.",
					scenario.config, err == nil, scenario.want, err)
			}
		})
	}
}

// The command line itself, which is what a registration IS. The case asserts the argument list. A
// rendered string hides the boundaries between arguments, and those are what is at risk. A value
// holding a space, a shell metacharacter or a newline has to arrive as one argument.
func TestTheCommandLineKeepsEveryArgumentWhole(t *testing.T) {
	t.Parallel()
	args, err := codexArgs(mcp.Server{Name: "literal", Config: json.RawMessage(
		`{"type":"stdio","command":"/bin/echo","args":["a b","$(literal)","a\nb"],"env":{"VALUE":"x=y z"}}`)})
	if err != nil {
		t.Fatalf("building the command line: %v", err)
	}
	want := []string{"mcp", "add", "literal", "--env", "VALUE=x=y z", "--", "/bin/echo", "a b", "$(literal)", "a\nb"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("the command line is\n  %q\nwant\n  %q\nA boundary lost here is a server launched with "+
			"arguments the human never wrote.", args, want)
	}
}

// `--` before the command, so a command or argument that starts with a dash is not read as a flag of
// Codex's own.
func TestTheCommandIsSeparatedFromCodexsOwnFlags(t *testing.T) {
	t.Parallel()
	args, err := codexArgs(server(`{"command":"--not-a-flag","args":["--neither"]}`))
	if err != nil {
		t.Fatalf("building the command line: %v", err)
	}
	if index := indexOf(args, "--"); index < 0 || args[index+1] != "--not-a-flag" {
		t.Errorf("the command is not separated from Codex's own flags: %q", args)
	}
}

func TestAnEnvironmentIsPassedInTheOrderTheDeclarationWroteIt(t *testing.T) {
	t.Parallel()
	args, err := codexArgs(server(`{"command":"/bin/echo","env":{"ZEBRA":"1","ALPHA":"2"}}`))
	if err != nil {
		t.Fatalf("building the command line: %v", err)
	}
	want := "mcp add one --env ZEBRA=1 --env ALPHA=2 -- /bin/echo"
	if strings.Join(args, " ") != want {
		t.Errorf("the environment came out as %q, want %q. Two runs over one file have to build the same "+
			"command line, or a re-sync looks like a change.", strings.Join(args, " "), want)
	}
}

func TestAnHTTPServerIsRegisteredAsAURL(t *testing.T) {
	t.Parallel()
	args, err := codexArgs(server(`{"type":"http","url":"http://127.0.0.1:9/mcp"}`))
	if err != nil {
		t.Fatalf("building the command line: %v", err)
	}
	if strings.Join(args, " ") != "mcp add one --url http://127.0.0.1:9/mcp" {
		t.Errorf("an HTTP server came out as %q", strings.Join(args, " "))
	}
}

func indexOf(values []string, want string) int {
	for index, value := range values {
		if value == want {
			return index
		}
	}
	return -1
}
