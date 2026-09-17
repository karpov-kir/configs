package projectsetup_test

import (
	"testing"

	projectsetup "kk-flavor/tools/project-setup"
)

// A project that is a git repository, with its shared git directory where this installer keeps its
// state. Declared rather than built: `git init` plus a commit plus a `worktree add` is five processes,
// and the shell suite this replaces spent 819 seconds on exactly that.
func (f *fixture) asRepository() gitPathsFixture {
	f.t.Helper()
	common := f.project + "/.git"
	f.mkdirAll(common)
	f.git.commonDir = common
	f.git.worktreeAt(f.project)
	return gitPathsFixture{common: common, state: common + "/kk-flavor", hook: common + "/hooks/post-checkout"}
}

type gitPathsFixture struct {
	common string
	state  string
	hook   string
}

// A sibling worktree of the same clone, which git would list beside the project.
func (f *fixture) newSibling(name string) string {
	f.t.Helper()
	sibling := f.base + "/" + name
	f.mkdirAll(sibling)
	f.git.worktreeAt(sibling)
	return sibling
}

// A project that is not a repository gets its skills and no worktree setup. That is the ordinary case
// for a directory someone is trying this out in, so it must not be a refusal.
func TestAProjectThatIsNotARepositoryStillGetsItsSkills(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
}

// Skill links are local and ignored, so git cannot carry them into a worktree created later. The hook
// is what puts them there, and the exclude file is what keeps them out of a branch that predates the
// install.
func TestAnInstallEnablesFutureWorktreesAndRecordsTheTier(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()

	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.expectFileBody(paths.hook, projectsetup.HookBody())
	f.expectFileBody(paths.state+"/claude", "true\n")
	f.expectFileContains(paths.common+"/info/exclude", ".claude/skills/kk-*")
}

