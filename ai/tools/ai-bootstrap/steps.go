package aibootstrap

import (
	"configs/ai/tools/machine"
	"configs/ai/tools/shell"
)

// --- the repository's own tools -------------------------------------------------------------------

// install.sh's exit codes send a reader to different places. 2 points at this machine's network, its
// auth or the release's own assets. 3 points at a repository that has cut no release at all. Only 3
// is survivable, and collapsing it into the refusal fails every fresh clone until the first release
// exists.
func (run *invocation) installTools() {
	installer := run.Repo + "/tools/install.sh"
	switch {
	case run.areToolsSkipped:
		run.mounting.Say("tools (skipped)")
		return
	case run.isDryRun:
		run.mounting.Say("tools: would run " + installer)
		return
	case !run.Machine.HasCommand("gh"):
		run.mounting.Refuse("gh is not installed, so " + installer + " could not fetch the tool binaries")
		return
	}
	run.mounting.Say("tools")
	switch status := run.Machine.Run(machine.Command{Name: installer, Loud: true}); {
	case status == 0:
	case status == 3:
		// A refusal names something the human at this machine must do, and only this repository's owner
		// can cut a release. Go is checked here, because --skip-verify turns the verify step off. A run
		// would otherwise end green having installed no tool onto a machine that cannot build one
		// either.
		if run.Machine.HasCommand("go") {
			run.mounting.Say("  ok       no release to install from; the tools build from source on first use, which needs Go")
			return
		}
		run.mounting.Refuse("no release to install from and no go on this machine, so the tools can be " +
			"neither downloaded nor built — nothing was installed")
	default:
		run.mounting.Refuse(installer + " failed")
	}
}

// --- the MCP registry -----------------------------------------------------------------------------

func (run *invocation) syncMcp() {
	sync := run.Repo + "/mcp-sync.sh"
	switch {
	case run.isMcpSkipped:
		run.mounting.Say("mcp (skipped)")
	case run.isDryRun:
		run.mounting.Say("mcp: would run " + sync + " --agent=" + run.agent)
	case !run.Machine.HasCommand(run.agent):
		run.mounting.Refuse("the " + run.agent + " CLI is not on PATH, so the MCP servers were not registered")
	default:
		run.mounting.Say("mcp")
		if run.Machine.Run(machine.Command{Name: sync, Args: []string{"--agent=" + run.agent}, Loud: true}) != 0 {
			run.mounting.Refuse(sync + " failed")
		}
	}
}

// --- verify ---------------------------------------------------------------------------------------

// A setup that reports success without running a check proves only that it ran, so the last step is
// the repository's own gate over what was just linked. The gate needs a machine that can run Go, and
// installTools, a step of this run, earns that. A machine that cannot download or build the tool
// binaries is refused there, so by the time verify runs there is a binary or a refusal.
func (run *invocation) verify() {
	gate := run.Repo + "/gate.sh"
	switch {
	// The re-entry marker matters here. The gate runs the Go suites. A case that drives this installer
	// reaches verify, verify runs the gate, and the gate runs that case again. The marker stops that
	// recursion at a failure, and a setup lacking one hangs.
	case run.IsInsideVerify:
		run.mounting.Say("verify (skipped: already inside a verify run)")
		return
	case run.isVerifySkipped:
		run.mounting.Say("verify (skipped)")
		return
	case !shell.IsRegularFile(gate):
		// A missing gate must not be reported as a failing check. The call would exit 127, and the
		// default arm would blame the checks for a file that was never there. A false diagnosis
		// pointing at fine code costs more than silence. The dry run is checked too: a dry run saying
		// "ok" over a checkout where the real run cannot work is the same lie one step earlier.
		run.mounting.Refuse(gate + " is not in this checkout — the checks were not run, which is not the " +
			"same as passing")
		return
	case run.isDryRun:
		run.mounting.Say("verify: would run " + gate)
		return
	}
	run.mounting.Say("verify")
	status := run.Machine.Run(machine.Command{
		Name: gate,
		// The checkout root, because the gate scopes every check against the repository it is standing
		// in and ai/ is not one.
		Directory: shell.DirName(run.Repo),
		Env:       []string{verifyMarker + "=1"},
		Loud:      true,
	})
	switch status {
	case 0:
	case 2:
		// The gate's "unknown": a check that could not run, or one whose input set came out empty. Neither
		// is a finding about the code, and a refusal saying otherwise sends the reader to the wrong place.
		// So this points at the gate's own output, where the check that never measured printed its reason.
		run.mounting.Refuse(gate + " could not measure every check — unproven is not disproven, and it is " +
			"not passing either. The gate named the checks it could not measure and each said why above")
	default:
		run.mounting.Refuse(gate + " reported a failing check")
	}
}

// The environment variable a verify run sets so the run it starts knows to skip verifying. The
// command reads it, and this package takes it as a value.
const verifyMarker = "BOOTSTRAP_VERIFYING"
