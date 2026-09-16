package bloatjudge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	modelpolicy "kk-flavor/tools/model-policy"
)

func all(Unit) bool { return true }

const source = "// file header\n// second line\n\nfunc a() {}\n// on a()\n*ptr = 1\n// trailing\n"

func TestSplitSourceOffersWholeBlocks(t *testing.T) {
	units, view := Split(strings.Split(strings.TrimSuffix(source, "\n"), "\n"), true, all)
	if len(units) != 3 {
		t.Fatalf("got %d units, want 3 (header block, on a(), trailing)", len(units))
	}
	if units[0].Line != 1 || units[0].Span != 2 {
		t.Fatalf("the header block is line %d span %d, want 1 span 2", units[0].Line, units[0].Span)
	}
	if !strings.Contains(view, "   1| // file header\n   .| // second line\n") {
		t.Fatalf("the view does not mark the continuation line:\n%s", view)
	}
	if !strings.Contains(view, "    | *ptr = 1\n") {
		t.Fatalf("a dereference was offered as a unit:\n%s", view)
	}
}

func TestSplitProseHoldsAFenceAsOneUnit(t *testing.T) {
	lines := []string{"para", "```xml", "<a/>", "<b/>", "```", "", "after"}
	units, view := Split(lines, false, all)
	if len(units) != 3 {
		t.Fatalf("got %d units, want 3 (para, fence, after)", len(units))
	}
	if units[1].Line != 2 || units[1].Span != 4 {
		t.Fatalf("the fence is line %d span %d, want 2 span 4", units[1].Line, units[1].Span)
	}
	if strings.Count(view, "   .| ") != 3 {
		t.Fatalf("the fence body is not marked as continuation:\n%s", view)
	}
}

func TestParseVerdictAcceptsNumbersAndNone(t *testing.T) {
	gone, err := ParseVerdict(" 3, 1,3\n", 3)
	if err != nil || len(gone) != 2 || gone[0] != 1 || gone[1] != 3 {
		t.Fatalf("got %v %v, want [1 3]", gone, err)
	}
	if gone, err := ParseVerdict("None\n", 3); err != nil || gone != nil {
		t.Fatalf("none parsed as %v %v", gone, err)
	}
}

func TestParseVerdictRefusesAnAnswerThatIsEmpty(t *testing.T) {
	if _, err := ParseVerdict("   \n", 3); err == nil {
		t.Fatal("an empty answer was accepted as none")
	}
}

func TestParseVerdictRefusesProseAndOutOfRange(t *testing.T) {
	if _, err := ParseVerdict("I would delete 2 because it restates the code", 3); err == nil {
		t.Fatal("prose with a number in it was accepted")
	}
	if _, err := ParseVerdict("4", 3); err == nil {
		t.Fatal("a unit past the end was accepted")
	}
	if _, err := ParseVerdict("0", 3); err == nil {
		t.Fatal("unit 0 was accepted")
	}
}

func TestApplyDeletesTheWholeSpanAndKeepsTheTrailingNewline(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	units, _ := Split(lines, true, all)
	got := Apply(lines, units, []int{1})
	want := "\nfunc a() {}\n// on a()\n*ptr = 1\n// trailing\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRunOnASourceFileCutsOnlyComments(t *testing.T) {
	path := write(t, source)
	var out, errOut strings.Builder
	call := func(prompt, view string) (string, error) {
		if !strings.Contains(prompt, "a later reader of this source file") {
			t.Fatalf("the comment kind's reader is missing from the prompt:\n%s", prompt)
		}
		return "1, 2, 3", nil
	}
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, nil); code != exitCut {
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
	Run("bloat-judge.sh", []string{"--numbers", "comment", path}, nil, &out, &errOut, call, nil)
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
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &first, &errOut, call, nil); code != exitCut {
		t.Fatalf("first run exit %d — %s", code, errOut.String())
	}
	again := write(t, first.String())
	if code := Run("bloat-judge.sh", []string{"comment", again}, nil, &second, &errOut, call, nil); code != exitClean {
		t.Fatalf("second run exit %d, want clean — %s", code, errOut.String())
	}
	if second.String() != first.String() {
		t.Fatalf("second run changed the text:\n%s\n---\n%s", first.String(), second.String())
	}
}

