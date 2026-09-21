package aibootstrap

import (
	"strings"

	"configs/ai/tools/machine"
)

// The formulae this install needs. This package's own suite holds the list against ai/README.md, so a
// formula added to the README alone stays documented and uninstalled with every other case green.
//
// rtk compresses this machine's shell output for the agent. That is personal tooling the instruction
// tree has no use for, so rtk is the owner tier's and the only formula here.
var formulae = []struct {
	name         string
	isOwnersOnly bool
}{
	{name: "rtk", isOwnersOnly: true},
}

// FormulaNames is every formula this installer installs, whatever the tier, in declared order.
//
// The case holding this list against ai/README.md lives outside this module, and the README does too.
// Go keys a package's test cache on its own module, and a case inside this package reports
// `ok (cached)` over a README that changed underneath it.
func FormulaNames() []string {
	names := make([]string, 0, len(formulae))
	for _, formula := range formulae {
		names = append(names, formula.name)
	}
	return names
}

// Each formula is checked before it is installed. An unconditional install is slow, noisy, and
// answers non-zero on a formula that is already there, which makes a finished machine look broken.
func (run *invocation) installPackages() {
	if run.isBrewSkipped {
		run.mounting.Say("brew (skipped)")
		return
	}
	// The count comes before the brew check. Since jq went, the default tier installs no formula at
	// all, and an install wanting none of them must still run on a machine that lacks brew. The skip
	// lines still print, because a tier that quietly leaves a formula out reads like a step that never
	// ran.
	wanted := 0
	for _, formula := range formulae {
		if formula.isOwnersOnly && !run.isOwner {
			continue
		}
		wanted++
	}
	if wanted == 0 {
		run.mounting.Say("brew (nothing this tier installs)")
		run.sayWhatThisTierSkips()
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

// One refusal does not end the run. A machine missing one formula should still get every link, and a
// human fixing three named problems in one pass beats three separate runs.
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

// What this tier does not install, by name. A formula left out silently reads the same as a step that
// never ran, which is why installPackages prints its own skip lines.
func (run *invocation) sayWhatThisTierSkips() {
	for _, formula := range formulae {
		if formula.isOwnersOnly && !run.isOwner {
			run.mounting.Say("  skipped  " + formula.name + " is the owner tier's")
		}
	}
}
