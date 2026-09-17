// The project install as a command.
//
//	usage: install-project.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--uninstall] <project>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"kk-flavor/tools/machine"
	projectsetup "kk-flavor/tools/project-setup"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. The skills, the MCP declaration and
	// the sibling scripts sit beside that stub, so this is where they are read from — never the process's
	// own working directory, which is wherever the human happened to be standing.
	self := filepath.Base(os.Args[0])
	repo, err := ownDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so nothing was written: %s\n", self, err)
		os.Exit(2)
	}
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = home + "/.config"
	}
	os.Exit(projectsetup.Run(projectsetup.Options{
		Self:       self,
		Args:       os.Args[1:],
		Repo:       repo,
		Home:       home,
		ConfigHome: configHome,
		Machine:    machine.New(),
		Git:        projectsetup.NewGit(),
		Mcp:        projectsetup.NewMcp(self, repo, home, os.Stdout, os.Stderr),
		Out:        os.Stdout,
		Err:        os.Stderr,
	}))
}

// Symlinks resolved, because the checkout is often reached through one and the sources are read from
// the real directory the stub lives in, not from the name it was reached by.
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
