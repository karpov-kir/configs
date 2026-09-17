package aibootstrap_test

import "testing"

// Uninstall unmounts first and links nothing on the way. Reached after the mounting step it would link
// every mount and then remove it, so on a machine holding none `--uninstall` would build the whole
// tree and tear it down again — and an interrupt between the two leaves the machine installed by the
// command that exists to uninstall it.
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

// The tier a machine was installed with is written down nowhere, so an uninstall that re-applied the
// audience filter would build its removal table for the tier being asked for NOW: --maintainer in and
// plain out would leave exactly the marked skills mounted, reporting ok.
func TestAPlainUninstallRemovesWhatMaintainerInstalled(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	if mounted := f.mounted(f.skillsMount("claude")); len(mounted) > 0 {
		t.Errorf("the uninstall left %v mounted, which is every skill --maintainer had added", mounted)
	}
}

// A project that still holds skills mounted from this checkout would be left with a dangling set the
// moment the checkout is deleted, and nothing but this line tells the human before they delete it.
func TestUninstallNamesTheProjectsThatStillNeedThisCheckout(t *testing.T) {
	f := newFixture(t)
	project := f.base + "/a-project"
	f.MkdirAll(project)
	f.Write(f.home+"/.config/kk-flavor/installs", project+"\n")

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectSaid("1 project(s) still hold skills mounted from this checkout")
	f.expectSaid(project)
}

// Driven at the default tier, which never installs rtk: the tier a machine was installed with is
// written down nowhere, so the line is hedged rather than conditioned on it and has to appear here too.
func TestUninstallSaysWhatItLeavesInstalled(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectSaid("rtk is left installed if this machine has it")
}

// An instruction file that was never there is not a refusal: a machine that never had this client
// configured is an ordinary thing to uninstall from.
func TestUninstallOverAMachineWithNoInstructionFileSaysSo(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectSaid(f.home + "/.claude/CLAUDE.md is not there")
}

// The note an older Codex install wrote under the region. Nothing puts one there any more, so an
// uninstall is the only thing that would ever take it out.
func TestUninstallRemovesTheOldCodexRtkNoteAsWellAsTheRegion(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=codex"), 0)
	appendTo(t, f.codexHome+"/AGENTS.md",
		"\n<!-- kk-flavor-rtk:begin -->\nRead the RTK notes.\n<!-- kk-flavor-rtk:end -->\n")

	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", "")
}
