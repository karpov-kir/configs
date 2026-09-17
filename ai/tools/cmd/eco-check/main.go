// The ecosystem wiring check as a command.
//
//	usage: ecocheck --agent=claude|codex [--gate] [<root>]   # <root> holds kk-flavor/ with skills/ inside it; defaults to . then ./ai
//
// --gate narrows the walk to what a commit can carry, so two checkouts of one commit cannot disagree.
package main

import (
	"os"

	ecocheck "configs/ai/tools/eco-check"
	"configs/ai/tools/repo"
)

func main() {
	os.Exit(ecocheck.Run(os.Args[1:], repo.Exec{}, ecocheck.InstalledBash{}, os.Stdout, os.Stderr))
}
