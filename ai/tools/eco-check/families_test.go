package ecocheck_test

import "testing"

func TestFamilyDirectionScan(t *testing.T) {
	t.Run("fires on an any-repo skill naming a workflow skill", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-intent")
		f.newMountedSkill("kk-grill")
		f.write(f.root+"/kk-flavor/skills/kk-grill/SKILL.md", "planning a feature into a spec is idsd-intent's\n")
		f.reports(crossFamily)
	})

	t.Run("fires on an any-repo skill naming the workflow's state directory", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-build")
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/skills/kk-drive/SKILL.md", "how to run it may be recorded in .idsd/for-agents/playbook.md\n")
		f.reports(crossFamily)
	})

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
		f.write(f.root+"/kk-flavor/skills/idsd-qualify/SKILL.md", "the pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`, run inline\n")
		f.doesNotReport(crossFamily, unfamiliedSkill)
	})

	t.Run("stays quiet on an any-repo skill naming its own family", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-tighten")
		f.newMountedSkill("kk-humanize")
		f.write(f.root+"/kk-flavor/skills/kk-tighten/SKILL.md", "outward text is kk-humanize's\n")
		f.doesNotReport(crossFamily)
	})

	// The router exception ecosystem.md grants. Without it the one skill whose job is routing between
	// the families cannot name the lanes it routes to, and the check would demand a change that breaks
	// the thing it is checking.
	t.Run("stays quiet on the router once it claims its exception", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-ship")
		f.newMountedSkill("kk-foreman")
		f.write(f.root+"/kk-flavor/skills/kk-foreman/SKILL.md",
			"routing between the families is this file's job, per ecosystem.md → **Family direction**.\nintent-shaped work goes to idsd-ship, under .idsd/\n")
		f.doesNotReport(crossFamily)
	})

	t.Run("fires on the router naming the workflow family with no claim", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-ship")
		f.newMountedSkill("kk-foreman")
		f.write(f.root+"/kk-flavor/skills/kk-foreman/SKILL.md", "intent-shaped work goes to idsd-ship, under .idsd/\n")
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

func TestFamilyDirectionAcrossTheWorkerTree(t *testing.T) {
	t.Run("fires on an any-repo worker naming a workflow skill", func(t *testing.T) {
		f := newWorkerLaneTree(t, "escalate to idsd-ship when the work is intent-shaped")
		f.newMountedSkill("idsd-ship")
		f.reports(crossFamily)
	})

	t.Run("fires on an any-repo worker naming the workflow's state directory", func(t *testing.T) {
		f := newWorkerLaneTree(t, "the decisions may be recorded in .idsd/for-agents/decisions.md")
		f.reports(crossFamily)
	})

	// The permitted direction, told apart by path since a worker's name carries no family prefix.
	t.Run("stays quiet on a workflow-owned worker naming that family", func(t *testing.T) {
		f := newWorkerLaneTree(t, workerBrief)
		f.mkdirAll(f.root + "/kk-flavor/workers/idsd")
		f.write(f.root+"/kk-flavor/workers/idsd/audit.md", "audit the intent under .idsd/ for idsd-ship\n")
		f.doesNotReport(crossFamily)
	})

	t.Run("stays quiet on an any-repo worker naming its own family", func(t *testing.T) {
		f := newWorkerLaneTree(t, workerNamingItsLane)
		f.doesNotReport(crossFamily)
	})

	// The router exception belongs to one skill with a door, and a worker has nothing to route — so
	// copying the claim into a worker prompt must not buy the silence it buys in that skill's file.
	t.Run("fires on a worker that copies the router's claim", func(t *testing.T) {
		f := newWorkerLaneTree(t, "per ecosystem.md → **Family direction**, hand intent work to idsd-ship")
		f.newMountedSkill("idsd-ship")
		f.reports(crossFamily)
	})
}
