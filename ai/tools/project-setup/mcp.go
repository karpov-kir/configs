package projectsetup

import (
	"io"

	projectmcp "kk-flavor/tools/project-mcp"
	"kk-flavor/tools/repo"
)

// The project MCP tool, called in process rather than exec'd through its own stub. It is a whole tool
// with its own suite; what reaches this installer is the exit code, which is the same contract the
// shell had when it ran the stub — minus the process.
type mcpTool struct {
	self       string
	configsDir string
	home       string
	git        repo.Git
	stdout     io.Writer
	stderr     io.Writer
}

// NewMcp is the adapter a real run configures through. configsDir is the directory holding mcp.jsonc —
// this installer's own, which is where the declaration that ships lives.
func NewMcp(self, configsDir, home string, stdout, stderr io.Writer) Mcp {
	return mcpTool{self: self, configsDir: configsDir, home: home, git: repo.Exec{}, stdout: stdout, stderr: stderr}
}

func (m mcpTool) Configure(project, agent string, isDryRun, isUninstall bool) int {
	args := []string{"--agent=" + agent}
	if isDryRun {
		args = append(args, "--dry-run")
	}
	if isUninstall {
		args = append(args, "--uninstall")
	}
	return projectmcp.Run(m.self, append(args, project), m.configsDir, m.home, m.git, m.stdout, m.stderr)
}
