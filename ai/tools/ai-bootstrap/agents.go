package aibootstrap

import (
	"os"
	"strings"
)

// declareAgents mounts each agent definition the flavor ships, for Claude, which reads defined agents
// from ~/.claude/agents. The comment writer is one. Run 18's writers held some 116k tokens before their
// first site, 47k of it the general agent's own start, and the thin writer opened 31k lighter.
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
