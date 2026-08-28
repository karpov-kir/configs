package ecocheck

// Enforces ecosystem.md → **Prefer the mechanism**: every subcommand a dispatch accepts needs one
// `<script> <subcommand>` somewhere an agent reads, or it is a capability nothing routes to.
//
// Two dispatch shapes are read here: a shell `case "${1:-}" in`, and a Go switch behind a stub that
// execs a binary. A stub holds no `case` of its own, so covering the shell shape alone returns
// nothing at all for it, which reads to every caller as a pass.
// Nothing below answers "no subcommands" by failing to find them: a stub that names a grammar it
// cannot read the dispatch for is reported, never skipped.

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

var (
	caseLabel = regexp.MustCompilePOSIX(`^  [a-z][a-z0-9|_-]*\)$`)
	// How a stub names the tool it execs. The charset is resolve.sh's, which refuses everything else:
	// a name that resolver would not touch is not one to build a path from here either.
	toolDeclaration = regexp.MustCompilePOSIX(`^tool="[a-z0-9-]+"$`)
	subcommandName  = regexp.MustCompilePOSIX(`^[a-z][a-z0-9-]*$`)
)

// The most subcommands this scan carries. Each one costs a substring test per file in the tree, and
// the dispatch every one of them was read from is a file the reviewed branch wrote. Past the bound the
// rest are reported and NOT checked — an unchecked subcommand must never read as one with a call site.
const subcommandCap = 256

func (c *checker) scanSubcommandCallSites() {
	type callSite struct{ script, subcommand string }
	var wanted []callSite
	queries := map[string]bool{}
	capped := 0
	want := func(script string, subcommands []string) {
		for _, subcommand := range subcommands {
			key := script + " " + subcommand
			if _, seen := queries[key]; seen {
				continue
			}
			if len(queries) >= subcommandCap {
				capped++
				continue
			}
			wanted = append(wanted, callSite{script: script, subcommand: subcommand})
			queries[key] = false
		}
	}
	// A call site is written the way an agent writes one — `report.sh cadence`, or a path ending in
	// that — so the *search* token has to be the basename, and it is the one thing here a path cannot
	// replace. What a path does replace is the attribution: two scripts under one basename have their
	// dispatches read into a single set of names, a call site for either then answers for both, and the
	// finding named the basename, so a reader could not tell which file it was about.
	//
	// Which is why the owners are collected in a pass of their own, ahead of the one that reports:
	// every finding below reaches for the path behind a basename, and a map still filling up while
	// they do would answer differently depending on walk order.
	c.scriptOwners = map[string][]string{}
	for _, script := range c.filesNamed(c.root.Named(), "*.sh") {
		base := shell.BaseName(script)
		c.scriptOwners[base] = append(c.scriptOwners[base], script)
	}
	for _, script := range c.filesNamed(c.root.Named(), "*.sh") {
		base := shell.BaseName(script)
		lines, err := c.readLines(script)
		if err != nil {
			continue
		}
		want(base, dispatchLabels(lines))
		want(base, c.toolSubcommands(base, lines))
	}
	c.reportWeldedScriptNames(c.scriptOwners)
	if capped > 0 {
		c.add(fmt.Sprintf("subcommand call-site scan is at its %d-subcommand bound: %d more were NOT checked",
			subcommandCap, capped))
	}
	if len(queries) == 0 {
		return
	}
	// One pass over the tree answers every subcommand. Walking it per label turns a script with many
	// arms into many whole-tree walks.
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh", "*.yaml") {
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		text := string(body)
		for query, seen := range queries {
			if !seen && strings.Contains(text, query) {
				queries[query] = true
			}
		}
	}
	for _, site := range wanted {
		// A welded name is reported above rather than checked here: its call sites cannot be told
		// apart, so both readings are wrong and neither is worth printing.
		if len(c.scriptOwners[site.script]) != 1 {
			continue
		}
		if !queries[site.script+" "+site.subcommand] {
			c.add(shell.Oneline(c.scriptOwners[site.script][0]) + " subcommand with no call site: " + shell.Oneline(site.subcommand))
		}
	}
}

