// The repo key as a command, behind `ai/kk-flavor/scripts/repo-key.sh`, which states the usage and
// what a caller gets back.
package main

import (
	"os"

	repokey "kk-flavor/tools/repo-key"
)

func main() {
	os.Exit(repokey.Run(os.Args[1:], os.Stdout, os.Stderr))
}
