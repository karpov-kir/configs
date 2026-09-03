// The ecosystem wiring check as a command, and the only os.Exit in the tool: everything it does
// lives in the package beside it, so the suite can drive the same code without a process per case.
//
//	usage: ecocheck [--gate] [<root>]   # <root> holds kk-flavor/ and skills/; defaults to . then ./ai
//
// --gate narrows the walk to what a commit can carry, so two checkouts of one commit cannot disagree.
package main

import (
	"os"

	ecocheck "kk-flavor/tools/eco-check"
)

func main() {
	os.Exit(ecocheck.Run(os.Args[1:], os.Stdout, os.Stderr))
}
