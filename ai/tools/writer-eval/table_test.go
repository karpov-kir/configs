package writereval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	modelpolicy "configs/ai/tools/model-policy"
	readerjudge "configs/ai/tools/reader-judge"
)

// rollResult is one roll of one case, with what it cost.
type rollResult struct {
	verdict  Verdict
	raw      string
	rounds   int
	findings [][]string
	stopped  bool
	// limit is a roll the account's usage limit refused. It stops the table, since every roll after it
	// meets the same limit.
	limit bool
}

// profile adds up where a table's time goes. Every roll waits on the writer row and on the record
// check, and the rest of a worker's time is idle.
type profile struct {
	mu            sync.Mutex
	started       time.Time
	wall          time.Duration
	workers       int
	model, check  time.Duration
	calls, checks int
	retries       int
	used          readerjudge.Served
}

// spend adds one call's usage to the table's.
func (p *profile) spend(u readerjudge.Served) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.used.Input += u.Input
	p.used.CacheCreated += u.CacheCreated
	p.used.CacheRead += u.CacheRead
	p.used.Output += u.Output
	p.used.CostUSD += u.CostUSD
}

func (p *profile) add(model bool, took time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if model {
		p.model += took
		p.calls++
		return
	}
	p.check += took
	p.checks++
}

func (p *profile) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	busy := p.model + p.check
	idle := time.Duration(p.workers)*p.wall - busy
	mean := func(sum time.Duration, n int) time.Duration {
		if n == 0 {
			return 0
		}
		return (sum / time.Duration(n)).Round(time.Millisecond)
	}
	return fmt.Sprintf("profile: %s wall over %d worker(s); %d writer call(s), %s in all, %s each; "+
		"%d check run(s), %s in all, %s each; %s idle across the workers\n"+
		"tokens: %d input, %d written to cache, %d read from cache, %d output; $%.2f; retried ×%d",
		p.wall.Round(time.Second), p.workers, p.calls, p.model.Round(time.Second), mean(p.model, p.calls),
		p.checks, p.check.Round(time.Second), mean(p.check, p.checks), idle.Round(time.Second),
		p.used.Input, p.used.CacheCreated, p.used.CacheRead, p.used.Output, p.used.CostUSD, p.retries)
}

// rollOnce runs one roll the way the pipeline runs a site, timing each writer call and each check.
func rollOnce(ctx context.Context, settings modelpolicy.Settings, c Case, asked string, p *profile) rollResult {
	var findings [][]string
	caller := readerjudge.ClaudeCallerObserved(callDeadline, settings, func(s readerjudge.Served) {
		servedModels.add(s)
		p.spend(s)
	})
	once := func(text string) (string, error) {
		release, err := takeSlot()
		if err != nil {
			return "", err
		}
		defer release()
		began := time.Now()
		defer func() { p.add(true, time.Since(began)) }()
		return caller(text, "")
	}
	// A call failing for any reason but a limit is tried once more after a pause. 404 calls of one table
	// on 2026-09-25 failed for a passing refusal and left a column with no reading in it.
	call := func(text string) (string, error) {
		answer, err := once(text)
		var exhausted *readerjudge.ProviderExhausted
		if err == nil || errors.As(err, &exhausted) || ctx.Err() != nil {
			return answer, err
		}
		p.mu.Lock()
		p.retries++
		p.mu.Unlock()
		time.Sleep(retryPause)
		return once(text)
	}
	check := func(input string, record bool) ([]string, error) {
		began := time.Now()
		got, err := recordCheckFor()(input, record)
		p.add(false, time.Since(began))
		if len(got) > 0 {
			findings = append(findings, got)
		}
		return got, err
	}
	r, raw, err := writeChecked(call, check, asked, c.Code)
	var exhausted *readerjudge.ProviderExhausted
	if errors.As(err, &exhausted) {
		return rollResult{limit: true, raw: err.Error(), verdict: Verdict{Name: c.Name, Want: c.Expect, Got: "error"}}
	}
	if err != nil {
		if ctx.Err() != nil {
			return rollResult{stopped: true, verdict: Verdict{Name: c.Name, Want: c.Expect, Got: "stopped"}}
		}
		return rollResult{raw: err.Error(), verdict: Verdict{Name: c.Name, Want: c.Expect, Got: "error"}}
	}
	return rollResult{verdict: JudgeCase(c, r), raw: strings.TrimSpace(raw), rounds: r.Rounds, findings: findings}
}

