package main

import (
	modelpolicy "kk-flavor/tools/model-policy"
	"os"
)

func main() {
	os.Exit(modelpolicy.Run(modelpolicy.Command{Args: os.Args[1:], Invocation: os.Args[0], Stdout: os.Stdout, Stderr: os.Stderr}))
}
