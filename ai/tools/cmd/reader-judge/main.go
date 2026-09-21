// The judge as a command. The body is `readerjudge.Main`, because the order its steps run in is a
// behaviour with a case on it and a `main()` is the one shape no case can call.
//
//	usage: reader-judge.sh [--config <policy.json>] [--numbers] [--changed[=<revisions>]] <kind> [<path>]
package main

import (
	"os"

	readerjudge "configs/ai/tools/reader-judge"
)

func main() {
	os.Exit(readerjudge.Main(os.Args[0], os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
