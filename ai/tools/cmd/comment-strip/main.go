// The command behind comment-strip.sh. It runs on its own, so it resolves a model or a policy nowhere.
package main

import (
	"os"

	commentstrip "configs/ai/tools/comment-strip"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		os.Stderr.WriteString("comment-strip.sh: no working directory — the strip did NOT run\n")
		os.Exit(2)
	}
	os.Exit(commentstrip.Strip("comment-strip.sh", os.Args[1:], cwd, os.Stdout, os.Stderr))
}
