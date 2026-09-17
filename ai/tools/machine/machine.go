// Package machine is everything outside this process that an installer touches: which commands this
// machine has, and what they answer. One port for all of them rather than one per tool, because what
// an installer decides on is always the same two facts, and a fake per command would be a fake per
// case.
//
// The installers hold this and never os/exec. That is what lets their suites drive brew's
// installed-first branch, rtk's argument list, and the exit codes the tools installer and the gate
// answer with — none of which this machine could be asked to produce, and all of which are branches a
// human's setup turns on.
//
// The adapter below is the one part with no test of its own: nothing in it branches, and faking the
// commands it runs would only assert the fake.
package machine

import (
	"os"
	"os/exec"
)

// Command is one invocation. A struct rather than a variadic call, because every caller sets at least
// the name and the arguments and several set the environment, and a positional signature would put
// the rarely-used ones where a reader cannot tell them apart.
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
	// Run executes the command and answers its exit code. A command that could not start at all answers
	// 127, which is the shell's own code for it, so a caller reading an exit code never has to also
	// handle an error that means the same thing.
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
