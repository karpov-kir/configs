// The ecosystem size ledger as a command.
//
//	usage: ecostats [<root>]                    print the current measurements
//	       ecostats --append "<note>" [<root>]  print them and append a dated row to stats.md
//
// The note is one argument — quote it, or its first word is read as <root>. <root> holds kk-flavor/
// and skills/, and defaults to . then ./ai, matching ecocheck. The row goes to ../stats.md relative
// to this program, because the ledger belongs to kk-reduce and this runs from its scripts/.
package main

import (
	"os"

	ecostats "kk-flavor/tools/eco-stats"
)

func main() {
	self := ""
	if len(os.Args) > 0 {
		self = os.Args[0]
	}
	os.Exit(ecostats.Run(self, os.Args[1:], os.Stdout, os.Stderr))
}
