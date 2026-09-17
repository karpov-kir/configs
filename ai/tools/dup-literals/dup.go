// Repeated-long-literal detector — byte-identical long text appearing 2+ times among the diff's ADDED
// lines: copy-pasted tokens, keys, fixtures. Run by the refactor worker's setup, and by a pipeline
// orchestrator before the refactor stage.
//
// Two granularities, and the difference decides what it can see: a whole trimmed line that repeats,
// and a repeated run of characters between delimiters, which is what finds a literal sitting in lines
// that otherwise differ. A space is one of those delimiters (isDelimiter below holds the list), so a
// long string with a space inside it is found only when its whole line repeats.
//
// The command-line contract (arguments, environment, exit codes) is the stub's:
// ai/kk-flavor/workers/refactor/dup-literals.sh.
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
	"kk-flavor/tools/flavorconfig"
	"kk-flavor/tools/shell"
)

const (
	defaultMinLength    = 100
	defaultMaxFileBytes = 262144
)

const maxShown = 200

// Bounded, and echoed rather than hashed: a reader has to recognise the literal, and it is someone
// else's text.
const prefixWidth = 60

const (
	exitClean     = 0
	exitFound     = 1
	exitDidNotRun = 2
)

// The stub this command runs behind, written out rather than read from argv[0]. `stub_usage_test.go`
// compares the usage line below against the one the stub's own header documents, and a name that
// changes with how the binary was reached leaves it nothing stable to compare.
const stubName = "dup-literals.sh"

// Every form the binary takes: revisions, and then paths after `--`, which narrow the scan to them. A
// bare path where a revision belongs is refused.
const usage = "usage: " + stubName + " [<git-diff revisions>] [-- <paths>]"

type Config struct {
	MinLength    int
	MaxFileBytes int64
}

const configName = "dup-literals.conf"

var configKeys = []string{"min-length", "max-file-bytes"}

// ConfigFromEnv resolves both thresholds: the tracked default under `~/.kk-flavor/configs/`, then
// this run's environment over it. HOME comes through lookup rather than the process, so a suite can
// point the mount at a fixture without touching anything process-global.
func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MinLength: defaultMinLength, MaxFileBytes: defaultMaxFileBytes}
	home, _ := lookup("HOME")
	path := flavorconfig.Path(home, configName)
	shipped, err := flavorconfig.Read(path, configKeys)
	if err != nil {
		return cfg, fmt.Errorf("%w — the scan did NOT run", err)
	}
	// The source rides along with the value, so a refusal names the environment variable or the file
	// line that set it.
	setting := func(variable, key string) (string, string) {
		if raw, ok := lookup(variable); ok && raw != "" {
			return raw, variable
		}
		return shipped[key], path + "'s `" + key + "`"
	}
	if raw, source := setting("DUP_MIN_LEN", "min-length"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("%s is '%s', which is no positive whole number — the scan did NOT run", source, shell.Oneline(raw))
		}
		cfg.MinLength = n
	}
	if raw, source := setting("DUP_MAX_FILE_BYTES", "max-file-bytes"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return cfg, fmt.Errorf("%s is '%s', which is no whole number — the scan did NOT run", source, shell.Oneline(raw))
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

type scan struct {
	cfg    Config
	tokens map[string]int
	lines  map[string]int
	result diffscan.Result
	// The secret-named files this scan declined, so one is announced and counted once rather than per
	// added line, and announce is where that notice goes.
	declined map[string]bool
	announce func(string)
}

func Run(self string, args []string, cwd string, cfg Config, stdout, stderr io.Writer) int {
	if err := diffscan.RefuseNonRevisions(args, cwd); err != nil {
		// The grammar goes with this refusal and with no other. Every other exit 2 below is a sound
		// invocation the scan could not carry out — a revision git would not resolve, a diff line past
		// the cap — and answering one with the grammar sends the caller to fix an argument that was
		// already right.
		fmt.Fprintf(stderr, "%s: %s\n%s: %s\n", self, err, self, usage)
		return exitDidNotRun
	}
	revisions, _ := diffscan.RevisionsNamed(args)

	diff, err := diffscan.Diff(cwd, args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", self, err)
		return exitDidNotRun
	}

	s := &scan{
		cfg: cfg, tokens: map[string]int{}, lines: map[string]int{}, declined: map[string]bool{},
		announce: func(line string) { fmt.Fprintf(stderr, "%s: %s\n", self, line) },
	}
	if err := s.result.WalkDiff(diff, s.count); err != nil {
		fmt.Fprintf(stderr, "%s: the diff could not be read to the end (%v) — exit 2, the scan did NOT run over all of it. Not a clean result.\n", self, err)
		return exitDidNotRun
	}

	if len(revisions) == 0 {
		opts := diffscan.Options{
			MaxFileBytes:    cfg.MaxFileBytes,
			SkipSecretNamed: true,
			Announce:        s.announce,
		}
		if err := s.result.WalkUntracked(cwd, opts, s.count); err != nil {
			fmt.Fprintf(stderr, "%s: could not list untracked files — the scan did NOT run\n", self)
			return exitDidNotRun
		}
	}

	return s.report(self, stdout, stderr)
}

func (s *scan) count(added diffscan.AddedLine) {
	// The untracked arm's guard, asked of the diff arm too. `SkipSecretNamed` was written for
	// untracked files, but what makes it necessary is that THIS tool echoes 60 bytes of every
	// duplicate, and that is a property of the scanner rather than of whether git tracks the file.
	// Measured on the unguarded form: two tracked `.env` files each gaining one `TOKEN=<130 chars>`
	// line printed `2x token (136 chars): TOKEN=SSSS…` — the two-files-one-token case the untracked
	// guard names, reached through the arm that had none. The untracked arm never delivers a
	// secret-named file, so this only ever fires on the diff.
	if diffscan.SecretNamed(added.File) {
		if !s.declined[added.File] {
			s.declined[added.File] = true
			s.result.SkippedUnread++
			s.announce(diffscan.SecretSkipNotice(added.File))
		}
		return
	}
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
