// The handoff-prompt gate as a command.
//
//	usage: handoff-check <draft.md> [<repo>]   # <repo> resolves the base commit and is the path the
//	                                           # draft must name; defaults to .
package main

import (
	"os"
	"path/filepath"

	handoffcheck "kk-flavor/tools/handoff-check"
)

func main() {
	draft, repo := "", ""
	if len(os.Args) > 1 {
		draft = os.Args[1]
	}
	if len(os.Args) > 2 {
		repo = os.Args[2]
	}
	// The stub execs this with `-a "$0"`, so argv[0] is the path the human invoked. Refusals name that
	// path's basename rather than this binary's, which sits under bin/ and is not what they ran.
	os.Exit(handoffcheck.Run(filepath.Base(os.Args[0]), draft, repo, os.Stdout, os.Stderr))
}
