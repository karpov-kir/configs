// Package projectsetup mounts this repository's skills into one project, writes the two instruction
// files and the ignore rules that go with them, and takes all of it back out again. The skills reach
// the project through the shared ~/.kk-flavor bucket, and no mount names this checkout directly, so a
// project holds a path every install of this flavor has.
//
// Uninstall is a mode here, over the same table an install declares. A second program re-deriving what
// to remove would disagree with what was installed, in the direction the human misses: things left
// behind and a run reporting ok. The home, the machine and git arrive as values here, so a suite can
// drive a throwaway home, a worktree listing and a mise install with no process spawned.
package projectsetup

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"configs/ai/tools/flavor"
	"configs/ai/tools/installer"
	"configs/ai/tools/machine"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// Exit codes on the tools' shared vocabulary. 2 is a grammar this tool did not understand, so the run
// stopped before its first write. 1 is something a human has to fix, and the report at the end of
// every run answers it.
const (
	exitDone     = 0
	exitBadUsage = 2
)

const label = "ai project install"

// How the usage line names this tool. The stub's own basename, for the reason ai/tools/env-bootstrap
// states: `usage: <basename>` is what eco-check's scans anchor on.
const stubPath = "install-project.sh"

// What the second-checkout guard looks for under a candidate root. A constant, and never the running
// program's own name, since two entry points reach this code: the installer and the post-checkout
// sync. The mounts were written by the installer, and a sync that looked for its own name would find
// none and repoint another checkout's project mounts in silence.
const guardScriptName = "install-project.sh"

const claudeAgent = "claude"

const codexAgent = "codex"

// Options is everything a run needs that it must not go looking for itself.
type Options struct {
	Self string
	Args []string
	// Repo is the ai/ directory this run mounts from — the stub's own, resolved physically.
	Repo string
	Home string
	// ConfigHome is the already-resolved ${XDG_CONFIG_HOME:-$HOME/.config}, where the install registry
	// lives.
	ConfigHome string
	Machine    machine.Machine
	// Git is ai/tools/repo's port, the single place these tools ask a git repository a question. This
	// installer asks four questions: a worktree's root, the store its clone shares, core.hooksPath, and
	// what `worktree list` names.
	Git repo.Git
	// Mcp configures the project's own MCP client files. A port, because it is a whole tool of its own
	// and what this installer decides is only what to do with the code it answers.
	Mcp       Mcp
	Out       io.Writer
	Err       io.Writer
	WriteRoot string
}

// Mcp is the project MCP configuration, as the single question this installer asks of it.
type Mcp interface {
	// Configure answers an exit code: 0 done, anything else a refusal the tool has already explained.
	Configure(project, agent string, isDryRun, isUninstall bool) int
}

// Run executes one invocation and answers its exit code.
func Run(options Options) int {
	_, code := perform(options)
	return code
}

// The same run, handing back the machinery's own record of it, so a suite can read back every write the
// containment bound turned away. A refused invocation never builds one, so the run is nil there.
func perform(options Options) (*installer.Run, int) {
	parsed, code, hasStopped := parseArguments(options.Self, options.Args, options.Err)
	if hasStopped {
		return nil, code
	}
	// The project path is resolved before anything is written, so every later path and the registry
	// entry name one directory however the caller spelled it. Absolute and symlink-free: `.` is what a
	// human standing in the project types, and an entry recorded as typed means a different directory
	// for every later reader. A missing project is refused: this installs into a repository someone has.
	project, err := shell.RealPath(parsed.project)
	if err != nil || !shell.IsDir(project) {
		// The spelling as it was typed, because a reader sent after a path this rewrote is being sent
		// somewhere they never named.
		fmt.Fprintf(options.Err, "%s: %s is not a directory — nothing was written\n", options.Self, parsed.project)
		return nil, exitBadUsage
	}
	parsed.project = project

	run := newRun(options, parsed)
	return run.mounting, run.do()
}

