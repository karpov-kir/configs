// Package aibootstrap installs this repository's agent ecosystem on one machine: the shared
// ~/.kk-flavor bucket, every skill the chosen tier takes, the instruction file's own region, and the
// packages and registries those need.
//
//	usage: bootstrap.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew] [--skip-tools] [--skip-mcp] [--skip-rtk] [--skip-verify] [--uninstall]
//
// Safe to re-run: every step checks the state it wants before changing anything, so a second run over
// a finished machine reports "ok" throughout and writes nothing. It refuses rather than deletes, and
// it will not move a machine whose configuration is mounted from a different checkout.
//
// Uninstall is a mode here rather than a script of its own, over the same table an install declares: a
// second program re-deriving what to remove drifts from what was installed, and drifts in the one
// direction nobody notices — leaving things behind and reporting ok.
//
// Nothing here reads the environment or runs a command directly. The home, the Codex profile and the
// verify marker arrive as values, and everything outside this process goes through machine.Machine.
// That is what lets a suite drive brew's installed-first branch, rtk's argument list and the exit
// codes the tools installer and the gate answer with, none of which the machine running the suite
// could be asked to produce.
package aibootstrap

import (
	"fmt"
	"io"
	"strings"

	"kk-flavor/tools/installer"
	"kk-flavor/tools/machine"
)

// Exit codes on the tools' shared vocabulary. 2 is a grammar this tool did not understand, so nothing
// was attempted. 1 is something a human has to fix, and the report at the end of every run answers it.
const (
	exitDone     = 0
	exitBadUsage = 2
)

// What the report line calls this run. Two installers print through the same machinery, and a human
// reading a terminal has to know which answered.
const label = "ai bootstrap"

// How the usage line names this tool. The stub's own basename, and not the path: `usage: <basename>`
// is the string eco-check's two scans anchor on to read a stub's grammar and find the dispatch behind
// it, and a line they cannot match reads as documented to a human while both scans go silent.
const stubPath = "bootstrap.sh"

const claudeAgent = "claude"

const codexAgent = "codex"

// Options is everything a run needs that it must not go looking for itself.
type Options struct {
	// Self is the name the stub was invoked by, which every refusal leads with and which the
	// second-checkout guard looks for under a candidate root.
	Self string
	Args []string
	// Repo is the ai/ directory this run mounts from — the stub's own, resolved physically.
	Repo string
	Home string
	// CodexHome is the already-resolved ${CODEX_HOME:-$HOME/.codex}.
	CodexHome string
	// ConfigHome is the already-resolved ${XDG_CONFIG_HOME:-$HOME/.config}, where the install registry
	// lives.
	ConfigHome string
	// IsInsideVerify is whether this run was started by another run's verify step. See the verify step
	// for the loop it closes.
	IsInsideVerify bool
	Machine        machine.Machine
	Out            io.Writer
	Err            io.Writer
	// WriteRoot bounds every write to one tree. Empty is unbounded, which is what a real machine runs
	// as; a suite driving the real linking logic against a throwaway home sets it.
	WriteRoot string
}

// Run executes one invocation and answers its exit code.
func Run(options Options) int {
	_, code := perform(options)
	return code
}

// The same run, handing back the machinery's own record of it. A suite reads Breaches off that record:
// a write the containment bound turned away is not a failing case, it is this run having gone for a
// file it had no business touching, and the two must not arrive as the same fact. A refused
// invocation never builds one, so the run is nil there.
func perform(options Options) (*installer.Run, int) {
	parsed, code, hasStopped := parseArguments(options.Self, options.Args, options.Err)
	if hasStopped {
		return nil, code
	}
	run := &invocation{Options: options, arguments: parsed, targets: newTargets(parsed.agent, options)}
	run.mounting = installer.NewRun(installer.RunOptions{
		Repo:       options.Repo,
		ScriptName: options.Self,
		Label:      label,
		BulkLabel:  "skills",
		ConfigHome: options.ConfigHome,
		DryRun:     parsed.isDryRun,
		Relocate:   parsed.willRelocate,
		Out:        options.Out,
		WriteRoot:  options.WriteRoot,
	})
	return run.mounting, run.do()
}

// One invocation's whole context: what was asked for, where it lands, and the machinery it writes
// through. Held together because every step needs the agent and its paths, and a run reading one
// client's files while writing another's is the inconsistency the three must not be able to express.
type invocation struct {
	Options
	arguments
	targets
	mounting *installer.Run
	// Whether the instruction step finished. The rtk step reads it: initialising rtk against an
	// instruction file this run refused to write leaves the machine describing tooling it has not got.
	areInstructionsReady bool
}

