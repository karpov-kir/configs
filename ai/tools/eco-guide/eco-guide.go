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
// The frontmatter is read through `ai/tools/shell`, the same parser eco-check routes on, so the
// page cannot describe a skill differently from the way it is actually reached. What that parser
// returns is the raw YAML scalar; unquoting it for a reader is this package's, in inventory.go.
//
// `--check` is the gate: it regenerates into memory and compares against the committed page, so the
// guide cannot rot unnoticed. `ai/gate.sh`'s `guide` unit runs it.
//
// `--graph` and `--cost` are the same two sources joined for a terminal instead of for a page, and
// graph.go beside this file is all of both. They live in this tool rather than in model-policy
// because neither question can be answered from the policy document alone — it prices a row and
// cannot say which rows one run of a skill reaches — and rather than in a tool of their own because
// this one already holds the join, and a second reader of the same two inputs is a second thing to
// disagree with the page.
//
// It is a library with a thin command beside it, for the reason ecocheck and ecostats are: the suite
// drives it once per case, and a process spawn per case is what puts a suite over the time budget
// testing.md sets.
// Nothing here writes to os.Stdout or calls os.Exit, and nothing holds state between calls.
//
// Three exit codes, and never anything else. 0 — the page is written, the committed one already
// matches, or the requested emitter wrote its answer to stdout. 1 — a finding: the committed page is
// stale, or the template is. 2 — it could not run, which covers a request this cannot answer as well
// as a tree it cannot read, because both leave the caller with no result and a check that did not
// run is not a clean one.
package ecoguide

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	ecoroot "configs/ai/tools/eco-root"
	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/shell"
)

const (
	// Beside the tool, so the narrative and the code that fills it are edited in one directory.
	templateRelative = "tools/eco-guide/field-guide.template.html"
	// Beside ai/README.md — the one orientation file a reader already opens, in the tree the page
	// describes. Committed, because a gate can only diff a file that is in the commit.
	outputRelative = "field-guide.html"

	usage = "usage: guide.sh [--check | --graph | --cost <skill>] [<root>]"
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

	check, graph, costGiven := false, false, false
	costOf := ""
	var named []string
	for at := 0; at < len(args); at++ {
		arg := args[at]
		switch {
		case arg == "--check":
			check = true
		case arg == "--graph":
			graph = true
		case arg == "--cost":
			// The skill is the flag's own value rather than a positional, so `--cost` and a root can
			// both be given without either having to guess which of two bare words it is.
			at++
			if at == len(args) {
				return fail("--cost names no skill\n" + usage)
			}
			// Flagged separately from its value. Read back off the value alone, `--cost ""` — which
			// is what `guide.sh --cost "$skill"` produces with `skill` unset — looked like no emit
			// flag at all, and the run fell through to overwriting the committed page, from a
			// command whose own contract says it writes no file.
			costOf, costGiven = args[at], true
		case strings.HasPrefix(arg, "-"):
			return fail("unknown flag '%s'\n"+usage, arg)
		default:
			named = append(named, arg)
		}
	}
	// Refused rather than ordered, because each of the three writes a different thing to one stdout
	// and any order picked here would be this tool's opinion about which the caller meant.
	if count := boolCount(check, graph, costGiven); count > 1 {
		return fail("--check, --graph and --cost each emit something different; name one\n" + usage)
	}
	if len(named) > 1 {
		return fail("more than one root named\n" + usage)
	}
	rootName := ""
	if len(named) == 1 {
		rootName = named[0]
	}

	// What this run was asked to produce, named so a refusal below says which of the three did not
	// happen. One message serving all three would have to say "the guide was NOT generated" to a
	// caller who asked for a cost profile, which is a sentence about a file they never mentioned.
	producing := "the guide was NOT generated"
	switch {
	case graph:
		producing = "the map was NOT emitted"
	case costGiven:
		producing = "the cost profile was NOT emitted"
	}

	root, ok := ecoroot.Checkout(rootName)
	if !ok {
		return fail("no checkout holding kk-flavor/ and kk-flavor/skills/ at '%s' — %s",
			or(rootName, ". or ./ai"), producing)
	}

	// The policy first, because all three of this tool's answers rest on it: the page prints the tier
	// each dispatch buys, and the two emitters are nothing but that joined to the tree. The tiers
	// come out of the same resolver a dispatch calls, so nothing here can print a model the run
	// would not take. A row the policy refuses resolves to nothing and the card says so; the policy
	// failing to parse at all is fatal, because then every tier would say nothing.
	policyPath := shell.Join(root.Flavor(), "configs/models.json")
	rawPolicy, err := os.ReadFile(policyPath)
	if err != nil {
		return fail("cannot read the model policy at %s: %v — %s", policyPath, err, producing)
	}
	assigned, err := modelpolicy.Parse(rawPolicy)
	if err != nil {
		return fail("the model policy at %s does not parse: %v — %s", policyPath, err, producing)
	}

	// Ahead of the template and the inventories, because neither emitter reads either. A checkout
	// whose narrative template has been moved can still be asked what a skill costs, and that
	// matters: the cost question is the one asked while the tree is being edited.
	if graph || costGiven {
		map_, err := readWorkflow(root, assigned)
		if err != nil {
			return fail("%v — %s", err, producing)
		}
		if graph {
			emitGraph(map_, out)
			return 0
		}
		return emitCost(name, map_, costOf, out, errOut)
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

// How many of the three emit flags were given. Counted rather than chained, so adding a fourth is
// one argument here and no new pair of comparisons.
func boolCount(flags ...bool) int {
	given := 0
	for _, flag := range flags {
		if flag {
			given++
		}
	}
	return given
}
