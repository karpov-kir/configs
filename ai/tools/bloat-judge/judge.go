// The judge: what a named reader would delete from an outward text, decided by a model that sees only
// what that reader sees.
//
//	usage: bloat-judge.sh [--numbers] [--changed[=<revisions>]] <kind> [<path>]
//	       every kind takes a file, or reads stdin when no path is given
//
// --changed offers only the blocks the diff added or touched — `git diff HEAD` plus untracked files, or
// the revisions given — while the whole file stays the view. The lanes run this form: without it a
// change touching one line of a human-written file would put every comment in that file up for
// deletion, and the sample says the judge takes about half of them.
//
// Prints the artifact with the judged units deleted, or with --numbers the 1-based line each deleted
// unit starts on, one per line — a block reports the line it starts on, never every line it took.
// Exit 0 when nothing went, 1 when something did, 2 when it did not run — an unknown kind, an
// unreadable path, a model that did not answer, or an answer that was not numbers.
//
// Only agent-written units should ever be offered, and that is still owed: for a source file the blocks
// the change added or edited, on a branch the agent authored; for a PR body or review comment, only until a human's first edit. The verdict
// is memoised per machine under $XDG_CACHE_HOME/kk-flavor/judged. It is owed as the judged content's hash
// on the artifact itself — a `Judged:` trailer, an HTML comment — so a second machine meeting a matching
// hash treats the text as judged instead of cutting it again. haiku, three rolls and a majority is
// provisional: the negatives eval, human-written comments that survived review, picks the model.
//
// The model returns numbers and nothing else, and this applies them. It never rewrites and never
// explains, so there is nothing for a writer to negotiate with, and for a source file the units offered
// are its comment blocks alone, so code cannot be touched whatever the model says. A block goes or stays
// whole: shortening one is a rewrite, which is the writer's job under code-style.md → Comments.
package bloatjudge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"kk-flavor/tools/diffscan"
	"kk-flavor/tools/shell"
)

const (
	exitClean     = 0
	exitCut       = 1
	exitDidNotRun = 2
)

// Kind names the reader the text is judged for. Source kinds offer only comment blocks as units.
type Kind struct {
	Reader string
	Source bool
}

var kinds = map[string]Kind{
	"comment":      {Reader: "a later reader of this source file, editing it with no knowledge of the change that introduced it", Source: true},
	"instruction":  {Reader: "an agent loading this file at the start of every session, paying for each line in context"},
	"pr-body":      {Reader: "a reviewer deciding whether to approve this change, with the diff in front of you"},
	"review":       {Reader: "the author of this change deciding what to change, with the line in front of you"},
	"ticket":       {Reader: "an engineer picking this ticket up cold"},
	"slack":        {Reader: "a person reading this message in the thread it lands in"},
	"commit":       {Reader: "someone reading `git log` deciding whether to open this commit's diff"},
	"report":       {Reader: "the human deciding what to do next from this report, with no other context"},
	"return":       {Reader: "an orchestrator deciding what to do next from this stage's return"},
	"reply":        {Reader: "the person you are replying to, in chat"},
	"record-entry": {Reader: "an agent reading this record before acting, paying for each entry in context"},
}

// Unit is one thing the model may delete, by the 1-based line it starts on and how many lines it spans.
type Unit struct {
	Line int
	Span int
}

// Caller runs the model. Injected so the suite drives every path without a process or a network.
type Caller func(prompt, view string) (string, error)

// Memo records each verdict by the hash of what was judged, and records the judged output as clean.
//
// The model is not consistent: the same text drew two different verdicts on consecutive runs, and a pass
// over its own output deleted more. Idempotence therefore cannot come from the model, so it comes from
// here: an artifact is judged once, its judged form is final, and a resend — or a second agent picking
// up the same text — meets the record rather than a new roll. Nil disables it, which the eval uses.
type Memo struct{ Dir string }

// DefaultMemo lives outside every repo, under the cache home, so a repo never carries judged state.
func DefaultMemo() *Memo {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		base = filepath.Join(home, ".cache")
	}
	return &Memo{Dir: filepath.Join(base, "kk-flavor", "judged")}
}

func (m *Memo) key(kind, content string) string {
	sum := sha256.Sum256([]byte(kind + "\n" + content))
	return filepath.Join(m.Dir, hex.EncodeToString(sum[:]))
}

// lookup answers a recorded verdict, bounded by the units this run is offering. A record naming a
// unit outside them was written by different code over the same bytes, so it is a miss and the model
// is asked again — unbounded it would index past the units and take the process down.
func (m *Memo) lookup(kind, content string, count int) ([]int, bool) {
	if m == nil {
		return nil, false
	}
	raw, err := os.ReadFile(m.key(kind, content))
	if err != nil {
		return nil, false
	}
	gone, err := ParseVerdict(string(raw), count)
	if err != nil {
		return nil, false
	}
	return gone, true
}

