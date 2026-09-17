// The machine-wide agent install as a command.
//
//	usage: bootstrap.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew] [--skip-tools] [--skip-mcp] [--skip-rtk] [--skip-verify] [--uninstall]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	aibootstrap "kk-flavor/tools/ai-bootstrap"
	"kk-flavor/tools/machine"
	"kk-flavor/tools/shell"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. The bucket, the skills and the
	// sibling scripts sit beside that stub, so this is where they are read from — never the process's
	// own working directory, which is wherever the human happened to be standing.
	self := filepath.Base(os.Args[0])
	repo, err := shell.OwnDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so nothing was installed: %s\n", self, err)
		os.Exit(2)
	}
	home := os.Getenv("HOME")
	os.Exit(aibootstrap.Run(aibootstrap.Options{
		Self:           self,
		Args:           os.Args[1:],
		Repo:           repo,
		Home:           home,
		CodexHome:      valueOr("CODEX_HOME", home+"/.codex"),
		ConfigHome:     valueOr("XDG_CONFIG_HOME", home+"/.config"),
		IsInsideVerify: os.Getenv("BOOTSTRAP_VERIFYING") != "",
		Machine:        machine.New(),
		Out:            os.Stdout,
		Err:            os.Stderr,
	}))
}

func valueOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
