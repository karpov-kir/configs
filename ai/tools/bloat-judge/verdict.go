// The closed verdict vocabulary, and how rolls over it become one answer per block.
//
// One kind answers what four instruments used to answer separately — the delete vote, the
// comprehension flag, the refactor lane's per-note verdict and what the writer is handed. A closed
// set is what makes that measurable: every labelled case sorts into one of these, so the eval scores
// per label, and a label the judge cannot separate from another is merged rather than tuned.
package bloatjudge

import (
	"fmt"
	"sort"
	"strings"
)

// The verdicts, highest precedence first. Order is the tie-break and is not alphabetical: it runs
// from the verdicts whose action retires the block to the one that leaves it alone.
//
// `carried` outranks `obvious` because making the carrier — a test, a lint rule, a rename, an
// extraction — retires the block and takes the fact with it, where deleting first loses the fact.
// `obvious` outranks the rewrite verdicts because a block whose every sentence restates what is
// already visible is deleted rather than rewritten, and rewriting it would spend a model call to
// produce nothing. `padded` sits below it and above the rest for the mixed block: one fact worth
// keeping, carried along with sentences the code already shows. Without it that block lands on
// `obvious` and is deleted whole, and the fact goes with it.
//
// How much a block says is never a verdict. Length is what the deterministic checks measure, and a
// verdict the model has to count words to reach is a verdict it will reach differently each roll.
var verdictOrder = []string{"carried", "obvious", "stale", "padded", "coined", "unclear", "keep"}

var verdictRank = func() map[string]int {
	rank := map[string]int{}
	for at, name := range verdictOrder {
		rank[name] = at
	}
	return rank
}()

// verdictPromptMark is written by the verdict prompt and read by the vote, so the vote can tell which
// reply shape it is counting without being handed the kind. One constant, so the two cannot drift.
const verdictPromptMark = "Answer with one line per numbered block"

// VerdictNames is the vocabulary as the prompt and a refusal both state it.
func VerdictNames() string { return strings.Join(verdictOrder, ", ") }

// ParseLabels reads one `<unit> <verdict>` line per block. Every offered unit must be answered,
// because the caller counts verdict lines against blocks: a reply that silently omits a block would
// read as a clean block rather than as an answer the judge never gave.
func ParseLabels(reply string, count int) (map[int]string, error) {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return nil, fmt.Errorf("the judge answered nothing at all, so no block was judged")
	}
	labels := map[int]string{}
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		number, name, found := strings.Cut(line, " ")
		if !found {
			return nil, fmt.Errorf("the judge answered %q, which is not `<block> <verdict>`", echoable(line))
		}
		var n int
		if _, err := fmt.Sscanf(number, "%d", &n); err != nil {
			return nil, fmt.Errorf("the judge answered %q, whose first field is not a block number", echoable(line))
		}
		if n < 1 || n > count {
			return nil, fmt.Errorf("the judge named block %d of %d", n, count)
		}
		name = strings.TrimSpace(name)
		if _, known := verdictRank[name]; !known {
			return nil, fmt.Errorf("the judge answered verdict %q, which is not one of: %s", echoable(name), VerdictNames())
		}
		if _, twice := labels[n]; twice {
			return nil, fmt.Errorf("the judge answered block %d twice", n)
		}
		labels[n] = name
	}
	for n := 1; n <= count; n++ {
		if _, answered := labels[n]; !answered {
			return nil, fmt.Errorf("the judge left block %d of %d unanswered", n, count)
		}
	}
	return labels, nil
}

// MajorityLabel is one block's verdict across the rolls.
//
// A single label carried by more than half the rolls wins outright. Where none is, but more than half
// of the rolls called the block something other than `keep`, the rolls agree the block is bad and
// disagree about why — which is what a block carrying two defects looks like — and the highest
// precedence among the labels actually cast is taken. Short of that the verdict is `keep`: acting on
// a block a majority did not call bad is worse than leaving it, and precision is what this pipeline
// is short of.
func MajorityLabel(cast []string) string {
	if len(cast) == 0 {
		return "keep"
	}
	count := map[string]int{}
	for _, name := range cast {
		count[name]++
	}
	for name, n := range count {
		if 2*n > len(cast) {
			return name
		}
	}
	bad := 0
	for name, n := range count {
		if name != "keep" {
			bad += n
		}
	}
	if 2*bad <= len(cast) {
		return "keep"
	}
	best := "keep"
	for name := range count {
		if name != "keep" && verdictRank[name] < verdictRank[best] {
			best = name
		}
	}
	return best
}

// SortedUnits is the block numbers of a label set, in order, so output is stable across rolls.
func SortedUnits(labels map[int]string) []int {
	var out []int
	for n := range labels {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}
