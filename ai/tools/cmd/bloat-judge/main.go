// The judge as a command, and the only os.Exit in the tool: everything it does lives in the package
// beside it, so the suite can drive the same code with a fake model and no process per case.
//
//	usage: bloat-judge.sh [--numbers] <kind> [<path>]
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
