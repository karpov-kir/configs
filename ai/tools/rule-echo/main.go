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
//
// A clean run is proof of no bolded duplication, not of no duplication. A rule written as a heading,
// or stated in plain prose, is structurally invisible here — read a clean exit as "nothing bolded
// twice", never as "one home holds every rule".
//
// Anything this prints to stderr means the read was partial. A narrowing that says nothing cannot be
// told apart from a clean read, so every file skipped or read under relaxed rules is named there, and
// the summary below counts only what was actually scanned.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

type span struct {
	file string
	line int
	text string
	key  map[string]bool
}

// Under this a match is a phrase, not a rule.
const minRuleChars = 12

// The discriminating words a span needs before a match means anything, and the same floor the
// dependency test below applies to what is left after the names are removed. One constant because the
// two are compared against each other: drifted apart, a pair could clear the match and fail the test
// for a reason no reader could see.
const minDiscriminatingWords = 4

// How much of a rule a report line carries.
const maxReportRunes = 110

// The fence delimiter, at the start of a line. Widening it to indented fences or to `~~~` would be
// flexibility nobody asked for: neither occurs in a single one of this tree's markdown files, and
// every widening adds another way for the scan to fall silent over content it should have read.
const fenceMarker = "```"

// The bound and its reason are shell.MaxFileBytes. Here it reports rather than skips, so a file
// nothing read is never mistaken for a file that held no rules.
const maxFileBytes = shell.MaxFileBytes

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

var backticked = regexp.MustCompile("`[^`]*`")

// The words a span carries only because it names something in backticks — a path, a command, a file.
// Two consumers declaring the same dependency at their own point of use share all of this vocabulary
// without stating the same rule, and cutting either leaves that file not naming what it runs.
func namedWords(text string) map[string]bool {
	named := map[string]bool{}
	for _, quoted := range backticked.FindAllString(text, -1) {
		for w := range keyOf(quoted) {
			named[w] = true
		}
	}
	return named
}

