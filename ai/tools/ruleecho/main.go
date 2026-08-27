// Finds a rule stated in more than one file, so a pass scoped to one of them can see the other.
//
//	usage: ruleecho <root> [file ...]        # scope defaults to every .md under root
//
// Four real contradictions in this tree survived every pass that could have caught them, for one
// reason: the file pairs were never in a single pass's scope together. A pass reads what it was
// scoped to, and a rule restated in a file outside that scope is invisible however carefully the
// pass reads what it holds. Grepping by inbound reference — the hunt the instruction lane already
// runs — only finds a pair where one file names the other. Two files that state the same rule and
// never mention each other are exactly the pair that drifts, because nothing connects them.
//
// The signal is bold. This tree marks a rule by bolding it, so a bolded span in two files is either
// one home and a cross-reference, or the restatement `ecosystem.md` → **One home** exists to stop.
// Matching is deliberately loose — rules get reworded as they drift, and an exact-match tool would
// go quiet precisely when the drift became worth reporting.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type span struct {
	file string
	line int
	text string
	key  map[string]bool
}

// Under this a match is a phrase, not a rule.
const minRuleChars = 12

type boldSpan struct {
	start int // byte offset of the opening `**`
	text  string
}

// Markdown pairs `**` in order down the line, so the delimiters alternate open, close, open, close.
// The regexp this replaced paired them by proximity instead, and any bold under the length floor —
// `**lossy**` — left its own closing delimiter free to pair with the next opening one. The plain
// prose between two real rules then came back as a third rule, matched against every file in the
// tree.
func boldSpans(line string) []boldSpan {
	var spans []boldSpan
	parts := strings.Split(line, "**")
	offset := 0
	for i, part := range parts {
		// The last part has no closing delimiter, so an odd count of `**` drops its tail rather than
		// reading an unterminated span as a rule.
		if i%2 == 1 && i < len(parts)-1 {
			spans = append(spans, boldSpan{start: offset - 2, text: part})
		}
		offset += len(part) + len("**")
	}
	return spans
}

// Words carrying no discrimination. A rule pair sharing only these shares nothing.
var common = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true, "to": true, "in": true,
	"is": true, "it": true, "that": true, "this": true, "for": true, "on": true, "as": true,
	"with": true, "by": true, "not": true, "never": true, "you": true, "your": true, "its": true,
	"be": true, "are": true, "was": true, "one": true, "at": true, "from": true, "what": true,
	"which": true, "when": true, "where": true, "who": true, "so": true, "but": true, "than": true,
}

var wordPattern = regexp.MustCompile(`[a-z0-9][a-z0-9-]*`)

func keyOf(text string) map[string]bool {
	key := map[string]bool{}
	for _, w := range wordPattern.FindAllString(strings.ToLower(text), -1) {
		if !common[w] && len(w) > 2 {
			key[w] = true
		}
	}
	return key
}

func overlap(a, b map[string]bool) int {
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return n
}

func collect(root string) ([]span, error) {
	var spans []span
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		inFence := false
		for i, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, b := range boldSpans(line) {
				text := strings.TrimSpace(b.text)
				if len(text) < minRuleChars {
					continue
				}
				// A bold span after an arrow is a citation naming another file's section — the
				// cross-reference `ecosystem.md` → **One home** requires, and the opposite of the
				// restatement this looks for. Without this the tool reports the tree's own compliance
				// as its finding, which is most of what it found on the first run.
				if before := strings.TrimSpace(line[:b.start]); strings.HasSuffix(before, "→") {
					continue
				}
				k := keyOf(text)
				// Under four discriminating words a match is a coincidence, not a restatement.
				if len(k) >= 4 {
					spans = append(spans, span{file: p, line: i + 1, text: text, key: k})
				}
			}
		}
		return nil
	})
	return spans, err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ruleecho <root> [file ...]")
		os.Exit(2)
	}
	root := os.Args[1]
	spans, err := collect(root)
	if err != nil || len(spans) == 0 {
		fmt.Fprintf(os.Stderr, "ruleecho: nothing read under %s — exit 2, which is not the same as nothing to report.\n", root)
		os.Exit(2)
	}

	// The scope decides which side of a pair is reported, never which pairs exist: the whole tree is
	// always read, or this tool would reproduce the blindness it is here to remove.
	inScope := map[string]bool{}
	for _, arg := range os.Args[2:] {
		if abs, err := filepath.Abs(arg); err == nil {
			inScope[abs] = true
		}
	}
	scoped := func(f string) bool {
		if len(inScope) == 0 {
			return true
		}
		abs, err := filepath.Abs(f)
		return err == nil && inScope[abs]
	}

	type pair struct {
		a, b   span
		shared int
	}
	var pairs []pair
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			if spans[i].file == spans[j].file {
				continue
			}
			if !scoped(spans[i].file) && !scoped(spans[j].file) {
				continue
			}
			shared := overlap(spans[i].key, spans[j].key)
			smaller := len(spans[i].key)
			if len(spans[j].key) < smaller {
				smaller = len(spans[j].key)
			}
			// Most of the shorter rule's discriminating words, so a long rule cannot drag in every
			// short one that happens to share vocabulary with part of it.
			if shared >= 4 && shared*100 >= smaller*70 {
				pairs = append(pairs, pair{spans[i], spans[j], shared})
			}
		}
	}
	sort.Slice(pairs, func(x, y int) bool { return pairs[x].shared > pairs[y].shared })

	for _, p := range pairs {
		fmt.Printf("rule stated twice (%d words shared):\n  %s:%d — %s\n  %s:%d — %s\n",
			p.shared, p.a.file, p.a.line, oneline(p.a.text), p.b.file, p.b.line, oneline(p.b.text))
	}
	fmt.Printf("%d bolded rule(s) read, %d pair(s) stating the same thing in two files\n", len(spans), len(pairs))
	if len(pairs) > 0 {
		os.Exit(1)
	}
}

func oneline(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if len(out) > 110 {
		return out[:110] + "…"
	}
	return out
}
