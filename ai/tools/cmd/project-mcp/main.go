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
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved: the declaration this reads sits
	// beside that stub, and every refusal names the path the human actually ran.
	self := filepath.Base(os.Args[0])
	configsDir, err := ownDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so no project was configured: %s\n", self, err)
		os.Exit(2)
	}
	os.Exit(projectmcp.Run(self, os.Args[1:], configsDir, os.Getenv("HOME"), repo.Exec{}, os.Stdout, os.Stderr))
}

// Symlinks resolved, because the checkout is often reached through one and the declaration is read
// from the real directory the stub lives in, not from the name it was reached by.
func ownDirectory(invocation string) (string, error) {
	absolute, err := filepath.Abs(invocation)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Dir(real), nil
}
