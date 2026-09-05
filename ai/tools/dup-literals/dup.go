// Repeated-long-literal detector — byte-identical long text appearing 2+ times among the diff's ADDED
// lines: copy-pasted tokens, keys, fixtures. Run by kk-refactor's setup, and by a pipeline
// orchestrator before the refactor stage.
//
// Two granularities, and the difference decides what it can see: a whole trimmed line that repeats,
// and a repeated run of characters between delimiters, which is what finds a literal sitting in lines
// that otherwise differ. A space is one of those delimiters (isDelimiter below holds the list), so a
// long string with a space inside it is found only when its whole line repeats.
//
//	usage: dup-literals.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
//	       a path argument is refused with exit 2, never scanned
//	env:   DUP_MIN_LEN — minimum literal length in chars (default 100)
//	       DUP_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
//
// Prints each duplicate (count, length, 60-char prefix). Exits 1 when any found, 0 when clean, 2 when
// the scan did not run.
//
// Because it echoes 60 bytes of every duplicate, the untracked scan skips secret-bearing names rather
// than print what is in them — `diffscan.Options.SkipSecretNamed`, and the reasoning lives there.
//
// Every run ends with its denominator on stderr — files reached, duplicates, files skipped unread,
// binary lines ignored. An empty report at exit 0 means "nothing repeated" only when the first number
// is above zero, and "nothing was read" when it is not.
package duplicates

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"kk-flavor/tools/diffscan"
	"kk-flavor/tools/shell"
)

const (
	defaultMinLength    = 100
	defaultMaxFileBytes = 262144
)

const maxShown = 200

// The width of the prefix each finding shows. A reader has to recognise the literal to go and find it,
// so it is echoed rather than hashed — and bounded, because it is someone else's text.
const prefixWidth = 60

const (
	exitClean     = 0
	exitFound     = 1
	exitDidNotRun = 2
)

type Config struct {
	MinLength    int
	MaxFileBytes int64
}

// ConfigFromEnv reads the two thresholds. A value that does not parse is a scan that did not run,
// never one against the default: the caller asked for something, and answering with 100 reports a
// result they did not ask for.
func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MinLength: defaultMinLength, MaxFileBytes: defaultMaxFileBytes}
	if raw, ok := lookup("DUP_MIN_LEN"); ok && raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("DUP_MIN_LEN is '%s', which is no positive whole number — the scan did NOT run", shell.Oneline(raw))
		}
		cfg.MinLength = n
	}
	if raw, ok := lookup("DUP_MAX_FILE_BYTES"); ok && raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return cfg, fmt.Errorf("DUP_MAX_FILE_BYTES is '%s', which is no whole number — the scan did NOT run", shell.Oneline(raw))
		}
		cfg.MaxFileBytes = n
	}
	return cfg, nil
}

type findingKind int

const (
	tokenFinding findingKind = iota
	lineFinding
)

type finding struct {
	kind   findingKind
	text   string
	count  int
	length int
}

// scan is one run's accumulating state. Held together because both walks feed the same two tallies
// and the report reads all of them beside the denominator.
type scan struct {
	cfg    Config
	tokens map[string]int
	lines  map[string]int
	result diffscan.Result
}

func Run(self string, args []string, cwd string, cfg Config, stdout, stderr io.Writer) int {
	if err := diffscan.RefuseNonRevisions(args, cwd); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", self, err)
		return exitDidNotRun
	}
	revisions, _ := diffscan.RevisionsNamed(args)

	diff, err := diffscan.Diff(cwd, args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", self, err)
		return exitDidNotRun
	}

	s := &scan{cfg: cfg, tokens: map[string]int{}, lines: map[string]int{}}
	if err := s.result.WalkDiff(diff, s.count); err != nil {
		fmt.Fprintf(stderr, "%s: the diff could not be read to the end (%v) — exit 2, the scan did NOT run over all of it. Not a clean result.\n", self, err)
		return exitDidNotRun
	}

	if len(revisions) == 0 {
		opts := diffscan.Options{
			MaxFileBytes:    cfg.MaxFileBytes,
			SkipSecretNamed: true,
			Announce: func(line string) {
				fmt.Fprintf(stderr, "%s: %s\n", self, line)
			},
		}
		if err := s.result.WalkUntracked(cwd, opts, s.count); err != nil {
			fmt.Fprintf(stderr, "%s: could not list untracked files — the scan did NOT run\n", self)
			return exitDidNotRun
		}
	}

	return s.report(self, stdout, stderr)
}

func (s *scan) count(added diffscan.AddedLine) {
	trimmed := strings.TrimSpace(added.Text)
	if len([]rune(trimmed)) >= s.cfg.MinLength {
		s.lines[trimmed]++
	}
	for _, token := range strings.FieldsFunc(added.Text, isDelimiter) {
		if len([]rune(token)) >= s.cfg.MinLength {
			s.tokens[token]++
		}
	}
}

// The delimiters a literal is split on. Whitespace, quotes, backtick, comma, semicolon and brackets:
// the punctuation a long token sits between in code.
func isDelimiter(r rune) bool {
	switch r {
	case ' ', '\t', '"', '\'', '`', ',', ';', '(', ')':
		return true
	}
	return false
}

func (s *scan) report(self string, stdout, stderr io.Writer) int {
	var found []finding
	for text, n := range s.tokens {
		if n >= 2 {
			found = append(found, finding{kind: tokenFinding, text: text, count: n, length: len([]rune(text))})
		}
	}
	for text, n := range s.lines {
		// A line whose whole text is also a token is one duplicate, not two.
		if n >= 2 && s.tokens[text] == 0 {
			found = append(found, finding{kind: lineFinding, text: text, count: n, length: len([]rune(text))})
		}
	}
	// Ordered, so two runs over one tree print one report. Unordered, a diff of two is unreadable and
	// the display cap takes a different 200 each time.
	sort.Slice(found, func(i, j int) bool {
		if found[i].count != found[j].count {
			return found[i].count > found[j].count
		}
		return found[i].text < found[j].text
	})

	for i, f := range found {
		if i >= maxShown {
			break
		}
		label := "token"
		if f.kind == lineFinding {
			label = "line "
		}
		fmt.Fprintf(stdout, "%dx %s (%d chars): %s…\n", f.count, label, f.length,
			shell.CutBytes(shell.Oneline(f.text), prefixWidth))
	}
	if len(found) > maxShown {
		fmt.Fprintf(stdout, "… and %d further duplicate(s), not shown\n", len(found)-maxShown)
	}

	// The denominator, on stderr so the report on stdout stays exactly the duplicates.
	fmt.Fprintf(stderr, "%s: %d file(s) reached the scan, %d duplicate(s), %d file(s) skipped unread, %d binary line(s) ignored.\n",
		self, s.result.Reached, len(found), s.result.SkippedUnread, s.result.BinaryLines)
	if s.result.Reached == 0 {
		fmt.Fprintf(stderr, "%s: nothing reached the scan, so this run says nothing about the change set.\n", self)
	}
	if len(found) > 0 {
		return exitFound
	}
	return exitClean
}
