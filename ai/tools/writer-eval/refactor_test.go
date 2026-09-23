package writereval

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	modelpolicy "configs/ai/tools/model-policy"
)

// The refactor lane's comment verdicts, measured the way the writer is. The lane's fixture file holds
// prose probes with no runner. A probe that asked for a verdict and left the tools unsaid got a model
// trying to run them.
const refactorCasesDir = "testdata/refactor"

var refactorBriefPaths = []string{
	"../../kk-flavor/standards/skill-protocol.md",
	"../../kk-flavor/workers/refactor.md",
}

const refactorCodeStyle = "../../kk-flavor/standards/code-style.md"

// refactorCase is one block the lane gives a verdict on: the verdict it has to open with, a shape it
// may not, and the context a real run would hold.
type refactorCase struct {
	name, expect, bars, why, context, code string
}

func loadRefactorCases(t *testing.T) []refactorCase {
	t.Helper()
	entries, err := os.ReadDir(refactorCasesDir)
	if err != nil {
		t.Fatal(err)
	}
	var out []refactorCase
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".case") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(refactorCasesDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		head, rest, found := strings.Cut(string(raw), "--- context\n")
		context, code, hasCode := strings.Cut(rest, "--- code\n")
		if !found || !hasCode {
			t.Fatalf("%s needs a `--- context` and a `--- code` section", entry.Name())
		}
		c := refactorCase{name: strings.TrimSuffix(entry.Name(), ".case"), context: strings.TrimSpace(context),
			code: strings.TrimSpace(code)}
		for _, line := range strings.Split(strings.TrimSpace(head), "\n") {
			key, value, _ := strings.Cut(line, ":")
			switch strings.TrimSpace(key) {
			case "expect":
				c.expect = strings.ToLower(strings.TrimSpace(value))
			case "bars":
				c.bars = strings.ToLower(strings.TrimSpace(value))
			case "why":
				c.why = strings.TrimSpace(value)
			}
		}
		if c.expect == "" || c.why == "" {
			t.Fatalf("%s says no expected verdict or no reason", entry.Name())
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// Every refactor case parses, so a malformed one fails the suite before it costs a paid run.
func TestTheRefactorCasesParse(t *testing.T) {
	if len(loadRefactorCases(t)) == 0 {
		t.Fatal("no refactor case")
	}
}

// verdictOpens is the first verdict a return names, wherever on its line it stands. The lane often
// writes a path or a backtick before it. A reader of a line's opening took ten of fifteen `carried by`
// returns for no verdict.
var verdictOpens = regexp.MustCompile(`(?i)(carried by|stays:|blocked:|for the pr body)`)

func refactorPrompt(t *testing.T, c refactorCase) string {
	t.Helper()
	var parts []string
	for _, path := range refactorBriefPaths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, string(body))
	}
	style, err := os.ReadFile(refactorCodeStyle)
	if err != nil {
		t.Fatal(err)
	}
	if comments := regexp.MustCompile(`(?ms)^## Comments\n.*?(^## |\z)`).Find(style); comments != nil {
		parts = append(parts, string(comments))
	}
	parts = append(parts, "You are running as a spawned stage with no interactive user. You have no tools here: "+
		"everything a run would read is below. Return the verdict line for the one comment block below, "+
		"exactly as the Verdict section shapes it, and nothing else.\n\n"+c.context+"\n\n```ts\n"+c.code+"\n```")
	return strings.Join(parts, "\n\n---\n\n")
}

func TestRefactorVerdicts(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends a model call per roll", evalEnv)
	}
	policy, err := modelpolicy.Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: "refactor"})
	if err != nil {
		t.Fatal(err)
	}
	cases := loadRefactorCases(t)
	var report strings.Builder
	failed := 0
	for _, c := range cases {
		text := refactorPrompt(t, c)
		got := make([]string, evalRolls)
		raws := make([]string, evalRolls)
		var wait sync.WaitGroup
		slots := make(chan struct{}, parallelCalls(t))
		for r := range got {
			wait.Add(1)
			slots <- struct{}{}
			go func(r int) {
				defer wait.Done()
				defer func() { <-slots }()
				raw, err := callWriter(decision.Requested, text)
				raws[r] = raw
				if err != nil {
					got[r] = "error"
					return
				}
				if m := verdictOpens.FindStringSubmatch(raw); m != nil {
					got[r] = strings.ToLower(strings.TrimSuffix(m[1], ":"))
				} else {
					got[r] = "unshaped"
				}
			}(r)
		}
		wait.Wait()
		passed, tally := 0, map[string]int{}
		for _, verdict := range got {
			tally[verdict]++
			if strings.HasPrefix(verdict, c.expect) && (c.bars == "" || !strings.HasPrefix(verdict, c.bars)) {
				passed++
			}
		}
		floor := (writtenFloor*evalRolls + 99) / 100
		if passed < floor {
			failed++
		}
		fmt.Fprintf(&report, "%-50s %d of %d (floor %d)  %v\n", c.name, passed, evalRolls, floor, tally)
		// A failing roll's own words, one line each. The tally says only which shape it missed by.
		for r, verdict := range got {
			if strings.HasPrefix(verdict, c.expect) && (c.bars == "" || !strings.HasPrefix(verdict, c.bars)) {
				continue
			}
			fmt.Fprintf(&report, "    %s: %s\n", verdict, oneLine(raws[r]))
		}
	}
	t.Logf("\nrefactor row: %s, %d roll(s) each\n%s", decision.Requested.Model, evalRolls, report.String())
	if failed > 0 {
		t.Fail()
	}
}
