package projectmcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"configs/ai/tools/mcp"
)

// Claude's project file is JSON, and it is the project's file rather than this tool's: it holds
// whatever else the project put there, and this only adds or removes the servers it owns.
//
// An entry already registered under one of those names and holding something else is a refusal, not
// an overwrite — the human wrote it, and this tool cannot tell an intentional override from a stale
// copy. Uninstalling skips such an entry for the same reason: it is not the one this tool wrote, so
// it is not this tool's to remove.
func mergeClaude(text string, servers []projectServer, isUninstall bool) (string, error) {
	config := mcp.NewObject()
	if text != "" {
		parsed, err := mcp.ParseObject([]byte(text))
		if err != nil {
			return "", fmt.Errorf(".mcp.json must be an object")
		}
		config = parsed
	}
	current := mcp.NewObject()
	if raw, held := config.Get(mcp.ServersKey); held {
		parsed, err := mcp.ParseObject(raw)
		if err != nil {
			return "", fmt.Errorf("%s must be an object", mcp.ServersKey)
		}
		current = parsed
	}

	hasChanged := false
	for _, server := range servers {
		entry, err := claudeEntry(server)
		if err != nil {
			return "", err
		}
		existing, held := current.Get(server.Name)
		switch {
		case held && !isSameJSON(existing, entry):
			if isUninstall {
				continue
			}
			return "", fmt.Errorf("project MCP server %s already exists with different settings", server.Name)
		case held && isUninstall:
			current.Delete(server.Name)
			hasChanged = true
		case !held && !isUninstall:
			current.Set(server.Name, entry)
			hasChanged = true
		}
	}
	if !hasChanged {
		return text, nil
	}
	raw, err := current.MarshalJSON()
	if err != nil {
		return "", err
	}
	config.Set(mcp.ServersKey, raw)
	return indentJSON(config)
}

// Two entries are the same server when they say the same thing, whatever order or spacing they say it
// in: the file may have been written by an editor, by an earlier version of this tool, or by the Node
// one it replaces.
func isSameJSON(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false
	}
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

// Two spaces and a trailing newline — what a human would write, and what every other JSON file in
// these projects is formatted as.
func indentJSON(config *mcp.Object) (string, error) {
	compact, err := config.MarshalJSON()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err = json.Indent(&out, compact, "", "  "); err != nil {
		return "", err
	}
	return out.String() + "\n", nil
}
