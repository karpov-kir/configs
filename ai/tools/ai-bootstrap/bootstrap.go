// Package aibootstrap installs this repository's agent ecosystem on one machine. That is the shared
// ~/.kk-flavor bucket, the skills the chosen tier takes, the instruction file's own region, and the
// packages and registries those need. `usage()` holds the grammar.
//
// Every step checks the state it wants before it writes, so a second run over a finished machine
// reports "ok" throughout and writes no file. It refuses a target it does not own, and it stops
// before moving a machine whose configuration is mounted from another checkout. Uninstall is a mode
// of this program, over the same table an install declares, and uninstall() says why.
package aibootstrap

import (
	"fmt"
	"io"
	"strings"

	"configs/ai/tools/installer"
	"configs/ai/tools/machine"
)

// Exit codes on the tools' shared vocabulary. 2 is a grammar this tool could not read, so the run
// stopped before its first step. 1 is something a human has to fix, and the report at the end of
// every run names it.
const (
	exitDone     = 0
	exitBadUsage = 2
)

// What the report line calls this run. Two installers print through the same machinery, and a human
// reading a terminal has to know which answered.
const label = "ai bootstrap"

// How the usage line names this tool: the stub's basename, without the path. eco-check anchors two
// scans on the string `usage: <basename>`, one reading a stub's grammar and one finding the dispatch
// behind it. A line that matches neither scan reads as documented to a human while both go quiet.
const stubPath = "bootstrap.sh"

const claudeAgent = "claude"

const codexAgent = "codex"

// Options is everything a run needs that it must not go looking for itself. The home, the Codex
// profile and the verify marker arrive as values, and everything outside the process goes through
// machine.Machine. That lets a suite drive brew's installed-first branch, rtk's argument list and the
// exit codes the tools installer and the gate answer with, which a real machine cannot produce.
type Options struct {
	// Self is the name the stub was invoked by. Every refusal leads with it, and the second-checkout
	// guard looks for it under a candidate root.
	Self string
	Args []string
	// Repo is the ai/ directory this run mounts from, which is the stub's own, resolved physically.
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
	// as. A suite driving the real linking logic against a throwaway home sets it.
	WriteRoot string
}

// Run executes one invocation and answers its exit code.
func Run(options Options) int {
	_, code := perform(options)
	return code
}

// The same run, handing back the machinery's own record of it. A suite reads Breaches off that
// record. A write the containment bound turned away is this run going for a file it has no business
// touching, which is a different fact from a failing case. A refused invocation builds no run, so the
// run is nil there.
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
// through. The three are held together because every step needs the agent and its paths. A run
// reading one client's files and writing another's is the inconsistency they must not express.
type invocation struct {
	Options
	arguments
	targets
	mounting *installer.Run
	// Whether the instruction step finished. The rtk step reads it, because an rtk pointed at an
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
	isVerifySkipped,
	areModelsSkipped bool
	isMaintainer bool
	isOwner      bool
	isUninstall  bool
}

// The third value says whether the run stops here. A refused invocation and a printed help both do,
// and they exit differently. A caller reading the exit code alone cannot tell the help arm's 0 from
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
		case "--skip-models":
			parsed.areModelsSkipped = true
		// Opt IN. A tree's own maintenance skills are useless to a machine that only uses the tree, and
		// every skill's description costs context in every session whether or not it is invoked. So the
		// default installs the smaller set, and the bigger set is asked for by name.
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

// One line, and every flag the parser accepts is in it. A flag added to the parser and left out here
// is one a reader can never find. A flag written here that the parser refuses sends a reader to a run
// that exits 2. Neither failure shows up anywhere else, so the suite holds the two lists against
// each other.
func usage() string {
	return "usage: " + stubPath + " --agent=" + claudeAgent + "|" + codexAgent +
		" [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew] [--skip-tools] [--skip-mcp]" +
		" [--skip-rtk] [--skip-verify] [--skip-models] [--uninstall]"
}

// The whole of one run, in the order the steps have to happen in.
func (run *invocation) do() int {
	// First, because a shadowed instruction file puts the file this run would write out of reach. A run
	// that carried on would mount a tree whose instructions no client loads.
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
	// The scan is declared before the mount, and it drops a mount whose source this checkout has lost.
	// The linking step alone iterates the sources this tree ships, so a skill deleted upstream keeps
	// its symlink at the mount for good. The scan covers every tier, because a skill left out for want
	// of --maintainer is still in the tree and its mount still resolves.
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
	run.checkModels()
	return run.mounting.Report()
}
