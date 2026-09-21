package ecoguide

// The two terminal emitters over the same tree the page is generated from: `--graph`, the workflow
// map, and `--cost <skill>`, one skill's per-run tier profile.
//
// They live here rather than in model-policy because neither question can be answered from the
// policy document alone. The policy prices a row; what it cannot say is which rows one run of a
// skill actually reaches, and that is the whole of both answers. The tree holds the edges and the
// policy holds the tiers, and this file is the only place the two are joined for a reader rather
// than for a page.
//
// Nothing here is committed and nothing gates. `--check` exists because field-guide.html is a file
// that can rot between edits; these two are computed when asked and cannot be stale by construction,
// so a guide unit holding their output would only pin today's tree into a fixture.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	ecoroot "configs/ai/tools/eco-root"
	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/shell"
)

// An edge is one file naming another by its `~/.kk-flavor/` path, or — for the Lanes table alone — by
// bare name. Three kinds, and two of them cost something: `ecosystem.md` → **Three kinds, two homes**
// makes a **dispatch** the edge that spends a row of its own, and **extension** the one that runs a
// second contract inside this session, billing at this row.
//
// **`reads` is the residue, and it is free.** Sequencing names the stage after this one, which is a
// different run; orientation merely points. Neither can be told from the other by the citation, and
// neither costs this run anything, so neither needs telling apart. What had to be separated from them
// was extension, and that is declared rather than guessed — `shell.ExtendsDeclarations`. An earlier
// version guessed, by treating every read as extension, and billed `idsd-ship` for four stages it
// sequences.
type edgeKind string

const (
	dispatches edgeKind = "dispatches"
	extendsOne edgeKind = "extends"
	reads      edgeKind = "reads"
)

type edge struct {
	to   string
	kind edgeKind
}

// A node is a skill or a worker, with its outgoing edges and the tiers its own row buys.
type node struct {
	name string
	// `dispatched`, `orchestrator` or `holds — <reason>` as a skill declares it, `mode of <skill>`
	// for a row keyed under one, and empty for a worker, which declares nothing because having no
	// door is the declaration.
	mode   string
	isWork bool
	out    []edge
	claude string
	codex  string
}

// The whole map: every priced row as a node, keyed by the name its row is written under.
type workflow struct {
	skills  []string
	workers []string
	nodes   map[string]node
}

// A skill reaches a target by naming its path. A worker's own name has no prefix, so the two
// patterns differ only in which tree they enter; both stop at the same character class the rest of
// the tree's names are written in.
//
// Matched anywhere on a line, including inside a table cell and inside backticks, because every
// spelling in the tree is one of those and a scan that only read prose would miss the Lanes table —
// which is where the quality pass names most of what it dispatches.
const (
	workerRefPrefix = "~/.kk-flavor/workers/"
	skillRefPrefix  = "~/.kk-flavor/skills/"
)

