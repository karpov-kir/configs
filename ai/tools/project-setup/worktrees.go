package projectsetup

import (
	"os"

	"kk-flavor/tools/shell"
)

// Skill links are local and ignored, so git cannot carry them into a new worktree. An install
// therefore syncs the worktrees that exist and leaves a post-checkout hook for the ones that do not.
//
// Where this installer keeps that: in the store every linked worktree of one clone shares, so one
// record answers for all of them.
type gitPaths struct {
	// common is the shared git directory, resolved physically.
	common string
	// state holds one file per client, whose content is the tier that client was installed with — the
	// only place that tier is written down, and what a later sync reads to mount the same set.
	state string
	hook  string
}

// The paths, or false where this project is not a readable git worktree at all. Not a refusal: a
// project that is not a repository gets its skills and no worktree setup, which is the ordinary case
// for a directory someone is trying this out in.
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

// Everything this run would write inside the git directory, checked before the first write. A symlink
// at any of them sends a write somewhere the repository never named; a hard link makes the write land
// in a file this run was never told about, which is what the region writer's own guard catches.
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
	canHook := run.hookIsOursToWrite(paths)
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

// Whether this run may write the hook at all. Anything already there that this did not write is left
// alone and reported with the command that does the same job by hand — the alternative is overwriting
// somebody's hook, and a hook is code they run on every checkout.
//
// An empty core.hooksPath counts as set: git then looks in the worktree root, so a hook written where
// this installer puts one never runs, and a run reporting success would be promising links that never
// arrive.
func (run *invocation) hookIsOursToWrite(paths gitPaths) bool {
	_, isSet := run.Git.HooksPath(run.project)
	existing, err := os.ReadFile(paths.hook)
	isForeign := isSet || shell.IsSymlink(paths.hook) || (err == nil && string(existing) != hookBody())
	if !isForeign {
		return true
	}
	run.mounting.Refuse(`existing Git hooks were preserved; add bash "` + run.Repo +
		`/project-skills.sh" --sync . to your post-checkout hook, or run it in each new worktree`)
	return false
}

// The tier this client was installed with, which is nowhere else on disk. A later sync reads it to
// mount the same set rather than whatever tier the machine is asked for then.
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
// committed and shared; this one is local, and it is what keeps the links out of a worktree whose
// branch predates the install.
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
	}
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
	if body, err := os.ReadFile(paths.hook); err == nil && !shell.IsSymlink(paths.hook) && string(body) == hookBody() {
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
		// Neither a bare repository nor an entry git itself calls stale is a tree to write into, and a
		// run that wrote into a prunable one would be acting on metadata git is about to drop.
		if worktree.IsBare || worktree.IsPrunable {
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

// One worktree's own pass over the same table, with its own mounting run so a refusal in a sibling
// does not become a refusal of the whole install without being named.
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

	if !tree.isRealWorktreeOf(paths) || !tree.projectSkillsWritable(tree.mounting, worktree) {
		return false
	}
	tree.declareSkills(tree.mounting)
	// The emptiness check is the caller's here rather than the table's, and the wording is simpler than
	// the installer's two: a sync mounts whatever the tier recorded at install time, so a table with
	// nothing in it says the checkout lost its skills rather than that a flag excluded them.
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

// Whether this really is a worktree of the clone being installed into, asked of git and of the
// filesystem rather than taken from the listing. A `.git/worktrees/` entry is a file anyone who can
// write the repository can forge, and it names the directory this would then fill with symlinks and
// enable an agent inside — so the tree has to agree that it is what the listing said, and its shared
// git directory has to be the one this run started from.
func (tree *invocation) isRealWorktreeOf(paths gitPaths) bool {
	physical := shell.CanonicalDir(tree.project)
	if physical == "" {
		tree.mounting.Refuse(tree.project + " is not a readable Git worktree — no skills were changed")
		return false
	}
	// The user's home is never a project. Mounting skills there would enable them for every session on
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