func TestRunRefusesAModelThatExplains(t *testing.T) {
	path := write(t, source)
	var out, errOut strings.Builder
	call := func(string, string) (string, error) { return "Unit 1 restates the file name.", nil }
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if out.Len() != 0 {
		t.Fatalf("a refused run still printed: %q", out.String())
	}
}

func TestRunRefusesAModelThatDoesNotAnswer(t *testing.T) {
	path := write(t, source)
	var out, errOut strings.Builder
	call := func(string, string) (string, error) { return "", errors.New("down") }
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
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
	if code := Run("bloat-judge.sh", []string{"poem"}, strings.NewReader("x"), &out, &errOut, nil, nil); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if !strings.Contains(errOut.String(), "comment commit instruction pr-body record-entry reply report return review slack ticket") {
		t.Fatalf("the refusal does not list the kinds: %s", errOut.String())
	}
}

// A path carrying a newline must not forge a second line in the refusal.
func TestARefusalCarriesNoControlByteFromItsArgument(t *testing.T) {
	var out, errOut strings.Builder
	Run("bloat-judge.sh", []string{"comment", "no\x1b[31msuch\nfile"}, nil, &out, &errOut, nil, nil)
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
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, nil); code != exitClean {
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
	if code := Run("bloat-judge.sh", []string{"pr-body"}, in, &out, &errOut, call, nil); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if out.String() != "What changes.\n\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestMemoMakesAnInconsistentModelIdempotent(t *testing.T) {
	path := write(t, source)
	memo := &Memo{Dir: filepath.Join(t.TempDir(), "judged")}
	calls := 0
	greedy := func(_, view string) (string, error) {
		calls++
		return "1", nil // always the first unit left, so unchecked it would empty the file
	}
	var first, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &first, &errOut, greedy, memo); code != exitCut {
		t.Fatalf("first run exit %d — %s", code, errOut.String())
	}
	var second strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", write(t, first.String())}, nil, &second, &errOut, greedy, memo); code != exitClean {
		t.Fatalf("the pruned text was judged again: exit %d, %q", code, second.String())
	}
	var replay strings.Builder
	Run("bloat-judge.sh", []string{"comment", path}, nil, &replay, &errOut, greedy, memo)
	if replay.String() != first.String() {
		t.Fatalf("the original drew a different verdict on replay")
	}
	if calls != 1 {
		t.Fatalf("the model was called %d times, want 1", calls)
	}
}

func TestMemoThatCannotWriteStillJudges(t *testing.T) {
	path := write(t, source)
	memo := &Memo{Dir: filepath.Join(write(t, "not a dir"), "judged")}
	var out, errOut strings.Builder
	call := func(string, string) (string, error) { return "1", nil }
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, memo); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
}

func TestMemoDoesNotReuseAnotherPolicy(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	call := func(prompt, view string) (string, error) { calls++; return "none", nil }
	for _, policy := range []string{"first", "second", "second"} {
		memo := &Memo{Dir: dir, Policy: policy}
		var out, errOut strings.Builder
		if code := Run("judge", []string{"reply"}, strings.NewReader("Keep this fact.\n"), &out, &errOut, call, memo); code != 0 {
			t.Fatalf("code=%d %s", code, errOut.String())
		}
	}
	if calls != 2 {
		t.Fatalf("model calls=%d, want 2 distinct policies", calls)
	}
}

