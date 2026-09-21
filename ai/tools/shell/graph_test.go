package shell

import (
	"sort"
	"strings"
	"testing"
)

func cycleTexts(loops [][]string) []string {
	texts := make([]string, len(loops))
	for i, loop := range loops {
		texts[i] = strings.Join(loop, ">")
	}
	sort.Strings(texts)
	return texts
}

func TestCyclesAreTheLoopsAndNothingElse(t *testing.T) {
	adj := map[string][]string{"a": {"b"}, "b": {"a", "c"}, "c": {"b", "d"}}
	loops := Cycles(adj, []string{"a", "b", "c", "d"}, NewWalkBudget(WalkSteps))
	got := strings.Join(cycleTexts(loops), " ")
	if want := "a>b>a b>c>b"; got != want {
		t.Errorf("cycles = %q, want %q — `d` ends a chain and closes nothing", got, want)
	}
}

// The budget's own property, held here because the report reads a spent budget as "the count below is
// a LOWER BOUND". A walk that spends nothing on a graph it cannot finish returns an answer it never
// computed, and the report then prints it at full confidence.
func TestADenseGraphDoesNotHangTheCycleWalk(t *testing.T) {
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
	budget := NewWalkBudget(5000)
	Cycles(adj, nodes, budget)
	if !budget.Exhausted() {
		t.Error("the cycle walk spent no budget on a graph it cannot finish")
	}
}

// Both halves of Spend's rule over the edge that closes a cycle, which exhaustion alone cannot show:
// that the close is charged at all, and that it is charged the bytes of the path rather than its
// nodes.
//
// Names of 10 bytes, precisely so node count and byte count cannot both fit the number: two nodes
// walked from each in turn considers four edges at one step each, and the two that close are charged
// the 20 bytes of the two-node path each closes. Forty-four in total; counting nodes owes eight.
func TestClosingACycleCostsTheBytesOfWhatItCloses(t *testing.T) {
	first, second := "aaaaaaaaaa", "bbbbbbbbbb"
	adj := map[string][]string{first: {second}, second: {first}}
	budget := NewWalkBudget(WalkSteps)
	Cycles(adj, []string{first, second}, budget)
	if spent := WalkSteps - budget.left; spent != 44 {
		t.Errorf("the walk spent %d step(s), want 44 — four edges at one each, plus two closing edges"+
			" charged the 20 bytes of the path each closes", spent)
	}
}

func TestARefusedChargeLeavesTheBudgetExhausted(t *testing.T) {
	budget := NewWalkBudget(3)
	if budget.Spend(10) {
		t.Fatal("a charge larger than the whole budget was allowed")
	}
	if !budget.Exhausted() {
		t.Error("the budget still reads as unspent, so a skipped edge would pass for a complete walk")
	}
}

// A charge below one step is still a step. Both walks reach this method now, so what stops a caller
// talking the bound upwards has to be held by the type — a negative left to subtract would ADD to the
// remainder, and the walk it bounds would then run on past the figure it was sized for.
func TestNoChargeCanAddToTheBudget(t *testing.T) {
	budget := NewWalkBudget(3)
	for range 3 {
		if !budget.Spend(-100) {
			t.Fatal("a budget with steps left in it refused one")
		}
	}
	if !budget.Exhausted() {
		t.Error("three charges of -100 left the budget unspent, so a charge can raise the bound")
	}
}

// The dedup key decides whether a cycle EXISTS, so it must not be able to read two different sets of
// files as one — the loop dropped can be the one that crosses layers.
//
// `a>b` is one node here, so {x, a>b} and {x, a, b} are different sets that a `>` join spells alike.
// The decoy is walked first, so under an ambiguous key the real pair is the one silently dropped.
func TestTwoDifferentCyclesAreNeverKeyedAsOne(t *testing.T) {
	adj := map[string][]string{
		"x":   {"a>b", "a"},
		"a>b": {"x"},
		"a":   {"b"},
		"b":   {"x"},
	}
	loops := Cycles(adj, []string{"x", "a>b", "a", "b"}, NewWalkBudget(WalkSteps))
	if len(loops) != 2 {
		t.Fatalf("cycles = %v, want both — one set of files was read as another", cycleTexts(loops))
	}
}
