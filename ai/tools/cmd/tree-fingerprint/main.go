// The tree fingerprint as a command, and the only os.Exit in the tool.
//
//	usage: tree-fingerprint.sh [<repo path>]
//
// Prints the tree hash, or exits 2 naming which way it could not be answered. There is no exit 1: a
// fingerprint either is the tree's hash or it is nothing, and a caller that read a refusal as a hash
// would write a ledger head no later run can match.
package main

import (
	"fmt"
	"os"

	treefingerprint "kk-flavor/tools/tree-fingerprint"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	tree, err := treefingerprint.Fingerprint(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(2)
	}
	fmt.Println(tree)
}
