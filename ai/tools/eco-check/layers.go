package ecocheck

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"configs/ai/tools/shell"
)

// The layering among the standards. The rule's home is ecosystem.md → **One home**: each standard
// declares which layer it is in, no standard abstains, and the defect is a citation cycle that
// CROSSES layers rather than an upward citation.
//
// What each half costs. An unlayered standard is a hole the next citation falls through in silence —
// nothing can say whether a cycle through it is legal, so one file with no declaration takes the scan
// below with it. And a cycle across layers says two files are each other's foundation: a reader has no
// file to start from, whichever they open first.
//
// Upward is not checked here and must not be. A lower file may say a higher one binds too, and a rule
// departing from another has to name the one it departs from; both are upward and neither knots
// anything.

const (
	standardWithoutLayer = "standard declares no layer: "
	unreadableLayer      = "unreadable layer declaration: "
	layerCrossingCycle   = "citation cycle across layers: "

	standardsNotADirectory = " is not a directory this scan walks into, so the layering of every"
	standardsPathHidden    = " symlink(s) under the standards were not followed, so whatever each one" +
		" names went unread and unlayered — the first is "
)

type layeredStandard struct {
	// The path as the walk built it, which is what a finding echoes.
	path  string
	layer string
	// Every file it cites, by node key, before the ones that are not layered standards are dropped.
	cites []string
}

func (c *checker) standardsDir() string {
	return shell.Join(c.root.Flavor(), "standards")
}

// Which file names a standard. Both letters are matched either way because a name is the reviewed
// tree's to choose: `rogue.MD` matched byte-exactly is a file agents read as a standard and this scan
// never opens — no layer checked, no node in the graph, and every edge through it dropped.
const standardGlob = "*.[mM][dD]"

// What one file answers to in the graph — the walked path of the file it IS, never the spelling a
// citation used to reach it.
//
// A path has several spellings and every one of them is a second node for one standard, which drops
// the edge between them and any cycle that edge would have closed. The scan then reports clean over a
// tree agents read the cycle in. Three of them are real here: a citation is relative to its citer, so
// `architecture/core.md` citing `../testing.md` carries that hop; a link committed elsewhere under the
// root gives the same file a second directory; and on a case-insensitive volume `LOW.md` and `low.md`
// are one file under two names. Identity folds all three, and hard links with them.
//
// Asking the filesystem here is not the probe underRoot refuses. Every path reaching this was already
// found on disk — by the walk, or by resolveRef, which checks containment first — so nothing stats a
// path the reviewed branch merely named. The answer is a map key alone; findings echo the walked path.
type nodeNames struct {
	walked []string
	ids    []os.FileInfo
}

func (n *nodeNames) add(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		info = nil
	}
	n.walked = append(n.walked, path)
	n.ids = append(n.ids, info)
	return path
}

func (n *nodeNames) keyOf(path string) string {
	if info, err := os.Stat(path); err == nil {
		for i, walked := range n.ids {
			if walked != nil && os.SameFile(walked, info) {
				return n.walked[i]
			}
		}
	}
	return filepath.Clean(path)
}

func (c *checker) scanStandardLayers() {
	c.assertTheStandardsCanBeWalked()
	names := &nodeNames{}
	standards := map[string]*layeredStandard{}
	var keys []string
	var bodies [][]string
	for file, lines := range c.filesWithLines(c.standardsDir(), standardGlob) {
		// A file with no lines was either not read — one over the read bound comes back empty, with its
		// own finding already on the report — or holds nothing at all. Neither is a standard that
		// declares the wrong thing, and reporting one as unlayered would name a defect this scan cannot
		// see a file to have.
		if len(lines) == 0 {
			continue
		}
		layer, declared := shell.LayerDeclaration(lines)
		if layer == "" {
			c.reportUnlayered(file, lines, declared)
			continue
		}
		key := names.add(file)
		standards[key] = &layeredStandard{path: file, layer: layer}
		keys = append(keys, key)
		bodies = append(bodies, lines)
	}
	// Citations are read only once every standard is known: a cited path answers to the walked file it
	// is, so nothing can be resolved against a set still being built.
	for i, key := range keys {
		standards[key].cites = c.citedFiles(standards[key].path, bodies[i], names)
	}
	c.reportLayerCrossingCycles(standards)
}

