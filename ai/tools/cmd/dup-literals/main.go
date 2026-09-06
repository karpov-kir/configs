// The repeated-literal detector as a command.
//
//	usage: dup-literals.sh [<git-diff revisions>]
package main

import (
	"os"
	"path/filepath"

	duplicates "kk-flavor/tools/dup-literals"
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
	os.Exit(duplicates.Run(self, os.Args[1:], cwd, cfg, os.Stdout, os.Stderr))
}
