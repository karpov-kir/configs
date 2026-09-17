// One project's MCP configuration as a command.
//
//	usage: project-mcp.sh --agent=claude|codex [--dry-run] [--uninstall] <project>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	projectmcp "kk-flavor/tools/project-mcp"
	"kk-flavor/tools/repo"
	"kk-flavor/tools/shell"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved: the declaration this reads sits
	// beside that stub, and every refusal names the path the human actually ran.
	self := filepath.Base(os.Args[0])
	configsDir, err := shell.OwnDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so no project was configured: %s\n", self, err)
		os.Exit(2)
	}
	os.Exit(projectmcp.Run(self, os.Args[1:], configsDir, os.Getenv("HOME"), repo.Exec{}, os.Stdout, os.Stderr))
}
