package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

// The tool reads whole files, and the tree supplying them is the tree under review. The bound and
// its reason are shell.MaxFileBytes; over it, a file is named and not read, so one nothing read is
// never mistaken for one that held no citations.
const maxFileBytes = shell.MaxFileBytes

// `<file>.md → **Section**`, with the file optionally carrying a path or backticks. The arrow is the
// whole signal: it is what separates naming a file from entering one at a named door.
//
// The run before the arrow admits a line break, and so does the section name, because a citation
// wraps: a formatter breaks the line and the citation is still the citation. Read one line at a
// time it is invisible, the door it opens goes uncounted, and the section it enters is reported
// UNENTERED while a file enters it.
var citePattern = regexp.MustCompile("`?([A-Za-z0-9._/~-]+\\.md)`?[\\s\\S]{0,4}→\\s*\\*\\*([^*]+)\\*\\*")

// `and → **Section**` running on from a citation that already named the file.
var chainPattern = regexp.MustCompile(`^\s*(?:and|or|,)?\s*→\s*\*\*([^*]+)\*\*`)

type edge struct {
	from, to, section string
	// The citer holds the target whole, so this citation names which rule rather than opening a door.
	precision bool
}

// Where the router lives under the root every tool here is pointed at. Named as a path, because the
// router is a node in this graph and a node keys on its path: found by basename, any `inject.md` a
// lane committed answered for it, and which one answered came out of a map iteration — the same tree
// gave different numbers on different runs.
const routerPath = "kk-flavor/inject.md"

// The files the router lists under its read-always heading. Which files those are is read from the
// router rather than named here, so a file promoted into or out of that tier is picked up without
// editing this.
func routerSets(root string, defined map[string]map[string]bool) (always, routed map[string]bool) {
	always, routed = map[string]bool{}, map[string]bool{}
	if _, ok := defined[routerPath]; !ok {
		return always, routed
	}
	body, err := os.ReadFile(filepath.Join(root, routerPath))
	if err != nil {
		return always, routed
	}
	routerDir := filepath.Dir(filepath.Join(root, routerPath))
	// Every file the router names is entered whole on its trigger, read-always or not. That decides
	// what an unentered section means in it: nothing, because its readers never enter by section.
	isAlways := false
	for _, line := range shell.SplitLines(string(body)) {
		if strings.HasPrefix(line, "#") {
			isAlways = strings.Contains(strings.ToLower(line), "read always")
			continue
		}
		for _, m := range linkPattern.FindAllStringSubmatch(line, -1) {
			target := relOf(root, filepath.Join(routerDir, m[1]))
			if _, ok := defined[target]; !ok {
				continue
			}
			routed[target] = true
			if isAlways {
				always[target] = true
			}
		}
	}
	routed[routerPath] = true // the router itself: nothing cites a router's own headings
	return always, routed
}

var linkPattern = regexp.MustCompile(`\]\(([^)]+\.md)\)`)