// One invocation's whole context.
type invocation struct {
	Options
	arguments
	targets
	mounting *installer.Run
}

func newRun(options Options, parsed arguments) *invocation {
	run := &invocation{Options: options, arguments: parsed, targets: newTargets(parsed.agent, parsed.project)}
	run.mounting = run.newMountingRun(options.Out, parsed.isDryRun)
	return run
}

// The machinery one pass writes through. It is built here and never once in newRun, because the
// install path makes a second, throwaway pass. The mounts are surveyed against this checkout's own
// skill directories before the sources are rewritten to reach through the bucket. That survey is the
// only thing that can see a project still mounted from another checkout.
func (run *invocation) newMountingRun(out io.Writer, isDryRun bool) *installer.Run {
	return installer.NewRun(installer.RunOptions{
		Repo:       run.Repo,
		ScriptName: guardScriptName,
		Label:      label,
		BulkLabel:  "skills",
		// Machine-wide, repointing these moves a human's whole configuration. Here it moves only one
		// project's skills. A refusal saying "this machine's configuration" about one project is false
		// in a way that teaches people to ignore it.
		MountScopeLabel: run.project + "'s skills",
		ConfigHome:      run.ConfigHome,
		DryRun:          isDryRun,
		Relocate:        run.willRelocate,
		Out:             out,
		WriteRoot:       run.WriteRoot,
	})
}

// One invocation's flags.
type arguments struct {
	agent        string
	project      string
	isDryRun     bool
	willRelocate bool
	isMaintainer bool
	isUninstall  bool
}

// The third value says whether the run stops here — a refused invocation and a printed help both do,
// and they exit differently. The code alone cannot tell exit 0 from the help arm apart from "parsed
// fine, carry on", which is how an empty invocation reaches the filesystem.
func parseArguments(self string, args []string, stderr io.Writer) (arguments, int, bool) {
	parsed := arguments{}
	for _, arg := range args {
		switch {
		case arg == "--agent="+claudeAgent, arg == "--agent="+codexAgent:
			parsed.agent = strings.TrimPrefix(arg, "--agent=")
		case strings.HasPrefix(arg, "--agent="):
			return parsed, badUsage(self, stderr, "unknown agent %s — use --agent=%s|%s",
				strings.TrimPrefix(arg, "--agent="), codexAgent, claudeAgent), true
		case arg == "--dry-run":
			parsed.isDryRun = true
		case arg == "--relocate":
			parsed.willRelocate = true
		case arg == "--maintainer":
			parsed.isMaintainer = true
		case arg == "--uninstall":
			parsed.isUninstall = true
		case arg == "-h", arg == "--help":
			fmt.Fprintln(stderr, usage())
			return parsed, exitDone, true
		case strings.HasPrefix(arg, "-"):
			return parsed, badUsage(self, stderr, "unknown option %s", arg), true
		case parsed.project != "":
			return parsed, badUsage(self, stderr, "one project at a time — got %s and %s", parsed.project, arg), true
		default:
			parsed.project = arg
		}
	}
	switch {
	case parsed.agent == "":
		return parsed, badUsage(self, stderr, "--agent=%s|%s is required — nothing was changed",
			claudeAgent, codexAgent), true
	case parsed.project == "":
		return parsed, badUsage(self, stderr, "name the project directory to install into"), true
	}
	return parsed, exitDone, false
}

func badUsage(self string, stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "%s: %s\n", self, fmt.Sprintf(format, args...))
	fmt.Fprintln(stderr, usage())
	return exitBadUsage
}

func usage() string {
	return "usage: " + stubPath + " --agent=" + claudeAgent + "|" + codexAgent +
		" [--dry-run] [--relocate] [--maintainer] [--uninstall] <project>"
}

// The whole of one run, in the order the steps have to happen in.
func (run *invocation) do() int {
	found := run.declareSkills(run.mounting)
	if refusal := run.emptyTableRefusal(found); refusal != "" {
		run.mounting.Refuse(refusal)
		return run.mounting.Report()
	}
	run.mounting.Say(label + ": " + run.project)
	if run.isUninstall {
		return run.uninstall()
	}
	return run.install(found)
}

