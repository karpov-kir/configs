package bloatjudge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	modelpolicy "kk-flavor/tools/model-policy"
	"kk-flavor/tools/repo/repotest"
)

func all(Unit) bool { return true }

// unasked is the repository handed to every case that does not name --changed. Those ask a repository
// nothing, and an empty one holds nothing to find, so a case that quietly started asking would offer
// no units rather than pass against a change set nobody arranged.
func unasked() *repotest.Fake { return repotest.New("/unasked") }

const source = "// file header\n// second line\n\nfunc a() {}\n// on a()\n*ptr = 1\n// trailing\n"

func TestRunOnASourceFileCutsOnlyComments(t *testing.T) {
	path := write(t, source)
	var out, errOut strings.Builder
	call := func(prompt, view string) (string, error) {
		if !strings.Contains(prompt, "opening this file for the first time") {
			t.Fatalf("the comment kind's reader is missing from the prompt:\n%s", prompt)
		}
		return "1, 2, 3", nil
	}
	if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &out, &errOut, call, nil); code != exitCut {
		t.Fatalf("exit %d, want %d — %s", code, exitCut, errOut.String())
	}
	if got, want := out.String(), "\nfunc a() {}\n*ptr = 1\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRunNumbersPrintsFileLines(t *testing.T) {
	path := write(t, source)
	var out, errOut strings.Builder
	call := func(string, string) (string, error) { return "2", nil }
	Run("bloat-judge.sh", []string{"--numbers", "comment", path}, unasked(), nil, &out, &errOut, call, nil)
	if out.String() != "5\n" {
		t.Fatalf("got %q, want the file line of unit 2", out.String())
	}
}

func TestRunIsIdempotentUnderAConsistentJudge(t *testing.T) {
	path := write(t, source)
	call := func(_, view string) (string, error) {
		for _, line := range strings.Split(view, "\n") {
			if strings.HasSuffix(line, "| // on a()") {
				return strings.TrimSpace(strings.SplitN(line, "|", 2)[0]), nil
			}
		}
		return "none", nil
	}
	var first, second, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &first, &errOut, call, nil); code != exitCut {
		t.Fatalf("first run exit %d — %s", code, errOut.String())
	}
	again := write(t, first.String())
	if code := Run("bloat-judge.sh", []string{"comment", again}, unasked(), nil, &second, &errOut, call, nil); code != exitClean {
		t.Fatalf("second run exit %d, want clean — %s", code, errOut.String())
	}
	if second.String() != first.String() {
		t.Fatalf("second run changed the text:\n%s\n---\n%s", first.String(), second.String())
	}
}

// Two ways a roll ends with no verdict, and neither may print the text: a model that explains instead
// of answering, and one that never comes back. Printing it would be the judged artifact passing
// through a gate that judged nothing.
func TestRunRefusesARollThatReachedNoVerdict(t *testing.T) {
	for name, call := range map[string]Caller{
		"a model that explains":      func(string, string) (string, error) { return "Unit 1 restates the file name.", nil },
		"a model that never answers": func(string, string) (string, error) { return "", errors.New("down") },
	} {
		t.Run(name, func(t *testing.T) {
			path := write(t, source)
			var out, errOut strings.Builder
			if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &out, &errOut, call, nil); code != exitDidNotRun {
				t.Fatalf("exit %d, want %d", code, exitDidNotRun)
			}
			if out.Len() != 0 {
				t.Fatalf("a refused run still printed: %q", out.String())
			}
		})
	}
}

func TestClaudeArgsGrantNoToolsServersOrRepoSettings(t *testing.T) {
	args := claudeArgs("You are", testSettings())
	at := func(flag string) int {
		for i, a := range args {
			if a == flag {
				return i
			}
		}
		t.Fatalf("%s is missing from %q", flag, args)
		return -1
	}
	if i := at("--tools"); args[i+1] != "" || !strings.HasPrefix(args[i+2], "--") {
		t.Fatalf("--tools must be followed by \"\" and then an option, got %q", args[i:])
	}
	at("--strict-mcp-config")
	// Empty, and followed by an option rather than by the prompt: this flag takes a comma-separated
	// list, so a value that slipped out would leave the next token read as one source name.
	if i := at("--setting-sources"); args[i+1] != "" || !strings.HasPrefix(args[i+2], "--") {
		t.Fatalf("--setting-sources must load nothing and be followed by an option, got %q", args[i:])
	}
	if args[len(args)-1] != "You are" {
		t.Fatalf("the prompt must come last, got %q", args)
	}
}