// retryPause is how long a failed call waits before its one retry.
const retryPause = 5 * time.Second

// runTable puts every roll of every case to a fixed set of workers, in case order, so the first
// cases finish first. Each case prints its row the moment its last roll lands. A case whose misses
// already leave its floor out of reach stops the table: a table that will miss is not worth
// finishing. A case that can still pass runs every roll, because the bar compares counts.
func runTable(cases []Case, workers int, need, regression func(Case) int, short func(Case) bool, full bool,
	roll func(context.Context, Case) rollResult) ([][]rollResult, string) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	type job struct{ at, roll int }
	jobs := make(chan job)
	results := make([][]rollResult, len(cases))
	landed := make([]int, len(cases))
	clean := make([]int, len(cases))
	missed := make([]bool, len(cases))
	hopeless := make([]bool, len(cases))
	for i := range cases {
		results[i] = make([]rollResult, evalRolls)
	}
	var mu sync.Mutex
	stopped := ""
	var wait sync.WaitGroup
	for w := 0; w < workers; w++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for j := range jobs {
				c := cases[j.at]
				res := rollResult{stopped: true, verdict: Verdict{Name: c.Name, Want: c.Expect, Got: "stopped"}}
				mu.Lock()
				// A proven bystander that has passed every one of its first shortRolls is read short. One
				// miss sends it to the full read.
				decided := !full && (clean[j.at] >= need(c) || hopeless[j.at] ||
					(short(c) && !missed[j.at] && clean[j.at] >= shortRolls))
				mu.Unlock()
				if decided {
					res = rollResult{stopped: true, verdict: Verdict{Name: c.Name, Want: c.Expect, Got: "reached"}}
				} else if ctx.Err() == nil {
					res = roll(ctx, c)
				}
				mu.Lock()
				results[j.at][j.roll] = res
				landed[j.at]++
				if res.verdict.Passed() {
					clean[j.at]++
				} else if !res.stopped {
					missed[j.at] = true
				}
				if res.limit && stopped == "" {
					stopped = c.Name
					fmt.Fprintf(os.Stderr, "stopping the table: %s\n", res.raw)
					stop()
				}
				reachable := clean[j.at] + evalRolls - landed[j.at]
				if !full && !res.stopped && reachable < need(c) {
					hopeless[j.at] = true
				}
				// The table stops only on a regression: a case that cannot reach main's own count less
				// two. A case already below its floor on main misses the floor here too, and that miss
				// belongs to main.
				if !full && !res.stopped && stopped == "" && reachable < regression(c) {
					stopped = c.Name
					fmt.Fprintf(os.Stderr, "stopping the table: %s cannot reach %d, main's count less two\n", c.Name, regression(c))
					stop()
				}
				if landed[j.at] == evalRolls && ctx.Err() == nil {
					row, _ := caseRow(c, results[j.at])
					fmt.Fprint(os.Stderr, row)
				}
				mu.Unlock()
			}
		}()
	}
	for i := range cases {
		for r := 0; r < evalRolls; r++ {
			jobs <- job{i, r}
		}
	}
	close(jobs)
	wait.Wait()
	return results, stopped
}