func (run *invocation) declareSkills(mounting *installer.Run) installer.SkillMounts {
	// Maintainer-only skills are for maintaining this instruction tree, and a project that merely uses
	// it has no use for them. A project install leaves them out unless asked. An uninstall takes them
	// anyway, because the run also forgets the project, and after that the machine can no longer point
	// at them.
	return mounting.AddSkillMounts(installer.SkillMountOptions{
		SkillsDirectory: run.Repo + "/kk-flavor/skills",
		MountParent:     run.skillsMount,
		Maintainer:      run.isMaintainer,
		Uninstalling:    run.isUninstall,
	})
}

// Two ways to mount no skill, and they send a reader to different places. An empty skills directory
// is a broken checkout, while a flag that excluded every skill it found is a flag doing exactly what
// it says. The exit code is the same for both, so the wording is the only thing telling them apart.
func (run *invocation) emptyTableRefusal(found installer.SkillMounts) string {
	if len(run.mounting.BulkMounts()) > 0 {
		return ""
	}
	directory := run.Repo + "/kk-flavor/skills/"
	if found.Found > 0 {
		return "every skill under " + directory + " is maintainer-only and none was selected — nothing was mounted"
	}
	return "no skill directories under " + directory + " — nothing was mounted"
}

func (run *invocation) install(found installer.SkillMounts) int {
	if !run.isMaintainer && len(found.SkippedNames) > 0 {
		run.mounting.Say(fmt.Sprintf("  skipped  %d maintainer-only skill(s): %s",
			len(found.SkippedNames), strings.Join(found.SkippedNames, " ")))
	}
	// The rule is read before the ignore region is written, because writing one would bury the answer.
	// A project already ignoring the whole agent directory covers this install and the project's own
	// settings alike. What to do about that is a decision for the human whose repository it is.
	broadRule := run.broadAgentRule()

	if !run.projectFilesWritable() || !run.skillsMountWritable(run.mounting, run.project) {
		return run.mounting.Report()
	}
	run.mounting.AddConfig(run.Repo+"/kk-flavor", run.Home+"/.kk-flavor")
	if !run.surveyExistingMounts() {
		return 1
	}
	if !run.ensureDependencies() {
		return run.mounting.Report()
	}
	if run.Mcp.Configure(run.project, run.agent, run.isDryRun, false) != 0 {
		run.mounting.Refuse("project MCP configuration needs attention; see the error above")
		return run.mounting.Report()
	}
	if !run.mountThroughBucket(run.mounting, run.project) {
		return run.mounting.Report()
	}
	run.enableWorktrees()

	run.mounting.Say("project files")
	run.writeIgnoreRules(broadRule)
	if run.writeInstructions(run.instructionsFile, flavor.RegionBody) {
		// The Claude file imports the shared one and holds no second copy, so the two clients cannot
		// disagree about the instructions. This runs only when the shared file landed: an import naming
		// a file this run refused to write points at a file that is missing.
		run.writeInstructions(run.claudeFile, claudeImport)
	}
	run.mounting.Say("registry")
	run.mounting.RecordInstall(run.project)
	return run.mounting.Report()
}

// The survey, which has to run before the mount sources are rewritten to reach through the bucket.
// After the rewrite every source names ~/.kk-flavor, so the second-checkout guard has no checkout to
// recognise, and a project mounted from somebody else's clone is repointed in silence.
func (run *invocation) surveyExistingMounts() bool {
	var quiet bytes.Buffer
	survey := run.newMountingRun(&quiet, true)
	run.declareSkills(survey)
	survey.AddConfig(run.Repo+"/kk-flavor", run.Home+"/.kk-flavor")
	survey.Mount()
	if len(survey.Refusals()) == 0 {
		return true
	}
	// The survey's output is printed only when it found something. A dry pass that printed regardless
	// would put a second account of every mount beside the real one.
	io.Copy(run.Out, &quiet)
	return false
}