// The shipped claude row sets no effort, so this is the real argv and the case above is not: with an
// empty-valued list flag last, the prompt becomes that flag's value and the judge reads nothing.
func TestClaudeArgsWithNoEffortStillEndAtThePrompt(t *testing.T) {
	args := claudeArgs("You are", modelpolicy.Settings{Model: "fixture-model"})
	for i, arg := range args {
		if (arg == "--tools" || arg == "--setting-sources") && !strings.HasPrefix(args[i+2], "--") {
			t.Fatalf("%s takes an empty value and is followed by %q, not by an option", arg, args[i+2])
		}
	}
	if args[len(args)-1] != "You are" {
		t.Fatalf("the prompt must come last, got %q", args)
	}
}

func TestRunRefusesAnUnknownKind(t *testing.T) {
	var out, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"poem"}, unasked(), strings.NewReader("x"), &out, &errOut, nil, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if !strings.Contains(errOut.String(), "comment commit instruction pr-body record-entry reply report return review slack ticket") {
		t.Fatalf("the refusal does not list the kinds: %s", errOut.String())
	}
}

// A path carrying a newline must not forge a second line in the refusal.
func TestARefusalCarriesNoControlByteFromItsArgument(t *testing.T) {
	var out, errOut strings.Builder
	Run("bloat-judge.sh", []string{"comment", "no\x1b[31msuch\nfile"}, unasked(), nil, &out, &errOut, nil, nil)
	if strings.ContainsAny(errOut.String()[:len(errOut.String())-1], "\n\x1b") {
		t.Fatalf("the refusal carried a control byte through: %q", errOut.String())
	}
}

func TestRunPassesThroughWithNoUnits(t *testing.T) {
	path := write(t, "func a() {}\n")
	var out, errOut strings.Builder
	call := func(string, string) (string, error) {
		t.Fatal("the model was called with nothing to judge")
		return "", nil
	}
	if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &out, &errOut, call, nil); code != exitClean {
		t.Fatalf("exit %d, want clean", code)
	}
	if out.String() != "func a() {}\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestRunReadsProseFromStdin(t *testing.T) {
	var out, errOut strings.Builder
	call := func(prompt, view string) (string, error) {
		if !strings.Contains(prompt, "a reviewer deciding whether to approve") {
			t.Fatalf("wrong reader:\n%s", prompt)
		}
		return "2", nil
	}
	in := strings.NewReader("What changes.\n\nWhy the writer is right about it.\n")
	if code := Run("bloat-judge.sh", []string{"pr-body"}, unasked(), in, &out, &errOut, call, nil); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if out.String() != "What changes.\n\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestChangedOffersOnlyTheBlocksTheDiffTouched(t *testing.T) {
	dir := t.TempDir()
	committed := "// human one\nfunc a() {}\n// human two\nfunc b() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte(committed+"// agent three\nfunc c() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The diff git would print for that third block arriving, spelt out rather than derived from the
	// two sides: which lines a change added is git's answer, and a fixture that rebuilt it would be
	// narrowing the offer to its own diff rather than to the one the reviewer is looking at.
	git := repotest.New(dir).Diff("diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n@@ -4,0 +5,2 @@\n" +
		"+// agent three\n+func c() {}\n")

	var out, errOut strings.Builder
	call := func(_, view string) (string, error) {
		if strings.Count(view, "   1| ") != 1 || strings.Contains(view, "   2| ") {
			t.Fatalf("expected exactly one offered unit, got:\n%s", view)
		}
		if !strings.Contains(view, "   1| // agent three") {
			t.Fatalf("the offered unit is not the added block:\n%s", view)
		}
		return "1", nil
	}
	if code := RunIn("bloat-judge.sh", []string{"--changed", "comment", "f.go"}, dir, git, nil, &out, &errOut, call, nil); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if out.String() != committed+"func c() {}\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestChangedRefusesWithoutAPath(t *testing.T) {
	var out, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"--changed", "pr-body"}, unasked(), strings.NewReader("x\n"), &out, &errOut, nil, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func rollsAnswering(replies ...string) Caller {
	var mu sync.Mutex
	next := 0
	return func(string, string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		reply := replies[next]
		next++
		return reply, nil
	}
}

// A caller that counts, safe to call from a vote's goroutines at once. The count is read after
// Voting returns, so a bare int here would be a race the -race build reports, not a short count.
func counting(inner Caller) (Caller, func() int) {
	var mu sync.Mutex
	calls := 0
	return func(prompt, view string) (string, error) {
			mu.Lock()
			calls++
			mu.Unlock()
			return inner(prompt, view)
		}, func() int {
			mu.Lock()
			defer mu.Unlock()
			return calls
		}
}

