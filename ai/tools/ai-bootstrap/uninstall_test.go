package aibootstrap_test

import "testing"

// Uninstall unmounts first, and no link is made on the way. An uninstall reached after the mounting
// step would link every mount and then remove it. On a machine holding none, `--uninstall` would
// build the whole tree and tear it down again. An interrupt between the two leaves the machine
// installed by the command that exists to uninstall it.
func TestUninstallOverAMachineHoldingNothingLinksNothingOnTheWay(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectNotSaid("  linked   ")
	f.ExpectAbsent(f.home + "/.kk-flavor")
}

func TestUninstallTakesTheBucketTheSkillsAndTheOwnerCopy(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectAbsent(f.home + "/.kk-flavor")
	if mounted := f.Mounted(f.skillsMount("claude")); len(mounted) > 0 {
		t.Errorf("the uninstall left %v mounted", mounted)
	}
	f.ExpectAbsent(f.home + "/.claude/CLAUDE.md")
}

// Nothing records whether rtk was on this machine before the install, so the line is worded for
// either case and appears on a machine that never got rtk too.
func TestUninstallSaysWhatItLeavesInstalled(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectSaid("rtk is left installed if this machine has it")
}

// A missing instruction file is no refusal. A machine that never had this client configured is an
// ordinary thing to uninstall from.
func TestUninstallOverAMachineWithNoInstructionFileSaysSo(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectSaid(f.home + "/.claude/CLAUDE.md is not there")
}
