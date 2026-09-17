// The shell and editor install as a command.
//
//	usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	envbootstrap "configs/ai/tools/env-bootstrap"
	"configs/ai/tools/machine"
	"configs/ai/tools/shell"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. The mount sources sit beside that
	// stub, so this is where they are read from — never the process's own working directory, which is
	// wherever the human happened to be standing.
	self := filepath.Base(os.Args[0])
	repo, err := shell.OwnDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so nothing was linked: %s\n", self, err)
		os.Exit(2)
	}
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = home + "/.config"
	}
	os.Exit(envbootstrap.Run(envbootstrap.Options{
		Self:       self,
		Args:       os.Args[1:],
		Repo:       repo,
		Home:       home,
		ConfigHome: configHome,
		Machine:    machine.New(),
		Out:        os.Stdout,
		Err:        os.Stderr,
	}))
}
