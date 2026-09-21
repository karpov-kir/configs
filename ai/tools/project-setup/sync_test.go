package projectsetup_test

import "testing"

// The post-checkout entry point restores the links one worktree should have, for whichever clients
// the clone was installed for. The tier comes from the state the install left, and a tier asked for
// now is ignored.
func TestASyncMountsTheTierEachClientWasInstalledWith(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.ExpectCode(f.install("--agent=claude", "--maintainer"), 0)
	f.ExpectCode(f.install("--agent=codex"), 0)
	f.RemoveAll(f.skillsMount("claude"))
	f.RemoveAll(f.skillsMount("codex"))

	f.ExpectCode(f.sync(f.project), 0)

	// Claude was installed with --maintainer, so it gets the marked skill back too.
	f.ExpectLinkTo(f.skillsMount("claude")+"/kk-ecosystem", f.home+"/.kk-flavor/skills/kk-ecosystem")
	f.ExpectLinkTo(f.skillsMount("codex")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	f.ExpectAbsent(f.skillsMount("codex") + "/kk-ecosystem")
	f.ExpectFileBody(paths.state+"/claude", "true\n")
}

// A checkout is not an install: a hook that rewrote a project's tracked files on every branch switch
// would be unusable.
func TestASyncWritesNoInstructionsAndNoIgnoreRules(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.ExpectCode(f.install("--agent=claude"), 0)
	instructions := f.Read(f.project + "/AGENTS.md")
	f.RemoveAll(f.project + "/.gitignore")

	f.ExpectCode(f.sync(f.project), 0)

	f.ExpectFileBody(f.project+"/AGENTS.md", instructions)
	f.ExpectAbsent(f.project + "/.gitignore")
}

// A clone this was never installed into has none to restore. That is a success: git runs this hook on
// every checkout of every repository the human has set up.
func TestASyncOverAClientlessCloneDoesNothingAndSucceeds(t *testing.T) {
	f := newFixture(t)
	f.asRepository()

	f.ExpectCode(f.sync(f.project), 0)

	f.ExpectAbsent(f.skillsMount("claude"))
}

func TestASyncOverSomethingThatIsNotAWorktreeSaysSo(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.sync(f.project), 1)

	f.ExpectSaid("is not a Git worktree")
}

// A state file holding anything but the two words the install writes is one somebody edited, or a
// write that was cut short. A tier guessed from it would mount a set the human never chose.
func TestAStateFileNobodyCanReadStopsTheSync(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.Write(paths.state+"/claude", "maybe\n")

	f.ExpectCode(f.sync(f.project), 1)

	f.ExpectSaid("Invalid project skill setup")
}

// Run from a directory inside the worktree. That is what a human doing this by hand does, while
// git's own hook runs at the root.
func TestASyncFromInsideTheWorktreeStillMountsAtItsRoot(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.RemoveAll(f.skillsMount("claude"))
	inside := f.project + "/somewhere/deep"
	f.MkdirAll(inside)
	f.git.TreeAt(inside, f.project)

	f.ExpectCode(f.sync(inside), 0)

	f.ExpectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
}

func TestASyncNamingNoWorktreeIsRefused(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.syncWith(), 2)

	f.ExpectSaid("usage: project-skills.sh --sync <worktree>")
}
