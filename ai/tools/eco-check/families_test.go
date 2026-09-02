package ecocheck_test

// The family-direction scan: an any-repo skill never names the workflow family, or the directory that
// family keeps its state in. The rule's home is ecosystem.md → **Family direction**.
//
// The two directions are asserted separately, because the whole rule IS the direction: a scan that
// fired both ways would flag a workflow skill citing an any-repo one, which is the composition the
// suite is built on and the one thing this must stay quiet about.

import "testing"

func TestFamilyDirectionScan(t *testing.T) {
	t.Run("fires on an any-repo skill naming a workflow skill", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-intent")
		f.newMountedSkill("kk-grill")
		f.write(f.root+"/skills/kk-grill/SKILL.md", "planning a feature into a spec is idsd-intent's\n")
		f.reports(crossFamily)
	})

	// The directory is worth its own case: a skill can carry the workflow's on-disk knowledge without
	// naming any of its skills, and that steers its reader exactly as hard.
	t.Run("fires on an any-repo skill naming the workflow's state directory", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-build")
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/skills/kk-drive/SKILL.md", "how to run it may be recorded in .idsd/playbook.md\n")
		f.reports(crossFamily)
	})

	// A script's comment steers the agent reading it the way the skill file does, and it is read on the
	// same trigger. Scanning SKILL.md alone would leave every scripts/ directory unchecked.
	t.Run("fires on a script under an any-repo skill, not only its SKILL.md", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-ship")
		f.newMountedSkill("kk-qualify")
		f.newScript("kk-qualify/scripts/x.sh", "#!/usr/bin/env bash\n# the report is idsd-qualify's")
		f.reports(crossFamily)
	})

	// The permitted direction, and the reason this scan cannot be symmetric: the workflow family is
	// built by layering on the any-repo one, so every workflow skill cites one.
	//
	// Both findings are asserted absent, and the second is what keeps the first honest: with the
	// direction guard removed, the workflow skill falls through to the neither-family branch and is
	// reported by THAT instead, so a case watching only the cross-family finding stays green while
	// observing nothing.
	t.Run("stays quiet on a workflow skill naming an any-repo skill", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-qualify")
		f.newMountedSkill("idsd-qualify")
		f.write(f.root+"/skills/idsd-qualify/SKILL.md", "the pass is `~/.claude/skills/kk-qualify/SKILL.md`, run inline\n")
		f.doesNotReport(crossFamily, unfamiliedSkill)
	})

	// An any-repo skill talking about its own family is the ordinary case, and a scan that flagged it
	// would fire on almost every file in the tree.
	t.Run("stays quiet on an any-repo skill naming its own family", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-tighten")
		f.newMountedSkill("kk-humanize")
		f.write(f.root+"/skills/kk-tighten/SKILL.md", "outward text is kk-humanize's\n")
		f.doesNotReport(crossFamily)
	})

	// The router exception ecosystem.md grants. Without it the one skill whose job is routing between
	// the families cannot name the lanes it routes to, and the check would demand a change that breaks
	// the thing it is checking.
	t.Run("stays quiet on the router once it claims its exception", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-ship")
		f.newMountedSkill("kk-foreman")
		f.write(f.root+"/skills/kk-foreman/SKILL.md",
			"routing between the families is this file's job, per ecosystem.md → **Family direction**.\nintent-shaped work goes to idsd-ship, under .idsd/\n")
		f.doesNotReport(crossFamily)
	})

	// The exception is keyed on the skill's NAME, so without this the router has a blanket pass: it could
	// stop explaining why it alone names the other family and nothing would fail. ecosystem.md asks for
	// the claim to be in the file, so the file is what gets checked.
	t.Run("fires on the router naming the workflow family with no claim", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-ship")
		f.newMountedSkill("kk-foreman")
		f.write(f.root+"/skills/kk-foreman/SKILL.md", "intent-shaped work goes to idsd-ship, under .idsd/\n")
		f.reports(unclaimedRouter)
	})

	// The guard that keeps the prefix key honest. laneNames deliberately does not trust the naming
	// convention, since nothing checked it. This finding is what makes trusting it here legitimate: a
	// third family fails loudly instead of going unscanned in silence.
	t.Run("fires on a mounted skill in neither declared family", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("zz-stranger")
		f.reports(unfamiliedSkill)
	})
}