// The comparison form for a heading and for the section a citation names. A wrapped citation carries
// the break inside the name, so the whitespace runs are flattened on both sides — comparing a
// flattened citation against a raw heading would refuse the citation the wrap was the only fault in.
func flattened(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func relOf(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}

// A citation before its target is known. Resolution is a second pass, because a bare `writing.md`
// names whichever file carries that basename, and only the whole tree answers that.
type rawCite struct{ from, fromDir, target, section string }

// A `*.md` token that is NOT part of a `→ **Section**` citation: the shape of "You run under
// `<file>`", which means the citer holds the whole file already.
var bareTokenPattern = regexp.MustCompile(`[A-Za-z0-9._/~-]+\.md`)

// Everything one pass over the tree found, before any citation is resolved to the file it names.
type scan struct {
	root string
	// Where a citation this scan refuses to count says so. A dropped citation moves every figure the
	// report prints, and this line is the only signal it went.
	errOut io.Writer
	// How many paths under the root this scan was pointed at and did not read. A stderr line alone
	// leaves the skip unreachable from the exit code, so a caller reading the status — `cite-graph.sh`
	// and CI both do — takes a scan that missed most of the tree for a clean one.
	skipped int
	// The headings each file defines, keyed by the file's path relative to the root.
	defined map[string]map[string]bool
	// The `<file> → **Section**` citations, in the order they were written.
	cites []rawCite
	// The bare `<file>` mentions: a citer naming a file outside a citation holds that file whole.
	mentions []rawCite
}

// The tree's headings and citations, plus how many paths under the root went unread. The third return
// is what the exit code is owed to: without it a caller can only tell a tree with no citations from a
// tree this never reached by reading prose on stderr, and nothing does.
func read(root string, errOut io.Writer) (map[string]map[string]bool, []edge, int) {
	found := scanTree(root, errOut)
	return found.defined, found.edges(), found.skipped
}

// A path this scan was pointed at and did not read. Every figure this tool prints is a count over the
// files it read, so one dropped moves all of them: the line says which path went, and the count is
// what makes the skip reachable from anywhere but a human's eye. Names reaching here are the tree's
// own — printed raw, a newline in one forges a row of this tool's own report.
func (s *scan) notReached(format string, args ...any) {
	s.skipped++
	fmt.Fprintf(s.errOut, format, args...)
}

// Walk stats with Lstat, so a symlink is never a directory here and never a regular file. The
// installed layout is a symlink farm — `~/.kk-flavor` is one and every `~/.claude/skills/*` is one —
// and this tool promises every `.md` under the root is read, so a link it does not follow is a hole in
// that promise rather than a path to pass over.
//
// A link resolving to a directory hides however many `.md` files that subtree holds, and it hides them
// where no `.md` suffix marks the path, which is why this runs before the suffix filter. A link named
// `.md` hides exactly one file, and a live citation into it comes back as a manufactured `no such
// path` that reads like a broken link the tree really has. A link to anything else was never this
// tool's to read.
func (s *scan) linkNotFollowed(p string) {
	resolved, err := os.Stat(p)
	switch {
	case err != nil:
		if strings.HasSuffix(p, ".md") {
			s.notReached("could not resolve %s: %s — it was NOT read\n", shell.Oneline(p), shell.Oneline(err.Error()))
		}
	case resolved.IsDir():
		s.notReached("not followed: %s links to a directory — the .md files under it were NOT read\n", shell.Oneline(p))
	case strings.HasSuffix(p, ".md"):
		s.notReached("not a regular file: %s — it was NOT read\n", shell.Oneline(p))
	}
}

// One pass over the tree, reading every markdown file it can.
func scanTree(root string, errOut io.Writer) scan {
	found := scan{root: root, errOut: errOut, defined: map[string]map[string]bool{}}
	// The callback answers every path with nil, so the walk itself has nothing left to report.
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			found.notReached("could not read %s: %s — it was NOT read\n", shell.Oneline(p), shell.Oneline(err.Error()))
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if !fi.Mode().IsRegular() {
			found.linkNotFollowed(p)
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		if fi.Size() > maxFileBytes {
			found.notReached("file too large to scan: %s is %d bytes, over the %d-byte bound — it was NOT read\n", shell.Oneline(p), fi.Size(), maxFileBytes)
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			found.notReached("could not read %s: %s — it was NOT read\n", shell.Oneline(p), shell.Oneline(err.Error()))
			return nil
		}
		found.readFile(p, body)
		return nil
	})
	return found
}

