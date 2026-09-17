package aibootstrap_test

import (
	"slices"
	"testing"
)

// The table, driven end to end: the shared bucket, and one mount per skill the tier takes. Discovery
// rather than a list, so a skill added tomorrow is mounted without anyone editing the installer.
func TestAFreshMachineGetsTheBucketAndEverySkillTheTierTakes(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectSaid("ai bootstrap: ok")
	f.expectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
	for _, name := range publicSkills {
		f.expectLinkTo(f.skillsMount("claude")+"/"+name, f.repo+"/kk-flavor/skills/"+name)
	}
	// The instruction region goes into the file itself, never a mount of this checkout's own: the
	// default tier shares one file with whatever else the human keeps in it.
	f.expectFileContains(f.home+"/.claude/CLAUDE.md", "kk-flavor:begin")
	f.expectNotSymlink(f.home + "/.claude/CLAUDE.md")
}

// Some skills exist only to maintain this instruction tree and do nothing for a machine that merely
// uses it. Each costs every session context through its `description:`, loaded whether or not the
// skill is ever invoked, so the default leaves them out and the bigger set is asked for by name.
func TestTheDefaultTierLeavesTheMarkedSkillsOutAndSaysSo(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	for _, name := range maintainerSkills {
		f.expectAbsent(f.skillsMount("claude") + "/" + name)
	}
	// Said out loud: a flag that quietly leaves skills out is indistinguishable from a discovery loop
	// that stopped finding them.
	f.expectSaid("maintainer-only skill(s): kk-ecosystem")
}

func TestMaintainerMountsAMarkedSkillLikeAnyOther(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	for _, name := range maintainerSkills {
		f.expectLinkTo(f.skillsMount("claude")+"/"+name, f.repo+"/kk-flavor/skills/"+name)
	}
}

// The zero case is the same claim about work that did not happen. Without it a tree carrying no marked
// skill and a tier that excluded three read the same to a human: silence.
func TestATreeWithNoMarkedSkillSaysItExcludedNothing(t *testing.T) {
	f := newFixture(t)
	for _, name := range maintainerSkills {
		f.RemoveAll(f.repo + "/kk-flavor/skills/" + name)
	}

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectSaid("no maintainer-only skill was there to exclude")
}

// Discovery's two vacuity cases, and they send a reader to different places: a skills directory with
// nothing in it is a broken checkout, while a flag that excluded every skill it found is a flag doing
// exactly what it says. The exit code is the same for both, so the wording is the only thing telling
// them apart.
func TestATreeWhoseEverySkillIsMarkedSaysTheTierExcludedThem(t *testing.T) {
	f := newFixture(t)
	for _, name := range publicSkills {
		f.RemoveAll(f.repo + "/kk-flavor/skills/" + name)
	}

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("excluded all 1")
	f.expectNotSaid("no skill directories under")
}

func TestATreeWithNoSkillAtAllRefusesRatherThanMountingNothing(t *testing.T) {
	f := newFixture(t)
	f.RemoveAll(f.repo + "/kk-flavor/skills")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("no skill directories under")
	f.expectNotSaid("excluded all")
}

// The control, and the load-bearing half of both cases above: the same tree with --maintainer mounts
// the marked skills, so neither refusal can be passing over a fixture that never had a skill to mount.
func TestTheSameTreeWithMaintainerMountsWhatTheDefaultExcluded(t *testing.T) {
	f := newFixture(t)
	for _, name := range publicSkills {
		f.RemoveAll(f.repo + "/kk-flavor/skills/" + name)
	}

	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.expectLinkTo(f.skillsMount("claude")+"/kk-ecosystem", f.repo+"/kk-flavor/skills/kk-ecosystem")
}

// A misspelled marker answers "not marked" exactly as a skill that declared nothing does, and those
// mean opposite things: the misspelling installs for everyone while whoever typed it believes they
// marked it, on a machine where nothing looks wrong.
func TestAnAudienceNoReaderKnowsIsReportedRatherThanInstalledQuietly(t *testing.T) {
	f := newFixture(t)
	f.RemoveAll(f.repo + "/kk-flavor/skills/kk-build/SKILL.md")
	f.Write(f.repo+"/kk-flavor/skills/kk-build/SKILL.md",
		"---\nname: kk-build\ndescription: a skill\naudience: maintainr\n---\n")

	f.expectCode(f.install("--agent=claude"), 1)

	// Echoed back as it was typed, so a reader hunting `maintainr` finds what they wrote.
	f.expectSaid("maintainr")
	f.expectSaid("audience: maintainer")
	// Mounted anyway, so the tree behaves as it does today and the non-zero exit is what carries the
	// news.
	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
}

// The control. Without it the case above is equally satisfied by an installer that refuses every skill
// it reads, and the suite would be measuring nothing.
func TestTheSameTreeWithTheMarkerSpelledRightRefusesNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.expectNotSaid("no reader knows")
}

// The whole reason the guard sits above linking rather than inside it: run from a scratch clone, every
// mount would be repointed at the clone, and deleting the clone afterwards leaves the human with no
// skills and no instructions. What only this side can cover is that a bulk mount takes part in the
// count the refusal reports — a reader told their one config would move, and not that every skill
// moves with it, has not been told the scale of what the run would do.
func TestASecondCheckoutIsRefusedWithTheSkillsInTheCount(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/ai"
	f.newCheckout(stranger)
	f.expectCode(f.runFrom(stranger, append(skipSteps, "--agent=claude")...), 0)

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("3 mounts (1 configs and 2 skills)")
	f.expectSaid(stranger)
	// The load-bearing half. A guard that refuses after repointing has still moved the machine, so the
	// mounts are read back rather than the message being taken at its word.
	f.expectLinkTo(f.home+"/.kk-flavor", stranger+"/kk-flavor")
	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", stranger+"/kk-flavor/skills/kk-build")
}

func TestRelocateMovesTheWholeSetToThisCheckout(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/ai"
	f.newCheckout(stranger)
	f.expectCode(f.runFrom(stranger, append(skipSteps, "--agent=claude")...), 0)

	f.expectCode(f.install("--agent=claude", "--relocate"), 0)

	f.expectSaid("moving 3 mount(s)")
	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
}

// The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
// it is the run that changed their machine.
func TestADryRunWritesNothingAtAll(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.expectSaid("would link")
	f.expectAbsent(f.home + "/.kk-flavor")
	f.expectAbsent(f.home + "/.claude")
}

// A second run over a finished machine reports ok throughout and writes nothing. It is the property
// that makes this safe to run on a working machine, and the only evidence for it is a second run over
// the first run's output.
func TestASecondRunOverAFinishedMachineRelinksNothing(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectSaid("  ok       " + f.home + "/.kk-flavor")
	f.expectNotSaid("linked   " + f.home + "/.kk-flavor")
	if mounted := f.mounted(f.skillsMount("claude")); !slices.Equal(mounted, publicSkills) {
		t.Errorf("the second run left %v mounted, wanted %v", mounted, publicSkills)
	}
}