// record writes a verdict. A failure to write is not a failure to judge: the verdict stands, and the
// next run merely pays the model again.
func (m *Memo) record(kind, content string, gone []int) {
	if m == nil {
		return
	}
	// 0700/0600: a file's name here is the sha256 of the text that was judged, so a readable memo
	// dir confirms a guess at the exact bytes of a report or PR body this machine judged.
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return
	}
	fields := make([]string, len(gone))
	for i, n := range gone {
		fields[i] = strconv.Itoa(n)
	}
	body := "none"
	if len(fields) > 0 {
		body = strings.Join(fields, ",")
	}
	_ = os.WriteFile(m.key(kind, content), []byte(body+"\n"), 0o600)
}

// ClaudeCaller is the real one: `claude -p` on the CLI's own login, so no key is needed locally.
func ClaudeCaller(prompt, view string) (string, error) {
	cmd := exec.Command("claude", claudeArgs(prompt)...)
	cmd.Stdin = strings.NewReader(view)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("the model did not answer (%v)", err)
	}
	return string(out), nil
}

// claudeArgs gives the model nothing but the reply: no tools, no MCP servers, and no settings from the
// repository it runs in. The view is whatever the judged text says, and `-p` skips the workspace trust
// dialog. Without these flags a checked-out branch's `.claude/settings.json` would apply, allow rules
// and hooks and all, and a comment telling the model to run a command would be obeyed before the
// numbers came back. `--tools` is variadic, so an option follows it, never the prompt.
func claudeArgs(prompt string) []string {
	return []string{
		"-p", "--model", "haiku", "--output-format", "text",
		"--tools", "", "--strict-mcp-config", "--setting-sources", "user",
		prompt,
	}
}

func Run(self string, args []string, stdin io.Reader, stdout, stderr io.Writer, call Caller, memo *Memo) int {
	return RunIn(self, args, ".", stdin, stdout, stderr, call, memo)
}

// RunIn is Run with the working directory named, which --changed needs to find the repository.
func RunIn(self string, args []string, cwd string, stdin io.Reader, stdout, stderr io.Writer, call Caller, memo *Memo) int {
	numbersOnly, changed := false, false
	var revisions []string
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch {
		case args[0] == "--numbers":
			numbersOnly = true
		case args[0] == "--changed":
			changed = true
		case strings.HasPrefix(args[0], "--changed="):
			changed = true
			revisions = strings.Fields(strings.TrimPrefix(args[0], "--changed="))
		default:
			fmt.Fprintf(stderr, "%s: unknown option %s — the judge did NOT run\n", self, echoable(args[0]))
			return exitDidNotRun
		}
		args = args[1:]
	}
	if len(args) == 0 || len(args) > 2 {
		fmt.Fprintf(stderr, "%s: usage: bloat-judge.sh [--numbers] [--changed[=<revisions>]] <kind> [<path>]\n", self)
		return exitDidNotRun
	}
	if changed && len(args) != 2 {
		fmt.Fprintf(stderr, "%s: --changed needs a path, since only a file has a diff — the judge did NOT run\n", self)
		return exitDidNotRun
	}
	kindName := args[0]
	kind, known := kinds[kindName]
	if !known {
		fmt.Fprintf(stderr, "%s: no kind %q — the judge did NOT run. Kinds: %s\n", self, args[0], kindNames())
		return exitDidNotRun
	}

	var content string
	if len(args) == 2 {
		readPath := args[1]
		if !filepath.IsAbs(readPath) {
			readPath = filepath.Join(cwd, readPath)
		}
		raw, err := os.ReadFile(readPath)
		if err != nil {
			fmt.Fprintf(stderr, "%s: cannot read %s — the judge did NOT run\n", self, echoable(args[1]))
			return exitDidNotRun
		}
		content = string(raw)
	} else {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%s: cannot read stdin — the judge did NOT run\n", self)
			return exitDidNotRun
		}
		content = string(raw)
	}

	offer := func(Unit) bool { return true }
	if changed {
		added, err := addedLines(cwd, args[1], revisions)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
			return exitDidNotRun
		}
		offer = func(u Unit) bool {
			for l := u.Line; l < u.Line+u.Span; l++ {
				if added[l] {
					return true
				}
			}
			return false
		}
	}

	lines := shell.SplitLines(content)
	units, view := Split(lines, kind.Source, offer)
	if len(units) == 0 {
		if !numbersOnly {
			io.WriteString(stdout, content)
		}
		return exitClean
	}

	// Judged output is final whatever scope produced it, so the unscoped record is looked up by content
	// alone before the scoped verdict is.
	unscopedKey := kindName + "\n"
	scopedKey := unscopedKey + offeredKey(units)
	gone, recorded := memo.lookup(unscopedKey, content, len(units))
	if !recorded {
		gone, recorded = memo.lookup(scopedKey, content, len(units))
	}
	if !recorded {
		reply, err := call(Prompt(kind), view)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
			return exitDidNotRun
		}
		gone, err = ParseVerdict(reply, len(units))
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
			return exitDidNotRun
		}
	}
	pruned := Apply(lines, units, gone)
	if !recorded {
		memo.record(scopedKey, content, gone)
		// The pruned form is recorded under the unscoped key: whatever diff produced it, a resend of
		// exactly this text has nothing left to judge.
		memo.record(unscopedKey, pruned, nil)
	}

	if numbersOnly {
		for _, index := range gone {
			fmt.Fprintln(stdout, units[index-1].Line)
		}
	} else {
		io.WriteString(stdout, pruned)
	}
	if len(gone) == 0 {
		return exitClean
	}
	return exitCut
}

