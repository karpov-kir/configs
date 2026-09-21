package machine

// The env and ai installers ask brew the same two questions, and the kind travels with the name
// through both. `brew list --formula ghostty` exits non-zero for an installed cask, and a call site
// spelling the kind by hand reinstalls the cask on every run. What each installer does with the
// answers stays its own, tiers and wording included.

// PackageKind is which of brew's two catalogues a name lives in.
type PackageKind int

const (
	Formula PackageKind = iota
	Cask
)

func (kind PackageKind) String() string {
	if kind == Cask {
		return "cask"
	}
	return "formula"
}

// HasBrew is whether this machine can install anything at all.
func HasBrew(m Machine) bool {
	return m.HasCommand("brew")
}

// IsPackageInstalled asks installed-first, which is what keeps a finished machine quiet: an
// unconditional install is slow, noisy, and answers non-zero on an already-installed cask.
func IsPackageInstalled(m Machine, kind PackageKind, name string) bool {
	return m.Run(Command{Name: "brew", Args: []string{"list", "--" + kind.String(), name}}) == 0
}

// InstallPackage answers brew's exit code, 0 having installed it.
func InstallPackage(m Machine, kind PackageKind, name string) int {
	return m.Run(Command{Name: "brew", Args: append([]string{"install"}, PackageArguments(kind, name)...)})
}

// PackageArguments is the install as a human would type it, so a refusal and a dry run's preview name
// something that can be re-run by hand.
func PackageArguments(kind PackageKind, name string) []string {
	if kind == Cask {
		return []string{"--cask", name}
	}
	return []string{name}
}
