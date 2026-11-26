package aibootstrap

import (
	"strings"

	"configs/ai/tools/machine"
)

// The formulae this install needs. This package's own suite holds the list against ai/README.md, so a
// formula added to the README alone stays documented and uninstalled with every other case green.
//
// rtk compresses this machine's shell output for the agent, and it is the only formula here.
var formulae = []string{"rtk"}

// FormulaNames is every formula this installer installs, in declared order.
//
// The case holding this list against ai/README.md lives outside this module, and the README does too.
// Go keys a package's test cache on its own module, and a case inside this package reports
// `ok (cached)` over a README that changed underneath it.
func FormulaNames() []string {
	return append([]string(nil), formulae...)
}

// Each formula is checked before it is installed. An unconditional install is slow, noisy, and
// answers non-zero on a formula that is already there, which makes a finished machine look broken.
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
		run.installOne(formula)
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
