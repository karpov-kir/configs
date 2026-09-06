// The qualify report tool as a command.
//
//	usage: report.sh {init <intent>|repo-mode|invalidate|stage-returned <stage>|no-items <stage>|
//	                  stamp "<stages>"|gate|carry|check-ignore|promote|discard|close|state|list} [<intent>]
//
// It reads its skill directory from argv[0], as the shell version read it from $0, so a copied skill
// directory resolves its own template and todo-gate.sh.
package main

import (
	"os"

	ecoreport "kk-flavor/tools/eco-report"
)

func main() {
	os.Exit(ecoreport.Run(os.Args[1:], os.Stdout, os.Stderr))
}
