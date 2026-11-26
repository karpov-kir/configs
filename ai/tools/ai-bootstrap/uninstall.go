package aibootstrap

import "configs/ai/tools/shell"

// Uninstall, over the same table an install declares. A second program deriving its own removal list
// ends up out of step with what was installed. It goes out of step in the direction a human misses:
// things left behind under a report that says ok.
func (run *invocation) uninstall() int {
	// The unmount comes first, and no link is made on the way. An uninstall reached after the mounting
	// step would link every mount and then remove it. On a machine holding none, `--uninstall` would
	// build the whole tree and tear it down again. An interrupt between the two leaves the machine
	// installed by the command that exists to uninstall it.
	run.mounting.Unmount()
	run.removeInstructions()
	run.removeAgentGuard()
	// The line is worded for either case, because nothing records whether rtk was there before this
	// install.
	run.mounting.Say("")
	run.mounting.Say("  rtk is left installed if this machine has it: nothing records whether it was already")
	run.mounting.Say("  there or what else needs it, and a brew formula is shared and unrefcounted.")
	return run.mounting.Report()
}

func (run *invocation) removeInstructions() {
	run.mounting.Say("instructions")
	if !shell.PathExists(run.instructionFile) && !shell.IsSymlink(run.instructionFile) {
		run.mounting.Say("  ok       " + run.instructionFile + " is not there")
	}
	run.removeOwnerInstructions()
}
