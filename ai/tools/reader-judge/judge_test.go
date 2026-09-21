package readerjudge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
)

func all(Unit) bool { return true }

// What a case hands a run that never passes --changed. Only that option asks the repository anything,
// so a run that started to would panic here rather than read an answer the case never arranged.
var noRepository repo.Git

const source = "// file header\n// second line\n\nfunc a() {}\n// on a()\n*ptr = 1\n// trailing\n"

// A rule file, for the cases that need a kind reading units out of a named file. They read a source
// file until the comment kind went, and what they drive — the numbers mode, idempotence, an empty
// offer, an expiry — belongs to every kind.
const instructions = "One plain paragraph a reader follows without effort.\n\n" +
	"A second paragraph, since a rule file's units are its paragraphs.\n\n" +
	"A third, to leave the vote something it can pass over.\n"

func TestRunNumbersPrintsFileLines(t *testing.T) {
	path := write(t, instructions)
	var out, errOut strings.Builder
	call := func(string, string) (string, error) { return "2", nil }
	Run("reader-judge.sh", []string{"--numbers", "instruction", path}, noRepository, nil, &out, &errOut, call, nil)
	if out.String() != "3\n" {
		t.Fatalf("got %q, want the file line of unit 2", out.String())
	}
}

func TestRunIsIdempotentUnderAConsistentJudge(t *testing.T) {
	path := write(t, instructions)
	call := func(_, view string) (string, error) {
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "| A second paragraph") {
				return strings.TrimSpace(strings.SplitN(line, "|", 2)[0]), nil
			}
		}
		return "none", nil
	}
	var first, second, errOut strings.Builder
	if code := Run("reader-judge.sh", []string{"instruction", path}, noRepository, nil, &first, &errOut, call, nil); code != exitCut {
		t.Fatalf("first run exit %d — %s", code, errOut.String())
	}
	again := write(t, first.String())
	if code := Run("reader-judge.sh", []string{"instruction", again}, noRepository, nil, &second, &errOut, call, nil); code != exitClean {
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
			if code := Run("reader-judge.sh", []string{"instruction", path}, noRepository, nil, &out, &errOut, call, nil); code != exitDidNotRun {
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
	if code := Run("reader-judge.sh", []string{"poem"}, noRepository, strings.NewReader("x"), &out, &errOut, nil, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	// Read off `kinds`, so a kind added tomorrow cannot break this case and a kind dropped from the
	// message still does. A spelled-out list gets edited to match whatever the tool prints. That is a
	// case agreeing with the code, and its job is to hold the code to something.
	for name := range kinds {
		if !strings.Contains(errOut.String(), name) {
			t.Fatalf("the refusal leaves out the kind %q: %s", name, errOut.String())
		}
	}
	if !strings.Contains(errOut.String(), kindNames()) {
		t.Fatalf("the refusal does not list the kinds in one run: %s", errOut.String())
	}
}

// A path carrying a newline must not forge a second line in the refusal.
func TestARefusalCarriesNoControlByteFromItsArgument(t *testing.T) {
	var out, errOut strings.Builder
	Run("reader-judge.sh", []string{"instruction", "no\x1b[31msuch\nfile"}, noRepository, nil, &out, &errOut, nil, nil)
	if strings.ContainsAny(errOut.String()[:len(errOut.String())-1], "\n\x1b") {
		t.Fatalf("the refusal carried a control byte through: %q", errOut.String())
	}
}

func TestRunPassesThroughWithNoUnits(t *testing.T) {
	path := write(t, "\n")
	var out, errOut strings.Builder
	call := func(string, string) (string, error) {
		t.Fatal("the model was called with nothing to judge")
		return "", nil
	}
	if code := Run("reader-judge.sh", []string{"instruction", path}, noRepository, nil, &out, &errOut, call, nil); code != exitClean {
		t.Fatalf("exit %d, want clean", code)
	}
	if out.String() != "\n" {
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
	if code := Run("reader-judge.sh", []string{"pr-body"}, noRepository, in, &out, &errOut, call, nil); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if out.String() != "What changes.\n\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestChangedRefusesWithoutAPath(t *testing.T) {
	var out, errOut strings.Builder
	if code := Run("reader-judge.sh", []string{"--changed", "pr-body"}, noRepository, strings.NewReader("x\n"), &out, &errOut, nil, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
}

// Only the blocks the diff added are offered, while the whole file stays in front of the model as
// context. The repository answers from a table: what a diff puts on offer is the subject here, and a
// real one would cost a process per question without asserting anything more (testing.md → 6).
func TestChangedOffersOnlyTheBlocksTheDiffAdded(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.md")
	if err := os.WriteFile(path, []byte(instructions), 0o644); err != nil {
		t.Fatal(err)
	}
	git := repotest.New(root).Diff("diff --git a/notes.md b/notes.md\n" +
		"--- a/notes.md\n" +
		"+++ b/notes.md\n" +
		"@@ -1,3 +1,5 @@\n" +
		" One plain paragraph a reader follows without effort.\n" +
		"+\n" +
		"+A second paragraph, since a rule file's units are its paragraphs.\n" +
		" \n" +
		" A third, to leave the vote something it can pass over.\n")

	offered := ""
	naming := func(_, view string) (string, error) {
		offered = view
		return "1", nil
	}
	var out, errs strings.Builder
	code := RunIn("j", []string{"--changed", "instruction", path}, root, git, nil, &out, &errs, naming, nil)
	if code != exitCut {
		t.Fatalf("exit %d, stderr %s", code, errs.String())
	}
	if unitsInView(offered) != 1 || !strings.Contains(offered, "1| A second paragraph") {
		t.Fatalf("the diff added one paragraph and the vote was offered something else:\n%s", offered)
	}
	if !strings.Contains(offered, "  | One plain paragraph") ||
		!strings.Contains(offered, "  | A third, to leave the vote") {
		t.Fatalf("the untouched paragraphs were not shown as context:\n%s", offered)
	}
	got := out.String()
	if strings.Contains(got, "A second paragraph") {
		t.Fatalf("the unit the vote named survived: %q", got)
	}
	if !strings.Contains(got, "One plain paragraph") || !strings.Contains(got, "A third, to leave") {
		t.Fatalf("a paragraph the diff never touched was cut: %q", got)
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
	for _, name := range []string{"instruction", "pr-body", "review", "commit", "report", "return", "reply", "ticket", "slack", "record-entry"} {
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
	if code := RunIn("j", []string{"commit", path}, ".", noRepository, nil, &out, &errs, greedy, nil); code != exitCut {
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
	if code := RunIn("j", []string{"commit", path}, ".", noRepository, nil, &out, &errs, greedy, nil); code != exitCut {
		t.Fatalf("exit %d, stderr %s", code, errs.String())
	}
	if got := out.String(); !strings.HasPrefix(got, "Name the commit a scanner number was read off\n") {
		t.Fatalf("the judge kept %q; the subject must survive a vote that named everything", got)
	}
}
