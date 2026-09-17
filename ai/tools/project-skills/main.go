// The post-checkout skill sync as a command.
//
//	usage: project-skills.sh --sync <worktree>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	projectsetup "kk-flavor/tools/project-setup"
	gitrepo "kk-flavor/tools/repo"
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
		// The location variables dropped: GIT_DIR and GIT_COMMON_DIR override `-C`, and the sync this
		// shares a port with runs from inside a post-checkout hook, which is given them.
		Git: gitrepo.Exec{Env: gitrepo.WithoutGitLocation(os.Environ())},
		Out: os.Stdout,
		Err: os.Stderr,
	}))
}
