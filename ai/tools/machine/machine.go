// Package machine is everything outside this process that an installer touches: which commands this
// machine has, and what they answer. One port covers all of them, because every installer decides on
// the same two facts, and a port per tool would mean a fake per case.
//
// An installer holds this interface and imports no os/exec, so its suite can drive brew's
// installed-first branch, rtk's argument list, and the exit codes project-setup answers with. A
// developer's own setup turns those branches on, and a suite cannot arrange one.
//
// The adapter here has no test of its own. Its body branches on no condition, and faking the commands
// it runs would only assert the fake.
package machine

import (
	"os"
	"os/exec"
)

// Command is one invocation. Several callers set the environment or the directory, and a positional
// signature would put those where a reader cannot tell them apart.
type Command struct {
	Name string
	Args []string
	// Env is added to this process's own environment, each entry `NAME=value`.
	Env []string
	// Directory the command runs in. Empty is this process's own.
	Directory string
	// Loud lets the command's own output through to this process's. Off by default: an installer prints
	// one line per step, and a download's progress bar interleaved with it buries the account of what
	// was linked. Turn it on where the command's output IS the thing the human needs.
	Loud bool
}

// Machine is the world outside this process.
type Machine interface {
	// HasCommand is `command -v`: whether this machine can run it at all.
	HasCommand(name string) bool
	// Run executes the command and answers its exit code. A command that could not start answers 127.
	// That is the shell's own code for it, and a caller then reads one exit code for both.
	Run(command Command) int
}

// New is the adapter a real run acts through.
func New() Machine {
	return live{}
}

type live struct{}

func (live) HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CouldNotStart is what Run answers for a command this machine could not execute — the shell's own
// code for it. A caller telling that apart from the command's own refusal compares against this.
const CouldNotStart = 127

func (live) Run(command Command) int {
	run := exec.Command(command.Name, command.Args...)
	if len(command.Env) > 0 {
		run.Env = append(os.Environ(), command.Env...)
	}
	run.Dir = command.Directory
	if command.Loud {
		run.Stdout, run.Stderr = os.Stdout, os.Stderr
	} else {
		// stderr still goes through: a failing command's own reason is what the caller's refusal sends
		// the reader to, and a refusal naming only an exit code sends them nowhere.
		run.Stderr = os.Stderr
	}
	if err := run.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return CouldNotStart
	}
	return 0
}
