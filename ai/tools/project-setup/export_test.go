package projectsetup

import "kk-flavor/tools/installer"

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

// HookBody is what a repository's post-checkout hook holds. Exported for the cases that assert a hook
// this installer did not write is left alone: they need the exact bytes, and a copy written into the
// suite would agree with itself rather than with what is installed.
func HookBody() string {
	return hookBody()
}

// SupersededHookBody is the hook this installer wrote before HookBody learned to name the checkout
// behind ~/.kk-flavor.
func SupersededHookBody() string {
	return supersededHookBody
}
