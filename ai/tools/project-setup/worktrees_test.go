package projectsetup_test

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	projectsetup "configs/ai/tools/project-setup"
	"configs/ai/tools/repo"
)

// A project that is a git repository, with its shared git directory where this installer keeps its
// state. Declared rather than built: `git init` plus a commit plus a `worktree add` is five processes,
// and the shell suite this replaces spent 819 seconds on exactly that.
func (f *fixture) asRepository() gitPathsFixture {
	f.t.Helper()
	common := f.project + "/.git"
	f.MkdirAll(common)
	f.git.Common = common
	// The project alone; every other directory stays what PerDirectory makes it.
	// The project alone; every other directory stays what PerDirectory makes it.
	f.git.WorktreeAt(f.project)
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
	f.MkdirAll(sibling)
	f.git.WorktreeAt(sibling)
	return sibling
}

// A project that is not a repository gets its skills and no worktree setup. That is the ordinary case
// for a directory someone is trying this out in, so it must not be a refusal.
//
// The `.git` directory is there and git still answers nothing for it, which is not a contradiction: a
// repository git refuses — dubious ownership, a gitdir pointer that leads nowhere — looks exactly like
// this. It is here so the case turns on GIT's answer. Without it the directory being absent decides
// the outcome on its own, and the case would pass with the port's refusal thrown away.
func TestAProjectThatIsNotARepositoryStillGetsItsSkills(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll(f.project + "/.git")

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	f.expectAbsent(f.project + "/.git/hooks/post-checkout")
	f.expectAbsent(f.project + "/.git/kk-flavor")
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
	// The write is executable code left in somebody else's repository, running on every checkout they
	// make. A run that placed it without a word left them to discover it, while --dry-run had always
	// promised the line — so the two runs disagreed about what an install does.
	f.expectSaid("enabled  claude skill links for future worktrees: " + paths.hook)
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
	f.Write(paths.hook, "#!/usr/bin/env python3\nprint(\"existing hook\")\n")

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
	f.git.Config["core.hooksPath"] = ""

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("existing Git hooks were preserved")
	f.expectAbsent(paths.hook)
}

func TestAHooksManagerElsewhereIsReportedAndNotOverridden(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.git.Config["core.hooksPath"] = ".custom-hooks"

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
			f.Symlink(f.base+"/never-written", target)

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
	f.Write(f.base+"/state-outside", "outside content\n")
	f.MkdirAll(paths.state)
	f.hardlink(f.base+"/state-outside", paths.state+"/claude")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectFileBody(f.base+"/state-outside", "outside content\n")
}

// A listing this installer cannot read is a refusal, not a silent pass: the worktrees it would have
// synced are exactly the ones nothing else will.
func TestAWorktreeListingThatCannotBeReadIsARefusal(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.git.Fail["Worktrees"] = errors.New("git said no")

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
	f.MkdirAll(bare)
	f.MkdirAll(stale)
	f.git.WorktreeList = append(f.git.WorktreeList,
		repo.Worktree{Path: bare, Bare: true},
		repo.Worktree{Path: stale, Prunable: true})

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
	f.MkdirAll(outsider + "/.git")
	// git answers for it, and answers with a different shared directory — which is the whole tell.
	f.git.ForeignWorktreeAt(outsider, outsider+"/.git")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("is not a worktree of " + f.git.Common)
	f.expectAbsent(outsider + "/.claude")
}