// readWorkflow builds the map from the policy's rows and the tree's citations.
//
// The rows are the node set, not the directories. A row with no file is still a dispatch that costs
// money, and a file with no row is what the policy census refuses — so building from rows means this
// map prices exactly what a run would pay, and never invents a node the policy has never heard of.
//
// It refuses rather than returning a map where a declaration did not parse. Both declarations report
// unreadable apart from absent for one reason — read as silence, a `**Runs:**` nobody can parse takes
// whatever the ceiling implies and an unreadable `**Extends:**` prices its contract's dispatches at
// zero — and a reader who is handed a number has no way to know a line was skipped. The shipped tree
// is held to this by the policy suite; this is the same guarantee for every other checkout, which is
// where `--cost` is actually pointed while a tree is being edited.
func readWorkflow(root ecoroot.Root, policy *modelpolicy.Policy) (workflow, error) {
	tier := func(task, client string) string {
		decision, err := policy.Resolve(modelpolicy.Request{Client: client, Task: task})
		if err != nil {
			return ""
		}
		return strings.TrimSpace(decision.Requested.Model + " " + decision.Requested.Effort)
	}

	map_ := workflow{
		skills:  append([]string(nil), policy.SessionTasks()...),
		workers: append([]string(nil), policy.WorkerTasks()...),
		nodes:   map[string]node{},
	}
	sort.Strings(map_.skills)
	sort.Strings(map_.workers)

	known := map[string]bool{}
	for _, name := range map_.skills {
		known[name] = true
	}
	workerRows := map[string]bool{}
	for _, name := range map_.workers {
		workerRows[name] = true
	}

	// A session row keyed under a skill — `kk-pr/refine-description` — is one of that skill's modes,
	// priced apart because it is the cheap one. It has no SKILL.md and declares nothing, which is
	// correct rather than missing, so it is labelled for what it is: printing `declares nothing`
	// beside it would report the tree's own design as a defect on every run.
	modes := map[string]string{}
	extended := map[string]map[string]bool{}
	for _, name := range map_.skills {
		if owner, _, isMode := strings.Cut(name, "/"); isMode && known[owner] {
			modes[name] = "mode of " + owner
			continue
		}
		path := shell.Join(shell.Join(root.Skills(), name), "SKILL.md")
		lines, err := readLines(path)
		if err != nil {
			continue
		}
		mode, runsDeclared := shell.RunsDeclaration(lines)
		if runsDeclared && mode == "" {
			return workflow{}, fmt.Errorf("%s declares how it runs in a form this cannot read; it is "+
				"`**Runs:** dispatched`, `**Runs:** orchestrator` or `**Runs:** holds — <reason>`", path)
		}
		modes[name] = mode
		declared, extendsDeclared := shell.ExtendsDeclarations(lines)
		if extendsDeclared && len(declared) == 0 {
			return workflow{}, fmt.Errorf("%s declares an extension in a form this cannot read; it is "+
				"`**Extends:** <skill> — <when>`, and the when is required", path)
		}
		if len(declared) == 0 {
			continue
		}
		extended[name] = map[string]bool{}
		for _, target := range declared {
			extended[name][target] = true
		}
	}

	for _, name := range map_.skills {
		map_.nodes[name] = node{
			name:   name,
			mode:   modes[name],
			out:    edgesFrom(skillFiles(root, name), name, known, workerRows, extended[name]),
			claude: tier(name, "claude"),
			codex:  tier(name, "codex"),
		}
	}
	owners := policy.PromptOwners()
	for _, name := range map_.workers {
		// Through the shared resolver, not through `workers/<name>.md`: six of this tree's rows hold
		// their contract somewhere else — three skills that kept doors, two rows borrowing a prompt,
		// one a Go tool assembles — and reading only the first home printed every one of them as a
		// leaf that dispatches nothing. `kk-ecosystem` dispatches `skillcraft` at opus.
		var files []string
		found := resolvePrompt(root, name, owners)
		if found.file != "" {
			files = []string{found.file}
		}
		// A worker row whose contract is a SKILL.md is one of the three lanes that kept a door, and
		// that door is what makes "what does this cost me" a question a human can ask of it. Read the
		// declaration off the same file, so `--cost` can tell those three from a worker nobody types.
		mode := ""
		if found.kind == skillContract {
			if lines, err := readLines(found.file); err == nil {
				declared := false
				mode, declared = shell.RunsDeclaration(lines)
				if declared && mode == "" {
					return workflow{}, fmt.Errorf("%s declares how it runs in a form this cannot read; it is "+
						"`**Runs:** dispatched`, `**Runs:** orchestrator` or `**Runs:** holds — <reason>`", found.file)
				}
			}
		}
		map_.nodes[name] = node{
			name:   name,
			mode:   mode,
			isWork: true,
			// `found.owner`, not `name`: a borrowed prompt's pointers at its own assets are that
			// row's self-citations, whoever is running it.
			out:    edgesFrom(files, found.owner, known, workerRows, extended[found.owner]),
			claude: tier(name, "claude"),
			codex:  tier(name, "codex"),
		}
	}
	return map_, nil
}

// Every markdown file a skill runs out of, not just its SKILL.md. `kk-pr/review.md` and
// `kk-qualify/stage-results.md` are modes the session reads inline, so what they dispatch is what
// that skill dispatches — a scan stopping at SKILL.md would price `kk-pr review` as reaching nothing.
//
// **A row keyed under a skill is one file, not a directory.** `kk-pr/refine-description` is priced
// apart because it is the cheap mode, and its contract is `skills/kk-pr/refine-description.md`;
// walking `skills/kk-pr/refine-description/` finds nothing, so that row read as dispatching nothing
// while its citations were billed to the expensive row beside it.
//
// Non-regular entries are skipped. A `.md` symlink under a skill directory would put an out-of-tree
// file's citations into the map, and a `.md` FIFO would hang the tool on the read — neither is a
// thing this walk had to consider before it went recursive.
func skillFiles(root ecoroot.Root, name string) []string {
	if owner, mode, isMode := strings.Cut(name, "/"); isMode {
		file := shell.Join(shell.Join(root.Skills(), owner), mode+".md")
		if shell.IsRegularFile(file) {
			return []string{file}
		}
		return nil
	}
	var found []string
	dir := shell.Join(root.Skills(), name)
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !entry.Type().IsRegular() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		found = append(found, path)
		return nil
	})
	sort.Strings(found)
	return found
}

