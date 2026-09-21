package mcpsync

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"configs/ai/tools/mcp"
)

// The transport Codex registers as a URL. Every other transport it takes is a child process.
const codexHTTP = "http"

// What Codex can be handed. It takes a server as command, arguments and environment, or as a
// streamable HTTP URL. All of it goes over as command-line arguments, so a declaration carrying a
// field beyond these loses it on the way in, and the sync refuses such a declaration.
var codexStdioFields = []string{"type", "command", "args", "env"}

var codexHTTPFields = []string{"type", "url"}

// A server name Codex takes on its command line without quoting or rewriting it.
var codexServerName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)

// An environment variable name, as every shell spells one.
var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// The URLs Codex registers as a streamable HTTP transport. Whitespace and control characters are
// excluded, because the URL becomes one command-line argument and neither survives being read back
// out of one.
var codexURL = regexp.MustCompile(`^https?://[^[:space:][:cntrl:]]+$`)

type codexClient struct {
	run func(args []string) error
}

// Codex takes the whole file or none of it. It has no way to represent a declaration's other
// top-level keys, and a name it cannot spell on a command line becomes a server registered under a
// different name.
func (c codexClient) AcceptsDocument(document *mcp.Document) error {
	unsupported := len(document.Keys) != 1 || document.Keys[0] != mcp.ServersKey
	for _, server := range document.Servers {
		if !codexServerName.MatchString(server.Name) {
			unsupported = true
		}
	}
	if unsupported {
		return fmt.Errorf("%s has unsupported fields or Codex server names", document.Source)
	}
	for _, server := range document.Servers {
		if _, err := codexArgs(server); err != nil {
			return fmt.Errorf("Codex cannot preserve the transport fields for '%s' in %s",
				server.Name, document.Source)
		}
	}
	return nil
}

func (c codexClient) Register(server mcp.Server) error {
	args, err := codexArgs(server)
	if err != nil {
		return fmt.Errorf("syncing '%s' to Codex failed. No later entries were synced", server.Name)
	}
	if err = c.run(args); err != nil {
		return fmt.Errorf("syncing '%s' to Codex failed. No later entries were synced", server.Name)
	}
	return nil
}

// The build IS the validation. Every field Codex can carry appears here as an argument, so a field
// that reaches no argument is a field the registration loses, and the same walk answers both
// questions. Two separate walks, one deciding and one building, let a field pass the first and drop
// out of the second.

// The command line that registers one server, or the reason it cannot be built.
func codexArgs(server mcp.Server) ([]string, error) {
	config, err := mcp.ParseObject(server.Config)
	if err != nil {
		return nil, fmt.Errorf("the entry is not an object")
	}
	transport, err := stringField(config, "type", mcp.Stdio)
	if err != nil {
		return nil, err
	}
	args := []string{"mcp", "add", server.Name}
	if transport == codexHTTP {
		if err = onlyFields(config, codexHTTPFields); err != nil {
			return nil, err
		}
		url, urlErr := stringField(config, "url", "")
		if urlErr != nil {
			return nil, urlErr
		}
		if !codexURL.MatchString(url) {
			return nil, fmt.Errorf("%q is not an http(s) URL Codex can be handed as one argument", url)
		}
		return append(args, "--url", url), nil
	}
	if transport != mcp.Stdio {
		return nil, fmt.Errorf("Codex has no %q transport", transport)
	}
	if err = onlyFields(config, codexStdioFields); err != nil {
		return nil, err
	}
	environment, err := environmentArgs(config)
	if err != nil {
		return nil, err
	}
	args = append(args, environment...)
	command, err := stringField(config, "command", "")
	if err != nil {
		return nil, err
	}
	if command == "" {
		return nil, fmt.Errorf("a stdio server needs a command")
	}
	args = append(args, "--", command)
	arguments, err := stringArray(config, "args")
	if err != nil {
		return nil, err
	}
	return append(args, arguments...), nil
}

// `--env NAME=VALUE` per entry, in the order the declaration wrote them, so two runs over one file
// build the same command line.
func environmentArgs(config *mcp.Object) ([]string, error) {
	raw, held := config.Get("env")
	if !held {
		return nil, nil
	}
	environment, err := mcp.ParseObject(raw)
	if err != nil {
		return nil, fmt.Errorf("env is not an object")
	}
	var args []string
	for _, name := range environment.Keys() {
		if !environmentName.MatchString(name) {
			return nil, fmt.Errorf("%q is not an environment variable name", name)
		}
		value, valueErr := argumentString(environment, name)
		if valueErr != nil {
			return nil, valueErr
		}
		args = append(args, "--env", name+"="+value)
	}
	return args, nil
}

func stringArray(config *mcp.Object, field string) ([]string, error) {
	raw, held := config.Get(field)
	if !held {
		return nil, nil
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%s is not an array", field)
	}
	list := make([]string, 0, len(values))
	for _, value := range values {
		text, err := argumentValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s holds %s", field, err)
		}
		list = append(list, text)
	}
	return list, nil
}

func stringField(config *mcp.Object, field, fallback string) (string, error) {
	if _, held := config.Get(field); !held {
		return fallback, nil
	}
	return argumentString(config, field)
}

func argumentString(config *mcp.Object, field string) (string, error) {
	raw, _ := config.Get(field)
	text, err := argumentValue(raw)
	if err != nil {
		return "", fmt.Errorf("%s is %s", field, err)
	}
	return text, nil
}

// The NUL is what makes this a check. A command-line argument is NUL-terminated, so a value carrying
// one arrives at the server truncated at it. A refusal here keeps a declaration from meaning one
// thing in the file and another in the registry.

// A JSON value that can be one command-line argument.
func argumentValue(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", fmt.Errorf("not a string")
	}
	if strings.ContainsRune(text, 0) {
		return "", fmt.Errorf("a string carrying a NUL, which no command-line argument can hold")
	}
	return text, nil
}

func onlyFields(config *mcp.Object, allowed []string) error {
	for _, key := range config.Keys() {
		known := false
		for _, name := range allowed {
			if key == name {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("Codex has no %q field on this transport", key)
		}
	}
	return nil
}