// A real view, so the numbers the cases below name are numbers the vote actually offered — a
// hand-written string carries no margins and offers nothing. One argument is one unit, and the blank
// line between them is why: a prose unit is the markdown block, so two adjacent lines would be one.
func viewOf(lines ...string) string {
	separated := make([]string, 0, 2*len(lines))
	for i, line := range lines {
		if i > 0 {
			separated = append(separated, "")
		}
		separated = append(separated, line)
	}
	_, view := Split(separated, proseBlocks(separated), all)
	return view
}

func TestEveryLaneKindExists(t *testing.T) {
	for _, name := range []string{"comment", "instruction", "pr-body", "review", "commit", "report", "return", "reply", "ticket", "slack", "record-entry"} {
		if _, ok := kinds[name]; !ok {
			t.Errorf("no kind %q", name)
		}
	}
}

func spansOf(units []Unit) []int {
	spans := make([]int, len(units))
	for i, u := range units {
		spans[i] = u.Span
	}
	return spans
}

// The message that prompted the block rule: hard-wrapped at 72 columns, so every line but the subject
// is the middle of a sentence. Offered by the line, a majority naming some of a paragraph's lines and
// not the rest leaves half a sentence behind, which is not a verdict any reader could have meant.
const wrapped = "Name the commit a scanner number was read off\n" +
	"\n" +
	"A pass measured PR 3197 while HEAD was somewhere else, then wrote the\n" +
	"result up as a range it never measured. Both readings are real, and the\n" +
	"one in the write-up was not taken.\n" +
	"\n" +
	"Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>\n"

func TestCommitTrailersAreShownAsContextAndNeverOffered(t *testing.T) {
	path := write(t, wrapped)
	var out, errs strings.Builder
	// The roll names every unit it is offered; the trailer survives because it is not one of them.
	greedy := func(_, view string) (string, error) {
		var named []string
		for n := 1; n <= unitsInView(view); n++ {
			named = append(named, strconv.Itoa(n))
		}
		if !strings.Contains(view, "    | Co-Authored-By:") {
			return "", fmt.Errorf("the trailer was not shown as context:\n%s", view)
		}
		return strings.Join(named, ","), nil
	}
	if code := RunIn("j", []string{"commit", path}, ".", unasked(), nil, &out, &errs, greedy, nil); code != exitCut {
		t.Fatalf("exit %d, stderr %s", code, errs.String())
	}
	// The separators a deleted block sat between stay, which git's own `--cleanup` collapses and
	// markdown renders as one blank; what matters here is that the trailer itself is untouched.
	if got := out.String(); got != "Name the commit a scanner number was read off\n\n\nCo-Authored-By: Claude Opus 5 <noreply@anthropic.com>\n" {
		t.Fatalf("the judge kept %q; the subject and the trailer should survive and nothing else", got)
	}
}

// A unit can reach past the block it opened as: an unclosed fence runs one unit to the end of the
// text. Withholding on the unit's first line alone then hands the trailers it swallowed to the vote.
func TestAnUnclosedFenceCannotCarryTheTrailersIntoAUnit(t *testing.T) {
	lines := []string{"Subject", "", "Body with a repro:", "```", "$ run it", "", "Co-Authored-By: C <c@d>"}
	units, _ := Split(lines, proseBlocks(lines), offerFor(lines, kinds["commit"]))
	for _, unit := range units {
		if unit.Line <= 7 && unit.Line+unit.Span > 7 {
			t.Fatalf("unit %+v was offered though it holds the trailer on line 7", unit)
		}
	}
}

// The subject is the line `git log --oneline` shows and the one git requires, and it summarises the
// body — which is the shape the prompt calls restating what the reader can already see. This change's
// own commit message lost its subject to a real vote before the kind withheld it.
func TestACommitSubjectIsShownButNeverOffered(t *testing.T) {
	path := write(t, wrapped)
	var out, errs strings.Builder
	greedy := func(_, view string) (string, error) {
		var named []string
		for n := 1; n <= unitsInView(view); n++ {
			named = append(named, strconv.Itoa(n))
		}
		if !strings.Contains(view, "    | Name the commit a scanner number was read off") {
			return "", fmt.Errorf("the subject was not shown as context:\n%s", view)
		}
		return strings.Join(named, ","), nil
	}
	if code := RunIn("j", []string{"commit", path}, ".", unasked(), nil, &out, &errs, greedy, nil); code != exitCut {
		t.Fatalf("exit %d, stderr %s", code, errs.String())
	}
	if got := out.String(); !strings.HasPrefix(got, "Name the commit a scanner number was read off\n") {
		t.Fatalf("the judge kept %q; the subject must survive a vote that named everything", got)
	}
}
