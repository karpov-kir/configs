// Package ecocheck is the mechanical half of kk-ecosystem: every reference an agent could follow
// resolves to something that exists, and every script still parses.
//
// It is a library with a thin command beside it, because the suite that proves it drives it once
// per case and a process spawn per case is the cost that makes a mutation run take hours. Nothing
// here writes to os.Stdout or calls os.Exit — Run reports through the writers it is handed and
// returns the code the command exits on — and nothing here holds state between calls, so two runs
// in one process cannot see each other's emit counters.
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

type checker struct {
	// The checkout under review, which holds the root exactly as it was named: every finding
	// echoes a path built from it.
	root ecoroot.Root

	findings []string
	trees    map[string]*tree

	// The refused budget files are named in their findings, so their count is bounded too.
	budgetRefusals int
}

// Run checks the tree under root and writes the report to out. An empty root means the two
// candidates the shell version tried, in order. It returns the process exit code: 0 clean, 1 with
// findings, 2 when it could not run at all. A check that did not run is not a clean one, which is
// why the last is not folded into either of the others.
func Run(root string, out, errOut io.Writer) int {
	c, ok := newChecker(root)
	if !ok {
		named := root
		if named == "" {
			named = ". and ./ai"
		}
		fmt.Fprintf(errOut, "check.sh: no root holding both kk-flavor/ and skills/ (tried '%s')\n", named)
		fmt.Fprintln(errOut, "check.sh: exit 2 — nothing was checked. Fix the invocation; do not read this as clean.")
		return 2
	}

	c.scanMounts()
	c.scanDanglingLinks()
	c.scanHomeRefs()
	c.scanDirection()
	c.scanPathRefs()
	c.scanCitations()
	c.scanBareRuleIDs()
	c.scanUnknownSkills()
	c.scanSkillDirectories()
	c.scanScriptsParse()
	c.scanSubcommandCallSites()
	c.scanTestPositions()
	c.scanSharedRegions()

	c.reportBudget(out)
	c.reportDescriptionCensus(out)
	return c.printFindings(out)
}

func newChecker(root string) (*checker, bool) {
	resolved, ok := ecoroot.New(root)
	if !ok {
		return nil, false
	}
	return &checker{root: resolved, trees: map[string]*tree{}}, true
}

func (c *checker) add(finding string) {
	c.findings = append(c.findings, finding)
}
