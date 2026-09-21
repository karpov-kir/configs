// The user-scope MCP sync as a command.
//
//	usage: mcp-sync.sh --agent=claude|codex
package main

import (
	"fmt"
	"os"
	"path/filepath"

	mcpsync "configs/ai/tools/mcp-sync"
	"configs/ai/tools/shell"
)

func main() {
	// argv[0] as the stub was invoked by, which `exec -a` preserved. The declarations and the launcher
	// sit beside that stub, so the sync reads them from there. The process's own working directory is
	// wherever the human happened to be standing, and no source is taken from it.
	self := filepath.Base(os.Args[0])
	configsDir, err := shell.OwnDirectory(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve my own directory, so nothing was synced: %s\n", self, err)
		os.Exit(2)
	}
	os.Exit(mcpsync.Run(self, os.Args[1:], configsDir,
		mcpsync.NewCLIClient(os.Stdout, os.Stderr), os.Stdout, os.Stderr))
}
