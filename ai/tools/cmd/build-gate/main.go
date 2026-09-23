// The build gate as a command. Its stub in ai/kk-flavor/scripts states the usage and what a caller
// gets back.
package main

import (
	"os"

	buildgate "configs/ai/tools/build-gate"
)

func main() {
	os.Exit(buildgate.Command(os.Args[1:], os.Stdout, os.Stderr))
}
