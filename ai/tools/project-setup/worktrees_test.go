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
// state. The fixture declares it: `git init` plus a commit plus a `worktree add` is five processes,
// and the shell suite this replaces spent 819 seconds on exactly that.
func (f *fixture) asRepository() gitPathsFixture {
	f.t.Helper()
	common := f.project + "/.git"
	f.MkdirAll(common)
	f.git.Common = common
	// The project alone. Every other directory stays what PerDirectory makes it.
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

// A project that is not a repository gets its skills, and the worktree setup is skipped: the ordinary
// case for a directory someone is trying this out in. The `.git` directory is planted, and git still
// refuses the project the way dubious ownership or a gitdir pointer leading nowhere does. An absent
// `.git` would otherwise decide the outcome, and the port's refusal would go untested.
func TestAProjectThatIsNotARepositoryStillGetsItsSkills(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll(f.project + "/.git")

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	f.ExpectAbsent(f.project + "/.git/hooks/post-checkout")
	f.ExpectAbsent(f.project + "/.git/kk-flavor")
}

// Skill links are local and ignored, so git cannot carry them into a worktree created later. The hook
// is what puts them there, and the exclude file is what keeps them out of a branch that predates the
// install.
func TestAnInstallEnablesFutureWorktreesAndRecordsTheTier(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()

	f.ExpectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.ExpectFileBody(paths.hook, projectsetup.HookBody())
	f.ExpectFileBody(paths.state+"/claude", "true\n")
	f.ExpectFileContains(paths.common+"/info/exclude", ".claude/skills/kk-*")
	// The write is executable code left in somebody else's repository, running on every checkout they
	// make. A run that placed it without a word left them to discover it. --dry-run had always said it
	// would enable these links, so the two runs disagreed about what an install does.
	f.ExpectSaid("enabled  claude skill links for future worktrees: " + paths.hook)
}

// Every worktree that exists gets the same links, because git cannot carry them itself.
func TestAnInstallSyncsTheWorktreesThatAlreadyExist(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	sibling := f.newSibling("existing sibling")

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectLinkTo(sibling+"/.claude/skills/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	// And the user level stays untouched: a project install never enables skills for every session on
	// the machine, which is the machine-wide install and a different decision entirely.
	f.ExpectAbsent(f.home + "/.claude")
	f.ExpectAbsent(f.home + "/.agents")
}

// Anything already at the hook that this did not write is left alone and reported with the command
// that does the same job by hand. A hook is code its owner runs on every checkout.
func TestAnExistingHookIsPreservedAndReportedWithTheRepair(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.Write(paths.hook, "#!/usr/bin/env python3\nprint(\"existing hook\")\n")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectFileBody(paths.hook, "#!/usr/bin/env python3\nprint(\"existing hook\")\n")
	f.ExpectSaid(`project-skills.sh" --sync .`)
}

// An empty core.hooksPath, the Git config setting, counts as set, since git then looks in the worktree
// root. A hook written where this installer puts one never runs, and a run reporting success promises
// links that fail to arrive.
func TestAnEmptyHooksPathIsStillAHookManagerAndIsReported(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.git.Config["core.hooksPath"] = ""

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("existing Git hooks were preserved")
	f.ExpectAbsent(paths.hook)
}

func TestAHooksManagerElsewhereIsReportedAndNotOverridden(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.git.Config["core.hooksPath"] = ".custom-hooks"

	f.ExpectCode(f.install("--agent=claude", "--dry-run"), 1)

	f.ExpectSaid("existing Git hooks were preserved")
	f.ExpectAbsent(paths.state + "/claude")
}

// Everything this run would write inside the git directory is checked before the first write. A
// symlink at any of them sends a write somewhere the repository never named, and a repository's own
// files count as untrusted.
func TestASymlinkAnywhereInTheGitStorageIsRefused(t *testing.T) {
	for _, planted := range []string{"kk-flavor", "kk-flavor/claude", "info/exclude", "hooks"} {
		t.Run(planted, func(t *testing.T) {
			f := newFixture(t)
			paths := f.asRepository()
			target := paths.common + "/" + planted
			f.Symlink(f.base+"/never-written", target)

			f.ExpectCode(f.install("--agent=claude"), 1)

			f.ExpectSaid("is a symlink — worktree setup was left alone")
			f.ExpectAbsent(f.base + "/never-written")
		})
	}
}

// A hard link makes a write land in a file this run was never told about. A symlink check and an
// existence check both pass it, and the region writer's own link-count guard is what catches it.
func TestHardlinkedGitStateIsRefusedAndTheOutsideFileSurvives(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.Write(f.base+"/state-outside", "outside content\n")
	f.MkdirAll(paths.state)
	f.Hardlink(f.base+"/state-outside", paths.state+"/claude")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectFileBody(f.base+"/state-outside", "outside content\n")
}

// A listing this installer cannot read is a refusal, and a silent pass would be wrong: the worktrees
// it would have synced are exactly the ones no other step reaches.
func TestAWorktreeListingThatCannotBeReadIsARefusal(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.git.Fail["Worktrees"] = errors.New("git said no")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("could not list worktrees for " + f.project)
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

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectAbsent(bare + "/.claude")
	f.ExpectAbsent(stale + "/.claude")
}

// A `.git/worktrees/` entry is a file anyone who can write the repository can forge, and it names the
// directory this would then fill with symlinks and enable an agent inside. So the tree has to agree
// that it is what the listing said, and its shared git directory has to be the same one this run
// started from.
func TestAForgedWorktreeEntryPointingAtAnotherRepositoryIsRefused(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	outsider := f.base + "/unrelated-repository"
	f.MkdirAll(outsider + "/.git")
	// git answers for it, and answers with a different shared directory — which is the whole tell.
	f.git.ForeignWorktreeAt(outsider, outsider+"/.git")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("is not a worktree of " + f.git.Common)
	f.ExpectAbsent(outsider + "/.claude")
}

// The user's home is never a project. Skills mounted there would be enabled for every session on the
// machine, which is the machine-wide install and a different decision entirely.
func TestAForgedEntryNamingTheUserHomeIsRefused(t *testing.T) {
	f := newFixture(t)
	f.asRepository()
	f.git.WorktreeAt(f.home)

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("is the user home")
	f.ExpectAbsent(f.home + "/.claude")
}

// The hook serves both clients, so it goes only once neither is installed any more.
func TestTheHookGoesWithTheLastClientAndNotBefore(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.ExpectCode(f.install("--agent=codex"), 0)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)
	f.ExpectFileBody(paths.hook, projectsetup.HookBody())

	f.ExpectCode(f.install("--agent=codex", "--uninstall"), 0)
	f.ExpectAbsent(paths.hook)
	f.ExpectAbsent(paths.state)
}

// A hook somebody edited since is theirs, whatever this installer wrote there first.
func TestAnEditedHookIsLeftBehindOnUninstall(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.WriteMode(paths.hook, projectsetup.HookBody()+"# and something of my own\n", 0o755)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectFileContains(paths.hook, "# and something of my own")
}

func TestUninstallClearsTheSiblingsLinksAndTheSharedIgnoreRules(t *testing.T) {
	f := newFixture(t)
	paths := f.asRepository()
	sibling := f.newSibling("sibling")
	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectAbsent(sibling + "/.claude/skills/kk-build")
	f.ExpectFileLacks(paths.common+"/info/exclude", ".claude/skills/kk-*")
	f.ExpectAbsent(paths.state + "/claude")
}

// The hook is code this installer leaves in somebody's repository to run on every checkout. It runs
// this checkout's project-skills.sh, a stub that now reaches a Go binary through ai/tools/resolve.sh
// where it once needed bash alone. The skills still mount: only the future worktrees the hook would
// have served are lost, and a full refusal would leave a project bare over a file it never read.
func TestAHookIsNotWrittenWhenTheCheckoutCannotRunIt(t *testing.T) {
	for _, c := range []struct{ name, missing, said string }{
		{"the stub the hook runs", "project-skills.sh", "project-skills.sh is missing"},
		{"the resolver the stub reaches", "tools/resolve.sh", "tools/resolve.sh is missing"},
	} {
		t.Run(c.name+" is gone", func(t *testing.T) {
			f := newFixture(t)
			paths := f.asRepository()
			f.RemoveAll(f.repo + "/" + c.missing)

			f.ExpectCode(f.install("--agent=claude"), 1)

			f.ExpectSaid(c.said)
			f.ExpectAbsent(paths.hook)
			f.ExpectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
		})
	}
}

// A resolver that is there and cannot be run is the same finding as one that is absent: the hook
// fires and gets no answer. The case probes for it, because root ignores the mode bits.
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

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("tools/resolve.sh cannot be run")
	f.ExpectAbsent(paths.hook)
}

