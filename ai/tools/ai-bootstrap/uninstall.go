package aibootstrap

import (
	"fmt"

	"configs/ai/tools/flavor"
	"configs/ai/tools/shell"
)

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
	run.reportRemainingProjects()
	// The line is worded for either case, because the tier a machine was installed with is recorded
	// nowhere and an uninstall cannot tell whether this machine ever got rtk.
	run.mounting.Say("")
	run.mounting.Say("  rtk is left installed if this machine has it: nothing records whether it was already")
	run.mounting.Say("  there or what else needs it, and a brew formula is shared and unrefcounted.")
	return run.mounting.Report()
}

func (run *invocation) removeInstructions() {
	run.mounting.Say("instructions")
	switch {
	case run.isOwner:
		run.removeOwnerInstructions()
		return
	case shell.PathExists(run.instructionFile) || shell.IsSymlink(run.instructionFile):
		run.mounting.RemoveRegion(run.instructionFile, flavor.RegionOpen, flavor.RegionClose)
	default:
		run.mounting.Say("  ok       " + run.instructionFile + " is not there")
		return
	}
	// The note an older Codex install wrote under the region. No install writes one any more, so this
	// is the only code left that takes one out, and the machines holding one are exactly the machines
	// being uninstalled.
	if run.agent == codexAgent && shell.IsRegularFile(run.instructionFile) {
		run.mounting.RemoveRegion(run.instructionFile, flavor.LegacyRtkRegionOpen, flavor.LegacyRtkRegionClose)
	}
}

// A project that still holds skills mounted from this checkout is left with a dangling set the moment
// the checkout is deleted. This line is the only warning the human gets before deleting it.
func (run *invocation) reportRemainingProjects() {
	projects := run.mounting.LiveInstalls()
	if len(projects) == 0 {
		return
	}
	run.mounting.Say("")
	run.mounting.Say(fmt.Sprintf("  %d project(s) still hold skills mounted from this checkout. "+
		"Uninstall them before deleting it:", len(projects)))
	for _, project := range projects {
		run.mounting.Say("    " + project)
	}
	run.mounting.Say("  Run ai/install-project.sh --agent=claude|codex --uninstall <project> for each " +
		"before removing this checkout.")
}
