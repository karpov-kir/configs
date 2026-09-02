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
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

type span struct {
	file string
	line int
	text string
	key  map[string]bool
	// The citation targets written between this span's start and the next one's, exactly as they were
	// written — this rule's own, never the whole line's. Held per span because the citing file is what
	// a relative target resolves against, and that is known here and lost by the time two spans are
	// paired.
	citedAs []string
	// The walked files those targets name, filled once the walk has seen every file. A target names a
	// file by its tail, so which file it names is not decidable until the file set is known.
	cites map[string]bool
}

// Under this a match is a phrase, not a rule.
const minRuleChars = 12

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

// The scan's own account of itself: the rules it read, and how many files it was pointed at and did
// not read. The second number is the one that makes the first mean anything — 376 bolded rules and
// 206 are the same line to a reader unless the run says it was shown less of the tree.
type scan struct {
	spans []span
	// Every file the walk read, rules or none. A citation resolves against this and not against the
	// files that happen to hold a rule: a target that names a file holding no rules still names it,
	// and answering "no such file" there would exempt a pair the tree can prove nothing about.
	files []string
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
		found.files = append(found.files, p)
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
			onLine := boldSpans(line)
			for idx, b := range onLine {
				// A rule owns the citations written from where it starts up to the next bold span on
				// the line, which is where the one real form puts them: `**rule** (`path` →
				// **Section**)`. The boundary is the next bold span and not the next rule, because a
				// citation's own `**Section**` is a bold span that is not a rule; stopping there keeps
				// the scope narrow, and narrowing can only lose an exemption, never grant one. Handed
				// the whole line instead, one legitimate pointer exempts every other rule beside it:
				// park a duplicate on a line that already carries a cross-reference and the pair goes
				// silent, which is this tool's one unacceptable failure.
				end := len(line)
				if idx+1 < len(onLine) {
					end = onLine[idx+1].start
				}
				cited := citedTargets(line[b.start:end])
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
					found.spans = append(found.spans, span{file: p, line: i + 1, text: text, key: k, citedAs: cited})
				}
			}
		}
		return nil
	})
	found.resolveCitations()
	return found, err
}

// Turn each span's written citations into the walked files they name. After the walk, never during
// it: a citation names a file by its tail, and until the last file is read the tree cannot say
// whether one file answers to that tail or three do.
func (s *scan) resolveCitations() {
	resolver := newCitationResolver(s.files)
	for i := range s.spans {
		rule := &s.spans[i]
		rule.cites = map[string]bool{}
		for _, target := range rule.citedAs {
			if named := resolver.fileNamed(target, rule.file); named != "" {
				rule.cites[named] = true
			}
		}
	}
}

// Two spans one verdict holds together, with the two counts a report line quotes.
type pair struct {
	a, b   span
	shared int
	beyond int
}

// The two sites under a report headline. One shape for all three verdicts, so a reader comparing a
// restatement against an accepted pair reads the same columns in the same order.
func (p pair) sites() string {
	return fmt.Sprintf("  %s:%d — %s\n  %s:%d — %s\n",
		shell.Oneline(p.a.file), p.a.line, quotedRule(p.a.text),
		shell.Oneline(p.b.file), p.b.line, quotedRule(p.b.text))
}

// What one run tells its reader. Written to an io.Writer rather than printed where it is built, so a
// case can read it back. The headline over a group, and the clause counting it in the summary, are
// the only place a verdict reaches a human. Printed straight to stdout, nothing short of running the
// process could check that either one is still there.
type report struct {
	read   int
	pairs  []pair
	naming []pair
	citing []pair
	unread int
}

func (r report) writeTo(w io.Writer) {
	for _, p := range r.pairs {
		fmt.Fprintf(w, "rule stated twice (%d words shared):\n%s", p.shared, p.sites())
	}
	for _, p := range r.naming {
		fmt.Fprintf(w, "same dependency named twice, not a rule (%d words shared, %d beyond the name):\n%s",
			p.shared, p.beyond, p.sites())
	}
	for _, p := range r.citing {
		fmt.Fprintf(w, "one cites the other, not a restatement (%d words shared):\n%s", p.shared, p.sites())
	}
	fmt.Fprintf(w, "%d bolded rule(s) read, %d pair(s) stating the same thing in two files", r.read, len(r.pairs))
	// An accepted group prints its count only when it has one. A run with none of them says so by the
	// clause not being there, and a zero beside every heading reads as a measurement nobody made.
	if len(r.naming) > 0 {
		fmt.Fprintf(w, ", %d naming the same dependency", len(r.naming))
	}
	if len(r.citing) > 0 {
		fmt.Fprintf(w, ", %d citing the other's file", len(r.citing))
	}
	// The denominator the rule count needs. 376 rules and 206 read the same without it, and the pair
	// count underneath is drawn from whichever of the two this run actually saw.
	if r.unread > 0 {
		fmt.Fprintf(w, " — %d path(s) NOT read, so this is a partial scan", r.unread)
	}
	fmt.Fprintln(w)
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

	// Restatements and the pairs that share only a name they both cite. The second kind is printed
	// rather than dropped: silencing it would make this tool's own narrowing invisible, and a real
	// rule whose prose is short beside a long path would vanish with it. Printed and set apart, an
	// accepted pair costs a reader one glance instead of an adjudication they have already made.
	var pairs, naming, citing []pair
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
			case citesOwner:
				citing = append(citing, pair{spans[i], spans[j], shared, beyond})
			}
		}
	}
	byShared := func(group []pair) func(x, y int) bool {
		return func(x, y int) bool { return group[x].shared > group[y].shared }
	}
	sort.Slice(pairs, byShared(pairs))
	sort.Slice(naming, byShared(naming))
	sort.Slice(citing, byShared(citing))

	report{read: len(spans), pairs: pairs, naming: naming, citing: citing, unread: found.unread}.writeTo(os.Stdout)
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
