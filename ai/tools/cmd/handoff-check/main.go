// The handoff-prompt gate as a command.
//
//	usage: handoff-check <draft.md> [<repo>]   # <repo> resolves the base commit and is the path the
//	                                           # draft must name; defaults to .
package main

import (
	"os"
	"path/filepath"

	handoffcheck "configs/ai/tools/handoff-check"
	"configs/ai/tools/repo"
)

func main() {
	draft, repoDir := "", ""
	if len(os.Args) > 1 {
		draft = os.Args[1]
	}
	if len(os.Args) > 2 {
		repoDir = os.Args[2]
	}
	// Every question the gate asks is about the repository named on the command line, so the adapter
	// gets an environment that cannot point git at another one. git reads GIT_DIR and GIT_COMMON_DIR
	// before it reads the directory it was handed. A session drafting a handoff is often standing in a
	// linked worktree or running out of a hook that carries them.
	git := repo.Exec{Env: repo.WithoutGitLocation(os.Environ())}
	// The stub execs this with `-a "$0"`, so argv[0] is the path the human invoked. Refusals name that
	// path's basename rather than this binary's, which sits under bin/ and is not what they ran.
	os.Exit(handoffcheck.Run(filepath.Base(os.Args[0]), draft, repoDir, git, os.Stdout, os.Stderr))
}
