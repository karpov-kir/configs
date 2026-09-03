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
		f.mkdirAll(f.skillsMount())
		f.reports("skill not mounted")
	})

	// The skip itself, which was invisible: without this line a run that checked no mount prints byte
	// for byte what a run that checked every one prints, and check.sh puts `wiring: clean` over both.
	t.Run("but says out loud that it skipped them", func(t *testing.T) {
		f := newRoot(t)
		f.newHomeWithoutFlavorMount()
		f.reports("mounts: skipped")
	})

	// The other half of the contract, and the reason absence of that line can be read as "it ran": on
	// the install the note is a claim about work that did happen.
	t.Run("and does not say so where the scan did run", func(t *testing.T) {
		newInstalledRoot(t).doesNotReport("mounts: skipped")
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
		f.newMountPointingAt("kk-drive", elsewhere)
		return f
	}

	// Without this the case below passes on a run that raised no mount finding at all.
	t.Run("reports a skill mounted somewhere else (control for the case below)", func(t *testing.T) {
		newSkillMountedElsewhere(t).reports("skill mounted elsewhere")
	})

	t.Run("and no control byte reaches the output", func(t *testing.T) {
		newSkillMountedElsewhere(t).doesNotReport("\x1b")
	})

	// The name arm, which the target arm above never reaches: the two ends of the message are
	// sanitised by two separate calls, and a case exercising one says nothing about the other. This
	// arm is not defence in depth — a skill directory name is chosen by the branch under review, and
	// the finding echoes it straight back.
	newSkillNamedWithAControlByte := func(t *testing.T) *fixture {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-\x1b[2Kdrive")
		f.mkdirAll(f.skillsMount())
		return f
	}

	t.Run("reports a skill whose own name carries one (control for the case below)", func(t *testing.T) {
		newSkillNamedWithAControlByte(t).reports("skill not mounted")
	})

	t.Run("and no control byte from that name reaches the output", func(t *testing.T) {
		newSkillNamedWithAControlByte(t).doesNotReport("\x1b")
	})

	// The elsewhere arm of the same name. It is a third message built from a third pair of calls, and
	// the two arms above leave it untouched: the case just above never reaches it, because a skill with
	// no mount at all is `skill not mounted` rather than this one.
	newSkillNamedWithAControlByteMountedElsewhere := func(t *testing.T) *fixture {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-\x1b[2Kdrive")
		elsewhere := f.base + "/elsewhere"
		f.mkdirAll(elsewhere)
		f.newMountPointingAt("kk-\x1b[2Kdrive", elsewhere)
		return f
	}

	t.Run("reports that name mounted somewhere else (control for the case below)", func(t *testing.T) {
		newSkillNamedWithAControlByteMountedElsewhere(t).reports("skill mounted elsewhere")
	})

	t.Run("and no control byte from it reaches the output either", func(t *testing.T) {
		newSkillNamedWithAControlByteMountedElsewhere(t).doesNotReport("\x1b")
	})
}

// Where bootstrap.sh puts a mount, under the same name the checker reaches it by. newHome builds
// $HOME/.claude and stops there, so a case wanting the mount directory makes it itself.
func (f *fixture) skillsMount() string {
	return f.home + "/.claude/skills"
}

// A symlink at the skills mount pointing wherever the case says, including at nothing. Built by hand
// rather than through newMountedSkill, which mounts a skill that is there — the whole subject here is
// the mount that has no skill behind it.
func (f *fixture) newMountPointingAt(name, target string) {
	f.t.Helper()
	f.mkdirAll(f.skillsMount())
	f.symlink(target, f.skillsMount()+"/"+name)
}

// The question the loop over this tree's own skill directories cannot ask. Every case but the last
// builds an installed checkout, because that is the only place either direction is asked at all.
func TestAMountThatOutlivedItsSkillIsReported(t *testing.T) {
	t.Run("reports a mount into this checkout with no skill behind it", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd-gone", f.root+"/skills/idsd-gone")
		f.reports("mount without a skill")
	})

	// Every mount this machine was set up from the README with reads back with a trailing slash, this
	// one included. Nothing here trims it: the scan reads the mount's parent through shell.DirName,
	// which answers dirname(1) and so trims for it. This case is what would catch that going away.
	t.Run("and reports one whose target carries the README-era trailing slash", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd-gone", f.root+"/skills/idsd-gone/")
		f.reports("mount without a skill")
	})

	// The control for both cases above: the ordinary mount every skill in this tree has.
	t.Run("says nothing about a mount whose skill is here", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-drive")
		f.newMountPointingAt("kk-drive", f.root+"/skills/kk-drive")
		f.doesNotReport("mount without a skill")
	})

	// $HOME carries skills from other checkouts — this machine mounts two. Reporting one would be a
	// finding about a tree this checkout does not own.
	t.Run("says nothing about a mount into another checkout", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.mkdirAll(f.base + "/other/skills/atlassian")
		f.newMountPointingAt("atlassian", f.base+"/other/skills/atlassian")
		f.doesNotReport("mount without a skill")
	})

	// The same, with that other checkout deleted. It dangles exactly like the mount this scan is for,
	// and it is still not this tree's to report.
	t.Run("nor about a mount into another checkout that is gone", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountPointingAt("atlassian", f.base+"/other/skills/atlassian")
		f.doesNotReport("mount without a skill")
	})

	// A skill this tree still has, whose own mount resolves nowhere, is the forward half's finding —
	// `skill not mounted`, which the gate cases above already hold it to. Said twice, one broken mount
	// would cost two findings and the reader would go looking for two.
	t.Run("leaves a skill this tree still has to the forward half", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-drive")
		f.newMountPointingAt("kk-drive", f.root+"/skills/nowhere")
		f.doesNotReport("mount without a skill")
	})

	// A mount whose name this tree has no skill for, but which still resolves to a real directory —
	// a skill renamed with its old mount left pointing at the new directory. It loads, so it is not
	// this finding; without the resolving guard the rename would be reported as a deletion.
	t.Run("says nothing about a mount under an old name that still resolves", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountedSkill("kk-drive")
		f.newMountPointingAt("kk-drive-old", f.root+"/skills/kk-drive")
		f.doesNotReport("mount without a skill")
	})

	// A relative mount target, which the README's own install line produces whenever it is run with a
	// relative path. Read against the process's working directory instead of the mount's own, the answer
	// would depend on where the tool was invoked rather than on what is mounted.
	t.Run("reads a relative target against the mount rather than the working directory", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd-gone", "../../../skills/idsd-gone")
		f.reports("mount without a skill")
	})

	// The other side of that fix: resolving relative targets must not turn every one of them into this
	// tree's. This one resolves, from the mount, to a directory that is not this tree's skills/. It sits
	// inside the root rather than in another checkout, because three levels up from the mount is the
	// root. The predicate the scan applies is the directory, not whose tree it belongs to.
	t.Run("and still leaves a relative target outside this tree's skills/ alone", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.mkdirAll(f.root + "/other/skills")
		f.newMountPointingAt("atlassian", "../../../other/skills/atlassian")
		f.doesNotReport("mount without a skill")
	})

	// Rank, not presence. At the default rank this finding sorts under every `dangling link:` line a
	// branch can write, and 40 of those are all one class shows — so a mount the whole scan exists to
	// name would be the one line the flood pushes off the screen.
	t.Run("and ranks it above a flood of link findings rather than inside one", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd-gone", f.root+"/skills/idsd-gone")
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
		f.ranksAbove("mount without a skill", "dangling link: ")
	})

	// And the gate, in the reverse direction: outside the install the mounts are somebody else's, so
	// there is nothing here to be stale about.
	t.Run("and says nothing at all from a checkout $HOME does not mount", func(t *testing.T) {
		f := newRoot(t)
		f.newHomeWithoutFlavorMount()
		f.newMountPointingAt("idsd-gone", f.root+"/skills/idsd-gone")
		f.doesNotReport("mount without a skill")
	})
}

