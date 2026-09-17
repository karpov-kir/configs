// The tree fingerprint as a command, behind `ai/kk-flavor/scripts/tree-fingerprint.sh`, which states
// the usage and what a caller gets back.
package main

import (
	"os"

	treefingerprint "configs/ai/tools/tree-fingerprint"
)

func main() {
	os.Exit(treefingerprint.Run(os.Args[1:], os.Stdout, os.Stderr))
}
