// The offer cadence as a command.
//
//	usage: cadence.sh audit {due|asked}
package main

import (
	"os"
	"path/filepath"
	"time"

	"kk-flavor/tools/cadence"
)

func main() {
	// A working directory that will not read is not a "not due": it reaches Run as a path git will
	// refuse, and comes back undetermined.
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	// argv[0] as the stub was invoked by, which `exec -a` preserved: every refusal then names the
	// path the human actually ran.
	os.Exit(cadence.Run(filepath.Base(os.Args[0]), os.Args[1:], cwd, time.Now, os.Stdout, os.Stderr))
}
