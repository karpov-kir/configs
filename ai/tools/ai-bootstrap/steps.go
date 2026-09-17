package aibootstrap

import (
	"kk-flavor/tools/machine"
	"kk-flavor/tools/shell"
)

// --- the repository's own tools -------------------------------------------------------------------

// install.sh's exit codes send a reader to different places: 2 to this machine's network, its auth or
// the release's own assets, and 3 to the fact that this repository has cut no release at all. Only the
// second is survivable, and collapsing it into the refusal fails every fresh clone until the first
// release exists.
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
		// Not a failure, and reporting it as one would fail every fresh clone until the first release is
		// cut: a refusal names something the human at this machine must do, and only this repository's
		// owner can cut a release. Go is checked here rather than left to the verify step, because
		// --skip-verify turns that off and without this a run would end green having installed no tools
		// onto a machine that cannot build them either.
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

// A setup that reports success without checking anything has reported nothing, so the last step is the
// repository's own gate over what was just linked.
//
// The gate and not ai/run-tests.sh. That runner stayed shell because a verify could land on a machine
// with no Go — a constraint this file drops: the step above installs the tool binaries and already
// refuses a machine that can neither download nor build them, so by the time verify runs there is a
// binary or there is a refusal.
//
// The re-entry marker is load-bearing. The gate discovers every `*-test.sh` and runs it, so a suite
// that drives this installer would reach verify again, and verify would run the gate again. Nothing in
// the tree does that today; the marker is what stops the one written tomorrow from hanging a machine
// setup instead of failing it.
func (run *invocation) verify() {
	gate := run.Repo + "/gate.sh"
	switch {
	case run.IsInsideVerify:
		run.mounting.Say("verify (skipped: already inside a verify run)")
		return
	case run.isVerifySkipped:
		run.mounting.Say("verify (skipped)")
		return
	case !shell.IsRegularFile(gate):
		// A missing gate must not be reported as a failing check. Without this the call exits 127 and the
		// arm below blames the checks for a file that was never there — a false diagnosis pointing at
		// code that is fine, which costs more than the silence would. Checked in the dry run too: a dry
		// run that says "ok" over a checkout where the real run cannot work is the same lie one step
		// earlier.
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
		// The gate's "nothing is known": a unit that could not run, one whose input set resolved to
		// nothing, or one that ran and then refused its own result because the checkout moved underneath
		// it. None of them is a finding about the code, and a refusal saying otherwise sends the reader
		// to the wrong place.
		run.mounting.Refuse(gate + " could not measure every check — unproven is not disproven, and it is " +
			"not passing either. If it says a check refused its own result, re-run once nothing else is " +
			"writing in this checkout")
	default:
		run.mounting.Refuse(gate + " reported a failing check")
	}
}

// The environment variable a verify run sets so the run it starts knows not to verify again. Read by
// the command rather than by this package, which takes it as a value.
const verifyMarker = "BOOTSTRAP_VERIFYING"