func TestMemoInvalidatesWhenTheReaderPolicyChanges(t *testing.T) {
	original := kinds["reply"]
	t.Cleanup(func() { kinds["reply"] = original })
	memo := &Memo{Dir: t.TempDir(), Policy: "same-models"}
	calls := 0
	call := func(prompt, view string) (string, error) { calls++; return "none", nil }
	for _, reader := range []string{"first reader", "new reader"} {
		kinds["reply"] = Kind{Reader: reader}
		var out, errOut strings.Builder
		if code := Run("judge", []string{"reply"}, strings.NewReader("An important fact.\n"), &out, &errOut, call, memo); code != 0 {
			t.Fatalf("judge=%d %s", code, errOut.String())
		}
	}
	if calls != 2 {
		t.Fatalf("reader policy changed but model calls=%d, want 2", calls)
	}
}

func TestChangedOffersOnlyTheBlocksTheDiffTouched(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		// The machine's own git config must not reach this fixture: a global core.excludesFile
		// matching `*.go` refuses the `git add` below, and commit.gpgsign refuses the commit —
		// both on a machine where this tool is working perfectly.
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	committed := "// human one\nfunc a() {}\n// human two\nfunc b() {}\n"
	if err := os.WriteFile(filepath.Join(repo, "f.go"), []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "f.go")
	git("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(repo, "f.go"), []byte(committed+"// agent three\nfunc c() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	if code := RunIn("bloat-judge.sh", []string{"--changed", "comment", "f.go"}, repo, nil, &out, &errOut, call, nil); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if out.String() != committed+"func c() {}\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestChangedRefusesWithoutAPath(t *testing.T) {
	var out, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"--changed", "pr-body"}, strings.NewReader("x\n"), &out, &errOut, nil, nil); code != exitDidNotRun {
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
	_, view := Split(separated, false, all)
	return view
}

func TestVotingDeletesOnlyWhatAMajorityNames(t *testing.T) {
	reply, err := Voting(rollsAnswering("1, 2", "1", "3"), 3)("p", viewOf("a", "b", "c"))
	if err != nil || reply != "1" {
		t.Fatalf("got %q %v, want \"1\"", reply, err)
	}
}

func TestVotingAnswersNoneWhenNothingAgrees(t *testing.T) {
	reply, err := Voting(rollsAnswering("1", "2", "3"), 3)("p", viewOf("a", "b", "c"))
	if err != nil || reply != "none" {
		t.Fatalf("got %q %v, want none", reply, err)
	}
}

func TestVotingRefusesIfAnyRollExplains(t *testing.T) {
	if _, err := Voting(rollsAnswering("1", "I think 1 goes", "1"), 3)("p", viewOf("a", "b", "c")); err == nil {
		t.Fatal("a prose roll was outvoted instead of refused")
	}
}

func TestEveryLaneKindExists(t *testing.T) {
	for _, name := range []string{"comment", "instruction", "pr-body", "review", "commit", "report", "return", "reply", "ticket", "slack", "record-entry"} {
		if _, ok := kinds[name]; !ok {
			t.Errorf("no kind %q", name)
		}
	}
}

// A `/*` block whose continuation lines carry no leading `*` is still one comment. Split as one unit per
// comment-looking line, deleting it took the first line alone and left `kept for history. */` to break
// the file.
func TestSplitSourceHoldsABlockCommentWhole(t *testing.T) {
	lines := []string{"/* Legacy block comment", "   kept for history. */", "code()", "/* one-liner */", "code()", "// after"}
	units, view := Split(lines, true, all)
	if len(units) != 3 || units[0].Span != 2 || units[1].Span != 1 || units[2].Span != 1 {
		t.Fatalf("got %d units with spans %v, want 3 with spans 2, 1, 1", len(units), spansOf(units))
	}
	if !strings.Contains(view, "   1| /* Legacy block comment\n   .|    kept for history. */\n") {
		t.Fatalf("the closing line is not marked as the block's continuation:\n%s", view)
	}
}

func spansOf(units []Unit) []int {
	spans := make([]int, len(units))
	for i, u := range units {
		spans[i] = u.Span
	}
	return spans
}