// One invocation's flags.
type arguments struct {
	agent         string
	isDryRun      bool
	willRelocate  bool
	isBrewSkipped bool
	areToolsSkipped,
	isMcpSkipped,
	isRtkSkipped,
	isVerifySkipped bool
	isMaintainer bool
	isOwner      bool
	isUninstall  bool
}

// The third value says whether the run stops here — a refused invocation and a printed help both do,
// and they exit differently. Read as a code alone, exit 0 from the help arm is indistinguishable from
// "parsed fine, carry on", which is how an empty invocation reaches the filesystem.
func parseArguments(self string, args []string, stderr io.Writer) (arguments, int, bool) {
	parsed := arguments{}
	for _, arg := range args {
		switch arg {
		case "--agent=" + claudeAgent, "--agent=" + codexAgent:
			parsed.agent = strings.TrimPrefix(arg, "--agent=")
		case "--dry-run":
			parsed.isDryRun = true
		case "--relocate":
			parsed.willRelocate = true
		case "--skip-brew":
			parsed.isBrewSkipped = true
		case "--skip-tools":
			parsed.areToolsSkipped = true
		case "--skip-mcp":
			parsed.isMcpSkipped = true
		case "--skip-rtk":
			parsed.isRtkSkipped = true
		case "--skip-verify":
			parsed.isVerifySkipped = true
		// Opt IN. A tree's own maintenance skills are useless to a machine that only uses the tree, and
		// every skill's description costs context in every session whether or not it is invoked — so the
		// default installs the smaller set and the bigger one is asked for by name.
		case "--maintainer":
			parsed.isMaintainer = true
		// The owner tier: --maintainer, plus rtk and the personal instruction file. ai/bootstrap.sh
		// --owner is the way in, and ai/README.md documents it.
		case "--owner":
			parsed.isOwner = true
			parsed.isMaintainer = true
		case "--uninstall":
			parsed.isUninstall = true
		case "-h", "--help":
			fmt.Fprintln(stderr, usage())
			return parsed, exitDone, true
		default:
			fmt.Fprintf(stderr, "%s: unknown option %s\n", self, arg)
			fmt.Fprintln(stderr, usage())
			return parsed, exitBadUsage, true
		}
	}
	if parsed.agent == "" {
		fmt.Fprintf(stderr, "%s: --agent=%s|%s is required — nothing was changed\n", self, claudeAgent, codexAgent)
		fmt.Fprintln(stderr, usage())
		return parsed, exitBadUsage, true
	}
	return parsed, exitDone, false
}

// One line, and every flag the parser accepts is in it. Two ways that breaks and neither shows up
// anywhere else: a flag added to the parser and never written here is one no reader can find, and a
// flag written here that the parser refuses sends a reader to a run that exits 2. The suite holds the
// two lists against each other.
func usage() string {
	return "usage: " + stubPath + " --agent=" + claudeAgent + "|" + codexAgent +
		" [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew] [--skip-tools] [--skip-mcp]" +
		" [--skip-rtk] [--skip-verify] [--uninstall]"
}

// The whole of one run, in the order the steps have to happen in.
func (run *invocation) do() int {
	// Ahead of everything, because a shadowed instruction file means the run cannot reach the file it
	// would write and carrying on would mount a tree whose instructions nothing loads.
	if refusal := run.shadowedInstructions(); refusal != "" {
		run.mounting.Refuse(refusal)
		return run.mounting.Report()
	}
	run.declareBucket()
	found := run.declareSkills()

	if run.isUninstall {
		return run.uninstall()
	}
	return run.install(found)
}

func (run *invocation) install(found installer.SkillMounts) int {
	// Scanned rather than listed, and declared before the mount: a mount whose source this checkout no
	// longer has is dropped, which linking cannot do — it iterates the sources this tree ships, so a
	// skill deleted upstream leaves its symlink at the mount for good. Not narrowed by tier: a skill
	// left out for want of --maintainer is still in the tree, so its mount still resolves and is not
	// stale.
	run.mounting.AddUnmountScan(run.skillsMount, run.Repo+"/kk-flavor/skills")

	if !run.mounting.Mount() {
		return run.mounting.Report()
	}
	run.migrateLegacyCodexMounts()
	run.reportTier(found)
	if refusal := run.emptyTableRefusal(found); refusal != "" {
		run.mounting.Refuse(refusal)
	}

	run.areInstructionsReady = run.writeInstructions()
	run.installPackages()
	run.configureRtk()
	run.installTools()
	run.syncMcp()
	run.verify()
	return run.mounting.Report()
}
