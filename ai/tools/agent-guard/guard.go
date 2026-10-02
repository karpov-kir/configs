// Package agentguard refuses a Claude dispatch to the general agent whose prompt has no `Needs:` line.
// It is the PreToolUse hook bootstrap registers on the Agent tool.
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
var reNeeds = regexp.MustCompile(`(?m)^\s*Needs:\s*\S`)

// refusal names the two narrow types, so the model can dispatch again without reading the rule.
const refusal = "agent-guard: a general-purpose dispatch names no tool it needs. Dispatch to read-worker " +
	"(Read, Grep, Glob, Bash) or edit-worker (adds Edit, Write); where the task needs another tool, or this " +
	"session predates those types, add a line `Needs: <the tool or the type>` to the prompt " +
	"(~/.kk-flavor/standards/skill-protocol.md → Caller).\n"

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
		} `json:"tool_input"`
	}
	if err := json.NewDecoder(stdin).Decode(&event); err != nil {
		fmt.Fprintf(stderr, "agent-guard: could not read the hook input, so the dispatch runs unchecked: %v\n", err)
		return exitAllow
	}
	if event.ToolName != "Agent" && event.ToolName != "Task" {
		return exitAllow
	}
	kind := event.ToolInput.SubagentType
	if kind != "" && kind != generalAgent {
		return exitAllow
	}
	if reNeeds.MatchString(event.ToolInput.Prompt) {
		return exitAllow
	}
	fmt.Fprint(stderr, refusal)
	return exitRefuse
}
