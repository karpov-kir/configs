// Package bloatjudge deletes prose by majority vote over independent rolls of the model.
// --changed offers only blocks touched by the diff, while showing the model the whole file.
// The model returns unit numbers; it cannot rewrite text or delete source code.
//
// JUDGE_PROVIDER is required. Missing, unknown or unavailable providers fail with exit 2.
// Model assignments come from kk-flavor/models.json. JUDGE_MODEL is retired and refused.
// A provider refusing that name fails as a refusal naming the policy file that chose it, not as a roll
// that did not answer. model-check asks the same question of every name in that file, as a gate unit.
// Calls use the selected CLI's existing login and the deadline configured in deadline.go.
// Codex ignores config, rules and workspace instructions, disables external tools, and uses
// a read-only sandbox. Built-in utility tools and apply_patch may remain exposed.
//
// Two obligations remain unimplemented:
//   - Offer only agent-written units: changed source blocks on an agent-authored branch,
//     and PR bodies or review comments only until a human's first edit.
//   - Carry the judged content's hash on the artifact, such as a Judged trailer or HTML comment,
//     so another machine can recognize the verdict. Today memoization is machine-local,
//     under $XDG_CACHE_HOME/kk-flavor/judged.
package bloatjudge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	// Trailers marks a kind whose closing block is machine-read metadata rather than prose. The
	// prompt tells the model to delete provenance, and a `Co-Authored-By:` line is exactly that, so
	// a kind that offered its trailers would be inviting the judge to eat the attribution the
	// commit is required to carry. Shown as context, never offered.
	Trailers bool
	// Subject marks a kind whose opening block is structure rather than prose. A commit's subject is
	// the line `git log --oneline` shows, the one git requires, and the only part most readers ever
	// see — and it is a summary of the body, which is what the prompt calls restating what you can
	// already see. Offered, it is the likeliest unit in the message to be cut, and cutting it leaves
	// a message git will not take. Measured 2026-09-16 on this change's own commit message.
	Subject bool
}

var kinds = map[string]Kind{
	"comment":      {Reader: "a later reader of this source file, editing it with no knowledge of the change that introduced it", Source: true},
	"instruction":  {Reader: "an agent loading this file at the start of every session, paying for each line in context"},
	"pr-body":      {Reader: "a reviewer deciding whether to approve this change, with the diff in front of you"},
	"review":       {Reader: "the author of this change deciding what to change, with the line in front of you"},
	"ticket":       {Reader: "an engineer picking this ticket up cold"},
	"slack":        {Reader: "a person reading this message in the thread it lands in"},
	"commit":       {Reader: "someone reading `git log` deciding whether to open this commit's diff", Trailers: true, Subject: true},
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
type Memo struct {
	Dir    string
	Policy string
}

// DefaultMemo lives outside every repo, under the cache home, so a repo never carries judged state.
func DefaultMemo(policy string) *Memo {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		base = filepath.Join(home, ".cache")
	}
	return &Memo{Dir: filepath.Join(base, "kk-flavor", "judged"), Policy: policy}
}

func (m *Memo) key(kind, content string) string {
	kindName, _, _ := strings.Cut(kind, "\n")
	specification := kinds[kindName]
	// Bump the algorithm version when unit extraction or majority semantics change, either of which
	// can move a verdict over identical bytes.
	identity := "judge-v4\n" + m.Policy + "\n" + Prompt(specification) + "\n" + strconv.FormatBool(specification.Source)
	sum := sha256.Sum256([]byte(identity + "\n" + kind + "\n" + content))
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

func Run(self string, args []string, stdin io.Reader, stdout, stderr io.Writer, call Caller, memo *Memo) int {
	return RunIn(self, args, ".", stdin, stdout, stderr, call, memo)
}

// RunIn is Run with the working directory named, which --changed needs to find the repository.
// The grammar, in one place, because two copies of it drift and `ai/tools/stub_usage_test.go` holds
// this one against the stub's header byte for byte.
const usageLine = "usage: bloat-judge.sh [--config <policy.json>] [--numbers] [--changed[=<revisions>]] <kind> [<path>]"

// What the option grammar says about these arguments, resolving nothing and reaching no provider.
// Returns the refusal to print, or "" when the arguments are the grammar.
//
// Separated from the run so it can be asked BEFORE a provider is configured. Asked after, a mistyped
// invocation on a machine carrying no CLI refused with "no provider" and never printed the grammar —
// a different fault, pointing the reader at something that was never wrong, and invisible on any
// machine that happens to have a provider installed.
func grammarRefusal(self string, args []string) (numbersOnly, changed bool, revisions, rest []string, refusal string) {
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
			return numbersOnly, changed, revisions, nil,
				fmt.Sprintf("%s: unknown option %s — the judge did NOT run", self, echoable(args[0]))
		}
		args = args[1:]
	}
	if len(args) == 0 || len(args) > 2 {
		return numbersOnly, changed, revisions, nil, fmt.Sprintf("%s: %s", self, usageLine)
	}
	if changed && len(args) != 2 {
		return numbersOnly, changed, revisions, nil,
			fmt.Sprintf("%s: --changed needs a path, since only a file has a diff — the judge did NOT run", self)
	}
	return numbersOnly, changed, revisions, args, ""
}