// The overlap that survives once the words both spans owe to a name they both cite are removed.
func overlapBeyond(a, b, named map[string]bool) int {
	n := 0
	for w := range a {
		if b[w] && !named[w] {
			n++
		}
	}
	return n
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

// Markdown pairs fences in order down the file, so an odd count leaves the last one open and the scan
// off from that line to the end. Counted before the lines are read rather than discovered at the end,
// because by then the spans below the fence have already been dropped.
func fencesClosed(lines []string) bool {
	markers := 0
	for _, line := range lines {
		if strings.HasPrefix(line, fenceMarker) {
			markers++
		}
	}
	return markers%2 == 0
}

// What one candidate pair turns out to be.
type verdict int

const (
	unrelated verdict = iota
	// The same rule stated in two files: what this tool exists to find, and what fails the run.
	restatement
	// Two files naming the same thing — a path, a command — and agreeing on nothing else. Reported
	// apart from a restatement because the answer is already known: see namedWords. The `→` filter in
	// collect is this same case caught earlier, when the citation is written after an arrow rather than
	// as prose.
	sharedName
)

func classify(a, b span) (v verdict, shared, beyond int) {
	shared = overlap(a.key, b.key)
	smaller := len(a.key)
	if len(b.key) < smaller {
		smaller = len(b.key)
	}
	// Most of the shorter rule's discriminating words, so a long rule cannot drag in every short one
	// that happens to share vocabulary with part of it.
	if shared < minDiscriminatingWords || shared*100 < smaller*70 {
		return unrelated, shared, 0
	}
	named := namedWords(a.text)
	for w := range namedWords(b.text) {
		named[w] = true
	}
	beyond = overlapBeyond(a.key, b.key, named)
	if beyond < minDiscriminatingWords {
		return sharedName, shared, beyond
	}
	return restatement, shared, beyond
}

// The scan's own account of itself: the rules it read, and how many files it was pointed at and did
// not read. The second number is the one that makes the first mean anything — 376 bolded rules and
// 206 are the same line to a reader unless the run says it was shown less of the tree.
type scan struct {
	spans []span
	// Files and directories this was handed and did not read. A directory counts once and takes its
	// whole subtree with it, so this is a floor on what was missed, never the total.
	unread int
}

func collect(root string) (scan, error) {
	var found scan
	// Named on stderr and counted, both. The count reaches the summary and the exit; the name is the
	// only thing that tells a reader *which* part of the tree they were not shown.
	refuse := func(format string, args ...any) {
		found.unread++
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		// Walk hands a directory it could not open to this function with err set and fi nil, and the
		// whole subtree under it is then never visited. Swallowed with the two conditions below, that
		// vanished without a word: two unreadable directories took this tree from 376 bolded rules to
		// 206 — 45% less — over an unchanged exit code and an empty stderr.
		if err != nil {
			refuse("cannot read %s: %s — nothing under it was read", shell.Oneline(p), shell.Oneline(err.Error()))
			return nil
		}
		if fi.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		// `Walk` uses `Lstat`, so a directory symlink is already not followed — but `os.ReadFile`
		// follows a file one, and the tree supplying the name is the tree under review. A committed
		// `notes.md -> ~/private/notes.md` would come back reported under its in-tree path, and the
		// matcher would be an oracle over content the author never showed the tool. Reported rather
		// than skipped, the way a citation target that is not a regular file already is: a scan that
		// narrowed says so, or the narrowing is indistinguishable from a clean read.
		if !fi.Mode().IsRegular() {
			refuse("not a regular file: %s — it was NOT read", shell.Oneline(p))
			return nil
		}
		if fi.Size() > maxFileBytes {
			refuse("file too large to scan: %s is %d bytes, over the %d-byte bound — it was NOT read",
				shell.Oneline(p), fi.Size(), maxFileBytes)
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			// A file the walk reached and the open refused. Every other way of reading less than the
			// whole tree already said so; this one returned an empty-handed nil and the rule that was
			// in the file simply did not exist as far as any caller could tell.
			refuse("cannot read %s: %s — it was NOT read", shell.Oneline(p), shell.Oneline(err.Error()))
			return nil
		}
		lines := strings.Split(string(body), "\n")
		// A fence toggles the scan off, so one nobody closed silences every rule below it — for the
		// rest of the file, and without a word. That is the quiet this tool exists to remove, arriving
		// through a typo. Unbalanced, the file is read with fencing off and the narrowing is
		// announced: a fenced sample reported as a rule costs a reader one glance, while a rule that
		// was never read costs the pass the finding.
		closed := fencesClosed(lines)
		if !closed {
			fmt.Fprintf(os.Stderr, "unclosed fence in %s — it was read with fencing off, so a fenced sample may be reported as a rule\n",
				shell.Oneline(p))
		}
		inFence := false
		for i, line := range lines {
			if closed && strings.HasPrefix(line, fenceMarker) {
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
				// Under the floor a match is a coincidence, not a restatement.
				if len(k) >= minDiscriminatingWords {
					found.spans = append(found.spans, span{file: p, line: i + 1, text: text, key: k})
				}
			}
		}
		return nil
	})
	return found, err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ruleecho <root> [file ...]")
		os.Exit(2)
	}
	root := os.Args[1]
	found, err := collect(root)
	spans := found.spans
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
		beyond int
	}
	// Restatements and the pairs that share only a name they both cite. The second kind is printed
	// rather than dropped: silencing it would make this tool's own narrowing invisible, and a real
	// rule whose prose is short beside a long path would vanish with it. Printed and set apart, an
	// accepted pair costs a reader one glance instead of an adjudication they have already made.
	var pairs, naming []pair
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			if spans[i].file == spans[j].file {
				continue
			}
			if !scoped(spans[i].file) && !scoped(spans[j].file) {
				continue
			}
			verdict, shared, beyond := classify(spans[i], spans[j])
			switch verdict {
			case restatement:
				pairs = append(pairs, pair{spans[i], spans[j], shared, beyond})
			case sharedName:
				naming = append(naming, pair{spans[i], spans[j], shared, beyond})
			}
		}
	}
	byShared := func(group []pair) func(x, y int) bool {
		return func(x, y int) bool { return group[x].shared > group[y].shared }
	}
	sort.Slice(pairs, byShared(pairs))
	sort.Slice(naming, byShared(naming))

	for _, p := range pairs {
		fmt.Printf("rule stated twice (%d words shared):\n  %s:%d — %s\n  %s:%d — %s\n",
			p.shared, shell.Oneline(p.a.file), p.a.line, quotedRule(p.a.text),
			shell.Oneline(p.b.file), p.b.line, quotedRule(p.b.text))
	}
	for _, p := range naming {
		fmt.Printf("same dependency named twice, not a rule (%d words shared, %d beyond the name):\n  %s:%d — %s\n  %s:%d — %s\n",
			p.shared, p.beyond, shell.Oneline(p.a.file), p.a.line, quotedRule(p.a.text),
			shell.Oneline(p.b.file), p.b.line, quotedRule(p.b.text))
	}
	fmt.Printf("%d bolded rule(s) read, %d pair(s) stating the same thing in two files", len(spans), len(pairs))
	if len(naming) > 0 {
		fmt.Printf(", %d naming the same dependency", len(naming))
	}
	// The denominator the rule count needs. 376 rules and 206 read the same without it, and the pair
	// count underneath is drawn from whichever of the two this run actually saw.
	if found.unread > 0 {
		fmt.Printf(" — %d path(s) NOT read, so this is a partial scan", found.unread)
	}
	fmt.Println()
	// A partial read outranks the pair count, and takes the exit with it. The pairs above are real and
	// stay printed; what cannot be claimed is the absence of the others, and exit 0 or 1 would claim
	// exactly that. This is the only cross-file restatement detector there is, so a scan that was
	// shown less than the tree must never be mistaken for one that found nothing in it.
	if found.unread > 0 {
		fmt.Fprintf(os.Stderr, "ruleecho: %d path(s) under %s could not be read — exit 2. The pairs above are real; the ones in what went unread are not ruled out.\n",
			found.unread, shell.Oneline(root))
		os.Exit(2)
	}
	// Only a restatement fails the run. A pair that shares nothing but a cited name is reported for
	// the reader, never held against the tree.
	if len(pairs) > 0 {
		os.Exit(1)
	}
}

// A rule as a report line quotes it: sanitised like every other text the tree chose, then truncated.
// Truncation counts runes, not bytes — rules here are prose and the tree carries `→`, `…` and em
// dashes, so a byte slice at a fixed offset lands mid-rune and the report ends in a replacement
// character, on the exact input the bound exists to handle.
//
// A path takes shell.Oneline alone: a filename may carry any byte but `/` and NUL, so a newline in
// one forges a whole line of this tool's output — but cutting a path to a rule's rune bound names a
// file the reader cannot open.
func quotedRule(s string) string {
	out := []rune(shell.Oneline(s))
	if len(out) > maxReportRunes {
		return string(out[:maxReportRunes]) + "…"
	}
	return string(out)
}
