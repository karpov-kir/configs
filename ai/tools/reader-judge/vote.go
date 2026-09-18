// The vote, and reading one roll's answer.
//
// A unit is deleted only where more than half the independent rolls name it, and every roll goes out
// at once. Each is parsed on its own, so one that explains instead of answering fails the whole vote
// rather than being outvoted into silence.
package readerjudge

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"kk-flavor/tools/shell"
)

// Caller runs the model. Injected so the suite drives every path without a process or a network.
type Caller func(prompt, view string) (string, error)

// ParseVerdict reads the model's answer as unit numbers. Anything that is not a number in range, or the
// word none, is refused whole: a model that starts explaining has stopped judging, and reading the
// numbers out of its prose would let the explanation back in.
//
// Saying nothing is refused too, and separately: `none` is a verdict, while an empty answer is a model
// that reached none. Read as one, a judge whose model never answered came back at exit 0 over unjudged
// text — silence dressed as a clean result, which is the one thing a mandatory gate may not produce.
func ParseVerdict(reply string, count int) ([]int, error) {
	trimmed := strings.TrimSpace(reply)
	if strings.EqualFold(trimmed, "none") {
		return nil, nil
	}
	if trimmed == "" {
		return nil, fmt.Errorf("the judge answered nothing at all, so no unit was judged")
	}
	seen := map[int]bool{}
	var gone []int
	for _, field := range strings.FieldsFunc(trimmed, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("the judge answered %q, which is not a list of unit numbers", echoable(trimmed))
		}
		if n < 1 || n > count {
			return nil, fmt.Errorf("the judge named unit %d of %d", n, count)
		}
		if !seen[n] {
			seen[n] = true
			gone = append(gone, n)
		}
	}
	sort.Ints(gone)
	return gone, nil
}

// Voting wraps a Caller so a unit is deleted only when MORE THAN HALF the independent rolls name it —
// at an even count a supermajority rather than a bare half: four rolls need three. The model is not
// consistent from one run to the next, and precision matters more than recall here. Each roll is
// parsed on its own, so one that explains instead of answering, or names a unit that was never
// offered, fails the whole vote rather than being outvoted into silence.
//
// Every roll goes out at once, so a vote costs the slowest single roll. Do not split them into waves
// to skip the rolls a majority has already made redundant: a roll's wall clock is dominated by the
// model's thinking, which varies several-fold over byte-identical input, so a second wave pays
// another draw from that tail and the saved calls are not the resource under pressure.
func Voting(call Caller, rolls int) Caller {
	return func(prompt, view string) (string, error) {
		count := unitsInView(view)
		if strings.Contains(prompt, verdictPromptMark) {
			return voteLabels(call, prompt, view, count, rolls)
		}
		named, err := rollAll(call, prompt, view, count, rolls)
		if err != nil {
			return "", err
		}
		tally := map[int]int{}
		for _, gone := range named {
			for _, n := range gone {
				tally[n]++
			}
		}
		var agreed []string
		for n := 1; n <= count; n++ {
			if tally[n]*2 > rolls {
				agreed = append(agreed, strconv.Itoa(n))
			}
		}
		if len(agreed) == 0 {
			return "none", nil
		}
		return strings.Join(agreed, ","), nil
	}
}

func rollAll(call Caller, prompt, view string, count, rolls int) ([][]int, error) {
	named := make([][]int, rolls)
	errs := make([]error, rolls)
	var wg sync.WaitGroup
	for i := 0; i < rolls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reply, err := call(prompt, view)
			if err != nil {
				errs[i] = err
				return
			}
			named[i], errs[i] = ParseVerdict(reply, count)
		}(i)
	}
	wg.Wait()
	// Read in roll order, not in the order they landed, so the same failures always report the
	// same one and a refusal is reproducible.
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return named, nil
}

// unitsInView counts what the view actually offers, which is the bound a roll's answer is read
// against. Split is the only writer of a view and numbers a unit's first line in the margin and
// nothing else, so the numbered margins are the units. Counted here rather than passed in, because a
// Caller is handed the prompt and the view and nothing besides.
//
// Never the view's line count, which stood here before and is a different number: prose units skip
// blank lines, a fenced block is one unit over many lines, and a source file's units are its comment
// blocks alone. Bounded by lines, a roll naming a unit nobody offered reached a majority before Run
// refused it, as the whole judge failing rather than as the one roll that lost the plot.
func unitsInView(view string) int {
	count := 0
	for _, line := range shell.SplitLines(view) {
		margin, _, found := strings.Cut(line, "|")
		if !found {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(margin)); err == nil {
			count++
		}
	}
	return count
}

// voteLabels is the vote for a kind that labels every block. It reads per block. Every block here
// carries a verdict, and the question is which one it carries.
//
// The verdict prompt writes the mark that selects it, which holds the two together. A Caller is
// handed the prompt and the view alone, and the wrapper is built before the kind is parsed.
func voteLabels(call Caller, prompt, view string, count, rolls int) (string, error) {
	cast := make([]map[int]string, rolls)
	errs := make([]error, rolls)
	var wg sync.WaitGroup
	for i := 0; i < rolls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reply, err := call(prompt, view)
			if err != nil {
				errs[i] = err
				return
			}
			cast[i], errs[i] = ParseLabels(reply, count)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return "", err
		}
	}
	var out []string
	for n := 1; n <= count; n++ {
		voted := make([]string, 0, rolls)
		for _, one := range cast {
			voted = append(voted, one[n])
		}
		out = append(out, strconv.Itoa(n)+" "+MajorityLabel(voted))
	}
	return strings.Join(out, "\n"), nil
}