// The user's home is never a project. Mounting skills there would enable them for every session on the
// machine, which is the machine-wide install and a different decision entirely.
func TestAForgedEntryNamingTheUserHomeIsRefused(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.git.WorktreeAt(f.home)

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

// The hook is code this installer leaves in somebody's repository to run on every checkout, and what
// it runs is this checkout's project-skills.sh — which is now a stub reaching a Go binary through
// ai/tools/resolve.sh, where it used to be bash and nothing else. install-project.sh refused before
// writing anything when its helpers were missing; that refusal is this, aimed at what the hook needs
// now.
//
// The skills still mount. Only the hook is withheld, because only the future worktrees it would have
// served are lost — and a run that refused everything would leave a project with no skills over a
// file it never needed to read.
func TestAHookIsNotWrittenWhenTheCheckoutCannotRunIt(t *testing.T) {
	for _, c := range []struct{ name, missing, said string }{
		{"the stub the hook runs", "project-skills.sh", "project-skills.sh is missing"},
		{"the resolver the stub reaches", "tools/resolve.sh", "tools/resolve.sh is missing"},
	} {
		t.Run(c.name+" is gone", func(t *testing.T) {
			f := newFixture(t)
			paths := f.asRepository()
			f.RemoveAll(f.repo + "/" + c.missing)

			f.expectCode(f.install("--agent=claude"), 1)

			f.expectSaid(c.said)
			f.expectAbsent(paths.hook)
			f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
		})
	}
}

// A resolver that is there and cannot be run is the same finding as one that is absent: the hook fires
// and nothing answers it. Probed rather than assumed, because root ignores the mode bits.
func TestAResolverThatCannotBeExecutedStopsTheHookToo(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	resolver := f.repo + "/tools/resolve.sh"
	if err := os.Chmod(resolver, 0o644); err != nil {
		t.Fatalf("the fixture could not clear the execute bit on %s: %v", resolver, err)
	}
	if syscall.Access(resolver, 0x1) == nil {
		t.Skip("this process executes a file with no execute bit — root, or a filesystem that drops the " +
			"bit — so the refusal this case names could not be built")
	}

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("tools/resolve.sh cannot be run")
	f.expectAbsent(paths.hook)
}

// What the hook says when it fires in somebody's project and the checkout behind ~/.kk-flavor has
// since lost its stub. Run rather than read: the body is bash, and the one thing worth holding is what
// a human standing in an unrelated repository sees after `git checkout`.
//
// The old body handed the path straight to `bash`, which answered "No such file or directory" under
// the hook's own name — a message that reads as this repository's hook being broken. It is the ai/
// checkout that is incomplete, and the hook is the only thing in a position to say so.
func TestTheHookBlamesTheFlavorCheckoutAndNotTheRepositoryItFiresIn(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH, so the hook body cannot be held to what it prints")
	}
	f := newFixture(t)
	f.asRepository()
	f.expectCode(f.install("--agent=claude"), 0)
	f.RemoveAll(f.repo + "/project-skills.sh")

	hook := f.base + "/fired-hook"
	f.rewrite(hook, projectsetup.HookBody())
	run := exec.Command(bash, hook)
	run.Dir = f.project
	run.Env = append(os.Environ(), "HOME="+f.home)
	out, err := run.CombinedOutput()
	if err == nil {
		t.Errorf("the hook exited 0 having restored nothing; it said: %s", out)
	}
	said := string(out)
	if !strings.Contains(said, f.repo) {
		t.Errorf("the hook did not name the checkout that is missing its stub. It said:\n%s", said)
	}
	for _, blame := range []string{f.project, "No such file"} {
		if strings.Contains(said, blame) {
			t.Errorf("the hook blamed %q, which is not what is wrong. It said:\n%s", blame, said)
		}
	}
}

// A hook this installer wrote before the body changed is still this installer's own. Compared byte for
// byte, an unrecognised hook is somebody else's: an install would refuse to touch it and an uninstall
// would leave it behind, firing on every checkout of a project nothing is installed in any more.
func TestTheHookThisInstallerWroteBeforeIsStillItsOwn(t *testing.T) {
	t.Run("an install replaces it", func(t *testing.T) {
		f := newFixture(t)
		paths := f.asRepository()
		f.MkdirAll(paths.common + "/hooks")
		f.rewrite(paths.hook, projectsetup.SupersededHookBody())

		f.expectCode(f.install("--agent=claude"), 0)

		f.expectFileBody(paths.hook, projectsetup.HookBody())
	})

	t.Run("an uninstall removes it", func(t *testing.T) {
		f := newFixture(t)
		paths := f.asRepository()
		f.expectCode(f.install("--agent=claude"), 0)
		f.rewrite(paths.hook, projectsetup.SupersededHookBody())

		f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

		f.expectAbsent(paths.hook)
	})
}
