// The qualify report tool as a command.
//
// The usage grammar is not repeated here. It lives in the stub's header and in the line eco-report.go
// prints when it refuses, and ai/tools/stub_usage_test.go holds those two to each other; a third copy
// answers to neither.
//
// It reads its skill directory from argv[0], as the shell version read it from $0, so a copied skill
// directory resolves its own report template.
package main

import (
	"os"

	ecoreport "kk-flavor/tools/eco-report"
)

func main() {
	os.Exit(ecoreport.Run(os.Args[1:], os.Stdout, os.Stderr))
}
