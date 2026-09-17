package projectsetup_test

import "testing"

// The post-checkout entry point restores the links one worktree should have, for whichever clients the
// clone was installed for — read from the state the install left, never from a tier asked for now.
func TestASyncMountsTheTierEachClientWasInstalledWith(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)
	f.expectCode(f.install("--agent=codex"), 0)
	f.RemoveAll(f.skillsMount("claude"))
	f.RemoveAll(f.skillsMount("codex"))

	f.expectCode(f.sync(f.project), 0)

	// Claude was installed with --maintainer, so it gets the marked skill back too.
	f.expectLinkTo(f.skillsMount("claude")+"/kk-ecosystem", f.home+"/.kk-flavor/skills/kk-ecosystem")
	f.expectLinkTo(f.skillsMount("codex")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	f.expectAbsent(f.skillsMount("codex") + "/kk-ecosystem")
	f.expectFileBody(paths.state+"/claude", "true\n")
}

// A checkout is not an install: a hook that rewrote a project's tracked files on every branch switch
// would be unusable.
func TestASyncWritesNoInstructionsAndNoIgnoreRules(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.expectCode(f.install("--agent=claude"), 0)
	instructions := f.read(f.project + "/AGENTS.md")
	f.RemoveAll(f.project + "/.gitignore")

	f.expectCode(f.sync(f.project), 0)

	f.expectFileBody(f.project+"/AGENTS.md", instructions)
	f.expectAbsent(f.project + "/.gitignore")
}

// A clone nothing was installed into has nothing to restore, and that is not a failure: git runs this
// hook on every checkout of every repository the human has set up.
func TestASyncOverAClientlessCloneDoesNothingAndSucceeds(t *testing.T) {
	f := newFixture(t)
	f.asRepository()

	f.expectCode(f.sync(f.project), 0)

	f.expectAbsent(f.skillsMount("claude"))
}

func TestASyncOverSomethingThatIsNotAWorktreeSaysSo(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.sync(f.project), 1)

	f.expectSaid("is not a Git worktree")
}

// A state file holding anything but the two words the install writes is one somebody edited or a write
// that was interrupted. Guessing a tier from it would mount a set nobody chose.
func TestAStateFileNobodyCanReadStopsTheSync(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.expectCode(f.install("--agent=claude"), 0)
	f.rewrite(paths.state+"/claude", "maybe\n")

	f.expectCode(f.sync(f.project), 1)

	f.expectSaid("Invalid project skill setup")
}

// Run from a directory inside the worktree rather than its root — which is what a human doing this by
// hand does, git's own hook running at the root notwithstanding.
func TestASyncFromInsideTheWorktreeStillMountsAtItsRoot(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.expectCode(f.install("--agent=claude"), 0)
	f.RemoveAll(f.skillsMount("claude"))
	inside := f.project + "/somewhere/deep"
	f.MkdirAll(inside)
	f.git.TreeAt(inside, f.project)

	f.expectCode(f.sync(inside), 0)

	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
}

func TestASyncNamingNoWorktreeIsRefused(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.syncWith(), 2)

	f.expectSaid("usage: project-skills.sh --sync <worktree>")
}