// A memo naming a unit the text does not have is a miss, not a verdict.
func TestMemoNamingAUnitOutOfRangeIsIgnored(t *testing.T) {
	path := write(t, source)
	memo := &Memo{Dir: filepath.Join(t.TempDir(), "judged")}
	units, _ := Split(strings.Split(strings.TrimSuffix(source, "\n"), "\n"), true, all)
	memo.record("comment\n"+offeredKey(units), source, []int{len(units) + 1})
	calls := 0
	call := func(string, string) (string, error) { calls++; return "1", nil }
	var out, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, memo); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if calls != 1 {
		t.Fatalf("the planted verdict was taken as a verdict: %d model calls", calls)
	}
}

// A roll that names a unit nobody offered has lost the plot exactly as a roll that explains has, and
// fails the vote the same way. The gap the old line-count bound left is widest in a source file, whose
// units are its comment blocks: a 500-line file with 40 of them accepted 501.
func TestVotingRefusesARollNamingAUnitThatWasNeverOffered(t *testing.T) {
	view := viewOf("a", "b")
	if _, err := Voting(rollsAnswering("1", "3", "1"), 3)("p", view); err == nil {
		t.Fatal("a unit number past the last unit was tallied instead of refused")
	}
	if got := unitsInView(view); got != 2 {
		t.Fatalf("the view offers %d units, not 2 — the bound is reading something else", got)
	}
}

// A fenced block is one unit over four lines, and blank lines are no unit at all, so counting lines
// would answer 7 here where the vote may only offer 2.
func TestUnitsInViewCountsUnitsAndNotLines(t *testing.T) {
	view := viewOf("intro", "", "```", "one", "two", "```", "")
	if got := unitsInView(view); got != 2 {
		t.Fatalf("got %d units, want 2\n%s", got, view)
	}
}

func TestEveryRollGoesOutEvenWhenTheyAgree(t *testing.T) {
	call, calls := counting(func(string, string) (string, error) { return "2, 1", nil })
	got, err := Voting(call, 3)("prompt", viewOf("one", "two"))
	if err != nil || got != "1,2" {
		t.Fatalf("majority = %q, %v, want \"1,2\"", got, err)
	}
	if calls() != 3 {
		t.Fatalf("%d call(s), want 3 — a roll was held back", calls())
	}
}