// Column two of the Lanes table in `kk-qualify`'s contract, which is the one place in the tree where
// a dispatch is written as a bare name rather than as a path. The table says why: a `workers/` path
// is a prompt you dispatch and a bare name is a door you invoke, and the three lanes that kept their
// doors are named the second way.
//
// **Without this, the map missed every one of them.** `kk-edit` and `kk-ecosystem` appeared anyway,
// off column three's `scripts/*.sh` paths cut at the first slash — the right answer for the wrong
// reason — and `kk-diagnose`, which owns no script, appeared nowhere in the map at all.
//
// The pattern is `model-policy`'s own `lanesTableRow`, which already reads this table for the policy
// census, and it is narrow by construction: column one must be a single lowercase word, which the
// Lanes table's rows are and which `kk-foreman`'s routing table — a sentence per row — is not. So
// this reaches the one table that spells a dispatch this way and no other.
var lanesTableRow = regexp.MustCompile("(?m)^\\| *[a-z-]+ *\\| *`(?:~/\\.kk-flavor/workers/)?([a-z0-9/-]+?)(?:\\.md)?` *\\|")

// The edges out of one node's files, deduplicated and ordered so two runs over one tree agree.
//
// **Which map the target's row is in decides the kind**, not which tree its file sits in. Those two
// agree for every worker under `workers/` and disagree for exactly three files: `kk-diagnose`,
// `kk-ecosystem` and `kk-edit` kept their doors while becoming lanes, so each is a `skills/` path
// with a `workers` row. Classifying by directory would put them on the free side of the ledger —
// which is the error those rows were moved to stop telling — and `models.json`'s two maps are the
// tree's own answer to which side a name is on.
//
// **Only a path naming a `.md` file is an edge**, on both sides. The tree cites a lane's scripts as
// `~/.kk-flavor/skills/<skill>/scripts/<x>.sh`, 8 of them from 18 sites, and reading the first
// segment of those put an opus dispatch on the bill of every skill that runs a shell script —
// `kk-reduce`'s invocation of `check.sh` priced as a dispatch of `kk-ecosystem`. A script costs
// nothing of its own: it runs inside whatever already paid for the file beside it.
//
// **`self` is the row whose prompt this is, which is not always the row being priced.** A row that
// borrows another's prompt reads that file, and the file points at its own assets; dropped against
// the borrower's name instead, those pointers left the tree as dispatches and billed `reduce/fan-out`
// for a second run of the contract it *is*.
//
// A path naming neither map is passed over rather than reported: this is an emitter, and a citation
// to a file the policy has never heard of is eco-check's finding to make.
func edgesFrom(files []string, self string, skills, workers, extends map[string]bool) []edge {
	seen := map[edge]bool{}
	classify := func(name string) {
		switch {
		case name == self:
		case workers[name]:
			seen[edge{to: name, kind: dispatches}] = true
		case skills[name] && extends[name]:
			seen[edge{to: name, kind: extendsOne}] = true
		case skills[name]:
			seen[edge{to: name, kind: reads}] = true
		}
	}
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, row := range lanesTableRow.FindAllStringSubmatch(string(body), -1) {
			classify(row[1])
		}
		for _, line := range shell.SplitLines(string(body)) {
			for _, ref := range refsOn(line, workerRefPrefix) {
				if name, isPrompt := strings.CutSuffix(ref, ".md"); isPrompt {
					classify(name)
				}
			}
			for _, ref := range refsOn(line, skillRefPrefix) {
				// The first segment, because a skill is cited through any of its files and every one
				// of them is that skill's contract: `kk-pr/review.md` is `kk-pr`. Only where the ref
				// names markdown — a `scripts/` path names an instrument, not a contract.
				if strings.HasSuffix(ref, ".md") {
					name, _, _ := strings.Cut(ref, "/")
					classify(name)
				}
			}
		}
	}
	out := make([]edge, 0, len(seen))
	for one := range seen {
		out = append(out, one)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].kind != out[b].kind {
			return out[a].kind < out[b].kind
		}
		return out[a].to < out[b].to
	})
	return out
}

