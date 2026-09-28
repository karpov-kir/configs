// The command behind comment-run.sh. It runs on its own, so it resolves a model or a policy nowhere.
package main

import (
	"os"

	commentrun "configs/ai/tools/comment-run"
	"configs/ai/tools/repo"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		os.Stderr.WriteString("comment-run.sh: no working directory — the stage did NOT run\n")
		os.Exit(2)
	}
	os.Exit(commentrun.Run("comment-run.sh", os.Args[1:], cwd, repo.Exec{}, os.Stdout, os.Stderr))
}