// caseRow is one case's line in the table, and the rolls it cleared.
func caseRow(c Case, rolls []rollResult) (string, int) {
	clean := 0
	got := map[string]int{}
	failed := map[string]bool{}
	rewritten := 0
	for _, res := range rolls {
		if res.verdict.Passed() {
			clean++
		}
		got[string(res.verdict.Got)]++
		for _, f := range res.verdict.Failures {
			failed[f.Check] = true
		}
		if res.rounds > 0 {
			rewritten++
		}
	}
	var classes []string
	for _, class := range []string{"none", "written", "rename", "error", "stopped", "reached"} {
		if got[class] > 0 {
			classes = append(classes, fmt.Sprintf("%s x%d", class, got[class]))
		}
	}
	if rewritten > 0 {
		classes = append(classes, fmt.Sprintf("check sent back %d", rewritten))
	}
	var checks []string
	for check := range failed {
		checks = append(checks, check)
	}
	sort.Strings(checks)
	read := fmt.Sprintf("%d of %d", clean, evalRolls)
	if got["reached"] > 0 && clean == shortRolls && clean+got["reached"] == len(rolls) {
		read = fmt.Sprintf("%d/%d (short)", clean, shortRolls)
	}
	row := fmt.Sprintf("%-46s %-8s %-7s (floor %d)  %s %s\n", c.Name, c.Expect, read,
		floorFor(c), strings.Join(classes, ", "), strings.Join(checks, ", "))
	if clean < floorFor(c) && len(rolls) > 0 {
		row += fmt.Sprintf("    wanted because: %s\n    answered: %s\n", c.Why, oneLine(rolls[0].raw))
	}
	return row, clean
}

// changedFirst puts the cases this branch added or changed ahead of the rest. A table stops at its
// first case that cannot pass, and the cases a change is about are the likeliest to.
func changedFirst(cases []Case) ([]Case, map[string]bool) {
	changed := map[string]bool{}
	for _, name := range strings.Split(os.Getenv(targetsEnv), ",") {
		for _, c := range cases {
			if name = strings.TrimSpace(name); name != "" && strings.HasPrefix(c.Name, name) {
				changed[c.Name] = true
			}
		}
	}
	base, err := exec.Command("git", "merge-base", "origin/main", "HEAD").Output()
	if err != nil {
		return cases, changed
	}
	for _, args := range [][]string{
		{"diff", "--name-only", strings.TrimSpace(string(base)), "--", casesDir},
		{"ls-files", "--others", "--exclude-standard", "--", casesDir},
	} {
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			return cases, changed
		}
		for _, path := range strings.Fields(string(out)) {
			name := path[strings.LastIndex(path, "/")+1:]
			changed[strings.TrimSuffix(name, ".case")] = true
		}
	}
	ordered := append([]Case(nil), cases...)
	sort.SliceStable(ordered, func(i, j int) bool { return changed[ordered[i].Name] && !changed[ordered[j].Name] })
	return ordered, changed
}

// targetsEnv names, by prefix and comma-separated, the cases a change touches beyond the case files it
// edits: a case whose rule paragraph the change rewrites. Those get the full read.
const targetsEnv = "WRITER_EVAL_TARGETS"

// A case whose misses leave its floor out of reach stops the table, and the rolls queued after it
// never run. A case that can still pass runs every roll.
func TestATableStopsAtTheFirstCaseThatCannotPass(t *testing.T) {
	cases := []Case{{Name: "a-passes", Expect: ExpectNone}, {Name: "b-misses", Expect: ExpectNone},
		{Name: "c-after", Expect: ExpectNone}}
	var mu sync.Mutex
	ran := map[string]int{}
	results, stopped := runTable(cases, 1, func(Case) int { return evalRolls }, func(Case) int { return evalRolls }, func(Case) bool { return false }, false, func(_ context.Context, c Case) rollResult {
		mu.Lock()
		ran[c.Name]++
		mu.Unlock()
		got := ExpectNone
		if c.Name == "b-misses" {
			got = ExpectWritten
		}
		return rollResult{verdict: Verdict{Name: c.Name, Want: ExpectNone, Got: got}}
	})
	if stopped != "b-misses" {
		t.Fatalf("stopped at %q, want b-misses", stopped)
	}
	if ran["a-passes"] != evalRolls || ran["b-misses"] != 1 || ran["c-after"] != 0 {
		t.Fatalf("ran %v: the passing case all its rolls, the missing one one roll, the next none", ran)
	}
	if _, clean := caseRow(cases[0], results[0]); clean != evalRolls {
		t.Fatalf("the passing case cleared %d of %d", clean, evalRolls)
	}
}

