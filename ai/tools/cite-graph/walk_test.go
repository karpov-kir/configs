package main

import (
	"testing"

	"kk-flavor/tools/shell"
)

func TestADenseGraphDoesNotHangTheDepthWalk(t *testing.T) {
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
	depth := shell.NewWalkBudget(5000)
	for _, node := range nodes {
		longest(adj, node, depth)
	}
	if !depth.Exhausted() {
		t.Error("the depth walk spent no budget on a graph it cannot finish")
	}
}

// Spend's rule over this walk's own non-constant step, which exhaustion on a dense graph cannot show:
// `from` opens by copying the whole path so far, so a descent costs that path's NODE count. Nodes and
// not bytes, unlike the cycle walk's closing edge — this one copies string headers and never the names
// themselves, and charging bytes here would spend a third of the whole budget on this repository's own
// graph against one percent.
//
// Ten-byte names, so the three candidate charges give three different answers over one chain of three
// whose descents cost 2 and 3. Budget six pays for both: flat and node-count finish the chain, bytes
// (11 for the first descent alone) cannot start it. Budget four separates the two that are left: node
// count buys the first descent and not the second, where a flat charge walks the whole chain.
func TestADescentIsChargedForThePathItCopies(t *testing.T) {
	one, two, three := "aaaaaaaaaa", "bbbbbbbbbb", "cccccccccc"
	adj := map[string][]string{one: {two}, two: {three}}

	paid := shell.NewWalkBudget(6)
	if got := longest(adj, one, paid); len(got) != 3 {
		t.Errorf("longest = %v over a budget of 6, want the whole chain — a descent was charged the"+
			" path's bytes rather than its nodes", got)
	}
	if paid.Exhausted() {
		t.Error("a chain of three spent a budget of six, so a descent cost more than the nodes it copies")
	}

	short := shell.NewWalkBudget(4)
	got := longest(adj, one, short)
	if len(got) != 2 {
		t.Errorf("longest = %v over a budget of 4, want the walk stopped at the hop it could not pay"+
			" for — a descent was charged a flat step", got)
	}
	if !short.Exhausted() {
		t.Error("a chain of three left a budget of four unspent, so a descent was charged a flat step")
	}
}

func TestASparseGraphLeavesTheBudgetUnspent(t *testing.T) {
	adj := map[string][]string{"a": {"b"}, "b": {"c"}}
	budget := shell.NewWalkBudget(shell.WalkSteps)
	got := longest(adj, "a", budget)
	if len(got) != 3 {
		t.Fatalf("longest = %v, want the whole chain", got)
	}
	if budget.Exhausted() {
		t.Error("a three-file chain exhausted a budget sized for the whole tree")
	}
}
