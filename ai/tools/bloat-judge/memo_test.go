// Cases for the record that makes an inconsistent model idempotent.
package bloatjudge

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoMakesAnInconsistentModelIdempotent(t *testing.T) {
	path := write(t, source)
	memo := &Memo{Dir: filepath.Join(t.TempDir(), "judged")}
	calls := 0
	greedy := func(_, view string) (string, error) {
		calls++
		return "1", nil // always the first unit left, so unchecked it would empty the file
	}
	var first, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &first, &errOut, greedy, memo); code != exitCut {
		t.Fatalf("first run exit %d — %s", code, errOut.String())
	}
	var second strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", write(t, first.String())}, unasked(), nil, &second, &errOut, greedy, memo); code != exitClean {
		t.Fatalf("the pruned text was judged again: exit %d, %q", code, second.String())
	}
	var replay strings.Builder
	Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &replay, &errOut, greedy, memo)
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
	if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &out, &errOut, call, memo); code != exitCut {
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
		if code := Run("judge", []string{"reply"}, unasked(), strings.NewReader("Keep this fact.\n"), &out, &errOut, call, memo); code != 0 {
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
		if code := Run("judge", []string{"reply"}, unasked(), strings.NewReader("An important fact.\n"), &out, &errOut, call, memo); code != 0 {
			t.Fatalf("judge=%d %s", code, errOut.String())
		}
	}
	if calls != 2 {
		t.Fatalf("reader policy changed but model calls=%d, want 2", calls)
	}
}

// A memo naming a unit the text does not have is a miss, not a verdict.
func TestMemoNamingAUnitOutOfRangeIsIgnored(t *testing.T) {
	path := write(t, source)
	memo := &Memo{Dir: filepath.Join(t.TempDir(), "judged")}
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	units, _ := Split(lines, commentBlocks(lines), all)
	memo.record("comment\n"+offeredKey(units), source, []int{len(units) + 1})
	calls := 0
	call := func(string, string) (string, error) { calls++; return "1", nil }
	var out, errOut strings.Builder
	if code := Run("bloat-judge.sh", []string{"comment", path}, unasked(), nil, &out, &errOut, call, memo); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if calls != 1 {
		t.Fatalf("the planted verdict was taken as a verdict: %d model calls", calls)
	}
}