// A case that has reached the count the bar needs stops rolling, since the bar reads only that count.
func TestACaseStopsRollingOnceItHasTheCountTheBarNeeds(t *testing.T) {
	cases := []Case{{Name: "a", Expect: ExpectNone}}
	var mu sync.Mutex
	ran := 0
	results, _ := runTable(cases, 1, func(Case) int { return 3 }, func(Case) int { return 3 }, func(Case) bool { return false }, false, func(_ context.Context, c Case) rollResult {
		mu.Lock()
		ran++
		mu.Unlock()
		return rollResult{verdict: Verdict{Name: c.Name, Want: ExpectNone, Got: ExpectNone}}
	})
	if ran != 3 {
		t.Fatalf("%d roll(s) ran, want the 3 the bar needs", ran)
	}
	if _, clean := caseRow(cases[0], results[0]); clean != 3 {
		t.Fatalf("cleared %d, want 3", clean)
	}
}

// columnsDir holds a full table's counts per case, under the hash of the rules it measured. A column
// is measured once per rule set and read by every later table.
const columnsDir = "testdata/columns"

// needFrom is the count the bar needs of a case: its floor, or the reference column's count less two,
// whichever is higher.
func needFrom(reference map[string]int) func(Case) int {
	return func(c Case) int {
		need := floorFor(c)
		if at, known := reference[c.Name]; known && at-2 > need {
			need = at - 2
		}
		return need
	}
}

// regressionFrom is a case's regression line: the kept column's count less two, or its floor where no
// column is kept. A case under that line has regressed.
func regressionFrom(reference map[string]int) func(Case) int {
	return func(c Case) int {
		if at, known := reference[c.Name]; known {
			return max(at-2, 0)
		}
		return floorFor(c)
	}
}

// mainColumn reads the column measured on main's rules, where one is kept.
func mainColumn() map[string]int {
	var sum = sha256.New()
	for _, path := range rulePaths {
		body, err := exec.Command("git", "show", "origin/main:ai/"+strings.TrimPrefix(path, "../../")).Output()
		if err != nil {
			return nil
		}
		sum.Write(body)
	}
	return readColumn(hex.EncodeToString(sum.Sum(nil))[:12])
}

func readColumn(rules string) map[string]int {
	body, err := os.ReadFile(filepath.Join(columnsDir, rules+".json"))
	if err != nil {
		return nil
	}
	var column map[string]int
	if json.Unmarshal(body, &column) != nil {
		return nil
	}
	return column
}

// writeColumn keeps a full table's counts under the hash of the rules it measured.
func writeColumn(rules string, counts map[string]int) error {
	if err := os.MkdirAll(columnsDir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(columnsDir, rules+".json"), append(body, '\n'), 0o644)
}

// A usage limit stops the table at once, since every roll after it meets the same limit.
func TestAUsageLimitStopsTheTable(t *testing.T) {
	cases := []Case{{Name: "a", Expect: ExpectNone}, {Name: "b", Expect: ExpectNone}}
	var mu sync.Mutex
	ran := 0
	_, stopped := runTable(cases, 1, func(Case) int { return evalRolls }, func(Case) int { return evalRolls }, func(Case) bool { return false }, true, func(_ context.Context, c Case) rollResult {
		mu.Lock()
		ran++
		mu.Unlock()
		return rollResult{limit: true, raw: "claude has no capacity left", verdict: Verdict{Name: c.Name, Got: "error"}}
	})
	if stopped != "a" || ran != 1 {
		t.Fatalf("stopped at %q after %d roll(s), want a after 1", stopped, ran)
	}
}

