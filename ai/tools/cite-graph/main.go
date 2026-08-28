// Measures the shape of what agents read: how deep a consumer reaches for a rule, how wide a file's
// surface is, and which of its sections nothing enters by.
//
//	usage: cite-graph <root>
//
// Restatement has a detector (rule-echo). Dependency shape had none, so "flatten the dependencies"
// was a judgment every pass re-made by eye and none could compare against the last. These are the
// three numbers a pass can actually move:
//
//   - DEPTH. A consumer reaching a rule through a chain of hops is a defect even when every hop is a
//     legal single home. Each link is individually correct and the chain is still too long.
//   - FAN-OUT. A file entered at one section has an interface. A file entered at nine has an open
//     surface, and every consumer is reaching for something different — which is what missing
//     encapsulation looks like from outside.
//   - UNENTERED SECTIONS. A section nothing cites is reachable only by reading the whole file. It is
//     either dead, or a rule that cannot be reused because no one can name it.
//
// The citation form is the interface, so that is what this reads: `<file>.md → **Section**`, the form
// `ecosystem.md` → **Conventions a new file joins** requires and check.sh already validates. Prose
// mentioning a file without that form is not a dependency — it is a mention, and counting it would
// make the graph denser than the tree really is.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

const maxFileBytes = 8 << 20

// `<file>.md → **Section**`, with the file optionally carrying a path or backticks. The arrow is the
// whole signal: it is what separates naming a file from entering one at a named door.
var citePattern = regexp.MustCompile("`?([A-Za-z0-9._/~-]+\\.md)`?[^\\n]{0,4}→\\s*\\*\\*([^*]+)\\*\\*")

// `and → **Section**` running on from a citation that already named the file.
var chainPattern = regexp.MustCompile(`^\s*(?:and|or|,)?\s*→\s*\*\*([^*]+)\*\*`)

var headingPattern = regexp.MustCompile(`^#{2,}\s+(.+?)\s*$`)

type edge struct {
	from, to, section string
	// The citer holds the target whole, so this citation names which rule rather than opening a door.
	precision bool
}

// Sections a file defines, by heading, keyed on the path relative to the root.
//
// Not on basename. Twenty-two skills each ship a `SKILL.md`, so a basename key silently welds them
// into one node — and the graph then reports chains that run through a file nobody cited, built out
// of edges belonging to twenty-two different files. The first version of this did exactly that and
// printed an eleven-hop chain through `SKILL.md` that no consumer walks.
// Whether a cited name enters a real heading, by check.sh's rule rather than a stricter one of our
// own: prose runs on past the heading it names, so a citation naming a leading run of one resolves.
// `→ **Phase 2 — Assemble Context**` enters `## Phase 2 — Assemble Context (progressive)`.
//
// Matching stricter than check.sh is worse than matching wrong: two detectors disagreeing about what
// resolves is invisible until someone reads both, and the first version of this guard reported a live
// citation as dangling on exactly this heading.
func entersAHeading(headings map[string]bool, section string) (string, bool) {
	if headings[section] {
		return section, true
	}
	// Truncate the CITATION to find a heading, which is check.sh's direction. Extending the citation
	// to reach a longer heading is the inverse, and it accepts what check.sh refuses:
	// `→ **Caller of a skill**` against `## Caller` resolves here and nowhere else. Both directions
	// happen to accept the em-dash case, which is why the inversion sat unnoticed behind a comment
	// claiming it implemented check.sh's rule.
	//
	// Longest run first, so a citation naming a real heading never resolves to a shorter one.
	// The run before an em dash is how a heading is cited when it carries a subtitle — `**Budget**`
	// for `## Budget — the keep test`. It resolves to the heading, and is not a section of its own:
	// registering it as one keyed the edge on the alias and left the real heading reported UNENTERED
	// while three files were entering it.
	for h := range headings {
		if before, _, found := strings.Cut(h, " — "); found && strings.TrimSpace(before) == section {
			return h, true
		}
	}
	for cut := len(section); cut > 0; cut-- {
		if cut < len(section) && section[cut] != ' ' && section[cut] != '\t' {
			continue // a word boundary, so half a word cannot satisfy a citation
		}
		run := strings.TrimRight(section[:cut], " \t")
		if run == "" {
			continue
		}
		if headings[run] {
			return run, true
		}
		// A truncation landing on a heading's em-dash prefix resolves too, because that is what
		// `ecocheck` accepts: its heading set carries the prefix as an alias. This tool deliberately
		// does not register the alias — registering it keys the edge on the alias and leaves the real
		// heading reported UNENTERED — so the alias is matched here and the FULL heading returned.
		//
		// Without this the two disagree on what resolves, and this tool calls a compliant citation
		// broken: `**Phase 2 — Assemble Context**` against `## Phase 2 — Assemble Context
		// (progressive)`. A citation naming a section that truly does not exist is still refused,
		// because no heading's prefix answers to it.
		for h := range headings {
			if before, _, found := strings.Cut(h, " — "); found && strings.TrimSpace(before) == run {
				return h, true
			}
		}
	}
	return "", false
}

