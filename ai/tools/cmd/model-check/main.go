// The model-name check as a command.
//
//	usage: model-check.sh [--config <policy.json>]
package main

import (
	modelcheck "kk-flavor/tools/model-check"
	"os"
)

func main() {
	os.Exit(modelcheck.Run(modelcheck.Command{Args: os.Args[1:], Invocation: os.Args[0], Stdout: os.Stdout, Stderr: os.Stderr}))
}
