package envbootstrap

import "kk-flavor/tools/installer"

// Perform is Run with the machinery's own record of it, so a case can read back every write the
// containment bound turned away. A suite that only read the exit code could pass having been stopped.
func Perform(options Options) (*installer.Run, int) {
	return perform(options)
}