// What follows each occurrence of prefix on one line, cut at the first character a name in this tree
// cannot hold. Written as a scan rather than a regexp because the two prefixes are literal and the
// tail's character class is the only rule — `[A-Za-z0-9._/-]`, the same one eco-check's home-ref
// scan admits, so a path one of them follows is a path the other reports on.
func refsOn(line, prefix string) []string {
	var found []string
	for rest := line; ; {
		at := strings.Index(rest, prefix)
		if at < 0 {
			return found
		}
		rest = rest[at+len(prefix):]
		end := 0
		for end < len(rest) && isRefByte(rest[end]) {
			end++
		}
		if end > 0 {
			found = append(found, rest[:end])
		}
		rest = rest[end:]
	}
}

func isRefByte(b byte) bool {
	return shell.IsAlnumByte(b) || b == '.' || b == '_' || b == '/' || b == '-'
}

// emitGraph writes the workflow map: every skill with what it dispatches and what it reads, then
// every worker row, with what it goes on to reach where it reaches anything.
//
// **Every priced row prints its tier**, on its own line and again beside each dispatch of it. Only
// the skills half carried tiers at first, which left the map silent about 22 of its 40 rows — and
// `--cost`'s refusal on a worker redirects the reader here for exactly that number. Two rows reach
// the map only here: `reader-judge` and `kk-diagnose` have no outgoing edge, and the `~/.kk-flavor/`
// tree leaves both unnamed, so an earlier version that printed only the workers
// which reach further left them out of a map whose own header counted them.
func emitGraph(map_ workflow, out io.Writer) {
	fmt.Fprintf(out, "workflow map — %d skills, %d workers, priced by models.json\n", len(map_.skills), len(map_.workers))
	fmt.Fprintf(out, "%s\n\n", graphLegend)
	for _, name := range map_.skills {
		one := map_.nodes[name]
		fmt.Fprintf(out, "%s  [%s]\n", name, or(one.mode, "declares nothing"))
		fmt.Fprintf(out, "    runs at  %s\n", clientTiers(one))
		emitEdges(map_, one, out)
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, "the dispatch sites underneath, and what each goes on to reach:")
	for _, name := range map_.workers {
		one := map_.nodes[name]
		fmt.Fprintf(out, "\n%s\n", name)
		fmt.Fprintf(out, "    runs at  %s\n", clientTiers(one))
		emitEdges(map_, one, out)
	}
}

// One node's outgoing edges, each dispatch carrying the tier it buys. The tier rides on the edge and
// not only on the target's own entry, because the question a reader brings to this map is what a
// given door costs, and answering it otherwise means scrolling to a second place per lane.
func emitEdges(map_ workflow, one node, out io.Writer) {
	for _, to := range one.out {
		if to.kind == dispatches {
			fmt.Fprintf(out, "    %-10s %-24s %s\n", to.kind, to.to, clientTiers(map_.nodes[to.to]))
			continue
		}
		fmt.Fprintf(out, "    %-10s %s\n", to.kind, to.to)
	}
}

// Three sentences, and each is something a reader would otherwise get wrong. The pricing rule; the
// map's reach, since a routing table naming a skill in bare prose — `kk-foreman`'s is the whole of
// that skill — sends a human somewhere rather than reading a contract, and counting it would put the
// tree on one row's bill; and the one ambiguity nothing in the tree resolves.
const graphLegend = "a dispatch spends a row of its own; a read runs in the reading session and bills at its row\n" +
	"edges are ~/.kk-flavor/ path citations, plus kk-qualify's Lanes table, which names a door by bare name —\n" +
	"a skill named in ordinary prose routes a human and costs nothing here\n" +
	"a citation of kk-diagnose, kk-ecosystem or kk-edit reads as a dispatch of it: each is a door over a\n" +
	"priced row, and nothing separates dispatching one from pointing at it"

// What a row with no readable tier prints. One spelling, because it appears in both emitters and a
// reader comparing them has to see the same words for the same absence.
const noTier = "no tier"

func clientTiers(one node) string {
	return fmt.Sprintf("claude %s · codex %s", or(one.claude, noTier), or(one.codex, noTier))
}

// A line of the cost profile: one row the run pays for, and how the run got there.
type charge struct {
	name string
	// Empty when the skill names it directly; otherwise the chain of reads that carried it here, so
	// a reader can see that a bill they did not expect came through a contract their skill extends.
	via    string
	claude string
	codex  string
}

