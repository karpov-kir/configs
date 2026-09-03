// The pre-commit gate as a command, and the only os.Exit in the tool: everything it does lives in the
// package beside it, so the suite can drive the same code without a process per case.
//
//	usage: gate.sh [--full] [--mutants] [--units] [--why <unit>]
package main

import (
	"os"
	"path/filepath"

	"kk-flavor/tools/gate"
)

func main() {
	// The seams the suite drives, read here so the package takes them as data. Each exists because a
	// suite must be able to point the whole run at a throwaway repository and its own cache: these
	// records decide what is skipped, and a suite writing into the developer's would make their next
	// run skip a unit it never measured.
	env := gate.Env{
		Root:      os.Getenv("GATE_ROOT"),
		Cache:     os.Getenv("GATE_CACHE"),
		UnitsFile: os.Getenv("GATE_UNITS_FILE"),
	}
	if env.Root == "" {
		// argv[0] as the stub was invoked by, which `exec -a` preserved. The repository is the parent of
		// the directory holding it — `ai/gate.sh` sits one below the root — which is the same place the
		// shell form derived from `$0`. Never the process's own cwd: the gate is run from anywhere in
		// the tree, and a root taken from there would scope every unit to a subdirectory.
		env.Root = filepath.Dir(filepath.Dir(mustAbs(os.Args[0])))
	}
	os.Exit(gate.Run(os.Args[1:], env, os.Stdout, os.Stderr))
}

func mustAbs(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
