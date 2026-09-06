// The field guide generator as a command.
//
//	usage: guide.sh [--check] [<root>]
//
// <root> holds kk-flavor/ and skills/, and defaults to . then ./ai, matching ecocheck and ecostats.
// The page is written to <root>/field-guide.html; --check regenerates into memory and compares
// against that file instead of writing it.
package main

import (
	"os"

	ecoguide "kk-flavor/tools/eco-guide"
)

func main() {
	self := ""
	if len(os.Args) > 0 {
		self = os.Args[0]
	}
	os.Exit(ecoguide.Run(self, os.Args[1:], os.Stdout, os.Stderr))
}