// The rolls go out together, which is the half of it a call count cannot see. Each one blocks until
// all three have arrived, so a vote that rolled any of them in a later wave never reaches the third
// and this ends on the timeout instead of the reply.
func TestTheRollsGoOutTogether(t *testing.T) {
	var arrived sync.WaitGroup
	arrived.Add(3)
	call := func(string, string) (string, error) {
		arrived.Done()
		arrived.Wait()
		return "1", nil
	}
	done := make(chan string, 1)
	go func() {
		reply, err := Voting(call, 3)("p", viewOf("a", "b"))
		if err != nil {
			done <- "refused: " + err.Error()
			return
		}
		done <- reply
	}()
	select {
	case reply := <-done:
		if reply != "1" {
			t.Fatalf("got %q, want 1", reply)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the rolls went out in more than one wave — the last never started while the others waited")
	}
}

// The count is Voting's own parameter and the majority is arithmetic over it, so the rule has to hold
// for counts other than the 3 production passes today. Nine because a high count is where an
// off-by-one hides, and because 3 alone would let a wrong general rule pass — at 3 a bare half and
// more than half name the same number of rolls.
func TestTheMajorityRuleHoldsAtAHigherRollCount(t *testing.T) {
	for _, c := range []struct {
		name    string
		replies []string
		want    string
	}{
		{"five of nine carries a unit", []string{"1", "1", "1", "1", "1", "2", "2", "2", "2"}, "1"},
		{"four of nine does not", []string{"1", "1", "1", "1", "2", "2", "3", "3", "none"}, "none"},
	} {
		t.Run(c.name, func(t *testing.T) {
			call, calls := counting(rollsAnswering(c.replies...))
			got, err := Voting(call, 9)("p", viewOf("a", "b", "c"))
			if err != nil {
				t.Fatalf("vote refused: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			if calls() != 9 {
				t.Fatalf("%d call(s), want 9 — a roll was held back", calls())
			}
		})
	}
}

// A roll that never answered fails the whole vote, exactly as a roll that explains does. The rolls
// that did answer are a majority of a smaller vote than the one the caller asked for, and reading a
// verdict out of them reports the deadline the model hit as a judgement it made.
func TestVotingRefusesWhenARollFails(t *testing.T) {
	var mu sync.Mutex
	rolled := 0
	call := func(string, string) (string, error) {
		mu.Lock()
		rolled++
		first := rolled == 1
		mu.Unlock()
		if first {
			return "", errors.New("the model did not answer within 420s")
		}
		return "1", nil
	}
	if _, err := Voting(call, 3)("p", viewOf("a", "b")); err == nil {
		t.Fatal("a roll that never answered was outvoted instead of failing the vote")
	}
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

func TestSplitProseHoldsAWrappedParagraphAsOneUnit(t *testing.T) {
	units, view := Split(strings.Split(strings.TrimSuffix(wrapped, "\n"), "\n"), false, all)
	if len(units) != 3 {
		t.Fatalf("got %d units, want 3 (subject, body, trailer)", len(units))
	}
	if units[1].Line != 3 || units[1].Span != 3 {
		t.Fatalf("the body is line %d span %d, want 3 span 3", units[1].Line, units[1].Span)
	}
	if !strings.Contains(view, "   2| A pass measured") || !strings.Contains(view, "   .| result up as") {
		t.Fatalf("the body's wrapped lines are not marked as one unit:\n%s", view)
	}
}

func TestSplitProseKeepsEachListItemItsOwnUnit(t *testing.T) {
	lines := []string{"Lead-in text", "- first item that wraps", "  onto a second line", "- second item", "1. numbered", "2. also numbered", "# heading", "| a | b |"}
	units, _ := Split(lines, false, all)
	want := []Unit{{Line: 1, Span: 1}, {Line: 2, Span: 2}, {Line: 4, Span: 1}, {Line: 5, Span: 1}, {Line: 6, Span: 1}, {Line: 7, Span: 1}, {Line: 8, Span: 1}}
	if len(units) != len(want) {
		t.Fatalf("got %d units, want %d: %v", len(units), len(want), units)
	}
	for i, unit := range units {
		if unit != want[i] {
			t.Fatalf("unit %d is %+v, want %+v", i+1, unit, want[i])
		}
	}
}

// A paragraph opening in bold is prose, not a list: `**Bold**` and `--- so` both start with a marker
// character and neither carries the space that makes one.
func TestSplitProseDoesNotReadBoldOrADashAsAListItem(t *testing.T) {
	units, _ := Split([]string{"**Bold** opens this", "--- and this continues it", "*emphasis* too"}, false, all)
	if len(units) != 1 || units[0].Span != 3 {
		t.Fatalf("got %v, want one unit spanning 3 lines", units)
	}
}

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
	if code := RunIn("j", []string{"commit", path}, ".", nil, &out, &errs, greedy, nil); code != exitCut {
		t.Fatalf("exit %d, stderr %s", code, errs.String())
	}
	// The separators a deleted block sat between stay, which git's own `--cleanup` collapses and
	// markdown renders as one blank; what matters here is that the trailer itself is untouched.
	if got := out.String(); got != "Name the commit a scanner number was read off\n\n\nCo-Authored-By: Claude Opus 5 <noreply@anthropic.com>\n" {
		t.Fatalf("the judge kept %q; the subject and the trailer should survive and nothing else", got)
	}
}

// Git's own rule, and the reason for it: with no block above it, the last block is the subject line,
// and a subject shaped like `Fix: the thing` is the message rather than metadata about it.
func TestASubjectLineShapedLikeATrailerIsStillJudged(t *testing.T) {
	if withheld := trailerLines([]string{"Fix: the thing"}); withheld != nil {
		t.Fatalf("a lone subject was withheld as a trailer: %v", withheld)
	}
	if withheld := trailerLines([]string{"Subject", "", "Body text"}); withheld != nil {
		t.Fatalf("an ordinary closing paragraph was withheld as a trailer: %v", withheld)
	}
	withheld := trailerLines([]string{"Subject", "", "Signed-off-by: A <a@b>", "Co-Authored-By: C <c@d>"})
	if len(withheld) != 2 || !withheld[3] || !withheld[4] {
		t.Fatalf("the trailer block is %v, want lines 3 and 4", withheld)
	}
}

// Git writes lines into the trailer block that are not `Token: value`, and the block has to survive
// them: all-or-nothing, a cherry-pick note hands the sign-off and the co-author line back to a vote
// told to delete provenance.
func TestATrailerBlockSurvivesTheLinesGitPutsInIt(t *testing.T) {
	for name, block := range map[string][]string{
		"a cherry-pick note": {"Signed-off-by: A <a@b>", "(cherry picked from commit deadbee)"},
		"a bare issue ref":   {"Co-Authored-By: C <c@d>", "Fixes #123"},
		"a folded value":     {"Co-Authored-By: C", "  <c@d>"},
	} {
		t.Run(name, func(t *testing.T) {
			withheld := trailerLines(append([]string{"Subject", "", "Body.", ""}, block...))
			if len(withheld) != len(block) || !withheld[5] || !withheld[6] {
				t.Fatalf("withheld %v, want lines 5 and 6", withheld)
			}
		})
	}
	// Still nothing to withhold where no line in the block is a trailer at all.
	if withheld := trailerLines([]string{"Subject", "", "Body.", "", "Closing thought.", "Another line."}); withheld != nil {
		t.Fatalf("an ordinary closing paragraph was withheld: %v", withheld)
	}
}

// The shape of a message piped from `git log`, which ends in blank lines.
func TestTrailingBlanksDoNotHideTheTrailerBlock(t *testing.T) {
	withheld := trailerLines([]string{"Subject", "", "Body.", "", "Co-Authored-By: C <c@d>", "", ""})
	if len(withheld) != 1 || !withheld[5] {
		t.Fatalf("the trailer block is %v, want line 5 alone", withheld)
	}
}

func TestOpensBlockTakesAMarkerOnlyWithTheSpaceAfterIt(t *testing.T) {
	// Inside a paragraph, which is where a stray marker does the damage.
	for line, want := range map[string]bool{
		"# heading": true, "#no-space": true, "> quoted": true, "| a | b |": true,
		"- item": true, "* item": true, "+ item": true, "1. item": true, "1) item": true,
		"-\titem": true, "1.\titem": true,
		"**Bold** opens a paragraph": false, "--- a comparison": false, "*emphasis*": false,
		"-": false, "12": false, "12.": false, "0": false, "2026-09-16 was the date": false,
		"plain continuation": false, "": false,
	} {
		if got := opensBlock(line, false); got != want {
			t.Errorf("opensBlock(%q, inList=false) = %v, want %v", line, got, want)
		}
	}
}

// CommonMark lets an ordered list interrupt a paragraph only where it numbers from one. Without that,
// a commit message wrapping onto a line like `163. Three wordings were tried` is split mid-sentence
// by the very rule that exists to stop that — this repo's own commit a1eb712f is where it was found.
func TestAnOrderedMarkerSplitsAParagraphOnlyAtOneOrInsideAList(t *testing.T) {
	for _, line := range []string{"163. Three wordings were tried", "2) and then"} {
		if opensBlock(line, false) {
			t.Errorf("%q opened a block mid-paragraph, cutting the sentence it belongs to", line)
		}
		if !opensBlock(line, true) {
			t.Errorf("%q did not open its own item inside a list", line)
		}
	}
	lines := []string{"A pass measured the range while HEAD was somewhere else, then wrote it up as", "163. Three wordings were tried and the shortest won.", "", "1. first", "2. second"}
	units, _ := Split(lines, false, all)
	want := []Unit{{Line: 1, Span: 2}, {Line: 4, Span: 1}, {Line: 5, Span: 1}}
	if len(units) != len(want) {
		t.Fatalf("got %v, want %v", units, want)
	}
	for i, unit := range units {
		if unit != want[i] {
			t.Fatalf("unit %d is %+v, want %+v", i+1, unit, want[i])
		}
	}
}

// A unit can reach past the block it opened as: an unclosed fence runs one unit to the end of the
// text. Withholding on the unit's first line alone then hands the trailers it swallowed to the vote.
func TestAnUnclosedFenceCannotCarryTheTrailersIntoAUnit(t *testing.T) {
	lines := []string{"Subject", "", "Body with a repro:", "```", "$ run it", "", "Co-Authored-By: C <c@d>"}
	units, _ := Split(lines, false, offerFor(lines, kinds["commit"]))
	for _, unit := range units {
		if unit.Line <= 7 && unit.Line+unit.Span > 7 {
			t.Fatalf("unit %+v was offered though it holds the trailer on line 7", unit)
		}
	}
}

// A bullet that wraps is the same half-sentence hazard one context over: an ordered marker after a
// bullet item begins a fresh ordered list, whose first item must be numbered 1 to interrupt anything.
func TestAWrappedBulletIsNotSplitByANumberOnItsNextLine(t *testing.T) {
	units, _ := Split([]string{"- a sentence that wraps onto", "163. Three wordings were tried", "and keeps going"}, false, all)
	if len(units) != 1 || units[0] != (Unit{Line: 1, Span: 3}) {
		t.Fatalf("got %v, want one unit spanning all three lines", units)
	}
	// What must keep working: a real ordered list still numbers past one, and a bullet list still
	// gives every bullet its own unit.
	for name, lines := range map[string][]string{
		"an ordered list":            {"1. first", "2. second", "3. third"},
		"an ordered list that wraps": {"1. first", "   onto a second line", "2. second"},
		"a bullet list":              {"- a", "- b"},
		"a bullet then an ordered":   {"- a", "1. b"},
	} {
		units, _ := Split(lines, false, all)
		if len(units) < 2 {
			t.Errorf("%s collapsed into %v", name, units)
		}
	}
}

// A heading takes nothing with it, so deleting the paragraph under one leaves the heading standing.
// A quote does wrap, by markdown's own lazy continuation.
func TestAHeadingAndATableRowDoNotSwallowTheLineBelow(t *testing.T) {
	for name, lines := range map[string][]string{
		"a heading":   {"# Heading", "Some prose here.", "More prose."},
		"a table row": {"| a | b |", "Some prose here.", "More prose."},
	} {
		units, _ := Split(lines, false, all)
		if len(units) != 2 || units[0].Span != 1 || units[1] != (Unit{Line: 2, Span: 2}) {
			t.Errorf("%s gave %v, want it alone then the paragraph below it", name, units)
		}
	}
	units, _ := Split([]string{"> quoted", "lazy continuation"}, false, all)
	if len(units) != 1 || units[0].Span != 2 {
		t.Fatalf("a quote's lazy continuation gave %v, want one unit spanning both", units)
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
	if code := RunIn("j", []string{"commit", path}, ".", nil, &out, &errs, greedy, nil); code != exitCut {
		t.Fatalf("exit %d, stderr %s", code, errs.String())
	}
	if got := out.String(); !strings.HasPrefix(got, "Name the commit a scanner number was read off\n") {
		t.Fatalf("the judge kept %q; the subject must survive a vote that named everything", got)
	}
}

// Git's own shape for a subject-only commit. Withholding the one block there is would leave nothing
// to judge, and the run would report clean over text no roll ever read.
func TestASubjectOnlyMessageIsStillJudged(t *testing.T) {
	if withheld := subjectLines([]string{"Fix the thing"}); withheld != nil {
		t.Fatalf("a subject-only message withheld %v, leaving nothing to judge", withheld)
	}
	if withheld := subjectLines([]string{"Subject", "", "Body."}); len(withheld) != 1 || !withheld[1] {
		t.Fatalf("the subject block is %v, want line 1", withheld)
	}
}
