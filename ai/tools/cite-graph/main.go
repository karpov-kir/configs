// Measures the shape of what agents read: how deep a consumer reaches for a rule, how wide a file's
// surface is, and which of its sections nothing enters by.
//
//	usage: cite-graph <root>
//
// Three finders, and not one of them a target. Each reads the tree through a proxy, so moving a
// number and improving what agents read are separate acts: a door is how a consumer reaches a rule it
// needs, so de-duplicating correctly raises the door count, and chasing that count down puts rules
// where they do not belong. Read a figure as a place to go and look, then act on what is there.
//
//   - DEPTH. A consumer reaching a rule through a chain of hops is a defect even when every hop is a
//     legal single home. Each link is individually correct and the chain is still too long.
//   - FAN-OUT. A file entered at one section has an interface. A file entered at nine has an open
//     surface, and every consumer is reaching for something different: missing encapsulation, seen
//     from outside.
//   - UNENTERED SECTIONS. A section nothing cites is reachable only by reading the whole file. It is
//     either dead, or a rule that cannot be reused because no one can name it.
//
// The citation form is the interface, so that is what this reads: `<file>.md → **Section**`, the form
// `ecosystem.md` → **Conventions a new file joins** requires and check.sh already validates. Prose
// mentioning a file without that form is a mention, not a dependency, and counting it would make the
// graph denser than the tree really is.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

// Every name this report prints is one the tree chose — a path it named a file with, a heading it
// wrote. The stderr messages sanitise each one; the report is read on the same terminal and so does
// too. Printed raw, a heading carrying ESC erases the rows above it, and a path carrying a newline
// forges a row of this tool's own report.
func printable(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = shell.Oneline(name)
	}
	return out
}

// What the report has to say about one cited file.
type target struct {
	// Every section entered, by a door or by a citer holding the file whole. This is what the
	// UNENTERED report is read against: a section a precision citer names is not unentered.
	enteredAt map[string]bool
	// The doors only: a citer that does not hold the file whole, and the sections it enters through.
	doorSections    map[string]bool
	doorCiters      map[string]bool
	precisionCiters map[string]bool
	// Per door citer, the sections it enters. A door entering more than one is a DEEP door.
	reach map[string]map[string]bool
}

func newTarget() *target {
	return &target{
		enteredAt:       map[string]bool{},
		doorSections:    map[string]bool{},
		doorCiters:      map[string]bool{},
		precisionCiters: map[string]bool{},
		reach:           map[string]map[string]bool{},
	}
}

// The graph the report walks and the per-target counts it prints, plus how many distinct file-to-file
// edges the citations amount to — several citations between one pair are one edge.
func summarize(edges []edge) (adj map[string][]string, targets map[string]*target, distinct int) {
	adj, targets = map[string][]string{}, map[string]*target{}
	seenEdge := map[string]bool{}
	for _, e := range edges {
		if !seenEdge[e.from+">"+e.to] {
			seenEdge[e.from+">"+e.to] = true
			adj[e.from] = append(adj[e.from], e.to)
		}
		if targets[e.to] == nil {
			targets[e.to] = newTarget()
		}
		t := targets[e.to]
		t.enteredAt[e.section] = true
		if e.precision {
			t.precisionCiters[e.from] = true
			continue
		}
		t.doorSections[e.section] = true
		t.doorCiters[e.from] = true
		if t.reach[e.from] == nil {
			t.reach[e.from] = map[string]bool{}
		}
		t.reach[e.from][e.section] = true
	}
	return adj, targets, len(seenEdge)
}

// The longest chain, printed and returned in hops.
func reportDepth(out, errOut io.Writer, adj map[string][]string, nodes []string) int {
	deepest := []string{}
	budget := &walkBudget{left: walkSteps}
	for _, n := range nodes {
		if got := longest(adj, n, budget); len(got) > len(deepest) {
			deepest = got
		}
	}
	if budget.exhausted() {
		fmt.Fprintf(errOut, "graph too densely connected to walk exhaustively — the depth below is a LOWER BOUND, not the longest chain\n")
	}
	fmt.Fprintf(out, "DEPTH  longest path through the graph is %d hop(s) — a coupling measure, not\n"+
		"       hops any one consumer walks. No reader follows this chain end to end:\n  %s\n\n",
		len(deepest)-1, strings.Join(printable(deepest), "\n    → "))
	return len(deepest) - 1
}

// One target's row in the fan-out table.
type fanOutRow struct {
	file                                            string
	doorSections, doors, deepDoors, precisionCiters int
}

