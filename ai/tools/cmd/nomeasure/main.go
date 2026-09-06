// The did-not-measure counter as a command.
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
