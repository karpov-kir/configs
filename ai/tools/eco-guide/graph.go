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

	ecoroot "kk-flavor/tools/eco-root"
	modelpolicy "kk-flavor/tools/model-policy"
	"kk-flavor/tools/shell"
)

// An edge is one file naming another by its `~/.kk-flavor/` path. Two kinds, because exactly one of
// them spends a row: `ecosystem.md` → **Three kinds, two homes** makes a dispatch the money edge and
// leaves extension, sequencing and orientation as the three that are not.
//
// Which of those three a read edge is cannot be told from the path — nothing in the tree declares
// it, and ecosystem.md names all three in one sentence. So this reports `reads` and stops there
// rather than guessing a kind; a reader who needs to know which opens the citing line, and a label
// this invented would be wrong in a way nothing could catch.
type edgeKind string

const (
	dispatches edgeKind = "dispatches"
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
func readWorkflow(root ecoroot.Root, policy *modelpolicy.Policy) workflow {
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
	for _, name := range map_.skills {
		if owner, _, isMode := strings.Cut(name, "/"); isMode && known[owner] {
			modes[name] = "mode of " + owner
			continue
		}
		lines, err := readLines(shell.Join(shell.Join(root.Skills(), name), "SKILL.md"))
		if err != nil {
			continue
		}
		modes[name], _ = shell.RunsDeclaration(lines)
	}

	for _, name := range map_.skills {
		map_.nodes[name] = node{
			name:   name,
			mode:   modes[name],
			out:    edgesFrom(skillFiles(root, name), name, known, workerRows),
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
		map_.nodes[name] = node{
			name:   name,
			isWork: true,
			// `found.owner`, not `name`: a borrowed prompt's pointers at its own assets are that
			// row's self-citations, whoever is running it.
			out:    edgesFrom(files, found.owner, known, workerRows),
			claude: tier(name, "claude"),
			codex:  tier(name, "codex"),
		}
	}
	return map_
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
func edgesFrom(files []string, self string, skills, workers map[string]bool) []edge {
	seen := map[edge]bool{}
	classify := func(name string) {
		switch {
		case name == self:
		case workers[name]:
			seen[edge{to: name, kind: dispatches}] = true
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
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '.' || b == '_' || b == '/' || b == '-':
		return true
	}
	return false
}

// emitGraph writes the workflow map: every skill with what it dispatches and what it reads, then
// every worker row, with what it goes on to reach where it reaches anything.
//
// **Every priced row prints its tier**, on its own line and again beside each dispatch of it. Only
// the skills half carried tiers at first, which left the map silent about 22 of its 40 rows — and
// `--cost`'s refusal on a worker redirects the reader here for exactly that number. Two rows are not
// reachable any other way: `bloat-judge` and `kk-diagnose` have no outgoing edge and no
// `~/.kk-flavor/` path anywhere names them, so an earlier version that printed only the workers
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

// emitCost writes one skill's per-run tier profile in two parts: what a run of it spends, and what
// it may additionally spend through the contracts it names.
//
// **The split is forced by what the tree does not declare.** A skill naming another by path is doing
// one of three things (`ecosystem.md` → **Three kinds, two homes**) and nothing says which:
// **extension** reads that contract inside this session, so its dispatches are on this bill;
// **sequencing** names the stage after this one, which is a different run with its own bill; and
// **orientation** merely points, and costs nothing. An earlier version walked through every read
// edge and put all of it in one column — which billed `idsd-ship` for four downstream stages it
// sequences, and called the total a ceiling. Refusing to label the edge and then pricing it as the
// dearest of the three is the same guess, made silently.
//
// So the second part is reported as conditional, grouped by the chain it came through, and left out
// of the first part's count. A reader who knows which of the three a particular citation is can add
// it up; nothing here pretends to know for them.
//
// Within the first part, reads are followed no further and dispatches are not followed at all. A
// dispatch is already priced at the row named on the line, and what it then dispatches is that
// worker's own bill — folding it in would charge one run twice.
func emitCost(self string, map_ workflow, skill string, out, errOut io.Writer) int {
	start, known := map_.nodes[skill]
	switch {
	case known && start.isWork:
		// Refused rather than answered, because a worker's profile is one line — its own row — and
		// what it dispatches is already on the bill of whichever skill dispatched it. Answering
		// would invite reading that one line as this worker's share of a run, which it is not.
		fmt.Fprintf(errOut, "%s: %s is a worker, and a worker's cost is the row --graph prints beside every skill that dispatches it\n", self, skill)
		return 2
	case !known:
		fmt.Fprintf(errOut, "%s: nothing named %q has a row in models.json; --graph lists what does\n", self, skill)
		return 2
	}

	charges := map[string]charge{}
	seen := map[string]bool{skill: true}
	// Breadth first, so the shortest chain of reads is the one a row is reported under. Depth first
	// would attribute a row to whichever branch happened to be walked first, which changes with the
	// citing file's name and not with the tree's shape.
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
				// recorded in one pass before any read edge out of it is followed, so a worker the
				// starting skill dispatches on its own face is charged with no chain before any
				// chain can reach it. Recording the last reach instead would move such a row into
				// the conditional half, where it reads as something the run might not pay for.
				if _, already := charges[to.to]; !already {
					charges[to.to] = charge{name: to.to, via: at.via, claude: target.claude, codex: target.codex}
				}
				continue
			}
			if seen[to.to] {
				continue
			}
			seen[to.to] = true
			queue = append(queue, step{name: to.to, via: joinVia(at.via, to.to)})
		}
	}

	var direct, inherited []charge
	for _, one := range charges {
		if one.via == "" {
			direct = append(direct, one)
		} else {
			inherited = append(inherited, one)
		}
	}
	byName := func(rows []charge) { sort.Slice(rows, func(a, b int) bool { return rows[a].name < rows[b].name }) }
	byName(direct)
	sort.Slice(inherited, func(a, b int) bool {
		if inherited[a].via != inherited[b].via {
			return inherited[a].via < inherited[b].via
		}
		return inherited[a].name < inherited[b].name
	})

	// The columns are sized to what is actually in them. A pinned width reads fine until one row is
	// `idsd/qualify/reconcile`, and then every tier steps right by the overflow and the two clients
	// stop being columns at all — on the one profile deep enough to need reading.
	label, claude := len("what runs"), len("claude")
	for _, one := range append(append([]charge{}, direct...), inherited...) {
		label = max(label, len("dispatch ")+len(one.name))
		claude = max(claude, len(or(one.claude, noTier)))
	}
	label = max(label, len("session  ")+len(skill))
	claude = max(claude, len(or(start.claude, noTier)))
	line := func(what, onClaude, onCodex string) {
		fmt.Fprintf(out, "%-*s  %-*s  %s\n", label, what, claude, or(onClaude, noTier), or(onCodex, noTier))
	}

	fmt.Fprintf(out, "%s  [%s]\n", skill, or(start.mode, "declares nothing"))
	if reason := shell.RunsHoldsReason(start.mode); reason != "" {
		fmt.Fprintf(out, "holds its own work — %s — so its row is the work it keeps\n", reason)
	}

	fmt.Fprintf(out, "\nwhat a run of it spends\n\n")
	fmt.Fprintf(out, "%-*s  %-*s  %s\n", label, "what runs", claude, "claude", "codex")
	line("session  "+skill, start.claude, start.codex)
	for _, one := range direct {
		line("dispatch "+one.name, one.claude, one.codex)
	}
	fmt.Fprintf(out, "\n%d dispatch(es) of its own, each once. A run takes some branches and not others, so this is what it can spend rather than a receipt.\n", len(direct))

	if len(inherited) == 0 {
		return 0
	}
	fmt.Fprintf(out, "\nwhat it may spend through the contracts it names\n\n")
	fmt.Fprintf(out, "%s\n\n", inheritedLegend)
	fmt.Fprintf(out, "%-*s  %-*s  %s\n", label, "what runs", claude, "claude", "codex")
	shown := ""
	for _, one := range inherited {
		if one.via != shown {
			fmt.Fprintf(out, "\nthrough %s:\n", one.via)
			shown = one.via
		}
		line("dispatch "+one.name, one.claude, one.codex)
	}
	fmt.Fprintf(out, "\n%d further row(s), on this bill only where the citation is extension.\n", len(inherited))
	return 0
}

// Why the second half is separate, in the reader's terms. Named rather than inlined because it is
// the whole reason the profile has two halves, and a sentence carrying that had better be edited in
// one place.
const inheritedLegend = "a skill naming another reads it as its own delta, names it as the next stage, or merely points at it;\n" +
	"nothing in the tree says which, so these land here only in the first case — the other two are a\n" +
	"different run's bill, or nothing at all"

func joinVia(via, name string) string {
	if via == "" {
		return name
	}
	return via + " → " + name
}
