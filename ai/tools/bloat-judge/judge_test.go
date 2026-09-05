package bloatjudge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

// Prose is refused whole rather than mined for digits: a model that explains has stopped judging.
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

// Judged twice, the second run must delete nothing: the hook's resend goes through the same judge, and
// this is the property that lets it converge rather than shave a share off each pass.
func TestRunIsIdempotentUnderAConsistentJudge(t *testing.T) {
	path := write(t, source)
	// A judge that always deletes the unit whose text is "// on a()", wherever it now sits.
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

// The view is untrusted text, so the real run reaches the model with nothing it could be talked into
// using: no tools, no MCP servers, and none of the repository's own settings.
func TestClaudeArgsGrantNoToolsServersOrRepoSettings(t *testing.T) {
	args := claudeArgs("You are")
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
	if i := at("--setting-sources"); args[i+1] != "user" {
		t.Fatalf("--setting-sources must be user alone, got %q", args[i+1])
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

// A file with nothing to judge is passed through byte for byte and never reaches the model.
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

// The memo is what makes the judge idempotent whatever the model does. Judged once, the text's verdict
// is recorded and its pruned form is recorded clean, so a resend calls no model and deletes nothing —
// even under a model that would delete something new every time it was asked.
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

// A memo the process cannot write is not a reason to refuse: the verdict is still applied, and only the
// next run pays for the model again.
func TestMemoThatCannotWriteStillJudges(t *testing.T) {
	path := write(t, source)
	memo := &Memo{Dir: filepath.Join(write(t, "not a dir"), "judged")}
	var out, errOut strings.Builder
	call := func(string, string) (string, error) { return "1", nil }
	if code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, call, memo); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
}

// The commit hook's form. A committed file with two blocks gains a third: only the third is offered,
// the two committed ones are shown as context and cannot be deleted whatever the model answers.
func TestChangedOffersOnlyTheBlocksTheDiffTouched(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
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

// Two of three rolls name unit 1 and one names unit 2: only 1 goes. One roll cannot delete alone.
func TestVotingDeletesOnlyWhatAMajorityNames(t *testing.T) {
	replies := []string{"1, 2", "1", "3"}
	i := 0
	roll := func(string, string) (string, error) { r := replies[i]; i++; return r, nil }
	reply, err := Voting(roll, 3)("p", "a\nb\nc")
	if err != nil || reply != "1" {
		t.Fatalf("got %q %v, want \"1\"", reply, err)
	}
}

func TestVotingAnswersNoneWhenNothingAgrees(t *testing.T) {
	replies := []string{"1", "2", "3"}
	i := 0
	roll := func(string, string) (string, error) { r := replies[i]; i++; return r, nil }
	reply, err := Voting(roll, 3)("p", "a\nb\nc")
	if err != nil || reply != "none" {
		t.Fatalf("got %q %v, want none", reply, err)
	}
}

// A roll that explains fails the vote outright rather than being outvoted into silence.
func TestVotingRefusesIfAnyRollExplains(t *testing.T) {
	replies := []string{"1", "I think 1 goes", "1"}
	i := 0
	roll := func(string, string) (string, error) { r := replies[i]; i++; return r, nil }
	if _, err := Voting(roll, 3)("p", "a\nb\nc"); err == nil {
		t.Fatal("a prose roll was outvoted instead of refused")
	}
}

// The lanes name these kinds in their instructions; one missing here fails their call at run time with
// "no kind", which no test of theirs would catch.
func TestEveryLaneKindExists(t *testing.T) {
	for _, name := range []string{"comment", "instruction", "pr-body", "review", "commit", "report", "return", "reply", "ticket", "slack", "record-entry"} {
		if _, ok := kinds[name]; !ok {
			t.Errorf("no kind %q", name)
		}
	}
}

// A `/*` block whose continuation lines carry no leading `*` is still one comment. Split as one unit per
// comment-looking line, deleting it took the first line alone and left `kept for history. */` to break
// the file — the drive gate's one divergence on the landing.
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