// What the hook says when it fires in somebody's project and the checkout behind ~/.kk-flavor has
// since lost its stub. The case runs the body, since it is bash. The old body let `bash` answer "No
// such file or directory" under the hook's own name, blaming a repository that is fine. The ai/
// checkout is the incomplete one, and the hook is the only thing placed to say so.
func TestTheHookBlamesTheFlavorCheckoutAndNotTheRepositoryItFiresIn(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH, so the hook body cannot be held to what it prints")
	}
	f := newFixture(t)
	f.asRepository()
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.RemoveAll(f.repo + "/project-skills.sh")

	hook := f.base + "/fired-hook"
	f.WriteMode(hook, projectsetup.HookBody(), 0o755)
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

// A hook this installer wrote before the body changed is still this installer's own. Hooks are
// compared byte for byte, so an unrecognised one is somebody else's. An install would refuse to touch
// it and an uninstall would leave it behind, firing on every checkout of a project with no install
// left.
func TestTheHookThisInstallerWroteBeforeIsStillItsOwn(t *testing.T) {
	t.Run("an install replaces it", func(t *testing.T) {
		f := newFixture(t)
		paths := f.asRepository()
		f.MkdirAll(paths.common + "/hooks")
		f.WriteMode(paths.hook, projectsetup.SupersededHookBody(), 0o755)

		f.ExpectCode(f.install("--agent=claude"), 0)

		f.ExpectFileBody(paths.hook, projectsetup.HookBody())
	})

	t.Run("an uninstall removes it", func(t *testing.T) {
		f := newFixture(t)
		paths := f.asRepository()
		f.ExpectCode(f.install("--agent=claude"), 0)
		f.WriteMode(paths.hook, projectsetup.SupersededHookBody(), 0o755)

		f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

		f.ExpectAbsent(paths.hook)
	})
}
