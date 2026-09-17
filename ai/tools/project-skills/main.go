// The post-checkout skill sync as a command.
//
//	usage: project-skills.sh --sync <worktree>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	projectsetup "kk-flavor/tools/project-setup"
	"kk-flavor/tools/shell"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. A repository's post-checkout hook
	// reaches this stub through ~/.kk-flavor, so its own directory is the only thing that names the
	// checkout the skills come from.
	self := filepath.Base(os.Args[0])
	repo, err := shell.OwnDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so no skills were changed: %s\n", self, err)
		os.Exit(2)
	}
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = home + "/.config"
	}
	os.Exit(projectsetup.Sync(projectsetup.SyncOptions{
		Self:       self,
		Args:       os.Args[1:],
		Repo:       repo,
		Home:       home,
		ConfigHome: configHome,
		Git:        projectsetup.NewGit(),
		Out:        os.Stdout,
		Err:        os.Stderr,
	}))
}
