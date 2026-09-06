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
	memo := bloatjudge.DefaultMemo()
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	os.Exit(bloatjudge.Run(filepath.Base(os.Args[0]), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, bloatjudge.Voting(bloatjudge.ClaudeCaller, 3), memo))
}