// One file's headings, citations and bare mentions, added to what the scan holds.
func (s *scan) readFile(path string, body []byte) {
	self := relOf(s.root, path)
	if s.defined[self] == nil {
		s.defined[self] = map[string]bool{}
	}
	// One block of adjacent lines, read as the citation the writer wrote rather than as the lines a
	// formatter left. Headings stay per line, because a heading is a line.
	readBlock := func(text string) {
		spans := citePattern.FindAllStringIndex(text, -1)
		lastNamed := ""
		for i, c := range citePattern.FindAllStringSubmatch(text, -1) {
			lastNamed = c[1]
			s.cites = append(s.cites, rawCite{from: self, fromDir: filepath.Dir(path), target: c[1], section: flattened(c[2])})
			// A second section chained onto the first (`X → **A** and → **B**`) names no file of its
			// own, so the pattern above sees one citation where the line makes two claims. Both bind;
			// counting one makes the target's door surface read narrower than it is.
			for _, extra := range chainPattern.FindAllStringSubmatch(text[spans[i][1]:], -1) {
				s.cites = append(s.cites, rawCite{from: self, fromDir: filepath.Dir(path), target: lastNamed, section: flattened(extra[1])})
			}
		}
		// Whether the citer also holds the file whole. Without this the metric inverts: the more
		// precisely a skill cites a file it has already read, which is what `ecosystem.md` →
		// **Conventions a new file joins** demands, the wider its target's surface appears. Those
		// citations are precision, not doors.
		for _, at := range bareTokenPattern.FindAllStringIndex(text, -1) {
			inCite := false
			for _, sp := range spans {
				if at[0] >= sp[0] && at[0] < sp[1] {
					inCite = true
					break
				}
			}
			if !inCite {
				s.mentions = append(s.mentions, rawCite{from: self, fromDir: filepath.Dir(path), target: text[at[0]:at[1]]})
			}
		}
	}

	inFence := false
	var block []string
	closeBlock := func() {
		if len(block) > 0 {
			readBlock(strings.Join(block, "\n"))
			block = nil
		}
	}
	for _, line := range shell.SplitLines(string(body)) {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			closeBlock()
			continue
		}
		if inFence {
			continue
		}
		if strings.TrimSpace(line) == "" {
			closeBlock()
			continue
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			s.defined[self][flattened(strings.Trim(m[1], "*"))] = true
		}
		if len(block) == 0 {
			block = append(block, line)
			continue
		}
		// A continued line's indentation is layout, never part of a path or a section name.
		block = append(block, strings.TrimLeft(line, " \t"))
	}
	closeBlock()
}

// The edges the scanned citations amount to: one counts where it names a file this tree defines and
// enters a heading that file really has.
func (s *scan) edges() []edge {
	resolver := newNameResolver(s.root, s.defined, s.errOut)
	// A file the router marks read-always is held whole by every reader on every task, so no citation
	// into it is a door: the reader already has it and the citation only says which rule. A bare
	// mention in the citer cannot detect that, because nothing mentions the file; inject.md loaded
	// it. Without this the always-read set reads as the widest surface in the tree, the opposite of
	// what always-read means.
	alwaysRead, _ := routerSets(s.root, s.defined)
	readsWhole := map[string]bool{}
	for _, m := range s.mentions {
		if to := resolver.fileNamed(m); to != "" && to != m.from {
			readsWhole[m.from+">"+to] = true
		}
	}
	var edges []edge
	for _, c := range s.cites {
		to := resolver.fileNamed(c)
		// A file citing its own section is navigation, not a dependency.
		if to == "" || to == c.from {
			continue
		}
		// A bolded list item matches the citation shape and resolves to no heading, so counting it
		// would add a door to a section that does not exist and inflate the very number this tool
		// reports. check.sh reports the dangling reference; this refuses to measure it.
		heading, ok := entersAHeading(s.defined[to], c.section)
		if !ok {
			fmt.Fprintf(s.errOut, "no such section: %s cites %s → **%s**, which is no heading there — NOT counted\n",
				shell.Oneline(c.from), shell.Oneline(to), shell.Oneline(c.section))
			continue
		}
		// The heading matched, never the string cited. Keyed on the citation, a section reached
		// through the truncation rule or the em-dash alias is reported UNENTERED while files enter it.
		edges = append(edges, edge{
			from:      c.from,
			to:        to,
			section:   heading,
			precision: readsWhole[c.from+">"+to] || alwaysRead[to],
		})
	}
	return edges
}

// The basenames every skill lane carries. Such a name is the *kind* of file rather than one of them
// ("run the skill in full, per its SKILL.md" names no file), so a citation naming one is a generic
// reference, not a dangling one to report. check.sh drops these before it reports anything.
//
// Every lane, not merely several: a basename two or three files answer to is genuinely ambiguous and
// stays reported, because nothing about it says which was meant. Below two lanes there is no "every
// lane" to speak of; one lane's whole contents would qualify, and a name shared between that lane and
// the shared layer is exactly the ambiguity worth reporting.
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

// Which file a citation names, over one scanned tree. Both indexes below are derived from the tree
// once, and every lookup shares them.
type nameResolver struct {
	root    string
	defined map[string]map[string]bool
	byBase  map[string][]string
	kinds   map[string]bool
	errOut  io.Writer
}

