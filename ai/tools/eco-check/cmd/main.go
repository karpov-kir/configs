// The ecosystem wiring check as a command, and the only os.Exit in the tool: everything it does
// lives in the package beside it, so the suite can drive the same code without a process per case.
//
//	usage: ecocheck [<root>]   # <root> holds kk-flavor/ and skills/; defaults to . then ./ai
package main

import (
	"os"

	ecocheck "kk-flavor/tools/eco-check"
)

func main() {
	root := ""
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	os.Exit(ecocheck.Run(root, os.Stdout, os.Stderr))
}
