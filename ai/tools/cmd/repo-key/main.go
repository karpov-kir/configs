// The repo key as a command.
//
//	usage: repo-key.sh [<repo path>]
//
// Prints the key, or exits 2 naming which way it could not be answered. There is no exit 1: a key
// either names this clone or it is nothing, and a caller that read a refusal as a key would write into
// a directory belonging to no repository.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	repokey "kk-flavor/tools/repo-key"
)

func main() {
	self := filepath.Base(os.Args[0])
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	key, err := repokey.Resolve(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", self, err)
		os.Exit(2)
	}
	fmt.Println(key)
}
