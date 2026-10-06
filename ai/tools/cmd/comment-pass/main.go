// The command behind comment-pass.sh: one model call per changed file, through the CLI.
package main

import (
	"os"

	commentpass "configs/ai/tools/comment-pass"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		os.Stderr.WriteString("comment-pass.sh: no working directory — the pass did NOT run\n")
		os.Exit(2)
	}
	os.Exit(commentpass.Run("comment-pass.sh", os.Args[1:], cwd, os.LookupEnv, commentpass.CLICaller, os.Stdout, os.Stderr))
}