// Every worktree that exists gets the same links, because git cannot carry them itself.
func TestAnInstallSyncsTheWorktreesThatAlreadyExist(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	sibling := f.newSibling("existing sibling")

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectLinkTo(sibling+"/.claude/skills/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	// And nothing at the user level: a project install never enables skills for every session on the
	// machine, which is the machine-wide install and a different decision entirely.
	f.expectAbsent(f.home + "/.claude")
	f.expectAbsent(f.home + "/.agents")
}

// Anything already at the hook that this did not write is left alone and reported with the command
// that does the same job by hand. A hook is code its owner runs on every checkout.
func TestAnExistingHookIsPreservedAndReportedWithTheRepair(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.write(paths.hook, "#!/usr/bin/env python3\nprint(\"existing hook\")\n")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectFileBody(paths.hook, "#!/usr/bin/env python3\nprint(\"existing hook\")\n")
	f.expectSaid(`project-skills.sh" --sync .`)
}

// An empty core.hooksPath counts as set: git then looks in the worktree root, so a hook written where
// this installer puts one never runs, and a run reporting success would be promising links that never
// arrive.
func TestAnEmptyHooksPathIsStillAHookManagerAndIsReported(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.git.isHooksPath, f.git.hooksPath = true, ""

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("existing Git hooks were preserved")
	f.expectAbsent(paths.hook)
}

func TestAHooksManagerElsewhereIsReportedAndNotOverridden(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.git.isHooksPath, f.git.hooksPath = true, ".custom-hooks"

	f.expectCode(f.install("--agent=claude", "--dry-run"), 1)

	f.expectSaid("existing Git hooks were preserved")
	f.expectAbsent(paths.state + "/claude")
}

// Everything this run would write inside the git directory is checked before the first write. A
// symlink at any of them sends a write somewhere the repository never named — and a repository's own
// files are not trusted input.
func TestASymlinkAnywhereInTheGitStorageIsRefused(t *testing.T) {
	for _, planted := range []string{"kk-flavor", "kk-flavor/claude", "info/exclude", "hooks"} {
		t.Run(planted, func(t *testing.T) {
			f := newFixture(t)
			paths := f.asRepository()
			target := paths.common + "/" + planted
			f.symlink(f.base+"/never-written", target)

			f.expectCode(f.install("--agent=claude"), 1)

			f.expectSaid("is a symlink — worktree setup was left alone")
			f.expectAbsent(f.base + "/never-written")
		})
	}
}

// A hard link makes a write land in a file this run was never told about. Neither a symlink nor a
// missing file, so nothing else catches it — the region writer's own link-count guard does.
func TestHardlinkedGitStateIsRefusedAndTheOutsideFileSurvives(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.write(f.base+"/state-outside", "outside content\n")
	f.mkdirAll(paths.state)
	f.hardlink(f.base+"/state-outside", paths.state+"/claude")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectFileBody(f.base+"/state-outside", "outside content\n")
}

// A listing this installer cannot read is a refusal, not a silent pass: the worktrees it would have
// synced are exactly the ones nothing else will.
func TestAWorktreeListingThatCannotBeReadIsARefusal(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.git.listingError = gitError("git said no")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("could not list worktrees for " + f.project)
}

// Neither a bare repository nor an entry git itself calls stale is a tree to write into. A run that
// wrote into a prunable one would be acting on metadata git is about to drop.
func TestBareAndPrunableEntriesAreNeverWrittenInto(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	bare := f.base + "/bare"
	stale := f.base + "/stale"
	f.mkdirAll(bare)
	f.mkdirAll(stale)
	f.git.worktrees = append(f.git.worktrees,
		projectsetup.Worktree{Path: bare, IsBare: true},
		projectsetup.Worktree{Path: stale, IsPrunable: true})

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectAbsent(bare + "/.claude")
	f.expectAbsent(stale + "/.claude")
}

// A `.git/worktrees/` entry is a file anyone who can write the repository can forge, and it names the
// directory this would then fill with symlinks and enable an agent inside. So the tree has to agree
// that it is what the listing said, and its shared git directory has to be the one this run started
// from.
func TestAForgedWorktreeEntryPointingAtAnotherRepositoryIsRefused(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	outsider := f.base + "/unrelated-repository"
	f.mkdirAll(outsider + "/.git")
	// git answers for it, and answers with a different shared directory — which is the whole tell.
	f.git.foreignWorktreeAt(outsider, outsider+"/.git")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("is not a worktree of " + f.git.commonDir)
	f.expectAbsent(outsider + "/.claude")
}

// The user's home is never a project. Mounting skills there would enable them for every session on the
// machine, which is the machine-wide install and a different decision entirely.
func TestAForgedEntryNamingTheUserHomeIsRefused(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.git.roots[f.home] = f.home
	f.git.worktrees = append(f.git.worktrees, projectsetup.Worktree{Path: f.home})

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("is the user home")
	f.expectAbsent(f.home + "/.claude")
}

// The hook serves both clients, so it goes only once neither is installed any more.
func TestTheHookGoesWithTheLastClientAndNotBefore(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.expectCode(f.install("--agent=claude"), 0)
	f.expectCode(f.install("--agent=codex"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)
	f.expectFileBody(paths.hook, projectsetup.HookBody())

	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)
	f.expectAbsent(paths.hook)
	f.expectAbsent(paths.state)
}

// A hook somebody edited since is theirs, whatever this installer wrote there first.
func TestAnEditedHookIsLeftBehindOnUninstall(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.expectCode(f.install("--agent=claude"), 0)
	f.rewrite(paths.hook, projectsetup.HookBody()+"# and something of my own\n")

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectFileContains(paths.hook, "# and something of my own")
}

func TestUninstallClearsTheSiblingsLinksAndTheSharedIgnoreRules(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	sibling := f.newSibling("sibling")
	f.expectCode(f.install("--agent=claude"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectAbsent(sibling + "/.claude/skills/kk-build")
	f.expectFileLacks(paths.common+"/info/exclude", ".claude/skills/kk-*")
	f.expectAbsent(paths.state + "/claude")
}
