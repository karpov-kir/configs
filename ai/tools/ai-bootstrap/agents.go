package aibootstrap

import (
	"os"
	"strings"
)

// declareAgents mounts each agent definition the flavor ships, for Claude, which reads defined agents
// from ~/.claude/agents. The comment writer is one: a general agent loaded some 147k tokens of MCP
// schemas, skills and instruction files before its first write, and the writer reads none of them.
func (run *invocation) declareAgents() {
	if run.agent != claudeAgent {
		return
	}
	entries, err := os.ReadDir(run.Repo + "/kk-flavor/agents")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			run.mounting.AddConfig(run.Repo+"/kk-flavor/agents/"+entry.Name(), run.Home+"/.claude/agents/"+entry.Name())
		}
	}
}
