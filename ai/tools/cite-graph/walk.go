package main

import (
	"kk-flavor/tools/shell"
)

func longest(adj map[string][]string, start string, budget *shell.WalkBudget) []string {
	var from func(node string, seen map[string]bool, path []string) []string
	from = func(node string, seen map[string]bool, path []string) []string {
		best := append([]string{}, path...)
		for _, next := range adj[node] {
			if seen[next] {
				continue
			}
			if !budget.Spend(1 + len(path)) {
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
