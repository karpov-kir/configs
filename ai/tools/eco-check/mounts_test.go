package ecocheck_test

// Whether this checkout's own skills are reachable at $HOME — asked only of the checkout $HOME
// actually mounts. Anywhere else the mounts resolve to somebody else's tree, and the answer would be
// about that tree rather than this one.

import "testing"

// An installed checkout: $HOME/.kk-flavor resolves to this tree's kk-flavor, which is what makes the
// question below apply at all. Every case here builds on it, because a fixture that is not installed
// reports nothing by design and could not tell a working scan from a deleted one.
func newInstalledRoot(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newHome()
	return f
}

// The scan is gated on Root.IsInstalled, so both directions need a case: a clone must stay silent, and
// the install must still be checked exactly as before. Silence alone is what a deleted scan looks
// like, so neither case means anything without the other.
func TestTheMountScanAsksOnlyAboutTheInstalledCheckout(t *testing.T) {
	// A clone, a PR review's worktree, or a CI runner: $HOME mounts nothing of this tree. Every mount
	// finding here would be restating that, and it made the wiring gate red on every CI run for a
	// reason having nothing to do with what it gates.
	t.Run("says nothing about the mounts of a checkout $HOME does not mount", func(t *testing.T) {
		f := newRoot(t)
		f.newHomeWithoutFlavorMount()
		f.newMountedSkill("kk-drive")
		f.doesNotReport("not mounted")
	})

	// The control, and the half that keeps the gate honest: the same unmounted skill on an installed
	// checkout is still a finding. Without it, gating everything off would pass the case above.
	t.Run("while an installed checkout with an unmounted skill still reports", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-drive")
		f.mkdirAll(f.home + "/.claude/skills")
		f.reports("skill not mounted")
	})

	// And the skills mount missing altogether, which is the other arm. newHome builds $HOME/.claude
	// but no skills/ under it.
	t.Run("and reports an installed checkout whose skills mount is missing", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-drive")
		f.reports("skills not mounted")
	})
}

// The mount findings echo a path resolved through $HOME, which the reviewed branch does not choose —
// so this is defence in depth rather than a branch-reachable hole. It is still a mount message built
// from text nothing in this checker wrote, and an ESC sequence in it erases the finding printed above
// it in the transcript a reviewing agent drafts its comments from.
//
// Written against the skill arm rather than the flavor one: past the IsInstalled gate the flavor mount
// resolves to this tree by definition, so a skill mounted somewhere else is the message a real install
// can still produce.
func TestMountFindingCarriesNoControlByte(t *testing.T) {
	// The directory sits outside the root on purpose: inside it, the walk would name it too, and the
	// case would then pass on a sanitiser somewhere else entirely.
	newSkillMountedElsewhere := func(t *testing.T) *fixture {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-drive")
		elsewhere := f.base + "/elsewhere\x1b[2Kskill"
		f.mkdirAll(elsewhere)
		f.mkdirAll(f.home + "/.claude/skills")
		f.symlink(elsewhere, f.home+"/.claude/skills/kk-drive")
		return f
	}

	// Without this the case below passes on a run that raised no mount finding at all.
	t.Run("reports a skill mounted somewhere else (control for the case below)", func(t *testing.T) {
		newSkillMountedElsewhere(t).reports("skill mounted elsewhere")
	})

	t.Run("and no control byte reaches the output", func(t *testing.T) {
		newSkillMountedElsewhere(t).doesNotReport("\x1b")
	})
}
