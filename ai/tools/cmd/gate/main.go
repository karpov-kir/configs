// The pre-commit gate as a command.
//
//	usage: gate.sh [--full]
package main

import (
	"os"
	"path/filepath"
	"strconv"

	"configs/ai/tools/gate"
)

func main() {
	// The seams the suite drives, read here so the package takes them as data. Both exist so a suite
	// can reach the run loop and the over-budget refusal without running the real checks — which are
	// the suite this gate is part of, and a hundred seconds of it.
	budget, _ := strconv.Atoi(os.Getenv("GATE_BUDGET_SECONDS"))
	env := gate.Env{
		Root:   os.Getenv("GATE_ROOT"),
		Budget: budget,
		Checks: os.Getenv("GATE_CHECKS_FILE"),
	}
	if env.Root == "" {
		// argv[0] as the stub was invoked by, which `exec -a` preserved. The repository is the parent of
		// the directory holding it — `ai/gate.sh` sits one below the root. Never the process's own cwd:
		// the gate runs from anywhere in the tree, and a root taken from there would scope every unit
		// to a subdirectory.
		env.Root = filepath.Dir(filepath.Dir(absoluteOrAsGiven(os.Args[0])))
	}
	os.Exit(gate.Run(os.Args[1:], env, os.Stdout, os.Stderr))
}

func absoluteOrAsGiven(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
