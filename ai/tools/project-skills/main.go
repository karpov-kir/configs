// The post-checkout skill sync as a command.
//
//	usage: project-skills.sh --sync <worktree>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	projectsetup "kk-flavor/tools/project-setup"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. A repository's post-checkout hook
	// reaches this stub through ~/.kk-flavor, so its own directory is the only thing that names the
	// checkout the skills come from.
	self := filepath.Base(os.Args[0])
	repo, err := ownDirectory(os.Args[0])
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

// Symlinks resolved, because the hook reaches this through the shared bucket and the skills are read
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
