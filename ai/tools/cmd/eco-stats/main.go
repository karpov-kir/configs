// The ecosystem size ledger as a command.
//
//	usage: stats.sh --agent=claude|codex [--append <note>] [<root>]
//
// Without --append it prints the current measurements; with it, it prints them and appends a dated
// row to stats.md. The note is one argument — quote it, or its first word is read as <root>. <root>
// holds kk-flavor/ with skills/ inside it, and defaults to . then ./ai, matching ecocheck. The row
// goes to ../stats.md relative to this program, because the ledger belongs to kk-reduce and this runs
// from its scripts/.
package main

import (
	"os"

	ecostats "configs/ai/tools/eco-stats"
)

func main() {
	self := ""
	if len(os.Args) > 0 {
		self = os.Args[0]
	}
	os.Exit(ecostats.Run(self, os.Args[1:], os.Stdout, os.Stderr))
}
