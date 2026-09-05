// The did-not-measure counter as a command, and the only os.Exit in the tool: everything it does
// lives in the package beside it, so the suite can drive the same code without a process per case.
//
//	usage: nomeasure <harness-exit-status> <count-file>
package main

import (
	"os"

	"kk-flavor/tools/nomeasure"
)

func main() {
	os.Exit(nomeasure.Run(os.Args[1:], os.Stdout, os.Stderr))
}
