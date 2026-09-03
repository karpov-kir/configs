// Package ecocheck is the mechanical half of kk-ecosystem: every reference an agent could follow
// resolves to something that exists, and every script still parses.
//
// It is a library with a thin command beside it, because the suite that proves it drives it once
// per case and a process spawn per case is the cost that makes a mutation run take hours. Nothing
// here writes to os.Stdout or calls os.Exit: Run reports through the writers it is handed and returns
// the code the command exits on. Every emit counter lives on the checker Run builds, so two runs in
// one process cannot see each other's. The one thing held across them is scripts.go's `bash -n` memo,
// keyed on file content so a second run is answered with what the first parsed.
//
// Most of the density below is hardening against a hostile tree, because this runs as kk-pr-review's
// stage over a branch that chose its own contents: NUL bytes in files, newlines in committed
// filenames, symlinks at every path it touches, control bytes in anything echoed into a finding,
// unbounded emit counts, and paths that resolve outside the root. A change here needs a case in the
// suite beside it, and a scan you add needs one that fails without it — `ai/tools/go-mutate` is what
// shows a case can fail. `check.sh` in kk-ecosystem's scripts/ is the stub that reaches this binary.
package ecocheck

import (
	"fmt"
	"io"

	ecoroot "kk-flavor/tools/eco-root"
)

// The bound each shape of the direction scan emits under, and the per-class bound the printer
// applies. Both are here so raising either is one edit.
const findingCap = 40

// How many budget-file refusals are named before the rest are summarised. Named where the relation to
// the suppression note is visible: written as a bare 5 and 6, a change to one silently outruns the
// other and the note never prints.
const budgetRefusalCap = 5

type checker struct {
	// The checkout under review, which holds the root exactly as it was named: every finding
	// echoes a path built from it.
	root ecoroot.Root

	findings []string
	trees    map[string]*tree

	// The paths --gate takes out of every walk, or nil with the flag off. gate.go holds what the flag
	// answers and why the default is off.
	gate *gateFilter

	// Which file holds each heading in the tree, built on the first dangling citation and never for
	// a clean tree. It names the one dangling variant the cited file's own contents cannot: the
	// section is real and the file moved.
	headingOwners map[string]string

	// Every script under the root, by the basename a call site names it with. A basename is not a
	// file, so a finding reached through one resolves it back to a path here, or says it cannot.
	scriptOwners map[string][]string

	// Each markdown file's headings and bolded runs, parsed once per run. The citation scan asks for
	// its target's headings once per citation, and the reviewed tree writes how many citations one
	// file carries as freely as it writes how large the target is.
	headings map[string]map[string]string
	bolded   map[string]map[string]string

	// The refused budget files are named in their findings, so their count is bounded too.
	budgetRefusals int

	// Scans that could not run at all, each with what did not happen. Distinct from a scan that ran
	// and found nothing, which is what exit 0 says — see exitCode.
	unrunnable []string

	// How the parse scan finds the bash binaries to fork. A field so a case can hand it an empty
	// list; installedBashBinaries says why that needed a seam.
	bashBinaries func() []string
}

// Run checks the tree under args and writes the report to out. The arguments are an optional root and
// an optional --gate, in either order: an empty root means the two candidates ecoroot tries, in order,
// and --gate narrows the walk to what a commit can carry (gate.go). It returns the process exit code:
// 0 clean, 1 with findings, 2 when it could not run — as a whole, or in any one scan.
// A check that did not run is not a clean one, which is why the last is not folded into either of the
// others.
func Run(args []string, out, errOut io.Writer) int {
	root, isGate, ok := parseArgs(args)
	if !ok {
		return refuseToRun(errOut, "usage: check.sh ["+gateFlag+"] [<root>]")
	}
	c, found := newChecker(root)
	if !found {
		named := root
		if named == "" {
			named = ". and ./ai"
		}
		return refuseToRun(errOut, fmt.Sprintf("no root holding both kk-flavor/ and skills/ (tried '%s')", named))
	}
	// Before the first scan, so a run that could not ask which files a commit carries reports nothing
	// rather than a page of findings over the unfiltered tree it was told not to judge.
	if isGate {
		if err := c.enableGate(); err != nil {
			return refuseToRun(errOut, err.Error())
		}
	}

	c.scanMounts()
	c.scanDanglingLinks()
	c.scanHomeRefs()
	c.scanDirection()
	c.scanFamilyDirection()
	c.scanPathRefs()
	c.scanCitations()
	c.scanBareRuleIDs()
	c.scanUnknownSkills()
	c.scanSkillDirectories()
	c.scanScriptsParse()
	c.scanSubcommandCallSites()
	c.scanTestPositions()
	c.scanSharedRegions()

	c.reportGate(out)
	c.reportSkippedMountScan(out)
	c.reportBudget(out)
	c.reportDescriptionCensus(out)
	return c.exitCode(out, errOut)
}

// The root and the flag, in either order. Only the exact flag is a flag and everything else is a path,
// because a root may legitimately be spelled `-r` and this tool opens the paths it is handed rather
// than parsing them — TestAScriptUnderADashLeadingRootIsParsedAndNotReadAsAnOption is that rule.
//
// So a mistyped flag lands as the root and reports that no checkout is there, which is loud and never
// a run that quietly went unfiltered. A second path is refused instead of overwriting the first: two
// roots is a caller that does not know which tree it asked about.
func parseArgs(args []string) (root string, isGate, ok bool) {
	hasRoot := false
	for _, arg := range args {
		switch {
		case arg == gateFlag:
			isGate = true
		case hasRoot:
			return "", false, false
		default:
			root, hasRoot = arg, true
		}
	}
	return root, isGate, true
}

// The one wording for a run that never started: what did not happen, then the line that stops a caller
// reading exit 2 as clean.
func refuseToRun(errOut io.Writer, reason string) int {
	fmt.Fprintf(errOut, "check.sh: %s\n", reason)
	fmt.Fprintln(errOut, "check.sh: exit 2 — nothing was checked. Fix the invocation; do not read this as clean.")
	return 2
}

// The code this check answers with. A scan that could not run outranks the finding count: 0 and 1 both
// say the tree was checked, and one of them would be read as a pass over a question nobody asked. The
// findings still print — what was checked is still worth reading — and the reason rides stderr beside
// the same wording the no-root path uses.
func (c *checker) exitCode(out, errOut io.Writer) int {
	status := c.printFindings(out)
	if len(c.unrunnable) == 0 {
		return status
	}
	for _, reason := range c.unrunnable {
		fmt.Fprintf(errOut, "check.sh: %s\n", reason)
	}
	fmt.Fprintln(errOut, "check.sh: exit 2 — part of this check did not run. Do not read this as clean.")
	return 2
}

// A scan reporting that it could not run at all, rather than running and finding nothing.
func (c *checker) cannotRun(reason string) {
	c.unrunnable = append(c.unrunnable, reason)
}

func newChecker(root string) (*checker, bool) {
	resolved, ok := ecoroot.New(root)
	if !ok {
		return nil, false
	}
	return &checker{root: resolved, trees: map[string]*tree{}, bashBinaries: installedBashBinaries}, true
}

func (c *checker) add(finding string) {
	c.findings = append(c.findings, finding)
}
