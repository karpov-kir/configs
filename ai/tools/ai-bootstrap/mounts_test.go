package aibootstrap_test

import (
	"slices"
	"testing"
)

// The table, driven end to end: the shared bucket, and one mount per skill. The set is discovered,
// so a skill added tomorrow is mounted without anyone editing the installer. A skill marked
// `audience: maintainer` is mounted like any other.
func TestAFreshMachineGetsTheBucketAndEverySkill(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectSaid("ai bootstrap: ok")
	f.ExpectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
	for _, name := range allSkills() {
		f.ExpectLinkTo(f.skillsMount("claude")+"/"+name, f.repo+"/kk-flavor/skills/"+name)
	}
	// The owner copy goes into the file itself, and no mount of this checkout's stands in for it.
	f.ExpectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	f.ExpectNotSymlink(f.home + "/.claude/CLAUDE.md")
}

func TestATreeWithNoSkillAtAllRefusesRatherThanMountingNothing(t *testing.T) {
	f := newFixture(t)
	f.RemoveAll(f.repo + "/kk-flavor/skills")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("no skill directories under")
}

// The guard runs before any linking. A run from a scratch clone would repoint every mount at the
// clone, and a later delete of that clone takes the human's skills and instructions with it. What
// only this side covers is that a bulk mount takes part in the count the refusal reports. A refusal
// naming one config, with the skills left out of the count, understates what the run would do.
func TestASecondCheckoutIsRefusedWithTheSkillsInTheCount(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/ai"
	f.newCheckout(stranger)
	f.ExpectCode(f.runFrom(stranger, append(skipSteps, "--agent=claude")...), 0)

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("5 mounts (2 configs and 3 skills)")
	f.ExpectSaid(stranger)
	// The half that matters. A guard that refuses after repointing has still moved the machine, so the
	// mounts are read back and the message alone is never trusted.
	f.ExpectLinkTo(f.home+"/.kk-flavor", stranger+"/kk-flavor")
	f.ExpectLinkTo(f.skillsMount("claude")+"/kk-build", stranger+"/kk-flavor/skills/kk-build")
}

func TestRelocateMovesTheWholeSetToThisCheckout(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/ai"
	f.newCheckout(stranger)
	f.ExpectCode(f.runFrom(stranger, append(skipSteps, "--agent=claude")...), 0)

	f.ExpectCode(f.install("--agent=claude", "--relocate"), 0)

	f.ExpectSaid("moving 5 mount(s)")
	f.ExpectLinkTo(f.skillsMount("claude")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
}

// A dry run that wrote anything would be worse than having no flag at all. Someone checks with it,
// and the check is the run that changes their machine.
func TestADryRunWritesNothingAtAll(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.ExpectSaid("would link")
	f.ExpectAbsent(f.home + "/.kk-flavor")
	f.ExpectAbsent(f.home + "/.claude")
}

// A second run over a finished machine reports ok throughout and writes no file. It is the property
// that makes this safe to run on a working machine, and the only evidence for it is a second run over
// the first run's output.
func TestASecondRunOverAFinishedMachineRelinksNothing(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectSaid("  ok       " + f.home + "/.kk-flavor")
	f.ExpectNotSaid("linked   " + f.home + "/.kk-flavor")
	if mounted := f.Mounted(f.skillsMount("claude")); !slices.Equal(mounted, allSkills()) {
		t.Errorf("the second run left %v mounted, wanted %v", mounted, allSkills())
	}
}

// Claude reads defined agents from ~/.claude/agents. Codex has no such directory, and a Codex run
// mounts none.
func TestClaudeGetsTheFlavorsAgentsAndCodexNone(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.ExpectLinkTo(f.home+"/.claude/agents/ledger-worker.md", f.repo+"/kk-flavor/agents/ledger-worker.md")

	g := newFixture(t)
	g.ExpectCode(g.install("--agent=codex"), 0)
	g.ExpectAbsent(g.home + "/.claude/agents/ledger-worker.md")
}
