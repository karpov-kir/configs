package ecocheck_test

import (
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

// A dropped flag leaves every path resolving, so no other check here sees it. The defect this catches
// cost ten days: a table an orchestrator reads named a scanner without the flag that selects the mode
// the lane requires, and the mode it ran instead reports something else entirely.
func TestInvocationSpellingScan(t *testing.T) {
	t.Run("fires on a dispatch naming fewer flags than the owning lane", func(t *testing.T) {
		f := newTwoLaneTree(t)
		f.write(f.root+"/kk-flavor/skills/kk-humanize/SKILL.md",
			"run `~/.kk-flavor/skills/kk-humanize/scripts/voice-check.sh --bar` over the change\n")
		f.write(f.root+"/kk-flavor/skills/kk-drive/SKILL.md",
			"the scanner is `~/.kk-flavor/skills/kk-humanize/scripts/voice-check.sh`\n")
		f.reports(ecocheck.DispatchNamesFewerFlags)
	})

	t.Run("stays quiet when the dispatch spells the lane's own flags", func(t *testing.T) {
		f := newTwoLaneTree(t)
		f.write(f.root+"/kk-flavor/skills/kk-humanize/SKILL.md",
			"run `~/.kk-flavor/skills/kk-humanize/scripts/voice-check.sh --bar` over the change\n")
		f.write(f.root+"/kk-flavor/skills/kk-drive/SKILL.md",
			"the scanner is `~/.kk-flavor/skills/kk-humanize/scripts/voice-check.sh --bar`\n")
		f.doesNotReport(ecocheck.DispatchNamesFewerFlags)
	})

	// A record row naming a script owes it no flags; only a path-form site is a dispatch.
	t.Run("stays quiet on prose that merely names the script", func(t *testing.T) {
		f := newTwoLaneTree(t)
		f.write(f.root+"/kk-flavor/skills/kk-humanize/SKILL.md",
			"run `~/.kk-flavor/skills/kk-humanize/scripts/voice-check.sh --bar` over the change\n")
		f.write(f.root+"/kk-flavor/skills/kk-drive/SKILL.md",
			"open: voice-check.sh's baseline scan has no case for this\n")
		f.doesNotReport(ecocheck.DispatchNamesFewerFlags)
	})
}