// The basenames more than one script in the tree carries. Silence here would be the mute the scan is
// cheapest to buy: commit a second `report.sh` anywhere under the root and every subcommand of the
// first stops being checked, while no other finding names either file. A narrowed scan reports that
// it narrowed, and it names both files rather than choosing one — which was meant is not in the tree.
//
// The paths are the tree's own, so each is sanitised, and the printer bounds the line.
func (c *checker) reportWeldedScriptNames(owners map[string][]string) {
	var welded []string
	for base, paths := range owners {
		if len(paths) < 2 {
			continue
		}
		var named []string
		for _, path := range paths {
			named = append(named, shell.Oneline(path))
		}
		welded = append(welded, fmt.Sprintf("subcommand call sites not checked: %d scripts are named %s (%s) — a call site naming it cannot be attributed to one of them; rename one",
			len(paths), shell.Oneline(base), strings.Join(shell.SortUnique(named), ", ")))
	}
	for _, finding := range shell.SortUnique(welded) {
		c.add(finding)
	}
}

// The case labels of a top-level shell `case "${1:-}" in` dispatch. Labels are read at the case arm's
// own indentation, so a nested `done)` is not one.
func dispatchLabels(lines []string) []string {
	var labels []string
	inside := false
	for _, line := range lines {
		if strings.HasPrefix(line, `case "${1:-}" in`) {
			inside = true
			continue
		}
		if inside && strings.HasPrefix(line, "esac") {
			break
		}
		if !inside || !caseLabel.MatchString(line) {
			continue
		}
		trimmed := strings.NewReplacer(" ", "", ")", "").Replace(line)
		labels = append(labels, strings.Split(trimmed, "|")...)
	}
	return shell.SortUnique(labels)
}

// A stub that execs a Go tool takes its subcommands from that tool's dispatch, and the stub's own
// usage grammar is what declares it has any. Both lists are read and neither stands alone: the grammar
// is what an agent reads, the dispatch is what the tool accepts, and a name in one and not the other
// is a defect in whichever it is missing from.
func (c *checker) toolSubcommands(base string, lines []string) []string {
	tool := declaredTool(lines)
	if tool == "" {
		return nil
	}
	documented := usageSubcommands(base, lines)
	dispatched, why := c.goDispatchLabels(base, tool)
	if why != "" {
		// A stub whose grammar names subcommands and whose dispatch could not be read leaves every
		// one of them unchecked, so say which way it could not be read and check the grammar's list
		// anyway: a worse authority than the dispatch, but not nothing. A stub whose grammar names
		// none and whose tool holds no dispatch takes no subcommand at all (check.sh and stats.sh
		// each take a root), which is a determination rather than a failure to reach one.
		if len(documented) > 0 {
			c.add(fmt.Sprintf("cannot read %s's dispatch: %s — the %d subcommand(s) its usage names were checked against that usage alone",
				c.scriptNamed(base), why, len(documented)))
		}
		return documented
	}
	for _, name := range onlyIn(dispatched, documented) {
		c.add(c.scriptNamed(base) + " accepts a subcommand its usage does not name: " + shell.Oneline(name))
	}
	for _, name := range onlyIn(documented, dispatched) {
		c.add(c.scriptNamed(base) + " usage names a subcommand its dispatch does not accept: " + shell.Oneline(name))
	}
	return shell.SortUnique(append(append([]string{}, dispatched...), documented...))
}

// The file a finding about a basename is about. Every finding here is reached through a basename,
// because that is how a call site is written, and a basename is not a file: 22 files in this tree
// answer to `SKILL.md` alone. One script under a name is the path; more than one has no answer in the
// tree, so this says how many rather than picking the first and printing it as fact.
func (c *checker) scriptNamed(base string) string {
	paths := c.scriptOwners[base]
	if len(paths) == 1 {
		return shell.Oneline(paths[0])
	}
	return fmt.Sprintf("%s (one of the %d files of that name — which is not in the tree)",
		shell.Oneline(base), len(paths))
}

// The tool directory a stub execs, from its own `tool="<name>"` line.
func declaredTool(lines []string) string {
	for _, line := range lines {
		if toolDeclaration.MatchString(line) {
			return strings.Trim(line[len("tool="):], `"`)
		}
	}
	return ""
}

