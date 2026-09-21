package projectsetup

import (
	"io"

	projectmcp "configs/ai/tools/project-mcp"
	"configs/ai/tools/repo"
)

// The project MCP tool as this installer reaches it: a call in this process. The shell exec'd the
// tool's own stub and read its exit code, and that exit code is still the whole of what comes back.
// The tool keeps its own suite.
type mcpTool struct {
	self       string
	configsDir string
	home       string
	git        repo.Git
	stdout     io.Writer
	stderr     io.Writer
}

// NewMcp is the adapter a real run configures through. configsDir holds mcp.jsonc: this installer's
// own directory, carrying the declaration that ships.
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
