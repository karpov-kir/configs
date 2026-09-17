// The repo key as a command, behind `ai/kk-flavor/scripts/repo-key.sh`, which states the usage and
// what a caller gets back.
package main

import (
	"os"

	repokey "configs/ai/tools/repo-key"
)

func main() {
	os.Exit(repokey.Run(os.Args[1:], repokey.CommandGit(), os.Stdout, os.Stderr))
}