// The subcommands a `usage: <name> {a <x>|b|c}` grammar names: the first word of each alternative,
// with the argument grammar after it dropped. Wrapped across as many comment lines as it takes, and
// bounded in bytes, because the stub is a file the reviewed branch wrote. A grammar truncated at that
// bound is not read as complete; its tail then shows up as a name the dispatch accepts and the usage
// does not, which is the cross-check above doing the reporting.
func usageSubcommands(base string, lines []string) []string {
	opening := "usage: " + base + " {"
	grammar, collecting := "", false
	for _, line := range lines {
		text := strings.TrimLeft(line, "# \t")
		if !collecting {
			at := strings.Index(text, opening)
			if at < 0 {
				continue
			}
			text, collecting = text[at+len(opening):], true
		}
		closed := false
		if end := strings.IndexByte(text, '}'); end >= 0 {
			text, closed = text[:end], true
		}
		// A space, so a wrapped alternative's two halves are two words and the first stays the
		// subcommand rather than fusing with the previous line's argument grammar.
		grammar += text + " "
		if closed || len(grammar) > 4096 {
			break
		}
	}
	var found []string
	for _, alternative := range strings.Split(grammar, "|") {
		fields := shell.SplitFields(alternative)
		if len(fields) > 0 && subcommandName.MatchString(fields[0]) {
			found = append(found, fields[0])
		}
	}
	return shell.SortUnique(found)
}

// The case labels of a tool's subcommand dispatch, read out of its source, never by running it. This
// runs as kk-pr-review's ecosystem stage (quality-pipeline.md → **The stages**) over a branch that
// chose the contents of tools/, so running that branch's code is a different act from reading it, and
// a release install may carry no Go toolchain at all. The tool's own usage output is no better an
// authority: it is a hand-written literal in the refusing arm, not generated from the case labels.
//
// The switch is found by the `usage: <stub> {` line its refusing arm carries, never by the name of the
// function around it: a function is renamed without a word, and an anchor that quietly stops matching
// is a scan gone silent. A non-empty reason means the dispatch could not be read at all, which the
// caller reports rather than passing off as an empty list.
func (c *checker) goDispatchLabels(base, tool string) (labels []string, why string) {
	dir := shell.Join(shell.Join(c.root.Named(), "tools"), tool)
	named := shell.CutBytes(shell.Oneline(dir), 120)
	if !shell.IsDir(dir) {
		return nil, "no source directory at " + named
	}
	marker := "usage: " + base + " {"
	for _, file := range c.filesNamed(dir, "*.go") {
		// A test file's fixtures hold dispatch switches of their own.
		if strings.HasSuffix(shell.BaseName(file), "_test.go") {
			continue
		}
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		if found := switchLabelsCarrying(lines, marker); len(found) > 0 {
			return found, ""
		}
	}
	return nil, "no switch under " + named + " refuses with a '" + shell.Oneline(marker) + "' line"
}

// The labels of the first switch in these lines that carries the marker. A switch runs from its
// `switch … {` line to the `}` at that line's own indentation, and only `case` lines at the arm's own
// indentation belong to it — a nested switch's arms are its own.
func switchLabelsCarrying(lines []string, marker string) []string {
	var labels []string
	indent, inside, carries := "", false, false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if !inside {
			if strings.HasPrefix(trimmed, "switch ") && strings.HasSuffix(trimmed, "{") {
				indent, inside, carries, labels = line[:len(line)-len(trimmed)], true, false, nil
			}
			continue
		}
		if line == indent+"}" {
			if carries {
				return shell.SortUnique(labels)
			}
			inside = false
			continue
		}
		carries = carries || strings.Contains(line, marker)
		if rest, ok := strings.CutPrefix(line, indent+"case "); ok {
			labels = append(labels, caseStringLabels(rest)...)
		}
	}
	return nil
}

// The string-literal labels of one Go `case "a", "b":` line — or none at all, when any label on it is
// not a plain subcommand name. Half a switch's arms read as subcommands would be worse than none of
// them: the halves that parsed would be checked for call sites and the rest would go unmentioned.
func caseStringLabels(rest string) []string {
	var labels []string
	for _, part := range strings.Split(strings.TrimSuffix(strings.TrimRight(rest, " \t"), ":"), ",") {
		part = strings.Trim(part, " \t")
		if len(part) < 3 || part[0] != '"' || part[len(part)-1] != '"' {
			return nil
		}
		name := part[1 : len(part)-1]
		if !subcommandName.MatchString(name) {
			return nil
		}
		labels = append(labels, name)
	}
	return labels
}

// The names in the first list that the second does not hold.
func onlyIn(names, other []string) []string {
	has := map[string]bool{}
	for _, name := range other {
		has[name] = true
	}
	var missing []string
	for _, name := range names {
		if !has[name] {
			missing = append(missing, name)
		}
	}
	return missing
}