// echoable is how an argument reaches a message. A refusal names the argument the caller typed, and
// an argument carrying a newline forges a second line of output that the orchestrator reading this
// stream cannot tell from one the tool wrote. Sanitised and bounded, the way ecoroot.UncountedNames
// treats every name it echoes.
func echoable(arg string) string {
	return shell.CutBytesMarked(shell.Oneline(arg), 80)
}

func kindNames() string {
	names := make([]string, 0, len(kinds))
	for name := range kinds {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

// Prompt is the whole of what the model is told besides the text. The reader line is the only part that
// differs by kind; the tests are one list because a reader of any kind deletes for the same reasons.
func Prompt(kind Kind) string {
	return "You are " + kind.Reader + ". You read this once, quickly, and will not come back to it. " +
		"Below is a text with some units numbered in the left margin; a `.` marks a line that continues the " +
		"unit above it, and unnumbered lines are context you can see but may not delete. Reply with only " +
		"the numbers of the units you would delete, separated by commas, or the single word none. No other " +
		"words.\n\n" +
		"Delete a unit when it restates what you can already see; when it justifies a choice you would not " +
		"have questioned; when it argues that the writer is right rather than stating what is so; when it is " +
		"provenance, an anecdote, or an alternative that was rejected; when it grades or hedges another " +
		"unit; or when it is a fact that only makes sense beside a unit you are deleting. Keep a unit only " +
		"where deleting it would make you edit or decide wrongly, and where it stands on its own."
}

// Split turns the lines into units and the view the model reads. For a source file the candidates are
// its comment blocks — consecutive comment lines, ended by code or a blank — and the code is shown
// unnumbered; for prose every non-blank line is one, with a fenced block held whole so the model can
// drop a pasted repro or not at all. offer says which candidates become units; the rest are shown as
// context. A continuation line of a unit is marked `.` in the margin.
func Split(lines []string, source bool, offer func(Unit) bool) ([]Unit, string) {
	candidates := blocks(lines, source)
	var units []Unit
	numberAt := map[int]int{}
	for _, c := range candidates {
		if offer(c) {
			units = append(units, c)
			numberAt[c.Line] = len(units)
		}
	}
	var view strings.Builder
	continuing := 0
	for i, raw := range lines {
		at := i + 1
		switch {
		case numberAt[at] > 0:
			fmt.Fprintf(&view, "%4d| %s\n", numberAt[at], raw)
			continuing = units[numberAt[at]-1].Span - 1
		case continuing > 0:
			fmt.Fprintf(&view, "   .| %s\n", raw)
			continuing--
		default:
			fmt.Fprintf(&view, "    | %s\n", raw)
		}
	}
	return units, view.String()
}

func blocks(lines []string, source bool) []Unit {
	var found []Unit
	inFence, inBlock, inStar := false, false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case source:
			// Inside a `/*` block every line belongs to it until one carries `*/`, whatever it starts
			// with: a continuation without a leading `*` is still the same comment, and ending the block
			// there would delete its first line alone and leave the tail to break the file.
			switch {
			case inStar:
				found[len(found)-1].Span++
				if strings.Contains(line, "*/") {
					inStar, inBlock = false, false
				}
			case !isComment(line):
				inBlock = false
			case inBlock:
				found[len(found)-1].Span++
				inStar = opensStar(line)
			default:
				found = append(found, Unit{Line: i + 1, Span: 1})
				inBlock = true
				inStar = opensStar(line)
			}
		case inFence:
			found[len(found)-1].Span++
			if shell.IsFenceDelimiter(line) {
				inFence = false
			}
		case shell.IsFenceDelimiter(line):
			found = append(found, Unit{Line: i + 1, Span: 1})
			inFence = true
		case line != "":
			found = append(found, Unit{Line: i + 1, Span: 1})
		}
	}
	return found
}

