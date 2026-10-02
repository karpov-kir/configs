// The dispatch guard as a command, behind `ai/kk-flavor/scripts/agent-guard.sh`, which states the
// usage and what a caller gets back.
package main

import (
	"os"

	agentguard "configs/ai/tools/agent-guard"
)

func main() {
	os.Exit(agentguard.Run(os.Stdin, os.Stderr))
}
