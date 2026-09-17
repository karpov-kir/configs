// The project install as a command.
//
//	usage: install-project.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--uninstall] <project>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"configs/ai/tools/machine"
	projectsetup "configs/ai/tools/project-setup"
	gitrepo "configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. The skills, the MCP declaration and
	// the sibling scripts sit beside that stub, so this is where they are read from — never the process's
	// own working directory, which is wherever the human happened to be standing.
	self := filepath.Base(os.Args[0])
	repo, err := shell.OwnDirectory(os.Args[0])
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
		// The location variables dropped: GIT_DIR and GIT_COMMON_DIR override `-C`, and the sync this
		// shares a port with runs from inside a post-checkout hook, which is given them.
		Git: gitrepo.Exec{Env: gitrepo.WithoutGitLocation(os.Environ())},
		Mcp: projectsetup.NewMcp(self, repo, home, os.Stdout, os.Stderr),
		Out: os.Stdout,
		Err: os.Stderr,
	}))
}
