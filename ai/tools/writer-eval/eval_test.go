package writereval

import (
	"context"
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

const casesDir = "testdata/cases"

// policyPath is the checkout's own policy. The eval runs the row this checkout assigns, so a row
// changed in the same branch is the row the table reports.
const policyPath = "../../kk-flavor/models.json"

// writerRow is the policy row whose model writes comment blocks. The eval runs the row the pipeline
// runs, so a table here says what a change to the rule does to the real writer.
const writerRow = "comment-writer"

// evalEnv asks for the run. Every case spends a model call, so a suite that ran it by default would
// bill a full sweep on every push.
const evalEnv = "WRITER_EVAL"

const callDeadline = 4 * time.Minute

// labelledBar is what every labelled case has to do: land in the class its label expects, and pass
// every check where that class is a written block. The plain half of the eval, which reads reviewed
// source from the environment and bounds how many of its written blocks may fail a check, is the
// next piece and is absent here. A bar declared before the thing that measures it exists reads as
// enforced and is not.
const labelledBar = "every labelled case in its expected class"

func TestEveryCaseParsesAndNamesAClassAndAReason(t *testing.T) {
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 14 {
		t.Fatalf("%d case(s); the labelled set is fourteen blocks plus the rename", len(cases))
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.Name] {
			t.Errorf("two cases named %q", c.Name)
		}
		seen[c.Name] = true
		if c.Label == "" {
			t.Errorf("%s says no date, and a case with no provenance cannot be re-read against its review", c.Name)
		}
		if strings.Contains(c.Code, "/*") || strings.Contains(c.Code, "//") {
			t.Errorf("%s shows the writer a comment, and the strip removes every block before the writer reads", c.Name)
		}
	}
}

// The fixture is the private review translated into the ledger domain. A word from the reviewed
// codebase here would put somebody else's code in a public repository.
func TestNoCaseCarriesTheReviewedCodebasesWords(t *testing.T) {
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	barred := []string{"codec", "fairplay", "widevine", "playready", "drm", "dash", "bitmovin",
		"player", "manifest", "adaptationset", "representation", "mimetype", "cenc", "cbcs"}
	for _, c := range cases {
		body := strings.ToLower(c.Code + " " + c.Facts + " " + c.Why)
		for _, word := range barred {
			if strings.Contains(body, word) {
				t.Errorf("%s carries %q from the reviewed codebase, and this repository is public", c.Name, word)
			}
		}
	}
}

func writerSettings(t *testing.T) modelpolicy.Settings {
	t.Helper()
	policy, err := modelpolicy.Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: writerRow})
	if err != nil {
		t.Fatalf("the policy assigns no row to %q: %v", writerRow, err)
	}
	return decision.Requested
}

// prompt assembles what an isolated writer is given: the rule, the worker and the site. The files
// come from the checkout under test, so a change to either is what the next run measures.
func prompt(t *testing.T, c Case) string {
	t.Helper()
	var out strings.Builder
	for _, path := range []string{
		"../../kk-flavor/standards/code-style.md",
		"../../kk-flavor/workers/comment-writer.md",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("the eval could not read %s: %v", path, err)
		}
		fmt.Fprintf(&out, "=== %s ===\n%s\n\n", filepath.Base(path), body)
	}
	fmt.Fprintf(&out, "=== the site ===\nThe file holds this code, with every comment block already removed:\n\n"+
		"```ts\n%s\n```\n\nThe facts file for the site holds:\n\n%s\n\n", c.Code, c.Facts)
	out.WriteString("Answer with the block you would write above the declaration, or the single word none, " +
		"or a line `rename: <what to rename>`. Add your audit lines, one per line, as " +
		"`term: <phrase> — identifier|domain|plain` and `verb: <word> — literal|figure`. Answer with nothing else.")
	return out.String()
}

func callWriter(settings modelpolicy.Settings, text string) (string, error) {
	args := []string{"-p", "--model", settings.Model, "--output-format", "text",
		"--tools", "", "--setting-sources", "", "--strict-mcp-config"}
	if settings.Effort != "" {
		args = append(args, "--effort", settings.Effort)
	}
	ctx, stop := context.WithTimeout(context.Background(), callDeadline)
	defer stop()
	command := exec.CommandContext(ctx, "claude", append(args, text)...)
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("the writer row did not answer: %w", err)
	}
	return string(out), nil
}

// TestWriterEval runs the labelled set through the real writer row and prints the table. The bar is
// above, fixed before the first run.
func TestWriterEval(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends one model call per case", evalEnv)
	}
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	settings := writerSettings(t)
	at := 4
	if set := os.Getenv("WRITER_EVAL_PARALLEL"); set != "" {
		if at, err = strconv.Atoi(set); err != nil || at < 1 {
			t.Fatalf("WRITER_EVAL_PARALLEL is %q, which is not a positive count", set)
		}
	}

	verdicts := make([]Verdict, len(cases))
	answers := make([]string, len(cases))
	gate := make(chan struct{}, at)
	var wait sync.WaitGroup
	for i, c := range cases {
		wait.Add(1)
		go func(i int, c Case) {
			defer wait.Done()
			gate <- struct{}{}
			defer func() { <-gate }()
			raw, err := callWriter(settings, prompt(t, c))
			if err != nil {
				verdicts[i] = Verdict{Name: c.Name, Want: c.Expect, Got: "error"}
				answers[i] = err.Error()
				return
			}
			answers[i] = strings.TrimSpace(raw)
			verdicts[i] = Judge(c.Name, c.Expect, ParseReturn(raw))
		}(i, c)
	}
	wait.Wait()

	var out strings.Builder
	fmt.Fprintf(&out, "\nwriter row: %s %s, %d case(s)\n\n", settings.Model, settings.Effort, len(cases))
	fmt.Fprintf(&out, "%-46s %-8s %-8s %s\n", "case", "want", "got", "failed")
	passed := 0
	for i, v := range verdicts {
		if v.Passed() {
			passed++
		}
		var names []string
		for _, f := range v.Failures {
			names = append(names, f.Check)
		}
		fmt.Fprintf(&out, "%-46s %-8s %-8s %s\n", v.Name, v.Want, v.Got, strings.Join(names, ", "))
		if !v.Passed() {
			fmt.Fprintf(&out, "    wanted because: %s\n    answered: %s\n", cases[i].Why, oneLine(answers[i]))
		}
	}
	fmt.Fprintf(&out, "\n%d of %d in the expected class with every check passed. The bar is %s.\n",
		passed, len(cases), labelledBar)
	t.Log(out.String())
	if passed != len(cases) {
		t.Errorf("%d of %d labelled case(s) cleared the bar", passed, len(cases))
	}
}

func oneLine(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if len(flat) > 240 {
		return flat[:240] + "..."
	}
	return flat
}
