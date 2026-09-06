package main

import (
	"sort"
	"strings"
)

// Both walks below enumerate simple paths, and the count of those is factorial in how densely the
// graph is connected — not in how large it is. The tree under measurement writes its own citations, so
// it picks that density: twelve files each citing the other eleven never finish, and this tool is run
// over a branch someone else wrote. A budget is the bound rather than a memo because the two questions
// asked here — the longest simple path, and every simple cycle — have no cheap exact answer; a memo
// over "longest from here" is also wrong in the presence of cycles, which this reports rather than
// assumes away.
//
// The figure a spent budget produces is a lower bound, so it is never printed as if it were the
// answer: exhausted() is what the caller reports on.
type walkBudget struct{ left int }

// Sized so no honest tree reaches it: this repository's own graph spends four figures, and a run that
// spends all of these returns in about a second.
const walkSteps = 4 << 20

func (b *walkBudget) spend() bool {
	if b.left <= 0 {
		return false
	}
	b.left--
	return true
}

func (b *walkBudget) exhausted() bool { return b.left <= 0 }

// The longest simple path leaving `start`, up to the budget. A coupling measure, never a walk any one
// consumer makes — the report says so where it prints it, because the number invites the other reading.
//
// The seen set and the path so far are the recursion's own state, seeded here rather than by the caller.
func longest(adj map[string][]string, start string, budget *walkBudget) []string {
	var from func(node string, seen map[string]bool, path []string) []string
	from = func(node string, seen map[string]bool, path []string) []string {
		best := append([]string{}, path...)
		for _, next := range adj[node] {
			if seen[next] {
				continue
			}
			if !budget.spend() {
				return best
			}
			seen[next] = true
			if got := from(next, seen, append(path, next)); len(got) > len(best) {
				best = got
			}
			delete(seen, next)
		}
		return best
	}
	return from(start, map[string]bool{start: true}, []string{start})
}

func cycles(adj map[string][]string, nodes []string, budget *walkBudget) [][]string {
	var found [][]string
	reported := map[string]bool{}
	var walk func(node string, path []string, onPath map[string]bool)
	walk = func(node string, path []string, onPath map[string]bool) {
		for _, next := range adj[node] {
			if onPath[next] {
				loop := cycleFrom(path, next)
				// The key drops the repeated endpoint before sorting. With it in, one cycle entered from two
				// sides (`a → b → a` and `b → a → b`) produces two keys and is reported twice, which
				// inflates the count by roughly the cycle's length.
				sorted := append([]string{}, loop[:len(loop)-1]...)
				sort.Strings(sorted)
				if key := strings.Join(sorted, ">"); !reported[key] {
					reported[key] = true
					found = append(found, loop)
				}
				continue
			}
			if !budget.spend() {
				return
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
