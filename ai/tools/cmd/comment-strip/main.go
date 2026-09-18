// The command behind comment-strip.sh. It reaches no model and no policy, so it resolves neither.
package main

import (
	"os"

	commentstrip "kk-flavor/tools/comment-strip"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		os.Stderr.WriteString("comment-strip.sh: no working directory — the strip did NOT run\n")
		os.Exit(2)
	}
	os.Exit(commentstrip.Strip("comment-strip.sh", os.Args[1:], cwd, os.Stdout, os.Stderr))
}
