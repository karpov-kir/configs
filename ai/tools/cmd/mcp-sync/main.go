// The user-scope MCP sync as a command.
//
//	usage: mcp-sync.sh --agent=claude|codex
package main

import (
	"fmt"
	"os"
	"path/filepath"

	mcpsync "kk-flavor/tools/mcp-sync"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. The declarations and the launcher
	// sit beside that stub, so this is where they are read from — never the process's own working
	// directory, which is wherever the human happened to be standing.
	self := filepath.Base(os.Args[0])
	configsDir, err := ownDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so nothing was synced: %s\n", self, err)
		os.Exit(2)
	}
	os.Exit(mcpsync.Run(self, os.Args[1:], configsDir,
		mcpsync.NewCLIClient(os.Stdout, os.Stderr), os.Stdout, os.Stderr))
}

// Symlinks resolved, because the checkout is often reached through one and the declarations are read
// from the real directory the stub lives in, not from the name it was reached by.
func ownDirectory(invocation string) (string, error) {
	absolute, err := filepath.Abs(invocation)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Dir(real), nil
}
