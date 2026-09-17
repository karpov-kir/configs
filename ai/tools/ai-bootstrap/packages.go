package aibootstrap

import (
	"strings"

	"kk-flavor/tools/machine"
)

// The formulae this install needs, held against ai/README.md by this package's own suite: adding one
// to the README alone would leave it documented and never installed, with every other case green.
//
// jq is everyone's. rtk compresses this machine's shell output for the agent — personal tooling, not
// something the instruction tree needs — so it is the owner tier's and every other tier says so rather
// than passing over it in silence.
var formulae = []struct {
	name         string
	isOwnersOnly bool
}{
	{name: "rtk", isOwnersOnly: true},
	{name: "jq"},
}

// Installed-first rather than an unconditional install: the latter is slow, noisy, and answers
// non-zero on an already-installed formula, which would make a finished machine look broken.
func (run *invocation) installPackages() {
	if run.isBrewSkipped {
		run.mounting.Say("brew (skipped)")
		return
	}
	if !machine.HasBrew(run.Machine) {
		run.mounting.Refuse("brew is not installed, so no formula was installed")
		return
	}
	run.mounting.Say("brew")
	for _, formula := range formulae {
		if formula.isOwnersOnly && !run.isOwner {
			run.mounting.Say("  skipped  " + formula.name + " is the owner tier's")
			continue
		}
		run.installOne(formula.name)
	}
}

// One refusal does not end the run: a machine missing one formula should still get every link, and a
// human fixing three named problems in one pass beats discovering them one run at a time.
func (run *invocation) installOne(name string) {
	spelled := strings.Join(machine.PackageArguments(machine.Formula, name), " ")
	switch {
	case machine.IsPackageInstalled(run.Machine, machine.Formula, name):
		run.mounting.Say("  ok       " + name)
	case run.isDryRun:
		run.mounting.Say("  would install " + spelled)
	case machine.InstallPackage(run.Machine, machine.Formula, name) != 0:
		run.mounting.Refuse("brew install " + spelled + " failed")
	}
}
