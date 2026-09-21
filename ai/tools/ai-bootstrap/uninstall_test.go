package aibootstrap_test

import "testing"

// Uninstall unmounts first, and no link is made on the way. An uninstall reached after the mounting
// step would link every mount and then remove it. On a machine holding none, `--uninstall` would
// build the whole tree and tear it down again. An interrupt between the two leaves the machine
// installed by the command that exists to uninstall it.
func TestUninstallOverAMachineHoldingNothingLinksNothingOnTheWay(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectNotSaid("  linked   ")
	f.expectAbsent(f.home + "/.kk-flavor")
}

func TestUninstallTakesTheBucketTheSkillsAndTheRegion(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectAbsent(f.home + "/.kk-flavor")
	if mounted := f.mounted(f.skillsMount("claude")); len(mounted) > 0 {
		t.Errorf("the uninstall left %v mounted", mounted)
	}
	f.expectFileBody(f.home+"/.claude/CLAUDE.md", "")
}

// The tier a machine was installed with is recorded nowhere. An uninstall that re-applied the
// audience filter would build its removal table for the tier being asked for NOW. --maintainer in and
// plain out would leave exactly the marked skills mounted, reporting ok.
func TestAPlainUninstallRemovesWhatMaintainerInstalled(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	if mounted := f.mounted(f.skillsMount("claude")); len(mounted) > 0 {
		t.Errorf("the uninstall left %v mounted, which is every skill --maintainer had added", mounted)
	}
}

// A project that still holds skills mounted from this checkout is left with a dangling set the moment
// the checkout is deleted. This line is the only warning the human gets before deleting it.
func TestUninstallNamesTheProjectsThatStillNeedThisCheckout(t *testing.T) {
	f := newFixture(t)
	project := f.base + "/a-project"
	f.MkdirAll(project)
	f.Write(f.home+"/.config/kk-flavor/installs", project+"\n")

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectSaid("1 project(s) still hold skills mounted from this checkout")
	f.expectSaid(project)
}

// This case runs at the default tier, which never installs rtk. The tier a machine was installed with
// is recorded nowhere, so the line is worded for either case and has to appear here too.
func TestUninstallSaysWhatItLeavesInstalled(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectSaid("rtk is left installed if this machine has it")
}

// A missing instruction file is no refusal. A machine that never had this client configured is an
// ordinary thing to uninstall from.
func TestUninstallOverAMachineWithNoInstructionFileSaysSo(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectSaid(f.home + "/.claude/CLAUDE.md is not there")
}

// The note an older Codex install wrote under the region. No install writes one any more, so an
// uninstall is the only code left that takes one out.
func TestUninstallRemovesTheOldCodexRtkNoteAsWellAsTheRegion(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=codex"), 0)
	appendTo(t, f.codexHome+"/AGENTS.md",
		"\n<!-- kk-flavor-rtk:begin -->\nRead the RTK notes.\n<!-- kk-flavor-rtk:end -->\n")

	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", "")
}
