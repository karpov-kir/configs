// Package envbootstrap sets this machine's shell and editor environment up from this repository: link
// every config in env/ into place and install what those links need. `usage()` holds the grammar.
//
// Every step checks the state it wants before it writes, so a second run over a finished machine
// reports "ok" and writes no file. A config already mounted from another checkout stops the run
// before its first write, because every link would point into that copy, and deleting the copy
// strands the human. `--relocate` is how you say you mean it. A target this run does not already
// own is reported and skipped, because a tool deleting a real config unattended is data loss. The
// ai bootstrap installs separately, and neither half reads the other's mounts.
package envbootstrap

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"configs/ai/tools/installer"
	"configs/ai/tools/machine"
)

// Exit codes on the tools' shared vocabulary. 2 is a grammar this tool could not read, so the run
// stopped before its first step. 1 is a target it will not write, and the report at the end of every
// run names that target.
const (
	exitDone     = 0
	exitBadUsage = 2
)

// What the report line calls this run. Two installers print through the same machinery, and a human
// reading a terminal has to know which answered.
const label = "env bootstrap"

// How the usage line names this tool: the stub's basename, without the path. eco-check anchors two
// scans on the string `usage: <basename>`, one reading a stub's grammar and one finding the dispatch
// behind it. A line that matches neither scan reads as documented to a human while both go quiet.
const invocation = "bootstrap.sh"

// The formulae and casks these links need. ai/tools/installer_readmes_test.go holds this list against
// env/README.md. A formula added to the README alone would stay documented and uninstalled, with every
// other case still green.
var (
	formulae = []string{"zsh-autocomplete", "mise", "hstr", "neovim", "starship"}
	casks    = []string{"ghostty"}
)

// FormulaNames is every brew formula this installer installs, and CaskNames every cask.
//
// The case holding this list against env/README.md lives outside this module, and the README does too.
// Go keys a package's test cache on its own module, and a case inside this package reports
// `ok (cached)` over a README that changed underneath it.
func FormulaNames() []string { return slices.Clone(formulae) }

// CaskNames is the cask half of that list.
func CaskNames() []string { return slices.Clone(casks) }

// Options is everything a run needs that it must not go looking for itself. Every field arrives as a
// value, and the package reads no environment of its own. A suite points a whole run at a throwaway
// home through these fields, and fakes no filesystem.
type Options struct {
	// Self is the name the stub was invoked by, which every refusal and the usage line are worded in.
	Self string
	Args []string
	// Repo is the env/ directory this run mounts from — the stub's own, resolved physically.
	Repo string
	Home string
	// ConfigHome is the already-resolved ${XDG_CONFIG_HOME:-$HOME/.config}. These mounts never touch it,
	// and the installer machinery reads it for the install registry. A run built without it reaches the
	// developer's own registry.
	ConfigHome string
	// Machine is the commands outside this process a run consults, and here that is brew alone.
	Machine machine.Machine
	Out     io.Writer
	Err     io.Writer
	// WriteRoot bounds every write to one tree, and empty is unbounded. A real machine runs unbounded,
	// and a suite driving the real linking logic against a throwaway home sets this field.
	WriteRoot string
}

// Run executes one invocation and answers its exit code.
func Run(options Options) int {
	_, code := perform(options)
	return code
}

// Runs one invocation and hands back the machinery's own record of it. A suite reads Breaches off the
// record to catch a write the containment bound turned away, which is this run reaching for a file
// outside the tree it was given. A refused invocation builds no record, so the run is nil there.
func perform(options Options) (*installer.Run, int) {
	arguments, code, hasStopped := parseArguments(options.Self, options.Args, options.Err)
	if hasStopped {
		return nil, code
	}
	run := installer.NewRun(installer.RunOptions{
		Repo: options.Repo,
		// The name a second copy of this repository is recognised by. It comes from the stub the human
		// ran, so a rename cannot leave the guard looking for a filename this repository stopped using.
		ScriptName: options.Self,
		Label:      label,
		ConfigHome: options.ConfigHome,
		DryRun:     arguments.isDryRun,
		Relocate:   arguments.willRelocate,
		Out:        options.Out,
		WriteRoot:  options.WriteRoot,
	})
	declareMounts(run, options.Repo, options.Home)
	// False is the guard stopping before the first write, and the packages step goes with it. A machine
	// whose configuration lives in another checkout is left untouched, installs included.
	if run.Mount() {
		installPackages(run, arguments, options.Machine)
	}
	return run, run.Report()
}

// The mount table. Four shapes go through it: two files at the top of the home, a whole directory, and
// a file whose parent directory does not exist yet.
func declareMounts(run *installer.Run, repo, home string) {
	run.AddConfig(repo+"/zsh/.zpreztorc", home+"/.zpreztorc")
	run.AddConfig(repo+"/zsh/.zshrc", home+"/.zshrc")
	run.AddConfig(repo+"/git/.gitconfig", home+"/.gitconfig")
	run.AddConfig(repo+"/ghostty", home+"/.config/ghostty")
	run.AddConfig(repo+"/nvim", home+"/.config/nvim")
	run.AddConfig(repo+"/starship/starship.toml", home+"/.config/starship.toml")
}

// One invocation's flags.
type arguments struct {
	isDryRun      bool
	willRelocate  bool
	isBrewSkipped bool
}

// The third value says whether the run stops here. A refused invocation and a printed help both stop,
// and they exit differently. A caller reading the code alone cannot tell exit 0 from the help arm
// apart from "parsed fine, carry on", which is how an empty invocation reaches the filesystem.
func parseArguments(self string, args []string, stderr io.Writer) (arguments, int, bool) {
	parsed := arguments{}
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			parsed.isDryRun = true
		case "--relocate":
			parsed.willRelocate = true
		case "--skip-brew":
			parsed.isBrewSkipped = true
		case "-h", "--help":
			fmt.Fprintln(stderr, usage())
			return parsed, exitDone, true
		default:
			fmt.Fprintf(stderr, "%s: unknown option %s\n", self, arg)
			fmt.Fprintln(stderr, usage())
			return parsed, exitBadUsage, true
		}
	}
	return parsed, exitDone, false
}

func usage() string {
	return "usage: " + invocation + " [--dry-run] [--relocate] [--skip-brew]"
}

// A machine without brew still gets every link. The refusal names the half that did not happen, so the
// human reading it knows the machine is unfinished.
func installPackages(run *installer.Run, parsed arguments, host machine.Machine) {
	if parsed.isBrewSkipped {
		run.Say("brew (skipped)")
		return
	}
	if host == nil || !machine.HasBrew(host) {
		run.Refuse("brew is not installed, so no formula or cask was installed")
		return
	}
	run.Say("brew")
	for _, name := range formulae {
		installOne(run, parsed, host, machine.Formula, name)
	}
	for _, name := range casks {
		installOne(run, parsed, host, machine.Cask, name)
	}
}

// One refusal does not end the run. A machine missing one cask still gets every other package, and a
// human fixes three named problems in one pass.
func installOne(run *installer.Run, parsed arguments, host machine.Machine, kind machine.PackageKind, name string) {
	spelled := strings.Join(machine.PackageArguments(kind, name), " ")
	switch {
	case machine.IsPackageInstalled(host, kind, name):
		run.Say("  ok       " + name)
	case parsed.isDryRun:
		run.Say("  would install " + spelled)
	case machine.InstallPackage(host, kind, name) != 0:
		run.Refuse("brew install " + spelled + " failed")
	}
}
