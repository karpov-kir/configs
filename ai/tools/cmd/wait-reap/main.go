// The reaper as a command, behind `ai/kk-flavor/scripts/wait-reap.sh`, which states the usage and
// what a caller gets back.
package main

import (
	"os"

	waitreap "kk-flavor/tools/wait-reap"
)

func main() {
	os.Exit(waitreap.Run(os.Args[1:], os.Stdout, os.Stderr))
}
