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

// The description rule is measured as the edit lane applies it. A description keeps what a reader
// cannot get elsewhere. A sentence goes where the diff, the ticket, the stack map, a linked page, the
// title or the template already shows it, and a body's length decides no cut.
const descriptionsDir = "testdata/descriptions"

const descriptionStandard = "../../kk-flavor/standards/human-writing.md"

type descriptionCase struct {
	name, kind, expect, cut, why, context, body string
}

func loadDescriptionCases(t *testing.T) []descriptionCase {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(descriptionsDir, "*.case"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no description case under %s", descriptionsDir)
	}
	sort.Strings(paths)
	var out []descriptionCase
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		head, rest, found := strings.Cut(string(raw), "--- context\n")
		context, body, hasBody := strings.Cut(rest, "--- body\n")
		if !found || !hasBody {
			t.Fatalf("%s needs a `--- context` and a `--- body` section", path)
		}
		c := descriptionCase{name: strings.TrimSuffix(filepath.Base(path), ".case"),
			context: strings.TrimSpace(context), body: strings.TrimSpace(body)}
		for _, line := range strings.Split(strings.TrimSpace(head), "\n") {
			key, value, _ := strings.Cut(line, ":")
			value = strings.TrimSpace(value)
			switch strings.TrimSpace(key) {
			case "kind":
				c.kind = value
			case "expect":
				c.expect = value
			case "cut":
				c.cut = strings.ToLower(value)
			case "why":
				c.why = value
			}
		}
		if c.why == "" || (c.expect != "none" && c.expect != "cut") || (c.expect == "cut" && c.cut == "") {
			t.Fatalf("%s needs a reason, and an expect of none or cut with the word the cut sentence holds", path)
		}
		out = append(out, c)
	}
	return out
}

// Every description case parses, so a malformed one fails the suite before it costs a paid run.
func TestTheDescriptionCasesParse(t *testing.T) {
	if len(loadDescriptionCases(t)) < 4 {
		t.Fatal("fewer than four description cases")
	}
}

// descriptionSections is the standard's two sections a description is read against.
func descriptionSections(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(descriptionStandard)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?ms)^## Change descriptions.*?(^## Review comments|\z)`).Find(raw)
	if m == nil {
		t.Fatal("the standard holds no Change descriptions section")
	}
	return strings.TrimSuffix(string(m), "## Review comments")
}

var cutLine = regexp.MustCompile(`(?im)^\s*cut:\s*(.+?)\s*$`)

func TestDescriptionCuts(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends a model call per roll", evalEnv)
	}
	policy, err := modelpolicy.Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: "kk-edit"})
	if err != nil {
		t.Fatal(err)
	}
	rules := descriptionSections(t)
	var report strings.Builder
	failed := 0
	for _, c := range loadDescriptionCases(t) {
		text := rules + "\n\n---\n\nYou are the edit lane reading a " + c.kind + " against the rule above. You have no tools " +
			"here: what the reader already has is below. Return one line per sentence to cut, as `cut: <the sentence>`, " +
			"or the single line `none`. Return nothing else.\n\nWhat the reader already has:\n" + c.context +
			"\n\nThe " + c.kind + ":\n" + c.body + "\n"
		passed := 0
		var wait sync.WaitGroup
		var mu sync.Mutex
		var misses []string
		slots := make(chan struct{}, parallelCalls(t))
		for r := 0; r < evalRolls; r++ {
			wait.Add(1)
			slots <- struct{}{}
			go func() {
				defer wait.Done()
				defer func() { <-slots }()
				raw, err := callWriter(decision.Requested, text)
				if err != nil {
					return
				}
				cuts := cutLine.FindAllStringSubmatch(raw, -1)
				ok := len(cuts) == 0 && strings.Contains(strings.ToLower(raw), "none")
				if c.expect == "cut" {
					ok = len(cuts) == 1 && strings.Contains(strings.ToLower(cuts[0][1]), c.cut)
				}
				mu.Lock()
				defer mu.Unlock()
				if ok {
					passed++
				} else if len(misses) < 2 {
					misses = append(misses, oneLine(raw))
				}
			}()
		}
		wait.Wait()
		floor := (writtenFloor*evalRolls + 99) / 100
		if passed < floor {
			failed++
		}
		fmt.Fprintf(&report, "%-58s %d of %d (floor %d)\n", c.name, passed, evalRolls, floor)
		for _, miss := range misses {
			fmt.Fprintf(&report, "    missed: %s\n", miss)
		}
	}
	t.Logf("\n%s", report.String())
	if failed > 0 {
		t.Fail()
	}
}
