// The voice-check detector as a command.
package main

import (
	"os"
	"path/filepath"

	voicecheck "configs/ai/tools/voice-check"
)

func main() {
	self := filepath.Base(os.Args[0])
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cfg, err := voicecheck.ConfigFromEnv(os.LookupEnv)
	if err != nil {
		os.Stderr.WriteString(self + ": " + err.Error() + "\n")
		os.Exit(2)
	}
	os.Exit(voicecheck.Run(self, os.Args[1:], cwd, cfg, os.Stdout, os.Stderr))
}
