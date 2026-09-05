// The threshold holder as a command, and the only os.Exit in the tool: everything it does lives in
// the package beside it, so the suite can drive the same code without a process per case.
//
//	usage: score.sh threshold <lane>
//	       score.sh cut [--kept-all <why>] <lane> <what a 10 is here>
package main

import (
	"os"
	"path/filepath"

	"kk-flavor/tools/score"
)

func main() {
	// The whole argv[0] the stub was invoked as, which `exec -a` preserved. ConfigPaths resolves the
	// tracked thresholds file relative to it, so it must not be shortened on the way there. Only Run
	// takes the basename, which is the name the human typed and the one a refusal prints.
	self := os.Args[0]
	env := score.ConfigPaths(self, os.LookupEnv)
	os.Exit(score.Run(filepath.Base(self), os.Args[1:], env, os.Stdin, os.Stdout, os.Stderr))
}