// RefuseIfNotTheGrammar prints the refusal and reports whether the caller should stop. The command
// calls it before resolving a provider, so an invocation error is always answered with the grammar.
func RefuseIfNotTheGrammar(self string, args []string, stderr io.Writer) bool {
	if _, _, _, _, refusal := grammarRefusal(self, args); refusal != "" {
		fmt.Fprintln(stderr, refusal)
		return true
	}
	return false
}

func RunIn(self string, args []string, cwd string, stdin io.Reader, stdout, stderr io.Writer, call Caller, memo *Memo) int {
	numbersOnly, changed, revisions, rest, refusal := grammarRefusal(self, args)
	if refusal != "" {
		fmt.Fprintln(stderr, refusal)
		return exitDidNotRun
	}
	args = rest
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

	lines := shell.SplitLines(content)
	offer := offerFor(lines, kind)
	if changed {
		added, err := addedLines(cwd, args[1], revisions)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
			return exitDidNotRun
		}
		offer = narrowToDiff(offer, added)
	}
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

// Split turns the lines into units and the view the model reads. A source file's candidates are its
// comment blocks and its code is shown unnumbered; prose's are its markdown blocks, so a fenced block
// is held whole and the model drops a pasted repro or not at all. offer says which candidates become
// units, the rest are shown as context, and a unit's continuation lines are marked `.` in the margin.
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
	if source {
		return commentBlocks(lines)
	}
	return proseBlocks(lines)
}

func commentBlocks(lines []string) []Unit {
	var found []Unit
	inBlock, inStar := false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
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
	}
	return found
}

// proseBlocks makes the markdown block the unit. A line in the middle of a hard-wrapped paragraph is
// not a unit any reader ever sees, and offered as one it lets a majority delete half a sentence.
// Prose written a paragraph to a line is unaffected; a commit message at 72 columns, and the four
// standards that wrap, are not.
func proseBlocks(lines []string) []Unit {
	var found []Unit
	inFence, inParagraph, inOrdered := false, false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case inFence:
			found[len(found)-1].Span++
			if shell.IsFenceDelimiter(line) {
				inFence = false
			}
		case shell.IsFenceDelimiter(line):
			found = append(found, Unit{Line: i + 1, Span: 1})
			inFence, inParagraph, inOrdered = true, false, false
		case line == "":
			inParagraph, inOrdered = false, false
		case inParagraph && !opensBlock(line, inOrdered):
			found[len(found)-1].Span++
		default:
			found = append(found, Unit{Line: i + 1, Span: 1})
			number, isItem := listMarker(line)
			inParagraph, inOrdered = wraps(line), isItem && number != ""
		}
	}
	return found
}

// opensBlock says a line begins a markdown block of its own instead of continuing the paragraph
// above it. inOrdered says an ordered list is already open, which is what keeps this from becoming
// the defect it removes: CommonMark lets an ordered list interrupt a paragraph only where it numbers
// from one, and a message wrapping onto `163. Three wordings were tried` is otherwise cut in half.
func opensBlock(line string, inOrdered bool) bool {
	switch {
	case strings.HasPrefix(line, "#"), strings.HasPrefix(line, ">"), strings.HasPrefix(line, "|"):
		return true
	}
	number, marked := listMarker(line)
	return marked && (number == "" || number == "1" || inOrdered)
}

// wraps says a plain line below this one belongs to the same unit. A paragraph, a list item and a
// quote all wrap — the quote by markdown's own lazy continuation. A heading and a table row do not:
// a heading that swallowed the paragraph under it would make deleting the paragraph delete the
// heading too, and a table row's neighbour is another row or the end of the table.
func wraps(line string) bool {
	return !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "|")
}