// Mount, with every skill source rewritten to reach through the bucket. A project holds
// ~/.kk-flavor/skills/<name>, the single path every install of this flavor has, and never an absolute
// path into whichever checkout happened to run the installer.
func (run *invocation) mountThroughBucket(mounting *installer.Run, project string) bool {
	bucket := run.Home + "/.kk-flavor"
	if run.isDryRun {
		mounting.Say("  would link " + bucket + " -> " + run.Repo + "/kk-flavor")
		for _, mount := range mounting.BulkMounts() {
			mounting.Say("  would link " + mount.Target + " -> " + bucket + "/skills/" + shell.BaseName(mount.Source))
		}
		return true
	}
	// The bucket first and on its own, because every skill source below resolves through it. A link
	// made against a bucket that could not be written would leave a project full of mounts pointing
	// nowhere.
	if !mounting.Link(run.Repo+"/kk-flavor", bucket) {
		return false
	}
	mounting.RewriteBulkSources(func(source string) string {
		return bucket + "/skills/" + shell.BaseName(source)
	})
	// The mounts are scanned, and scoped to what this checkout wrote. A skill renamed or deleted
	// upstream leaves a mount the table no longer names, and only the run that would have written it
	// can notice.
	mounting.AddUnmountScan(project+"/"+run.agentDirectory+"/skills", run.Repo+"/kk-flavor/skills")
	mounting.Mount()
	return true
}

func (run *invocation) uninstall() int {
	if run.Mcp.Configure(run.project, run.agent, run.isDryRun, true) != 0 {
		run.mounting.Refuse("project MCP configuration needs attention; see the error above")
		return run.mounting.Report()
	}
	if !run.skillsMountWritable(run.mounting, run.project) {
		return run.mounting.Report()
	}
	run.mounting.Unmount()
	run.disableWorktrees()

	run.mounting.Say("project files")
	// The other client's mounts are the reason these files stay. They are shared, and taking them out
	// from under a client that is still installed would leave that client with no instructions to load.
	if run.otherClientMounted() {
		run.mounting.Say("  kept     shared project instructions: another client still uses them")
	} else {
		for _, file := range []string{run.instructionsFile, run.claudeFile} {
			if shell.PathExists(file) || shell.IsSymlink(file) {
				run.mounting.RemoveRegion(file, flavor.RegionOpen, flavor.RegionClose)
			}
		}
	}
	// The ignore rules go with the mounts they were hiding. Only this client's own fenced region, so a
	// rule the human wrote — or the other client's — is not swept up with it.
	if shell.PathExists(run.ignoreFile) {
		run.mounting.RemoveRegion(run.ignoreFile, run.ignoreOpen, run.ignoreClose)
	}

	run.mounting.Say("registry")
	if run.otherClientMounted() {
		run.mounting.Say("  kept     " + run.project + " still has " + run.otherAgentDirectory +
			" skill mounts from this checkout")
	} else {
		run.mounting.ForgetInstall(run.project)
	}

	run.mounting.Say("")
	run.mounting.Say("  Shared " + run.Home + "/.kk-flavor was kept. User-level client setup was not changed.")
	return run.mounting.Report()
}

// Whether the project still holds skills of this checkout's for the other client. Each link is
// resolved on disk, and never string-compared, so a mount written through a differently-spelled but
// equivalent path still counts.
func (run *invocation) otherClientMounted() bool {
	directory := run.project + "/" + run.otherAgentDirectory + "/skills"
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		target := shell.Join(directory, entry.Name())
		value, err := os.Readlink(target)
		// Absolute only: a relative value resolves against the link's own directory, and resolving it
		// here would use the wrong one. Every link written here is absolute.
		if err != nil || !strings.HasPrefix(value, "/") {
			continue
		}
		if shell.IsWithin(shell.CanonicalDir(shell.DirName(value)), run.Repo) {
			return true
		}
	}
	return false
}
