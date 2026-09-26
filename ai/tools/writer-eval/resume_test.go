package writereval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"testing"
)

// A usage limit mid-table is the normal end of a full table on a team account. Item 20's table
// stopped at case 21 of 47 on 2026-09-26, with 307 calls read. WRITER_EVAL_RESUME reads the rolls
// already in the dump back, and the run spends calls only on the rolls still missing. A roll is
// reused only where the rules and the prompt it was asked are the ones this run would ask.
const resumeEnv = "WRITER_EVAL_RESUME"

// askedSum names the prompt a roll was asked. The harness's own text is part of that prompt, and a
// harness change shifts the sum where the rules stay as they were.
func askedSum(asked string) string {
	sum := sha256.Sum256([]byte(asked))
	return hex.EncodeToString(sum[:])[:12]
}

// resumed hands out the kept rolls of each case, one per call.
type resumed struct {
	mu    sync.Mutex
	kept  map[string][]rollResult
	taken int
}

// resumeFrom reads the kept rolls of a dump: those that landed a verdict under these rules and this
// prompt. A roll a limit, an error or a stop ended is asked again.
func resumeFrom(path, rules string, askedOf func(string) string) *resumed {
	r := &resumed{kept: map[string][]rollResult{}}
	body, err := os.ReadFile(path)
	if err != nil {
		return r
	}
	var held []dumped
	if json.Unmarshal(body, &held) != nil {
		return r
	}
	for _, row := range held {
		if row.Verdict == nil || row.Rules != rules || row.Asked != askedOf(row.Case) {
			continue
		}
		r.kept[row.Case] = append(r.kept[row.Case], rollResult{verdict: *row.Verdict, raw: row.Raw,
			rounds: row.Rounds, findings: row.Findings, reused: true})
	}
	return r
}

// take is a kept roll of the case, and whether one was left.
func (r *resumed) take(name string) (rollResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.kept[name]
	if len(kept) == 0 {
		return rollResult{}, false
	}
	r.kept[name] = kept[1:]
	r.taken++
	return kept[0], true
}

// A resumed run reuses a roll that landed under the same rules and prompt. It asks again a roll a limit
// ended, one read under other rules and one asked another prompt.
func TestAResumedRunReusesOnlyRollsItWouldAskAgain(t *testing.T) {
	passed := &Verdict{Name: "k01", Want: ExpectWritten, Got: ExpectWritten}
	asked := func(name string) string { return askedSum("prompt of " + name) }
	rows := []dumped{
		{Case: "k01", Raw: "kept", Rules: "r1", Asked: asked("k01"), Verdict: passed},
		{Case: "k01", Raw: "limit", Rules: "r1", Asked: asked("k01")},
		{Case: "k01", Raw: "old rules", Rules: "r0", Asked: asked("k01"), Verdict: passed},
		{Case: "k01", Raw: "old prompt", Rules: "r1", Asked: askedSum("an older prompt"), Verdict: passed},
	}
	path := t.TempDir() + "/dump.json"
	body, _ := json.Marshal(rows)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	r := resumeFrom(path, "r1", asked)
	got, ok := r.take("k01")
	if !ok || got.raw != "kept" || !got.reused || !got.verdict.Passed() {
		t.Fatalf("got %+v, %v; want the one kept roll", got, ok)
	}
	if _, ok := r.take("k01"); ok {
		t.Fatal("a roll a limit ended, or one under other rules or another prompt, was reused")
	}
}
