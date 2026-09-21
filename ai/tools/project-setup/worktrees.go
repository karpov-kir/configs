package projectsetup

import (
	"os"
	"syscall"

	"configs/ai/tools/shell"
)

// Skill links are local and ignored, so git cannot carry them into a new worktree. An install
// therefore syncs the worktrees that exist and leaves a post-checkout hook for the ones that do not.
//
// Where this installer keeps that: in the store every linked worktree of one clone shares, so one
// record answers for all of them.
type gitPaths struct {
	// common is the shared git directory, resolved physically.
	common string
	// state holds one file per client, whose content is the tier that client was installed with. It is
	// the only place that tier is written down, and a later sync reads it to mount the same set.
	state string
	hook  string
}

// The paths, or false where this project is not a readable git worktree at all. That is a normal
// outcome: a project that is no repository gets its skills, and the worktree setup is skipped. It is
// the ordinary case for a directory someone is trying this out in.
func (run *invocation) gitPaths(project string) (gitPaths, bool) {
	common, err := run.Git.CommonDir(project)
	if err != nil {
		return gitPaths{}, false
	}
	if _, err = run.Git.TopLevel(project); err != nil {
		return gitPaths{}, false
	}
	physical := shell.CanonicalDir(common)
	if physical == "" {
		return gitPaths{}, false
	}
	return gitPaths{common: physical, state: physical + "/kk-flavor", hook: physical + "/hooks/post-checkout"}, true
}

// Everything this run would write inside the git directory is checked before the first write. A
// symlink at any of them sends a write somewhere the repository never named. A hard link makes the
// write land in a file this run was never told about, and the region writer's own guard catches that.
func (run *invocation) storageWritable(paths gitPaths) bool {
	for _, path := range []string{paths.state, paths.state + "/" + run.agent, paths.common + "/info",
		paths.common + "/info/exclude", paths.common + "/hooks"} {
		if shell.IsSymlink(path) {
			run.mounting.Refuse(path + " is a symlink — worktree setup was left alone")
			return false
		}
	}
	for _, path := range []string{paths.state + "/" + run.agent, paths.common + "/info/exclude", paths.hook} {
		if shell.PathExists(path) && !shell.IsSymlink(path) {
			if !run.mounting.RegionWritable(path) {
				return false
			}
		}
	}
	return true
}

func (run *invocation) enableWorktrees() {
	paths, isRepository := run.gitPaths(run.project)
	if !isRepository {
		return
	}
	if !run.storageWritable(paths) {
		return
	}
	// Both asked, neither short-circuited: a checkout that cannot run the hook and a repository whose
	// hook is somebody else's are separate findings, and a reader needs whichever applies.
	canRun := run.hookCanRun()
	canHook := run.hookIsOursToWrite(paths) && canRun
	run.syncOtherWorktrees(paths)
	if run.isDryRun {
		if canHook {
			run.mounting.Say("  would enable " + run.agent + " skill links for future worktrees")
		}
		return
	}
	if !run.recordTier(paths) || !run.writeSharedIgnoreRules(paths) || !canHook {
		return
	}
	run.writeHook(paths)
}

// Whether there is anything for the hook to run. This is asked before the hook is written, and the
// alternative is somebody discovering the problem on their next checkout. Only the HOOK is withheld.
// The skills this pass mounts are in place already, and what is lost is the future worktrees the hook
// would have served. A refusal of those too would leave a project bare over a file it never had to read.
func (run *invocation) hookCanRun() bool {
	// The hook runs this checkout's project-skills.sh through the shared bucket, and that stub reaches
	// a Go binary through ai/tools/resolve.sh where it once needed bash alone. An incomplete checkout
	// therefore buys the project a hook that fires on every checkout and can only fail.
	// install-project.sh already refused when its four helpers were missing, and this is that refusal.
	stub := run.Repo + "/" + syncStubPath
	if !shell.IsRegularFile(stub) {
		run.mounting.Refuse(stub + " is missing, so no post-checkout hook was written — worktrees made " +
			"later will have no skill links until this checkout is complete")
		return false
	}
	// The resolver is asked for EXECUTABILITY, because that is what the stub asks of it. A stub that
	// cannot run the resolver says as much and exits without running the tool. access(2) answers that,
	// since root ignores the mode bits and a capability grants them without one.
	resolver := run.Repo + "/tools/resolve.sh"
	switch {
	case !shell.IsRegularFile(resolver):
		run.mounting.Refuse(resolver + " is missing, so the hook would reach no binary and no " +
			"post-checkout hook was written — worktrees made later will have no skill links")
		return false
	case syscall.Access(resolver, 0x1) != nil:
		run.mounting.Refuse(resolver + " cannot be run, so the hook would reach no binary and no " +
			"post-checkout hook was written — chmod +x it and install again")
		return false
	}
	return true
}

