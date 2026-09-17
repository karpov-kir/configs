// The repeated-literal detector as a command.
//
//	usage: dup-literals.sh [<git-diff revisions>] [-- <paths>]
package main

import (
	"os"
	"path/filepath"

	duplicates "configs/ai/tools/dup-literals"
	"configs/ai/tools/repo"
)

func main() {
	self := filepath.Base(os.Args[0])
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cfg, err := duplicates.ConfigFromEnv(os.LookupEnv)
	if err != nil {
		os.Stderr.WriteString(self + ": " + err.Error() + "\n")
		os.Exit(2)
	}
	os.Exit(duplicates.Run(self, os.Args[1:], cwd, repo.Exec{}, cfg, os.Stdout, os.Stderr))
}