// emitCost writes one skill's per-run tier profile: its own session at its own row, then every row a
// run of it can reach.
//
// **Extension is followed and nothing else is.** A declared extension runs the second contract inside
// this session, so what that contract dispatches is on this bill (`ecosystem.md` → **Three kinds, two
// homes** → **Extension is free of a row, not free**). A dispatch is already priced at the row on the
// line, and folding in what it then dispatches would charge one run twice. A plain read is a pointer
// or the next stage, and costs this run nothing either way.
//
// The profile was two columns for as long as extension was a guess: an earlier version walked every
// skill-to-skill edge and billed `idsd-ship` for four stages it sequences, so the inherited rows had
// to be printed as conditional. With the edge declared there is one number again, and the chain a row
// arrived through is printed beside it rather than as a hedge.
//
// "Can reach" rather than "does reach": no run takes every branch, so this is the ceiling a run could
// pay. Saying so is emitCost's, because a column of tiers with no such sentence reads as a receipt.
func emitCost(self string, map_ workflow, skill string, out, errOut io.Writer) int {
	start, known := map_.nodes[skill]
	switch {
	case known && start.isWork && start.mode != "dispatched":
		// A worker with no door has a profile of one line — its own row — and that line is already
		// printed beside every skill that dispatches it. Answering invites reading it as this
		// worker's share of a run, which it is not. The three lanes that kept their doors are the
		// exception and fall through: a human types those, so "what does this cost me" is a question
		// they can actually ask.
		fmt.Fprintf(errOut, "%s: %s is a worker with no door, and its cost is the row --graph prints beside every skill that dispatches it\n", self, skill)
		return 2
	case !known:
		fmt.Fprintf(errOut, "%s: nothing named %q has a row in models.json; --graph lists what does\n", self, skill)
		return 2
	}

	charges := map[string]charge{}
	seen := map[string]bool{skill: true}
	// Breadth first, so the shortest chain of extensions is the one a row is reported under. Depth
	// first would attribute a row to whichever branch happened to be walked first, which changes with
	// the citing file's name and not with the tree's shape.
	type step struct {
		name string
		via  string
	}
	queue := []step{{name: skill}}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, to := range map_.nodes[at.name].out {
			target := map_.nodes[to.to]
			if to.kind == dispatches {
				// Keep the first reach, which is always the shortest: every out-edge of a node is
				// recorded in one pass before any extension out of it is followed, so a worker the
				// starting skill dispatches on its own face is charged with no chain before any
				// chain can reach it.
				if _, already := charges[to.to]; !already {
					charges[to.to] = charge{name: to.to, via: at.via, claude: target.claude, codex: target.codex}
				}
				continue
			}
			if to.kind != extendsOne || seen[to.to] {
				continue
			}
			seen[to.to] = true
			queue = append(queue, step{name: to.to, via: joinVia(at.via, to.to)})
		}
	}

	ordered := make([]charge, 0, len(charges))
	for _, one := range charges {
		ordered = append(ordered, one)
	}
	sort.Slice(ordered, func(a, b int) bool { return ordered[a].name < ordered[b].name })

	// The columns are sized to what is actually in them. A pinned width reads fine until one row is
	// `idsd/qualify/reconcile`, and then every tier steps right by the overflow and the two clients
	// stop being columns at all — on the one profile deep enough to need reading.
	label, claude := len("what runs"), len("claude")
	for _, one := range ordered {
		label = max(label, len("dispatch ")+len(one.name))
		claude = max(claude, len(or(one.claude, noTier)))
	}
	label = max(label, len("session  ")+len(skill))
	claude = max(claude, len(or(start.claude, noTier)))

	fmt.Fprintf(out, "%s  [%s]\n", skill, or(start.mode, "declares nothing"))
	if reason := shell.RunsHoldsReason(start.mode); reason != "" {
		fmt.Fprintf(out, "holds its own work — %s — so its row is the work it keeps\n", reason)
	}
	fmt.Fprintf(out, "\n%-*s  %-*s  %s\n", label, "what runs", claude, "claude", "codex")
	fmt.Fprintf(out, "%-*s  %-*s  %s\n", label, "session  "+skill, claude, or(start.claude, noTier), or(start.codex, noTier))
	for _, one := range ordered {
		fmt.Fprintf(out, "%-*s  %-*s  %s", label, "dispatch "+one.name, claude, or(one.claude, noTier), or(one.codex, noTier))
		if one.via != "" {
			fmt.Fprintf(out, "   (through %s, which it extends)", one.via)
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "\n%d row(s) one run can reach, each once. A run takes some branches and not others, so this is what it can spend rather than a receipt.\n", len(ordered))
	return 0
}

func joinVia(via, name string) string {
	if via == "" {
		return name
	}
	return via + " → " + name
}
