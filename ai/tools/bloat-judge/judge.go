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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// candidates is where this kind's units come from, and the one place the source/prose split is
// decided. On Kind rather than inside Split, because Kind is what already carries the distinction —
// a flag passed down to Split would put the same branch at each of its callers instead.
func (k Kind) candidates(lines []string) []Unit {
	if k.Source {
		return commentBlocks(lines)
	}
	return proseBlocks(lines)
}

var kinds = map[string]Kind{
	"comment":      {Reader: "an engineer opening this file for the first time to change something near this line, who has not read the rest of the file and does not know the change that introduced it", Source: true},
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

func Run(self string, args []string, stdin io.Reader, stdout, stderr io.Writer, call Caller, memo *Memo) int {
	return RunIn(self, args, ".", stdin, stdout, stderr, call, memo)
}

// The grammar, in one place, because two copies of it drift and `ai/tools/stub_usage_test.go` holds
// this one against the stub's header byte for byte.
const usageLine = "usage: bloat-judge.sh [--config <policy.json>] [--numbers | --strip=<dir>] [--changed[=<revisions>]] <kind> [<path>]"

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

// RunIn is Run with the working directory named, which --changed needs to find the repository.
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
	units, view := Split(lines, kind.candidates(lines), offer)
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
//
// Prompt is part of the memo key, so every edit here invalidates every cached verdict with no version
// bump to make. Say so in the commit that changes it.
func Prompt(kind Kind) string {
	return "You are " + kind.Reader + ". You read this once, quickly, and will not come back to it. " +
		"Below is a text with some units numbered in the left margin; a `.` marks a line that continues the " +
		"unit above it, and unnumbered lines are context you can see but may not delete. Reply with only " +
		"the numbers of the units you would delete, separated by commas, or the single word none. No other " +
		"words.\n\n" +
		"Delete a unit when it restates what you can already see; when it justifies a choice you would not " +
		"have questioned; when it argues that the writer is right rather than stating what is so; when it is " +
		"provenance, an anecdote, or an alternative that was rejected; when it grades or hedges another " +
		"unit; when you could not restate it in your own words after one reading; or when it is a fact that " +
		"only makes sense beside a unit you are deleting. Keep a unit only where deleting it would make you " +
		"edit or decide wrongly, and where it stands on its own. Keep a one-sentence summary on a " +
		"declaration where it says something the identifier and signature do not; a summary that " +
		"restates the name is a unit you delete."
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