// Whether this run may write the hook at all. A hook this did not write is left alone, and reported
// with the command that does the same job by hand. The alternative is overwriting code somebody runs
// on every checkout.
func (run *invocation) hookIsOursToWrite(paths gitPaths) bool {
	// An empty core.hooksPath counts as set, since git then looks in the worktree root. A hook written
	// where this installer puts one never runs, and a run reporting success promises links that fail to
	// arrive.
	_, isSet := run.Git.ConfigValue(run.project, "core.hooksPath")
	existing, err := os.ReadFile(paths.hook)
	isForeign := isSet || shell.IsSymlink(paths.hook) || (err == nil && !isOurHookBody(string(existing)))
	if !isForeign {
		return true
	}
	run.mounting.Refuse(`existing Git hooks were preserved; add bash "` + run.Repo +
		`/project-skills.sh" --sync . to your post-checkout hook, or run it in each new worktree`)
	return false
}

// The tier this client was installed with, which is nowhere else on disk. A later sync reads it and
// mounts the same set, whatever tier the machine is asked for then.
func (run *invocation) recordTier(paths gitPaths) bool {
	if err := os.MkdirAll(paths.state, 0o755); err != nil {
		run.mounting.Refuse("could not create " + paths.state)
		return false
	}
	tier := "false\n"
	if run.isMaintainer {
		tier = "true\n"
	}
	if err := os.WriteFile(paths.state+"/"+run.agent, []byte(tier), 0o644); err != nil {
		run.mounting.Refuse("could not record " + run.agent + " worktree setup")
		return false
	}
	return true
}

// The same ignore rules again, in the repository's own exclude file. The project's .gitignore is
// committed and shared. This file is local, and it keeps the links out of a worktree whose branch
// predates the install.
func (run *invocation) writeSharedIgnoreRules(paths gitPaths) bool {
	exclude := paths.common + "/info/exclude"
	if err := os.MkdirAll(shell.DirName(exclude), 0o755); err != nil {
		run.mounting.Refuse("could not create " + shell.DirName(exclude))
		return false
	}
	if !shell.PathExists(exclude) {
		if err := os.WriteFile(exclude, nil, 0o644); err != nil {
			run.mounting.Refuse("could not create " + exclude)
			return false
		}
	}
	return run.mounting.WriteRegion(exclude, run.ignoreOpen, run.ignoreClose, run.ignoreBody())
}

func (run *invocation) writeHook(paths gitPaths) {
	if err := os.MkdirAll(shell.DirName(paths.hook), 0o755); err != nil {
		run.mounting.Refuse("could not create " + shell.DirName(paths.hook))
		return
	}
	if err := os.WriteFile(paths.hook, []byte(hookBody()), 0o755); err != nil {
		run.mounting.Refuse("could not enable " + paths.hook)
		return
	}
	// The line is said on the way out, and it names the file. This step leaves executable code in
	// somebody else's repository to run on every checkout they make, and a run that wrote it silently
	// left them to find it. The dry run has always said it would enable these links, so a silent real
	// run made the dry run's promise a lie.
	run.mounting.Say("  enabled  " + run.agent + " skill links for future worktrees: " + paths.hook)
}

func (run *invocation) disableWorktrees() {
	paths, isRepository := run.gitPaths(run.project)
	if !isRepository {
		return
	}
	if !run.storageWritable(paths) {
		return
	}
	run.syncOtherWorktrees(paths)
	if run.isDryRun {
		run.mounting.Say("  would disable " + run.agent + " skill links for future worktrees")
		return
	}
	tier := paths.state + "/" + run.agent
	if shell.IsRegularFile(tier) && !shell.IsSymlink(tier) {
		os.Remove(tier)
	}
	if shell.IsRegularFile(paths.common + "/info/exclude") {
		run.mounting.RemoveRegion(paths.common+"/info/exclude", run.ignoreOpen, run.ignoreClose)
	}
	// The hook serves both clients, so it goes only once neither is installed any more. Compared byte
	// for byte first: a hook somebody has since edited is theirs.
	if shell.PathExists(paths.state+"/"+claudeAgent) || shell.PathExists(paths.state+"/"+codexAgent) {
		return
	}
	if body, err := os.ReadFile(paths.hook); err == nil && !shell.IsSymlink(paths.hook) && isOurHookBody(string(body)) {
		os.Remove(paths.hook)
	}
	// Only when empty, which is what Remove answers for a directory: a state file this run did not write
	// is a client still installed.
	os.Remove(paths.state)
}

