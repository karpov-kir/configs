// The comment-density detector as a command, and the only os.Exit in the tool: everything it does
// lives in the package beside it, so the suite can drive the same code without a process per case.
//
//	usage: comment-density.sh [<git-diff revisions>]
package main

import (
	"os"
	"path/filepath"

	density "kk-flavor/tools/comment-density"
)

func main() {
	self := filepath.Base(os.Args[0])
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cfg, err := density.ConfigFromEnv(os.LookupEnv)
	if err != nil {
		os.Stderr.WriteString(self + ": " + err.Error() + "\n")
		os.Exit(2)
	}
	os.Exit(density.Run(self, os.Args[1:], cwd, cfg, os.Stdout, os.Stderr))
}
