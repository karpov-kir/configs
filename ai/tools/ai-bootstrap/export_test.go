package aibootstrap

import "configs/ai/tools/installer"

// Perform is Run with the machinery's own record of it, so a case can read back every write the
// containment bound turned away. A suite that only read the exit code could pass having been stopped.
func Perform(options Options) (*installer.Run, int) {
	return perform(options)
}

// Usage is the line a refused invocation prints, so the case holding it against the parser reads one
// spelling rather than a copy.
func Usage() string {
	return usage()
}
