package main

import (
	"testing"
)

func TestADenseGraphDoesNotHangTheWalks(t *testing.T) {
	const n = 14
	adj := map[string][]string{}
	var nodes []string
	for i := range n {
		from := string(rune('a' + i))
		nodes = append(nodes, from)
		for j := range n {
			if i != j {
				adj[from] = append(adj[from], string(rune('a'+j)))
			}
		}
	}

	// A budget small enough that an unbounded walk is the only way to exceed it.
	depth := &walkBudget{left: 5000}
	for _, node := range nodes {
		longest(adj, node, depth)
	}
	if !depth.exhausted() {
		t.Error("the depth walk spent no budget on a graph it cannot finish")
	}

	cycle := &walkBudget{left: 5000}
	cycles(adj, nodes, cycle)
	if !cycle.exhausted() {
		t.Error("the cycle walk spent no budget on a graph it cannot finish")
	}
}

// The other side: an honest tree must not trip the bound, or every real report would be labelled a
// lower bound. Two hops on a chain of three spends two steps, not the budget.
func TestASparseGraphLeavesTheBudgetUnspent(t *testing.T) {
	adj := map[string][]string{"a": {"b"}, "b": {"c"}}
	budget := &walkBudget{left: walkSteps}
	got := longest(adj, "a", budget)
	if len(got) != 3 {
		t.Fatalf("longest = %v, want the whole chain", got)
	}
	if budget.exhausted() {
		t.Error("a three-file chain exhausted a budget sized for the whole tree")
	}
}
