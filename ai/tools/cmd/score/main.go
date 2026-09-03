// The threshold holder as a command, and the only os.Exit in the tool: everything it does lives in
// the package beside it, so the suite can drive the same code without a process per case.
//
//	usage: score.sh threshold <lane>
//	       score.sh cut [--kept-all <why>] <lane> <what a 10 is here>
package main

import (
	"os"

	"kk-flavor/tools/score"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved: the tracked thresholds file sits
	// beside the scripts directory that path is in, so a checkout reached through its symlink mount
	// still finds its own config rather than the install's.
	self := os.Args[0]
	env := score.ConfigPaths(self, os.LookupEnv)
	name := self
	if cut := len(name) - 1; cut >= 0 {
		for i := cut; i >= 0; i-- {
			if name[i] == '/' {
				name = name[i+1:]
				break
			}
		}
	}
	os.Exit(score.Run(name, os.Args[1:], env, os.Stdin, os.Stdout, os.Stderr))
}