// The files `inject.md` lists under its read-always heading. Read from the router rather than named
// here, so a file promoted into or out of that tier is picked up without editing this.
func routerSets(root string, defined map[string]map[string]bool) (always, routed map[string]bool) {
	always, routed = map[string]bool{}, map[string]bool{}
	var injectPath string
	for f := range defined {
		if filepath.Base(f) == "inject.md" {
			injectPath = f
			break
		}
	}
	if injectPath == "" {
		return always, routed
	}
	body, err := os.ReadFile(filepath.Join(root, injectPath))
	if err != nil {
		return always, routed
	}
	// Every file the router names is entered whole on its trigger, read-always or not. That decides
	// what an unentered section means in it: nothing, because its readers never enter by section.
	isAlways := false
	for _, line := range shell.SplitLines(string(body)) {
		if strings.HasPrefix(line, "#") {
			isAlways = strings.Contains(strings.ToLower(line), "read always")
			continue
		}
		for _, m := range linkPattern.FindAllStringSubmatch(line, -1) {
			target := relOf(root, filepath.Join(filepath.Dir(filepath.Join(root, injectPath)), m[1]))
			if _, ok := defined[target]; !ok {
				continue
			}
			routed[target] = true
			if isAlways {
				always[target] = true
			}
		}
	}
	routed[injectPath] = true // the router itself: nothing cites a router's own headings
	return always, routed
}

var linkPattern = regexp.MustCompile(`\]\(([^)]+\.md)\)`)

func relOf(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}

// A citation before its target is known. Resolution needs every file read first, so it is a second
// pass: a bare `writing.md` names whichever file carries that basename, and that is only answerable
// once the whole tree is in hand.
type rawCite struct{ from, fromDir, target, section string }

// A `*.md` token that is NOT part of a `→ **Section**` citation: the shape of "You run under
// `<file>`", which means the citer holds the whole file already.
var bareTokenPattern = regexp.MustCompile(`[A-Za-z0-9._/~-]+\.md`)