// listMarker reads a line's list marker: the empty string for a bullet, the digits for an ordered
// item, and false where there is no marker or no space after one. A marker needs that space —
// `**Bold**` opening a paragraph and `--- a comparison` are not list items, and reading them as ones
// would split a paragraph the writer did not split.
func listMarker(line string) (string, bool) {
	number, rest := "", ""
	switch {
	case strings.HasPrefix(line, "-"), strings.HasPrefix(line, "*"), strings.HasPrefix(line, "+"):
		rest = line[1:]
	default:
		digits := 0
		for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits+1 > len(line) || (line[digits] != '.' && line[digits] != ')') {
			return "", false
		}
		number, rest = line[:digits], line[digits+1:]
	}
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	return number, true
}

// offerFor is which candidates a kind puts to the vote — `Kind.Trailers` for what a commit withholds.
// One function, so a run and anything measuring a run offer the same units over the same text.
func offerFor(lines []string, kind Kind) func(Unit) bool {
	withheld := map[int]bool{}
	if kind.Trailers {
		maps.Copy(withheld, trailerLines(lines))
	}
	if kind.Subject {
		maps.Copy(withheld, subjectLines(lines))
	}
	if len(withheld) == 0 {
		return func(Unit) bool { return true }
	}
	// Every line of the unit, not its first. A unit can reach further than the block it started as —
	// an unclosed fence earlier in the message runs one unit to the end of the text — and a unit
	// whose first line is not withheld would carry the structure into a vote.
	return func(u Unit) bool {
		for at := u.Line; at < u.Line+u.Span; at++ {
			if withheld[at] {
				return false
			}
		}
		return true
	}
}

// subjectLines is the first block of a commit message. Empty where that block is the whole message,
// which is git's own shape for a subject-only commit: withholding it would leave nothing to judge and
// report a clean run over text no roll ever read.
func subjectLines(lines []string) map[int]bool {
	first := 0
	for first < len(lines) && strings.TrimSpace(lines[first]) == "" {
		first++
	}
	last := first
	for last < len(lines) && strings.TrimSpace(lines[last]) != "" {
		last++
	}
	if !hasContent(lines[min(last, len(lines)):]) {
		return nil
	}
	withheld := map[int]bool{}
	for at := first + 1; at <= last; at++ {
		withheld[at] = true
	}
	return withheld
}

func narrowToDiff(offer func(Unit) bool, added map[int]bool) func(Unit) bool {
	return func(u Unit) bool {
		if !offer(u) {
			return false
		}
		for line := u.Line; line < u.Line+u.Span; line++ {
			if added[line] {
				return true
			}
		}
		return false
	}
}

// trailerLines is the git trailer block of a commit message: the last block, once any of its lines
// reads `Token: value`, and only where a block stands above it — git's own rule, so `Fix: the thing`
// alone is a subject. One line is enough because git writes `(cherry picked from commit <sha>)` and
// bare issue refs in there too, and a block handed back is attribution the prompt tells a vote to cut.
func trailerLines(lines []string) map[int]bool {
	last := len(lines)
	for last > 0 && strings.TrimSpace(lines[last-1]) == "" {
		last--
	}
	first := last
	for first > 1 && strings.TrimSpace(lines[first-2]) != "" {
		first--
	}
	if !hasContent(lines[:max(first-1, 0)]) {
		return nil
	}
	trailered := false
	withheld := map[int]bool{}
	for at := first; at <= last; at++ {
		trailered = trailered || isTrailer(lines[at-1])
		withheld[at] = true
	}
	if !trailered {
		return nil
	}
	return withheld
}

func hasContent(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// isTrailer is git's own shape for one: a token of letters, digits and dashes, a colon, then a space.
func isTrailer(line string) bool {
	token, rest, found := strings.Cut(line, ":")
	if !found || token == "" || rest == "" || rest[0] != ' ' {
		return false
	}
	for _, r := range token {
		if !(r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
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

// Apply deletes the chosen units' lines and returns what is left, always ending in one newline. Text
// that ended in one and lost no unit comes back byte-identical; text that did not gains one.
// A block cut from the middle leaves both its blank lines, so the seam doubles. Left alone: git's
// `--cleanup` collapses them and markdown renders one and two alike, while closing the seam would
// delete the blank between two functions in a source file, where a blank is structure.
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
