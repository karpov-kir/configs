package projectsetup

import "configs/ai/tools/machine"

// mise is the one prerequisite a project install has. It reuses a working installation or installs
// only mise through an existing Homebrew; with neither, it says what to do and stops rather than
// installing a package manager on somebody's machine to get at a runtime.
//
// Verified rather than assumed present: a mise on PATH that cannot answer `--version` is a broken
// executable, and every step after this one would fail somewhere less obvious.
const miseInstructions = "https://mise.jdx.dev/installing-mise.html"

func (run *invocation) ensureDependencies() bool {
	if run.Machine.HasCommand("mise") {
		return run.verifyMise()
	}
	if !machine.HasBrew(run.Machine) {
		run.mounting.Refuse("mise is required and brew is unavailable. Install mise using " +
			miseInstructions + ", add it to PATH, then retry.")
		return false
	}
	if run.isDryRun {
		run.mounting.Say("  would run HOMEBREW_NO_AUTO_UPDATE=1 brew install mise")
		return true
	}
	run.mounting.Say("  installing mise with existing Homebrew")
	// The auto-update suppressed, because an install of one formula that first updates every tap turns
	// a project setup into a several-minute wait nobody asked for.
	status := run.Machine.Run(machine.Command{
		Name: "brew", Args: []string{"install", "mise"},
		Env: []string{"HOMEBREW_NO_AUTO_UPDATE=1"}, Loud: true,
	})
	if status != 0 {
		run.mounting.Refuse("brew install mise failed; resolve the Homebrew error, then retry.")
		return false
	}
	if !run.Machine.HasCommand("mise") {
		run.mounting.Refuse("brew completed but mise is unavailable on PATH. " +
			"Add the Homebrew bin directory to PATH, then retry.")
		return false
	}
	return run.verifyMise()
}

func (run *invocation) verifyMise() bool {
	if run.Machine.Run(machine.Command{Name: "mise", Args: []string{"--version"}}) != 0 {
		run.mounting.Refuse("mise --version failed. Repair the mise executable on PATH, then retry: " +
			miseInstructions)
		return false
	}
	run.mounting.Say("  ok       mise is available on PATH")
	return true
}