func read(root string) (defined map[string]map[string]bool, edges []edge, err error) {
	defined = map[string]map[string]bool{}
	var raw []rawCite
	var bare []rawCite
	err = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".md") || !fi.Mode().IsRegular() {
			return nil
		}
		if fi.Size() > maxFileBytes {
			fmt.Fprintf(os.Stderr, "file too large to scan: %s is %d bytes, over the %d-byte bound — it was NOT read\n", shell.Oneline(p), fi.Size(), maxFileBytes)
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		self := relOf(root, p)
		if defined[self] == nil {
			defined[self] = map[string]bool{}
		}
		inFence := false
		for _, line := range shell.SplitLines(string(body)) {
			if shell.IsFenceDelimiter(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			if m := headingPattern.FindStringSubmatch(line); m != nil {
				heading := strings.TrimSpace(strings.Trim(m[1], "*"))
				defined[self][heading] = true
				// A heading may carry a subtitle after an em dash, and a citation names only the run
				// before it — `**Budget**` for `## Budget — the keep test`. check.sh accepts that alias,
				// so a tool that does not reports three live citations as entering nothing and the
				// section itself as unentered. Cut at the em dash and nowhere else: a trailing run, or a
				// word-by-word prefix, would let half a heading satisfy a citation.

			}
			spans := citePattern.FindAllStringIndex(line, -1)
			lastNamed := ""
			for i, c := range citePattern.FindAllStringSubmatch(line, -1) {
				lastNamed = c[1]
				raw = append(raw, rawCite{from: self, fromDir: filepath.Dir(p), target: c[1], section: strings.TrimSpace(c[2])})
				// A second section chained onto the first — `X → **A** and → **B**` — names no file of
				// its own, so the pattern above sees one citation where the line makes two claims. Both
				// bind, and counting one made the target's door surface read narrower than it is.
				for _, extra := range chainPattern.FindAllStringSubmatch(line[spans[i][1]:], -1) {
					raw = append(raw, rawCite{from: self, fromDir: filepath.Dir(p), target: lastNamed, section: strings.TrimSpace(extra[1])})
				}
			}
			// Whether the citer also holds the file whole. Without this the metric inverts: the more
			// precisely a skill cites a file it has already read — which is exactly what
			// `ecosystem.md` → **Conventions a new file joins** demands — the wider its target's
			// surface appears. Those citations are precision, not doors.
			for _, at := range bareTokenPattern.FindAllStringIndex(line, -1) {
				inCite := false
				for _, sp := range spans {
					if at[0] >= sp[0] && at[0] < sp[1] {
						inCite = true
						break
					}
				}
				if !inCite {
					bare = append(bare, rawCite{from: self, fromDir: filepath.Dir(p), target: line[at[0]:at[1]]})
				}
			}
		}
		return nil
	})
	if err != nil {
		return defined, nil, err
	}

	byBase := map[string][]string{}
	for f := range defined {
		byBase[filepath.Base(f)] = append(byBase[filepath.Base(f)], f)
	}
	// A file the router marks read-always is held whole by every reader on every task, so no citation
	// into it is a door — the reader already has it and the citation only says which rule. Detecting
	// "reads whole" from a bare mention in the citer cannot see this: nothing mentions the file,
	// because inject.md loaded it. Without this the always-read set reads as the widest surface in the
	// tree, which is the opposite of what being always-read means.
	alwaysRead, _ := routerSets(root, defined)
	kinds := kindBasenames(defined)
	readsWhole := map[string]bool{}
	for _, b := range bare {
		if to := resolve(root, b, defined, byBase, kinds); to != "" && to != b.from {
			readsWhole[b.from+">"+to] = true
		}
	}
	for _, c := range raw {
		to := resolve(root, c, defined, byBase, kinds)
		// A file citing its own section is navigation, not a dependency.
		if to == "" || to == c.from {
			continue
		}
		// A bolded list item matches the citation shape and resolves to no heading, so counting it
		// would add a door to a section that does not exist and inflate the very number this tool
		// reports. check.sh reports the dangling reference; this refuses to measure it.
		heading, ok := entersAHeading(defined[to], c.section)
		if !ok {
			fmt.Fprintf(os.Stderr, "no such section: %s cites %s → **%s**, which is no heading there — NOT counted\n",
				shell.Oneline(c.from), shell.Oneline(to), shell.Oneline(c.section))
			continue
		}
		// The heading matched, never the string cited. Keyed on the citation, a section reached
		// through the truncation rule or the em-dash alias is reported UNENTERED while files enter it.
		edges = append(edges, edge{c.from, to, heading, readsWhole[c.from+">"+to] || alwaysRead[to]})
	}
	return defined, edges, err
}

// The basenames every skill lane carries. Such a name is the *kind* of file rather than one of them
// — "run the skill in full, per its SKILL.md" names no file — so a citation naming one is a generic
// reference, not a dangling one to report. check.sh drops these before it reports anything, and two
// detectors disagreeing about what resolves is invisible until someone reads both.
//
// Every lane, not merely several: a basename two or three files answer to is genuinely ambiguous and
// stays reported, because nothing about it says which of them was meant. Below two lanes there is no
// "every lane" to speak of — one lane's whole contents would qualify, and a name shared between that
// lane and the shared layer is exactly the ambiguity worth reporting.
func kindBasenames(defined map[string]map[string]bool) map[string]bool {
	lanes := map[string]bool{}
	carriedBy := map[string]map[string]bool{}
	for file := range defined {
		rest, underSkills := strings.CutPrefix(file, "skills/")
		if !underSkills {
			continue
		}
		lane, _, nested := strings.Cut(rest, "/")
		if !nested {
			continue
		}
		lanes[lane] = true
		base := filepath.Base(file)
		if carriedBy[base] == nil {
			carriedBy[base] = map[string]bool{}
		}
		carriedBy[base][lane] = true
	}
	kinds := map[string]bool{}
	if len(lanes) < 2 {
		return kinds
	}
	for base, carrying := range carriedBy {
		if len(carrying) == len(lanes) {
			kinds[base] = true
		}
	}
	return kinds
}

