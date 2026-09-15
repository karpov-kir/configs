package shell

import (
	"sort"
	"strings"
)

// The cycle walk over a citation graph, and the budget any walk of one runs under. It is here rather
// than in either tool because both ask it of the same tree, and two implementations is two answers to
// "is this a cycle." The nodes are whatever strings the caller keys its files on.

// WalkBudget bounds a walk that enumerates simple paths. The count of those is factorial in how
// densely the graph is connected — not in how large it is — and the tree under measurement writes its
// own citations, so it picks that density: twelve files each citing the other eleven never finish.
//
// A budget rather than a memo, because the questions asked of these graphs have no cheap exact
// answer, and a memo over "longest from here" is also wrong in the presence of cycles — which Cycles
// reports rather than assumes away.
//
// The figure a spent budget produces is a lower bound, so it is never printed as if it were the
// answer: Exhausted is what the caller reports on.
type WalkBudget struct{ left int }

// WalkSteps is sized so no honest tree reaches it: this repository's own graph spends under a tenth of
// it on the widest walk bounded by it — cite-graph's depth walk, its cycle walk, and eco-check's walk
// over the standards alone, each of which takes a budget of its own. Spent in full it still returns in
// a fraction of a second.
const WalkSteps = 4 << 20

func NewWalkBudget(steps int) *WalkBudget { return &WalkBudget{left: steps} }

// Spend charges `cost` steps and answers whether the budget could pay for them. `cost` is 1 for a step whose
// work is constant, and the length of what the step copies where it is not. That distinction is the
// whole of what keeps this bound a bound — a step charged flat while its cost grows with the path buys
// unbounded work under a budget counted in steps, and each walk under this type has such a step.
// Measured over 200 mutually citing files, the three ways of charging one of them wrong cost 5.2s,
// 7m48s and 21s against the fraction of a second the charges below cost; a case holds each.
//
// A refused charge takes the remainder with it, so Exhausted answers true. Left at its remainder, a
// walk that skipped a step it could not pay for would carry on and then be read as a complete
// answer — which is this tool reporting clean over a cycle it never formed.
//
// A step costs at least one however it is charged, so no caller can talk the bound upwards by passing
// a negative.
func (b *WalkBudget) Spend(cost int) bool {
	if cost < 1 {
		cost = 1
	}
	if b.left < cost {
		b.left = 0
		return false
	}
	b.left -= cost
	return true
}

func (b *WalkBudget) Exhausted() bool { return b.left <= 0 }

// Cycles is one simple cycle per distinct SET of nodes reachable from `nodes`, each written as the
// path that closes it with its first node repeated at the end. One per set and not one per loop: the
// key below is the node set, so `a → b → c → a` and `a → c → b → a` are two simple cycles reported
// once between them. Both readers judge a cycle by which files are in it and nothing else — the
// layers those files declare — so neither verdict moves; what a caller counts off this is sets of
// files, not loops.
//
// Order is the caller's: the walk starts from `nodes` in the order they arrive, so a caller that
// wants a stable report sorts them.
func Cycles(adj map[string][]string, nodes []string, budget *WalkBudget) [][]string {
	var found [][]string
	reported := map[string]bool{}
	var walk func(node string, path []string, onPath map[string]bool)
	walk = func(node string, path []string, onPath map[string]bool) {
		for _, next := range adj[node] {
			// Closing edges cost too, or a graph of mostly-closing edges spends nothing —
			// TestClosingACycleCostsTheBytesOfWhatItCloses holds it shut.
			if !budget.Spend(1) {
				return
			}
			if onPath[next] {
				// Charged the bytes of the path it closes, not its node count — the same test covers this half.
				if !budget.Spend(pathBytes(path)) {
					return
				}
				loop := cycleFrom(path, next)
				// The repeated endpoint is dropped before sorting, or one cycle entered from both sides keys
				// twice and is reported twice.
				sorted := append([]string{}, loop[:len(loop)-1]...)
				sort.Strings(sorted)
				// NUL joins them, the one byte a path cannot hold, because this key decides whether a cycle
				// EXISTS. Any printable separator is a byte some node name may carry, and two different node
				// sets then key alike: a standard committed as `c.md>d.md` makes the pair {a, that file} key
				// exactly as {a, c.md, d.md} does, so whichever is walked second is dropped in silence — and
				// the one dropped can be the cycle that crosses layers. The key is never printed.
				if key := strings.Join(sorted, "\x00"); !reported[key] {
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

func pathBytes(path []string) int {
	total := 0
	for _, node := range path {
		total += len(node)
	}
	return total
}

func cycleFrom(path []string, next string) []string {
	at := 0
	for i, n := range path {
		if n == next {
			at = i
			break
		}
	}
	return append(append([]string{}, path[at:]...), next)
}