// Every other worktree of this clone gets the same links, because git cannot carry them itself.
func (run *invocation) syncOtherWorktrees(paths gitPaths) {
	listed, err := run.Git.Worktrees(run.project)
	if err != nil {
		run.mounting.Refuse("could not list worktrees for " + run.project)
		return
	}
	for _, worktree := range listed {
		// A bare repository is no tree to write into, and an entry git itself calls stale is none either.
		// A run that wrote into a prunable one would be acting on metadata git is about to drop.
		if worktree.Bare || worktree.Prunable {
			continue
		}
		if !shell.IsDir(worktree.Path) || worktree.Path == run.project {
			continue
		}
		if !run.syncTree(paths, worktree.Path, run.agent, run.isMaintainer) {
			run.mounting.Refuse("could not update " + run.agent + " skills in " + worktree.Path)
		}
	}
}

// One worktree's own pass over the same table, with its own mounting run. A refusal in a sibling is
// then named before it becomes a refusal of the whole install.
func (run *invocation) syncTree(paths gitPaths, worktree, agent string, isMaintainer bool) bool {
	tree := &invocation{
		Options: run.Options,
		arguments: arguments{
			agent: agent, project: worktree, isDryRun: run.isDryRun,
			willRelocate: run.willRelocate, isMaintainer: isMaintainer, isUninstall: run.isUninstall,
		},
		targets: newTargets(agent, worktree),
	}
	tree.mounting = tree.newMountingRun(run.Out, run.isDryRun)

	if !tree.isRealWorktreeOf(paths) || !tree.skillsMountWritable(tree.mounting, worktree) {
		return false
	}
	tree.declareSkills(tree.mounting)
	// The emptiness check belongs to the caller here, and its wording is simpler than the installer's
	// two. A sync mounts whatever the tier recorded at install time, so an empty table says the checkout
	// lost its skills. No flag can have excluded them here.
	if len(tree.mounting.BulkMounts()) == 0 {
		tree.mounting.Refuse("no project skills found under " + run.Repo + "/kk-flavor/skills")
		return false
	}
	if tree.isUninstall {
		tree.mounting.Unmount()
		return len(tree.mounting.Refusals()) == 0
	}
	if !tree.surveyExistingMounts() {
		return false
	}
	if !tree.mountThroughBucket(tree.mounting, worktree) {
		return false
	}
	return len(tree.mounting.Refusals()) == 0
}

// Whether this really is a worktree of the clone being installed into. Git and the filesystem are both
// asked. A `.git/worktrees/` entry is a file anyone who can write the repository can forge. It names
// the directory this would fill with symlinks and enable an agent inside. The tree has to agree with
// the listing, and its shared git directory has to be the same one this run started from.
func (tree *invocation) isRealWorktreeOf(paths gitPaths) bool {
	physical := shell.CanonicalDir(tree.project)
	if physical == "" {
		tree.mounting.Refuse(tree.project + " is not a readable Git worktree — no skills were changed")
		return false
	}
	// The user's home is never a project. Skills mounted there would be enabled for every session on
	// the machine, which is the machine-wide install and a different decision entirely.
	if home := shell.CanonicalDir(tree.Home); home != "" && home == physical {
		tree.mounting.Refuse(tree.project + " is the user home — project skill sync cannot enable user-level skills")
		return false
	}
	root, rootErr := tree.Git.TopLevel(tree.project)
	common, commonErr := tree.Git.CommonDir(tree.project)
	if rootErr != nil || commonErr != nil {
		tree.mounting.Refuse(tree.project + " is not a readable Git worktree — no skills were changed")
		return false
	}
	if shell.CanonicalDir(root) != physical || shell.CanonicalDir(common) != paths.common {
		tree.mounting.Refuse(tree.project + " is not a worktree of " + paths.common + " — no skills were changed")
		return false
	}
	return true
}
