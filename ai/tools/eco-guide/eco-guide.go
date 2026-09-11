// Package ecoguide generates the field guide — one self-contained HTML page telling someone who has
// just installed this ecosystem what it does and which skill to reach for.
//
// The page has two halves and they are kept apart on purpose. The **narrative** — the walkthrough,
// the "which door do I use" table, the section prose — is hand-written and lives in
// `field-guide.template.html` beside this file; a person editing it never reads Go. The
// **inventories** are generated, because a hand-maintained catalogue of a tree this size drifts in
// silence: one card per skill from its own frontmatter — `description:`, `argument-hint:`, and
// whether a human always types it — and one card per worker from its brief and the tier the model
// policy resolves for it. A worker declares no frontmatter, so those two are its only sources, and
// neither can be edited to flatter the other.
//
// The frontmatter is read through `kk-flavor/tools/shell`, the same parser eco-check routes on, so the
// page cannot describe a skill differently from the way it is actually reached. What that parser
// returns is the raw YAML scalar; unquoting it for a reader is this package's, in inventory.go.
//
// `--check` is the gate: it regenerates into memory and compares against the committed page, so the
// guide cannot rot unnoticed. `ai/gate.sh`'s `guide` unit runs it.
//
// It is a library with a thin command beside it, for the reason ecocheck and ecostats are: the suite
// drives it once per case, and a process spawn per case is what makes a mutation run take hours.
// Nothing here writes to os.Stdout or calls os.Exit, and nothing holds state between calls.
//
// Three exit codes, and never anything else. 0 — the page is written, or the committed one already
// matches. 1 — a finding: the committed page is stale, or the template is. 2 — it could not run, and
// a check that did not run is not a clean one.
package ecoguide

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	ecoroot "kk-flavor/tools/eco-root"
	modelpolicy "kk-flavor/tools/model-policy"
	"kk-flavor/tools/shell"
)

const (
	// Beside the tool, so the narrative and the code that fills it are edited in one directory.
	templateRelative = "tools/eco-guide/field-guide.template.html"
	// Beside ai/README.md — the one orientation file a reader already opens, in the tree the page
	// describes. Committed, because a gate can only diff a file that is in the commit.
	outputRelative = "field-guide.html"

	usage = "usage: guide.sh [--check] [<root>]"
)

func Run(self string, args []string, out, errOut io.Writer) int {
	name := shell.BaseName(self)
	if name == "" {
		name = "guide.sh"
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(errOut, name+": "+format+"\n", a...)
		return 2
	}

	check := false
	var named []string
	for _, arg := range args {
		switch {
		case arg == "--check":
			check = true
		case strings.HasPrefix(arg, "-"):
			return fail("unknown flag '%s'\n"+usage, arg)
		default:
			named = append(named, arg)
		}
	}
	if len(named) > 1 {
		return fail("more than one root named\n" + usage)
	}
	rootName := ""
	if len(named) == 1 {
		rootName = named[0]
	}

	root, ok := ecoroot.Checkout(rootName)
	if !ok {
		return fail("no checkout holding kk-flavor/ and kk-flavor/skills/ at '%s' — the guide was NOT generated",
			or(rootName, ". or ./ai"))
	}

	templatePath := shell.Join(root.Named(), templateRelative)
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return fail("cannot read the narrative template at %s: %v — the guide was NOT generated", templatePath, err)
	}

	skills, err := readInventory(root)
	if err != nil {
		return fail("cannot read the skills at %s: %v — the guide was NOT generated", root.Skills(), err)
	}
	if len(skills) == 0 {
		return fail("no skill under %s declares a description — read this as the reader broken, never as an empty ecosystem", root.Skills())
	}

	// The tiers come out of the same resolver a dispatch calls, so the page cannot print a model the
	// run would not take. A row the policy refuses resolves to nothing and the card says so; the
	// policy failing to parse at all is fatal, because then every tier on the page would say nothing.
	policyPath := shell.Join(root.Flavor(), "models.json")
	rawPolicy, err := os.ReadFile(policyPath)
	if err != nil {
		return fail("cannot read the model policy at %s: %v — the guide was NOT generated", policyPath, err)
	}
	assigned, err := modelpolicy.Parse(rawPolicy)
	if err != nil {
		return fail("the model policy at %s does not parse: %v — the guide was NOT generated", policyPath, err)
	}
	tier := func(task, client string) string {
		decision, err := assigned.Resolve(modelpolicy.Request{Client: client, Task: task})
		if err != nil {
			return ""
		}
		return strings.TrimSpace(decision.Requested.Model + " " + decision.Requested.Effort)
	}

	workers := readWorkers(root, assigned.WorkerTasks(), assigned.PromptOwners(), tier)
	if len(workers) == 0 {
		return fail("the policy at %s prices no dispatch at all — read this as the reader broken, never as a tree with no workers", policyPath)
	}

	page, err := render(string(template), skills, workers)
	if err != nil {
		fmt.Fprintf(errOut, "%s: %v\n", name, err)
		fmt.Fprintf(errOut, "%s: nothing was written to %s\n", name, shell.Join(root.Named(), outputRelative))
		return 1
	}

	target := shell.Join(root.Named(), outputRelative)
	if check {
		return compare(name, target, page, len(skills), len(workers), out, errOut)
	}
	if err := os.WriteFile(target, []byte(page), 0o644); err != nil {
		return fail("cannot write %s: %v", target, err)
	}
	fmt.Fprintf(out, "%s: wrote %s — %d skills across %d families, and %d workers\n",
		name, target, len(skills), countFamilies(skills), len(workers))
	return 0
}

