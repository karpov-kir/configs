// The judge as a command.
//
//	usage: bloat-judge.sh [--numbers] [--changed[=<revisions>]] <kind> [<path>]
package main

import (
	"os"
	"path/filepath"

	bloatjudge "kk-flavor/tools/bloat-judge"
)

func main() {
	self := filepath.Base(os.Args[0])
	// An unreadable home leaves nowhere for an override to sit, which reads as no override rather
	// than as a broken one.
	home, _ := os.UserHomeDir()
	deadline, ok := bloatjudge.ResolveRollDeadline(self, os.Getenv("XDG_CONFIG_HOME"), home, os.Stderr)
	if !ok {
		os.Exit(2)
	}
	memo := bloatjudge.DefaultMemo()
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	os.Exit(bloatjudge.Run(self, os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		bloatjudge.Voting(bloatjudge.ClaudeCaller(deadline), 3), memo))
}