// The door surface per file, widest first, and the width of the widest.
func reportFanOut(out io.Writer, targets map[string]*target) int {
	fmt.Fprintln(out, "FAN-OUT  doors are citers that do NOT hold the file whole. A citer that reads it whole is")
	fmt.Fprintln(out, "         being precise about which rule, not entering. Of the doors, a DEEP one enters more")
	fmt.Fprintln(out, "         than one section — it uses the file enough to read it whole, and that is the debt.")
	fmt.Fprintln(out, "         A door entering one section wants one rule from a file it need not load; that is")
	fmt.Fprintln(out, "         what cutting a restatement correctly produces, and it is not debt.")
	var rows []fanOutRow
	for file, t := range targets {
		deepDoors := 0
		for _, sections := range t.reach {
			if len(sections) > 1 {
				deepDoors++
			}
		}
		rows = append(rows, fanOutRow{
			file:            file,
			doorSections:    len(t.doorSections),
			doors:           len(t.doorCiters),
			deepDoors:       deepDoors,
			precisionCiters: len(t.precisionCiters),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].doorSections != rows[j].doorSections {
			return rows[i].doorSections > rows[j].doorSections
		}
		return rows[i].file < rows[j].file
	})
	for _, r := range rows {
		fmt.Fprintf(out, "  %-42s %2d door section(s) / %2d door(s), %d of them deep, %2d precision citer(s)\n",
			shell.Oneline(r.file), r.doorSections, r.doors, r.deepDoors, r.precisionCiters)
	}
	if len(rows) == 0 {
		return 0
	}
	return rows[0].doorSections // sorted widest-first above
}

// The sections nothing enters, and how many of those are in the shared layer.
func reportUnentered(out io.Writer, defined map[string]map[string]bool, targets map[string]*target, routed map[string]bool, nodes []string) (unentered, shared int) {
	fmt.Fprintln(out, "\nUNENTERED  defined, but no file names it — a finding only where readers enter by section.")
	fmt.Fprintln(out, "           A file the router loads is entered whole on its trigger, and a skill's sections")
	fmt.Fprintln(out, "           are its own, so neither is listed. What remains is a file reached only by")
	fmt.Fprintln(out, "           citation, holding a rule nothing reaches.")
	for _, f := range nodes {
		var dead []string
		for s := range defined[f] {
			if targets[f] == nil || !targets[f].enteredAt[s] {
				dead = append(dead, s)
			}
		}
		sort.Strings(dead)
		if len(dead) == 0 {
			continue
		}
		unentered += len(dead)
		if !strings.HasPrefix(f, "skills/") && !routed[f] {
			shared += len(dead)
			fmt.Fprintf(out, "  shared  %-34s %s\n", shell.Oneline(f), strings.Join(printable(dead), ", "))
		}
	}
	return unentered, shared
}

func reportCycles(out, errOut io.Writer, adj map[string][]string, nodes []string) {
	budget := &walkBudget{left: walkSteps}
	loops := cycles(adj, nodes, budget)
	if budget.exhausted() {
		fmt.Fprintf(errOut, "graph too densely connected to walk exhaustively — the cycle count below is a LOWER BOUND\n")
	}
	if len(loops) == 0 {
		return
	}
	fmt.Fprintf(out, "\nCYCLES  %d. Between peers this is a cross-reference, not a defect —\n"+
		"        two standards may each be useful at the other's point of use. It is a defect\n"+
		"        only where the two are meant to be layered.\n", len(loops))
	for _, l := range loops {
		fmt.Fprintf(out, "  %s\n", strings.Join(printable(l), " → "))
	}
}

// The whole report, in the order it is read.
func report(out, errOut io.Writer, defined map[string]map[string]bool, edges []edge, routed map[string]bool) {
	adj, targets, distinct := summarize(edges)
	var nodes []string
	for f := range defined {
		nodes = append(nodes, f)
	}
	sort.Strings(nodes)

	fmt.Fprintf(out, "%d file(s), %d citation edge(s)\n\n", len(nodes), distinct)
	depth := reportDepth(out, errOut, adj, nodes)
	widest := reportFanOut(out, targets)
	unentered, shared := reportUnentered(out, defined, targets, routed, nodes)
	reportCycles(out, errOut, adj, nodes)
	fmt.Fprintf(out, "\ndepth %d, widest door surface %d section(s), %d unentered section(s) of which %d are in the shared layer\n",
		depth, widest, unentered, shared)
}

// The whole command, taking its writers and returning the code rather than reaching for either — the
// convention `eco-stats.go` states and the reason it holds: the suite drives this once per case, in
// process, which is what lets a case prove the exit code callers branch on. Spawn a process per case
// instead and the exit code is the one thing never covered, because covering it costs the most.
func run(args []string, out, errOut io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(errOut, "usage: cite-graph <root>")
		return 2
	}
	root := args[0]
	defined, edges, skipped := read(root, errOut)
	// Before the report rather than after it. Every figure below counts over the files that were read,
	// so a scan that missed part of the tree measures a different tree — and it prints at full
	// confidence, shaped exactly like a measurement of this one. A reader who takes the depth off
	// stdout has no way back to the stderr lines above, so printing it and exiting 2 would still hand
	// them the number.
	if skipped > 0 {
		fmt.Fprintf(errOut, "cite-graph: %d path(s) under %s were NOT read (each named above) — the tree measured is not the tree given, so there is no report. Exit 2.\n", skipped, root)
		return 2
	}
	if len(defined) == 0 {
		fmt.Fprintf(errOut, "cite-graph: read nothing under %s — exit 2, which is not the same as a flat tree.\n", root)
		return 2
	}
	// The router's own view, for the unentered report: a file it loads is entered whole.
	_, routed := routerSets(root, defined)
	report(out, errOut, defined, edges, routed)
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
