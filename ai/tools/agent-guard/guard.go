// Package agentguard refuses a dispatch to the general agent whose prompt has no `Needs:` line: a Claude
// Agent call to general-purpose or to no type, or a Codex spawn_agent call naming no role. It is the
// PreToolUse hook bootstrap registers in each client.
//
//	usage: agent-guard.sh < <the hook's JSON>
//
// The rule is `~/.kk-flavor/standards/skill-protocol.md` → `Caller`: a dispatch takes the narrowest
// agent type holding its tools. The general agent opens near 45k tokens in the desktop app, and a
// narrow type near 5k. Held only in an instruction file, the rule left 76% of a month's requests to
// the general agent with no template.
//
// tested by: the Go suite beside this file.
package agentguard

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

// Exit codes the hook protocol reads: 0 lets the call run, 2 refuses it and hands stderr to the model.
const (
	exitAllow  = 0
	exitRefuse = 2
)

// generalAgent is the type the rule holds to a `Needs:` line. An omitted type dispatches to it too.
const generalAgent = "general-purpose"

// reNeeds is a prompt line naming what sends the dispatch to the general agent: a tool, or a type the
// session never loaded.
var reNeeds = regexp.MustCompile(`(?m)^[ \t]*Needs:[ \t]*\S`)

// RefusalPrefix opens the guard's own refusal. The hook passes every other exit, the resolver's
// included, so a guard that cannot build stops no dispatch.
const RefusalPrefix = "agent-guard refused this dispatch:"

// refusal names the two narrow types, so the model can dispatch again without reading the rule.
const refusal = RefusalPrefix + " a general-purpose dispatch names no tool it needs. In Claude, dispatch to " +
	"read-worker (Read, Grep, Glob, Bash) or edit-worker (adds Edit, Write); in Codex, spawn the light-worker role. " +
	"Where the task needs another tool, or this session predates those types, add a line " +
	"`Needs: <the tool or the type>` to the prompt (~/.kk-flavor/standards/skill-protocol.md → Caller).\n"

// usage is the line the stub states. A hook passes no argument, so an argument is a caller's mistake.
const usage = "usage: agent-guard.sh < <the hook's JSON>"

// Run reads one PreToolUse event and returns the exit code that lets the dispatch run or refuses it.
// Input it cannot read lets the dispatch run with a note on stderr. A guard that failed closed would
// stop every dispatch on a change to the hook protocol.
func Run(args []string, stdin io.Reader, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "agent-guard.sh: takes no argument, only the hook's JSON on stdin\n%s\n", usage)
		return exitRefuse
	}
	var event struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			SubagentType string `json:"subagent_type"`
			Prompt       string `json:"prompt"`
			// Codex's spawn_agent names its role and carries its task under these keys.
			AgentType string `json:"agent_type"`
			Role      string `json:"role"`
			Message   string `json:"message"`
		} `json:"tool_input"`
	}
	if err := json.NewDecoder(stdin).Decode(&event); err != nil {
		fmt.Fprintf(stderr, "agent-guard: could not read the hook input, so the dispatch runs unchecked: %v\n", err)
		return exitAllow
	}
	input := event.ToolInput
	switch event.ToolName {
	case "Agent", "Task":
		if input.SubagentType != "" && input.SubagentType != generalAgent {
			return exitAllow
		}
	case "spawn_agent":
		// A Codex spawn naming a role took the light worker or another the owner declared.
		if input.AgentType != "" || input.Role != "" {
			return exitAllow
		}
	default:
		return exitAllow
	}
	if reNeeds.MatchString(input.Prompt + "\n" + input.Message) {
		return exitAllow
	}
	fmt.Fprint(stderr, refusal)
	return exitRefuse
}
