package ecoroot

import (
	"fmt"
	"strings"
)

// AgentArgs removes exactly one required provider selector before tool-specific parsing.
// An append note is opaque, even if its text resembles a provider selector.
func AgentArgs(args []string) (agent string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--append" {
			rest = append(rest, arg)
			if i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "--agent=") {
			if agent != "" {
				return "", nil, fmt.Errorf("--agent must be specified exactly once")
			}
			agent = strings.TrimPrefix(arg, "--agent=")
			if agent != "claude" && agent != "codex" {
				return "", nil, fmt.Errorf("--agent must be claude or codex")
			}
		} else {
			rest = append(rest, arg)
		}
	}
	if agent == "" {
		return "", nil, fmt.Errorf("--agent=claude|codex is required; no provider is selected by default")
	}
	return agent, rest, nil
}
