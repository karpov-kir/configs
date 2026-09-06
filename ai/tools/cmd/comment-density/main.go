// The comment-density detector as a command.
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