// The gate's half. A page that is missing, unreadable or different is one finding with one meaning:
// the committed guide no longer describes the tree. It names what moved rather than only that
// something did, because "the guide is stale" sends someone to read a diff the tool already has.
func compare(name, target, want string, skillCount, workerCount int, out, errOut io.Writer) int {
	held, err := os.ReadFile(target)
	if err != nil {
		fmt.Fprintf(errOut, "%s: no committed guide at %s (%v) — regenerate it with `%s` and commit it\n",
			name, target, err, name)
		return 1
	}
	if string(held) == want {
		// "it lists", not "in the tree": the skills half leaves out the maintainer-only ones, so a
		// count of what the page carries is not a count of what the tree holds. Saying the second
		// would let a green line under-report the tree every time a maintainer-only skill is added.
		fmt.Fprintf(out, "%s: %s matches the %d skills and %d dispatch sites it lists\n",
			name, target, skillCount, workerCount)
		return 0
	}
	fmt.Fprintf(errOut, "%s: %s no longer matches the skills in the tree\n", name, target)
	for _, line := range describeDrift(string(held), want) {
		fmt.Fprintf(errOut, "  %s\n", line)
	}
	fmt.Fprintf(errOut, "  regenerate it with `%s` and commit the result\n", name)
	return 1
}

// What changed, in the terms someone can act on: which skills came and went first, since that is what
// nearly every drift is, then the first handful of differing lines for the rest.
func describeDrift(held, want string) []string {
	var lines []string
	added, removed := namedDelta(held, want)
	for _, skill := range removed {
		lines = append(lines, "gone from the tree, still on the page: "+skill)
	}
	for _, skill := range added {
		lines = append(lines, "in the tree, missing from the page: "+skill)
	}
	shown := 0
	heldLines, wantLines := strings.Split(held, "\n"), strings.Split(want, "\n")
	for i := 0; i < max(len(heldLines), len(wantLines)); i++ {
		if at(heldLines, i) == at(wantLines, i) {
			continue
		}
		if shown == driftLinesShown {
			lines = append(lines, "…and further differences below line "+fmt.Sprint(i+1))
			break
		}
		shown++
		lines = append(lines, fmt.Sprintf("line %d committed: %s", i+1, oneLine(at(heldLines, i))))
		lines = append(lines, fmt.Sprintf("line %d generated: %s", i+1, oneLine(at(wantLines, i))))
	}
	return lines
}

const (
	driftLinesShown = 6
	driftLineWidth  = 120
)

// The skill names each version of the page lists, taken from the card markup the renderer writes. A
// page hand-edited into a shape this does not recognise simply reports no delta, and the line diff
// below still says the two differ.
func namedDelta(held, want string) (added, removed []string) {
	heldNames, wantNames := cardNames(held), cardNames(want)
	for _, name := range wantNames {
		if !slices.Contains(heldNames, name) {
			added = append(added, name)
		}
	}
	for _, name := range heldNames {
		if !slices.Contains(wantNames, name) {
			removed = append(removed, name)
		}
	}
	return added, removed
}

func cardNames(page string) []string {
	var names []string
	seen := map[string]bool{}
	// The card's opening markup, not a bare `<code class="k">`: the narrative names skills in that same
	// form, and counting those would report a skill as present on a page that stopped listing it.
	for _, class := range []string{`<div class="lane-top"><code class="i">`, `<div class="lane-top"><code class="k">`} {
		rest := page
		for {
			found := strings.Index(rest, class)
			if found < 0 {
				break
			}
			rest = rest[found+len(class):]
			end := strings.Index(rest, "</code>")
			if end < 0 {
				break
			}
			if name := rest[:end]; !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(no line)"
}

func oneLine(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > driftLineWidth {
		return text[:driftLineWidth] + "…"
	}
	return text
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
