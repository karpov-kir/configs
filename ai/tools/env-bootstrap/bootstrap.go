// Package envbootstrap sets this machine's shell and editor environment up from this repository: link
// every config in env/ into place and install what those links need.
//
//	usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew]
//
// Safe to re-run: every step checks the state it wants before changing anything, so a second run over
// a finished machine reports "ok" throughout and writes nothing.
//
// It will not move a machine that is already mounted from somewhere else. Run from a second checkout —
// a scratch clone, a colleague's copy — every link this writes would be repointed at the copy, and
// deleting the copy afterwards leaves the human with no shell config and no git config. That is
// refused before anything is written; `--relocate` is how you say you mean it.
//
// It refuses rather than deletes. env/README.md's hand-run form is `rm -rf ~/.config/nvim && ln -s ...`,
// which is fine when a human types it having just looked at the directory, and is data loss when a
// tool does it unattended on a machine that already had a real config there. A target this does not
// already own is reported and skipped, and the run exits non-zero with the list.
//
// Independent of the ai bootstrap in both directions: neither reads the other's mounts, and either
// half can be installed on a machine that never gets the other.
package envbootstrap

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"kk-flavor/tools/installer"
	"kk-flavor/tools/machine"
)

// Exit codes on the tools' shared vocabulary. 2 is a grammar this tool did not understand, so nothing
// was attempted. 1 is a target it will not write, and the report at the end of every run is what
// answers it.
const (
	exitDone     = 0
	exitBadUsage = 2
)

// What the report line calls this run. Two installers print through the same machinery, and a human
// reading a terminal has to know which answered.
const label = "env bootstrap"

// How the usage line names this tool. The stub's own basename, and not the path: `usage: <basename>`
// is the string eco-check's two scans anchor on to read a stub's grammar and find the dispatch behind
// it, and a line they cannot match reads as documented to a human while both scans go silent.
const invocation = "bootstrap.sh"

// The formulae and casks these links need, held against env/README.md by the case in `ai/tools`:
// adding one to the README alone would leave it documented and never installed, with every other case
// still green.
var (
	formulae = []string{"zsh-autocomplete", "mise", "hstr", "neovim", "starship"}
	casks    = []string{"ghostty"}
)

// FormulaNames is every brew formula this installer installs, and CaskNames every cask.
//
// Exported for that case. It has to read the shipped README, and the README sits outside this module:
// Go keys a package's test cache on the module it belongs to, so a case here that opened it would
// answer `ok (cached)` over a README that had changed underneath the run.
func FormulaNames() []string { return slices.Clone(formulae) }

// CaskNames is the cask half of that list, exported for the same reason.
func CaskNames() []string { return slices.Clone(casks) }

// Options is everything a run needs that it must not go looking for itself. Nothing here reads the
// environment: the home arrives as a value, which is what lets a suite point a whole run at a
// throwaway one rather than fake a filesystem.
type Options struct {
	// Self is the name the stub was invoked by, which every refusal and the usage line are worded in.
	Self string
	Args []string
	// Repo is the env/ directory this run mounts from — the stub's own, resolved physically.
	Repo string
	Home string
	// ConfigHome is the already-resolved ${XDG_CONFIG_HOME:-$HOME/.config}. Unused by these mounts and
	// taken anyway, because the machinery reads it for the install registry and a run built without it
	// would reach the developer's own.
	ConfigHome string
	// Machine is the commands outside this process a run consults — brew, and nothing else here.
	Machine machine.Machine
	Out     io.Writer
	Err     io.Writer
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
// file it had no business touching, and the two must not arrive as the same fact. A refused invocation
// never builds one, so the run is nil there.
func perform(options Options) (*installer.Run, int) {
	arguments, code, hasStopped := parseArguments(options.Self, options.Args, options.Err)
	if hasStopped {
		return nil, code
	}
	run := installer.NewRun(installer.RunOptions{
		Repo: options.Repo,
		// The name a second copy of this repository is recognised by. Taken from the stub the human ran
		// rather than written down, so a rename cannot leave the guard hunting for a name nothing has.
		ScriptName: options.Self,
		Label:      label,
		ConfigHome: options.ConfigHome,
		DryRun:     arguments.isDryRun,
		Relocate:   arguments.willRelocate,
		Out:        options.Out,
		WriteRoot:  options.WriteRoot,
	})
	declareMounts(run, options.Repo, options.Home)
	// False is the guard stopping before the first write, and the packages step is skipped with it: a
	// machine whose configuration lives in another checkout gets nothing from this run, installs
	// included.
	if run.Mount() {
		installPackages(run, arguments, options.Machine)
	}
	return run, run.Report()
}

// The mount table. Two files at the top of the home, a whole directory, and a file whose parent
// directory does not exist yet — the four shapes, and every one of them individually consequential,
// so each is declared by name rather than counted.
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

// The third value says whether the run stops here — a refused invocation and a printed help both do,
// and they exit differently. Read as a code alone, exit 0 from the help arm is indistinguishable from
// "parsed fine, carry on", which is how an empty invocation reaches the filesystem.
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

// A machine without brew still gets every link. The refusal says which half did not happen, because a
// run that mounted everything and installed nothing must not read as a finished machine.
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

// One refusal does not end the run: a machine missing one cask should still get every other package,
// and a human fixing three named problems in one pass beats discovering them one run at a time.
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