// Which file a citation names. A path resolves as one; a bare basename resolves only when exactly one
// file answers to it, and an ambiguous one is reported rather than guessed — guessing is what welds
// twenty-two SKILL.md files into a node.
func resolve(root string, c rawCite, defined map[string]map[string]bool, byBase map[string][]string, kinds map[string]bool) string {
	if strings.ContainsRune(c.target, '/') {
		cleaned := strings.TrimPrefix(strings.TrimPrefix(c.target, "~/"), "./")
		for _, candidate := range []string{
			relOf(root, filepath.Join(c.fromDir, c.target)),
			cleaned,
			strings.TrimPrefix(cleaned, ".kk-flavor/"),
			strings.TrimPrefix(cleaned, ".claude/"),
		} {
			if _, ok := defined[candidate]; ok {
				return candidate
			}
		}
		// A path that resolves nowhere is check.sh's finding, not this tool's; fall through to the
		// basename so a live dependency is still counted rather than dropped.
	}
	base := filepath.Base(c.target)
	switch hits := byBase[base]; len(hits) {
	case 1:
		return hits[0]
	case 0:
		return ""
	default:
		// A kind names no one file, so there is nothing here to guess at and nothing to report.
		if kinds[base] {
			return ""
		}
		// Both are names the tree chose. Printed raw, a newline in one forges a line of this tool's own
		// report, and that line is the only signal a citation was dropped.
		fmt.Fprintf(os.Stderr, "ambiguous: %s cites %s, which %d files answer to — NOT counted\n", shell.Oneline(c.from), shell.Oneline(base), len(hits))
		return ""
	}
}

// The longest chain a consumer actually walks. Exhaustive rather than memoised: the tree is tens of
// files, and a memo over "longest from here" is wrong in the presence of cycles, which this reports
// rather than assumes away.
func longest(adj map[string][]string, start string, seen map[string]bool, path []string) []string {
	best := append([]string{}, path...)
	for _, next := range adj[start] {
		if seen[next] {
			continue
		}
		seen[next] = true
		if got := longest(adj, next, seen, append(path, next)); len(got) > len(best) {
			best = got
		}
		delete(seen, next)
	}
	return best
}