func newNameResolver(root string, defined map[string]map[string]bool, errOut io.Writer) *nameResolver {
	byBase := map[string][]string{}
	for f := range defined {
		byBase[filepath.Base(f)] = append(byBase[filepath.Base(f)], f)
	}
	return &nameResolver{root: root, defined: defined, byBase: byBase, kinds: kindBasenames(defined), errOut: errOut}
}

// A path resolves as one; a bare basename resolves only when exactly one file answers to it, and an
// ambiguous one is reported rather than guessed. Guessing welds the tree's twenty-two SKILL.md files
// into one node and reports chains nobody walks through it.
func (r *nameResolver) fileNamed(c rawCite) string {
	if strings.ContainsRune(c.target, '/') {
		// The forms one cited path is written in: relative to the citer, and with each mount prefix
		// off, because a citation names `~/.kk-flavor/standards/writing.md` for a file this graph
		// keys as `kk-flavor/standards/writing.md`.
		cleaned := strings.TrimPrefix(strings.TrimPrefix(c.target, "~/"), "./")
		forms := []string{
			relOf(r.root, filepath.Join(c.fromDir, c.target)),
			cleaned,
			strings.TrimPrefix(cleaned, ".kk-flavor/"),
			strings.TrimPrefix(cleaned, ".claude/"),
		}
		for _, candidate := range forms {
			if _, ok := r.defined[candidate]; ok {
				return candidate
			}
		}
		// None of them names a file verbatim, so the path answers to the file it is the *tail* of —
		// `find -path "*/<ref>"`, the rule check.sh resolves by, and two detectors disagreeing about
		// what resolves is invisible until someone reads both. The tail is the whole of it: dropping
		// to the last segment alone lets `made/up/writing.md` answer to the real `writing.md`, and the
		// graph then reports an edge to a file the citation never named.
		return r.singleTail(c, forms)
	}
	base := filepath.Base(c.target)
	switch hits := r.byBase[base]; len(hits) {
	case 1:
		return hits[0]
	case 0:
		return ""
	default:
		// A kind names no one file, so there is nothing here to guess at and nothing to report.
		if r.kinds[base] {
			return ""
		}
		// Both are names the tree chose. Printed raw, a newline in one forges a line of this tool's own
		// report, and that line is the only signal a citation was dropped.
		fmt.Fprintf(r.errOut, "ambiguous: %s cites %s, which %d files answer to — NOT counted\n", shell.Oneline(c.from), shell.Oneline(base), len(hits))
		return ""
	}
}

// The one file the cited path is the tail of, or nothing. More than one is reported rather than
// guessed at; nothing is reported only when a file *does* carry the last segment, because that near
// miss is exactly the edge a basename key used to forge. A path answering to nothing at all stays
// quiet: it is check.sh's finding, not this tool's, and every skill citing a consuming project's own
// `.idsd/charter.md` would otherwise bury this report in paths that are correct where they land.
func (r *nameResolver) singleTail(c rawCite, forms []string) string {
	tails := r.tails(forms)
	switch len(tails) {
	case 1:
		return tails[0]
	case 0:
		// Both names are the tree's own, so both are sanitised before they are printed.
		if carriers := r.byBase[filepath.Base(forms[len(forms)-1])]; len(carriers) > 0 {
			fmt.Fprintf(r.errOut, "no such path: %s cites %s — %d file(s) carry that name and none of them is the file it names — NOT counted\n",
				shell.Oneline(c.from), shell.Oneline(c.target), len(carriers))
		}
	default:
		fmt.Fprintf(r.errOut, "ambiguous: %s cites %s, which %d files are the tail of — NOT counted\n",
			shell.Oneline(c.from), shell.Oneline(c.target), len(tails))
	}
	return ""
}

// The files any written form of the path is the tail of, in one deterministic order: a map iteration
// is what let one tree answer differently on two runs.
func (r *nameResolver) tails(forms []string) []string {
	seen := map[string]bool{}
	var tails []string
	for _, form := range forms {
		for _, candidate := range r.byBase[filepath.Base(form)] {
			if seen[candidate] || !(candidate == form || strings.HasSuffix(candidate, "/"+form)) {
				continue
			}
			seen[candidate] = true
			tails = append(tails, candidate)
		}
	}
	sort.Strings(tails)
	return tails
}
