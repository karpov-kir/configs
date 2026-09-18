// The closed verdict vocabulary, and how rolls over it become one answer per block.
//
// One kind answers what four instruments used to answer separately — the delete vote, the
// comprehension flag, the refactor lane's per-note verdict and what the writer is handed. A closed
// set makes that measurable. Every labelled case sorts into one of these, so the eval scores per
// label, and a label the judge cannot separate from another is merged.
//
// How much a block says is never a verdict. Length is what the deterministic checks measure, and a
// verdict the model has to count words to reach is one it reaches differently each roll.
package readerjudge

import (
	"fmt"
	"sort"
	"strings"
)

// The verdicts, highest precedence first. The order is the tie-break: it runs from the verdict whose
// action retires the block to the verdict that leaves it alone. `carried` leads because making the
// carrier takes the fact with it, and deleting first loses that fact. `padded` sits under `obvious`
// for the mixed block, which without it is deleted whole and takes its one fact along.
var verdictOrder = []string{"carried", "obvious", "stale", "padded", "coined", "unclear", "keep"}

var verdictRank = func() map[string]int {
	rank := map[string]int{}
	for at, name := range verdictOrder {
		rank[name] = at
	}
	return rank
}()

// verdictPromptMark is written by the verdict prompt and read by the vote. The vote reads it to tell
// which reply shape it is counting, having been handed no kind. One constant holds the two together.
const verdictPromptMark = "Answer with one line per numbered block"

// VerdictNames is the vocabulary as the prompt and a refusal both state it.
func VerdictNames() string { return strings.Join(verdictOrder, ", ") }

// ParseLabels reads one `<unit> <verdict>` line per block. Every offered unit must be answered,
// because the caller counts verdict lines against blocks. A reply omitting a block would read as a
// clean block, and the answer the judge never gave would pass as one it did.
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

// MajorityLabel is one block's verdict across the rolls. A label carried by more than half wins
// outright. Short of that, a majority calling the block something other than `keep` agrees it is bad
// and disagrees about why, so the highest precedence among the labels cast is taken. Every other
// split answers `keep`, since acting on a block a majority left alone costs more than leaving it.
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