func cycles(adj map[string][]string, nodes []string) [][]string {
	var found [][]string
	reported := map[string]bool{}
	var walk func(node string, path []string, onPath map[string]bool)
	walk = func(node string, path []string, onPath map[string]bool) {
		for _, next := range adj[node] {
			if onPath[next] {
				at := 0
				for i, n := range path {
					if n == next {
						at = i
						break
					}
				}
				loop := append(append([]string{}, path[at:]...), next)
				// The key drops the repeated endpoint before sorting. With it in, `a → b → a` and
				// `b → a → b` — one cycle entered from two sides — produce different keys and get
				// reported twice, which inflates the count by roughly the cycle's length.
				sorted := append([]string{}, loop[:len(loop)-1]...)
				sort.Strings(sorted)
				if key := strings.Join(sorted, ">"); !reported[key] {
					reported[key] = true
					found = append(found, loop)
				}
				continue
			}
			onPath[next] = true
			walk(next, append(path, next), onPath)
			delete(onPath, next)
		}
	}
	for _, n := range nodes {
		walk(n, []string{n}, map[string]bool{n: true})
	}
	return found
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: cite-graph <root>")
		os.Exit(2)
	}
	defined, edges, err := read(os.Args[1])
	if err != nil || len(defined) == 0 {
		fmt.Fprintf(os.Stderr, "cite-graph: read nothing under %s — exit 2, which is not the same as a flat tree.\n", os.Args[1])
		os.Exit(2)
	}

	adj := map[string][]string{}
	seenEdge := map[string]bool{}
	enteredAt := map[string]map[string]bool{}
	citedBy := map[string]map[string]bool{}
	doorSections := map[string]map[string]bool{}
	doorCiters := map[string]map[string]bool{}
	precisionCiters := map[string]map[string]bool{}
	// Per target, how many distinct sections each door citer enters.
	doorReach := map[string]map[string]map[string]bool{}
	for _, e := range edges {
		if !seenEdge[e.from+">"+e.to] {
			seenEdge[e.from+">"+e.to] = true
			adj[e.from] = append(adj[e.from], e.to)
		}
		if enteredAt[e.to] == nil {
			enteredAt[e.to] = map[string]bool{}
			citedBy[e.to] = map[string]bool{}
			doorSections[e.to] = map[string]bool{}
			doorCiters[e.to] = map[string]bool{}
			precisionCiters[e.to] = map[string]bool{}
		}
		enteredAt[e.to][e.section] = true
		citedBy[e.to][e.from] = true
		if e.precision {
			precisionCiters[e.to][e.from] = true
			continue
		}
		doorSections[e.to][e.section] = true
		doorCiters[e.to][e.from] = true
		if doorReach[e.to] == nil {
			doorReach[e.to] = map[string]map[string]bool{}
		}
		if doorReach[e.to][e.from] == nil {
			doorReach[e.to][e.from] = map[string]bool{}
		}
		doorReach[e.to][e.from][e.section] = true
	}

	// The router's own view, for the unentered report: a file it loads is entered whole.
	_, routed := routerSets(os.Args[1], defined)

	var nodes []string
	for f := range defined {
		nodes = append(nodes, f)
	}
	sort.Strings(nodes)

	fmt.Printf("%d file(s), %d citation edge(s)\n\n", len(nodes), len(seenEdge))

	deepest := []string{}
	for _, n := range nodes {
		if got := longest(adj, n, map[string]bool{n: true}, []string{n}); len(got) > len(deepest) {
			deepest = got
		}
	}
	fmt.Printf("DEPTH  longest chain is %d hop(s):\n  %s\n\n", len(deepest)-1, strings.Join(deepest, "\n    → "))

	fmt.Println("FAN-OUT  doors are citers that do NOT hold the file whole. A citer that reads it whole is")
	fmt.Println("         being precise about which rule, not entering. Of the doors, a DEEP one enters more")
	fmt.Println("         than one section — it uses the file enough to read it whole, and that is the debt.")
	fmt.Println("         A door entering one section wants one rule from a file it need not load; that is")
	fmt.Println("         what cutting a restatement correctly produces, and it is not debt.")
	type row struct {
		file                                    string
		doorSections, doors, deep, precisionRef int
	}
	var rows []row
	for f := range enteredAt {
		deep := 0
		for _, sections := range doorReach[f] {
			if len(sections) > 1 {
				deep++
			}
		}
		rows = append(rows, row{f, len(doorSections[f]), len(doorCiters[f]), deep, len(precisionCiters[f])})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].doorSections != rows[j].doorSections {
			return rows[i].doorSections > rows[j].doorSections
		}
		return rows[i].file < rows[j].file
	})
	for _, r := range rows {
		fmt.Printf("  %-42s %2d door section(s) / %2d door(s), %d of them deep, %2d precision citer(s)\n",
			r.file, r.doorSections, r.doors, r.deep, r.precisionRef)
	}

	fmt.Println("\nUNENTERED  defined, but no file names it — a finding only where readers enter by section.")
	fmt.Println("           A file the router loads is entered whole on its trigger, and a skill's sections")
	fmt.Println("           are its own, so neither is listed. What remains is a file reached only by")
	fmt.Println("           citation, holding a rule nothing reaches.")
	unentered, unenteredShared := 0, 0
	for _, f := range nodes {
		var dead []string
		for s := range defined[f] {
			if enteredAt[f] == nil || !enteredAt[f][s] {
				dead = append(dead, s)
			}
		}
		sort.Strings(dead)
		if len(dead) == 0 {
			continue
		}
		unentered += len(dead)
		if !strings.HasPrefix(f, "skills/") && !routed[f] {
			unenteredShared += len(dead)
			fmt.Printf("  shared  %-34s %s\n", f, strings.Join(dead, ", "))
		}
	}

	if loops := cycles(adj, nodes); len(loops) > 0 {
		fmt.Printf("\nCYCLES  %d. Between peers this is a cross-reference, not a defect —\n"+
			"        two standards may each be useful at the other's point of use. It is a defect\n"+
			"        only where the two are meant to be layered.\n", len(loops))
		for _, l := range loops {
			fmt.Printf("  %s\n", strings.Join(l, " → "))
		}
	}

	widest := 0
	if len(rows) > 0 {
		widest = rows[0].doorSections // sorted widest-first above
	}
	fmt.Printf("\ndepth %d, widest door surface %d section(s), %d unentered section(s) of which %d are in the shared layer\n",
		len(deepest)-1, widest, unentered, unenteredShared)
}