// The reverse finding echoes a mount name read out of $HOME and a link target read off that mount,
// neither of which the reviewed branch chooses — defence in depth, for the reason the file header
// gives for the elsewhere arm's case.
func TestAMountWithoutASkillCarriesNoControlByte(t *testing.T) {
	newMountWithAControlByte := func(t *testing.T) *fixture {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd-gone", f.root+"/skills/idsd\x1b[2Kgone")
		return f
	}

	// Without this the case below passes on a run that raised no reverse finding at all.
	t.Run("reports the mount (control for the case below)", func(t *testing.T) {
		newMountWithAControlByte(t).reports("mount without a skill")
	})

	t.Run("and no control byte reaches the output", func(t *testing.T) {
		newMountWithAControlByte(t).doesNotReport("\x1b")
	})

	// The name arm. bootstrap.sh mounts every directory under skills/ and unmounts none. So a branch
	// that adds a skill whose directory name carries an ESC, gets bootstrapped, then deletes it, leaves
	// a mount at $HOME whose *name* is the branch's own bytes, and this finding is the one that echoes
	// it. The target here is clean, so only the name's sanitiser can keep the ESC out.
	newMountNamedWithAControlByte := func(t *testing.T) *fixture {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd\x1b[2Kgone", f.root+"/skills/idsd-gone")
		return f
	}

	t.Run("reports a mount whose own name carries one (control for the case below)", func(t *testing.T) {
		newMountNamedWithAControlByte(t).reports("mount without a skill")
	})

	t.Run("and no control byte from that name reaches the output", func(t *testing.T) {
		newMountNamedWithAControlByte(t).doesNotReport("\x1b")
	})
}