// A path under the standards that the walk will not enter is this whole check going quiet: the walk
// reads each directory entry's own type, so a committed symlink yields the link and nothing behind
// it, while whatever it names goes unread and this scan reports clean over it.
//
// Asked of every entry, not only of the directory's own path. Guarding the root alone leaves the
// same hole one level down: `standards/architecture` committed as a symlink hides `core.md`, and a
// cycle through it crossing layers reports `wiring: clean` where the byte-identical real-directory
// tree fails the branch. A symlinked `standards/sneaky.md` is that hole again, because only regular
// files are read.
//
// The path having nothing under it is not that: a tree with no standards, or one whose standards
// directory is empty, has nothing here to check.
func (c *checker) assertTheStandardsCanBeWalked() {
	dir := c.standardsDir()
	if shell.IsSymlink(dir) || (shell.PathExists(dir) && !shell.IsDir(dir)) {
		c.cannotRun(shell.Oneline(dir) + standardsNotADirectory + " standard behind it went unchecked")
		return
	}
	var hidden []string
	for _, entry := range c.walkTree(dir).entries {
		// Symlinks only. Every other irregular entry — a device, a socket — names no files this scan
		// would otherwise have read, and the citation scan already reports one as not a regular file.
		// Refusing on those would trade this hole for a second scan's findings turned into an exit 2.
		if entry.isSymlink() {
			hidden = append(hidden, entry.path)
		}
	}
	if len(hidden) == 0 {
		return
	}
	// One line however many there are. The reviewed tree writes this directory, so a line per entry
	// would let it choose how long this refusal is — the bound report.go holds every finding to, held
	// here too because these leave through stderr and never reach that path.
	c.cannotRun(strconv.Itoa(len(hidden)) + standardsPathHidden +
		shell.CutBytesMarked(shell.Oneline(hidden[0]), findingNameCap))
}

func (c *checker) reportUnlayered(file string, lines []string, declared bool) {
	safeFile := shell.Oneline(file)
	named := strings.Join(shell.Layers, "|")
	if !declared {
		c.add(standardWithoutLayer + safeFile + " — write `**Layer:** " + named +
			"` on its first line; no standard abstains (ecosystem.md → **One home**)")
		return
	}
	written, _ := shell.UnknownLayer(lines)
	c.add(unreadableLayer + safeFile + " declares `" + shell.CutBytesMarked(shell.Oneline(written), findingNameCap) +
		"` — the layers are " + named + ", and nothing reads a fourth (ecosystem.md → **One home**)")
}

// Every file this one cites, by node key, whether or not the target is a standard. A citation is an
// instruction to load a file, so the citation is the edge; which section it names decides nothing
// here, and a dangling one is already the citation scan's finding.
func (c *checker) citedFiles(file string, lines []string, names *nodeNames) []string {
	var cited []string
	seen := map[string]bool{}
	for _, one := range citationsIn(file, lines) {
		// The citation reportCitation refuses rather than resolves: nothing before the arrow named a
		// file. Left in, resolveRef would answer with the citing file's own directory.
		if one.path == "" {
			continue
		}
		target := c.resolveRef(shell.DirName(file), one.path)
		if target == "" {
			continue
		}
		if key := names.keyOf(target); !seen[key] {
			seen[key] = true
			cited = append(cited, key)
		}
	}
	return cited
}

func (c *checker) reportLayerCrossingCycles(standards map[string]*layeredStandard) {
	var nodes []string
	for key := range standards {
		nodes = append(nodes, key)
	}
	// Sorted, because the walk starts from these in order: a map iteration would let one tree report
	// the same cycle by two different entry points on two runs, and a finding that moves is one a
	// reviewer cannot hold a branch to.
	sort.Strings(nodes)

	adj := map[string][]string{}
	for _, key := range nodes {
		for _, target := range standards[key].cites {
			// A standard citing its own section is navigation, not a dependency. Everything outside the
			// standards goes too: a skill declares no layer, so a cycle through one is one nothing here
			// can judge — cite-graph prints those as unjudged.
			if target != key && standards[target] != nil {
				adj[key] = append(adj[key], target)
			}
		}
	}

	budget := shell.NewWalkBudget(shell.WalkSteps)
	found := 0
	for _, loop := range shell.Cycles(adj, nodes, budget) {
		if !crossesLayers(loop, standards) {
			continue
		}
		found++
		switch {
		case found <= findingCap:
			c.add(layerCrossingCycle + describeCycle(loop, standards) +
				" — each is the other's foundation, so neither can be read first; cut one citation, or move the" +
				" rule to the layer both can reach (ecosystem.md → **One home**)")
		case found == findingCap+1:
			c.add(layerCrossingCycle + strconv.Itoa(findingCap) +
				" already shown among the standards; the rest are not listed")
		}
	}
	if budget.Exhausted() {
		c.cannotRun("the standards cite each other too densely to walk exhaustively — the cycles found are a lower" +
			" bound, so one crossing layers may be unreported")
	}
}

// The files of one cycle, each with the layer it declares. Both ends of the loop are printed, so the
// reader sees where it closes.
func describeCycle(loop []string, standards map[string]*layeredStandard) string {
	named := make([]string, len(loop))
	for i, key := range loop {
		named[i] = shell.Oneline(standards[key].path) + " (" + standards[key].layer + ")"
	}
	return strings.Join(named, " → ")
}

// True when the files of one cycle do not all sit in the same layer. The repeated endpoint is dropped
// first: it is the node the loop opened on, and its layer is already counted.
func crossesLayers(loop []string, standards map[string]*layeredStandard) bool {
	first := standards[loop[0]].layer
	for _, key := range loop[1 : len(loop)-1] {
		if standards[key].layer != first {
			return true
		}
	}
	return false
}
