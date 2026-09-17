// The judge as a command. The body is `bloatjudge.Main`, because the order its steps run in is a
// behaviour with a case on it and a `main()` is the one shape no case can call.
//
//	usage: bloat-judge.sh [--config <policy.json>] [--numbers] [--changed[=<revisions>]] <kind> [<path>]
package main

import (
	"os"

	bloatjudge "kk-flavor/tools/bloat-judge"
)

func main() {
	os.Exit(bloatjudge.Main(os.Args[0], os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
