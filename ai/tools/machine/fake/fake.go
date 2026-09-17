// Package fake is a working machine the installers' suites drive instead of this one. Its own package
// rather than a file beside the adapter, so nothing a release ships carries it.
//
// Working, not canned: a case hands it a handler per command, and the handler decides both the exit
// code and what the command did to the fixture — brew install marking a formula present, rtk writing
// the file the next step reads. A fake that only returned codes could not express the installed-first
// branch at all, which is the branch these installers spend most of their lines on.
package fake

import (
	"strings"

	"configs/ai/tools/machine"
)

// Machine records every command it was asked to run and answers each one the way its case said to.
type Machine struct {
	// Present is the commands this machine has. A name with no entry is one `command -v` cannot find,
	// which is the case every "X is not installed" refusal is driven by.
	Present map[string]bool
	// Answer decides what one command does, keyed by its name. A name with no handler runs and exits 0,
	// which is what most steps want and what keeps a case to the one command it is about.
	Answer map[string]func(command machine.Command) int
	// Calls is every command in the order it was asked for, so a case can assert on an argument list a
	// tool's own behaviour depends on — rtk's init flags are the one that has been wrong.
	Calls []machine.Command
}

func New() *Machine {
	return &Machine{Present: map[string]bool{}, Answer: map[string]func(machine.Command) int{}}
}

// Add declares a command this machine has, with no behaviour of its own.
func (m *Machine) Add(names ...string) *Machine {
	for _, name := range names {
		m.Present[name] = true
	}
	return m
}

// Answering declares a command and what it does. Two steps in one, because a handler for a command
// this machine does not have is a case whose assertion can never be reached.
func (m *Machine) Answering(name string, answer func(command machine.Command) int) *Machine {
	m.Present[name] = true
	m.Answer[name] = answer
	return m
}

func (m *Machine) HasCommand(name string) bool {
	return m.Present[name]
}

func (m *Machine) Run(command machine.Command) int {
	m.Calls = append(m.Calls, command)
	if !m.Present[command.Name] {
		return machine.CouldNotStart
	}
	if answer, has := m.Answer[command.Name]; has {
		return answer(command)
	}
	return 0
}

// Ran is whether a command with exactly this name and argument list was asked for. The whole list,
// because a flag dropped from the middle of one is the defect these assertions exist to catch.
func (m *Machine) Ran(name string, args ...string) bool {
	for _, call := range m.Calls {
		if call.Name == name && strings.Join(call.Args, "\x00") == strings.Join(args, "\x00") {
			return true
		}
	}
	return false
}

// RanAny is whether the command was asked for at all, whatever its arguments — for a case whose
// subject is that a step was reached, or that it was not.
func (m *Machine) RanAny(name string) bool {
	for _, call := range m.Calls {
		if call.Name == name {
			return true
		}
	}
	return false
}

// Spelled is every invocation of one command, each as the human would have typed it. What a failure
// message quotes, so a case that expected different arguments says what it got.
func (m *Machine) Spelled(name string) []string {
	var spelled []string
	for _, call := range m.Calls {
		if call.Name == name {
			spelled = append(spelled, strings.Join(append([]string{call.Name}, call.Args...), " "))
		}
	}
	return spelled
}