func offeredKey(units []Unit) string {
	parts := make([]string, len(units))
	for i, u := range units {
		parts[i] = strconv.Itoa(u.Line) + "+" + strconv.Itoa(u.Span)
	}
	return strings.Join(parts, ",")
}

// addedLines is the set of 1-based lines the diff added to one file, that file named as the caller
// typed it and resolved against the repository root. With no revisions it is `git diff HEAD` plus, for
// an untracked file, every line.
func addedLines(cwd, path string, revisions []string) (map[int]bool, error) {
	if err := diffscan.RefuseNonRevisions(revisions, cwd); err != nil {
		return nil, err
	}
	rel, err := repoRelative(cwd, path)
	if err != nil {
		return nil, err
	}
	diff, err := diffscan.Diff(cwd, revisions)
	if err != nil {
		return nil, err
	}
	added := map[int]bool{}
	note := func(a diffscan.AddedLine) {
		if a.File == rel && a.Line > 0 {
			added[a.Line] = true
		}
	}
	var result diffscan.Result
	if err := result.WalkDiff(diff, note); err != nil {
		return nil, fmt.Errorf("the diff could not be read to the end (%v)", err)
	}
	if len(revisions) == 0 {
		if err := result.WalkUntracked(cwd, diffscan.Options{MaxFileBytes: 1 << 20}, note); err != nil {
			return nil, fmt.Errorf("could not list untracked files")
		}
	}
	return added, nil
}

func repoRelative(cwd, path string) (string, error) {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git repository", cwd)
	}
	prefix := strings.TrimSpace(string(out))
	if filepath.IsAbs(path) {
		top, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return "", fmt.Errorf("%s is not inside a git repository", cwd)
		}
		rel, err := filepath.Rel(strings.TrimSpace(string(top)), path)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(rel), nil
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(prefix, path))), nil
}

func opensStar(line string) bool {
	return strings.HasPrefix(line, "/*") && !strings.Contains(line[2:], "*/")
}

// isComment mirrors comment-density's: `//`, `/*`, `#`, and a continuation `*` or closing `*/` followed
// by a space or the end of the line, so `*ptr = 1` stays code.
func isComment(line string) bool {
	switch {
	case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "/*"), strings.HasPrefix(line, "#"):
		return true
	}
	rest := ""
	switch {
	case strings.HasPrefix(line, "*/"):
		rest = line[2:]
	case strings.HasPrefix(line, "*"):
		rest = line[1:]
	default:
		return false
	}
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// ParseVerdict reads the model's answer as unit numbers. Anything that is not a number in range, or the
// word none, is refused whole: a model that starts explaining has stopped judging, and reading the
// numbers out of its prose would let the explanation back in.
func ParseVerdict(reply string, count int) ([]int, error) {
	trimmed := strings.TrimSpace(reply)
	if strings.EqualFold(trimmed, "none") || trimmed == "" {
		return nil, nil
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

// Apply deletes the chosen units' lines and returns what is left, always ending in one newline. Text
// that ended in one and lost no unit comes back byte-identical; text that did not gains one.
func Apply(lines []string, units []Unit, gone []int) string {
	drop := map[int]bool{}
	for _, index := range gone {
		unit := units[index-1]
		for offset := 0; offset < unit.Span; offset++ {
			drop[unit.Line+offset] = true
		}
	}
	var kept []string
	for i, line := range lines {
		if !drop[i+1] {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n") + "\n"
}

// Voting wraps a Caller so a unit is deleted only when a majority of independent rolls name it. The
// model is not consistent from one run to the next, and precision matters more than recall here: a
// block only one roll of three would delete stays. Each roll is parsed on its own, so one roll that
// explains instead of answering fails the whole vote rather than being outvoted into silence.
func Voting(call Caller, rolls int) Caller {
	return func(prompt, view string) (string, error) {
		count := strings.Count(view, "\n") + 1
		tally := map[int]int{}
		for i := 0; i < rolls; i++ {
			reply, err := call(prompt, view)
			if err != nil {
				return "", err
			}
			gone, err := ParseVerdict(reply, count)
			if err != nil {
				return "", err
			}
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