// shortRolls is the read a proven bystander gets: a case outside the change and outside Kirill's eight,
// at 14 or 15 of 15 on the kept column. The README says why the eight and a target get the full read,
// and what a short read costs.
const shortRolls = 5

// kirillsEight are the cases every change is read on in full.
var kirillsEight = []string{"k15", "k19", "k20", "k21", "k22", "k23", "k25", "k26"}

// shortFor says which cases a table reads short: a case proven on the reference column, outside the
// change's targets and outside the eight. Without a column every case gets the full read.
func shortFor(reference map[string]int, targeted map[string]bool) func(Case) bool {
	return func(c Case) bool {
		for _, name := range kirillsEight {
			if strings.HasPrefix(c.Name, name+"-") {
				return false
			}
		}
		at, known := reference[c.Name]
		return known && at >= evalRolls-1 && !targeted[c.Name]
	}
}

// A proven bystander stops after five clean rolls and reads as short. One miss sends it to the full read.
func TestAProvenBystanderIsReadShort(t *testing.T) {
	cases := []Case{{Name: "a", Expect: ExpectNone}, {Name: "b", Expect: ExpectNone}}
	var mu sync.Mutex
	ran := map[string]int{}
	results, _ := runTable(cases, 1, func(Case) int { return evalRolls - 1 }, func(Case) int { return evalRolls - 1 }, func(Case) bool { return true }, false,
		func(_ context.Context, c Case) rollResult {
			mu.Lock()
			ran[c.Name]++
			n := ran[c.Name]
			mu.Unlock()
			got := ExpectNone
			if c.Name == "b" && n == 2 {
				got = ExpectWritten
			}
			return rollResult{verdict: Verdict{Name: c.Name, Want: ExpectNone, Got: got}}
		})
	if ran["a"] != shortRolls || ran["b"] != evalRolls {
		t.Fatalf("ran %v, want a read short and b read in full after its miss", ran)
	}
	if row, _ := caseRow(cases[0], results[0]); !strings.Contains(row, "5/5 (short)") {
		t.Fatalf("row %q, want the short read marked", row)
	}
}

// The eight, and a case the change touches, are never read short.
func TestTheEightAndATargetAreReadInFull(t *testing.T) {
	short := shortFor(map[string]int{"k19-x": 15, "k04-y": 15, "k09-z": 15, "k11-w": 12}, map[string]bool{"k09-z": true})
	for name, want := range map[string]bool{"k19-x": false, "k04-y": true, "k09-z": false, "k11-w": false} {
		if got := short(Case{Name: name}); got != want {
			t.Errorf("%s read short %v, want %v", name, got, want)
		}
	}
}

// A case already below its floor on main stops its own rolls on a miss and leaves the table running,
// since that miss belongs to main.
func TestACaseBelowItsFloorOnMainLeavesTheTableRunning(t *testing.T) {
	cases := []Case{{Name: "a", Expect: ExpectNone}, {Name: "b", Expect: ExpectNone}}
	var mu sync.Mutex
	ran := map[string]int{}
	_, stopped := runTable(cases, 1, func(Case) int { return evalRolls }, func(Case) int { return 0 },
		func(Case) bool { return false }, false, func(_ context.Context, c Case) rollResult {
			mu.Lock()
			ran[c.Name]++
			mu.Unlock()
			return rollResult{verdict: Verdict{Name: c.Name, Want: ExpectNone, Got: ExpectWritten}}
		})
	if stopped != "" || ran["a"] != 1 || ran["b"] != 1 {
		t.Fatalf("stopped %q, ran %v; want each case stopped after its miss and the table run through", stopped, ran)
	}
}
