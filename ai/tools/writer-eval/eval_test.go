package writereval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	census "configs/ai/tools/comment-census"
	modelpolicy "configs/ai/tools/model-policy"
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

// caseEnv narrows a run to the cases whose name starts with it.
const caseEnv = "WRITER_EVAL_CASE"

// parallelEnv bounds how many model calls are in flight at once. Both halves of the eval read it, so
// a sweep is one setting instead of two that can drift.
const parallelEnv = "WRITER_EVAL_PARALLEL"

func parallelCalls(t *testing.T) int {
	t.Helper()
	set := os.Getenv(parallelEnv)
	if set == "" {
		return 4
	}
	at, err := strconv.Atoi(set)
	if err != nil || at < 1 {
		t.Fatalf("%s is %q, which is not a positive count", parallelEnv, set)
	}
	return at
}

const callDeadline = 4 * time.Minute

// evalRolls is how many times each case is put to the writer. The writer is a model, so one roll per
// case cannot tell a rule that changed the answer from a case that answers differently twice. Three
// rolls fell short too. Three cases moved by two rolls between runs, and neither run touched any of
// them. A case counts as passed only where every roll passed. The table prints the split, so a
// reader sees the variance.
const evalRolls = 5

// labelledBar is what every labelled case has to do. A step decides a none and a rename, so those
// clear every roll. The writer judges a written block, so that one clears writtenFloor.
//
// The floors came apart after the steps that fixed every none case collapsed every written one. The
// writer had moved from judging to refusing, and one number could not see that happen.
const labelledBar = "a none case on every roll, a written case on three of five"

// writtenFloor is how many rolls a case the writer judges has to clear.
const writtenFloor = 3

// An invariant's worth is the obligation it states. A run on 2026-09-21 turned "must stay in step
// with" into "copies", which leaves a reader of the changed side unaware of what they owe. Every
// part of that return was otherwise right, and the harness had no way to see the loss.
func TestABlockThatDropsTheObligationFails(t *testing.T) {
	c := Case{Name: "k08", Expect: ExpectWritten, Keeps: []string{"must match", "must stay in step"}}
	dropped := Return{Block: "// This table copies the ledger's settlement map.", Summary: PartNone, Note: PartWritten}
	if JudgeCase(c, dropped).Passed() {
		t.Errorf("a block stating what is, where the claim states what is owed, passed")
	}
	kept := Return{Block: "// This table must match the ledger's settlement map.", Summary: PartNone, Note: PartWritten}
	if v := JudgeCase(c, kept); !v.Passed() {
		t.Errorf("a block keeping the obligation failed: %v", v.Failures)
	}
}

// The block a reviewer sent back on 2026-09-21 states the mechanism twice and leaves out the callers
// that make the copy worth having. Its own words carry a removal verb. A floor asking for that verb
// passes the block, so k11 asks for the caller or the walk over the result.
func TestTheBlockThatStatesTheMechanismTwiceFails(t *testing.T) {
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	var k11 Case
	for _, c := range cases {
		if strings.HasPrefix(c.Name, "k11") {
			k11 = c
		}
	}
	if len(k11.Keeps) == 0 {
		t.Fatalf("k11 names no wording to keep, so its floor asks for nothing")
	}
	sentBack := Return{Summary: PartNone, Note: PartWritten, Block: "/**\n" +
		" * The returned array is a copy of a LiveEntryView, which stops listing an entry the book has removed.\n" +
		" * The copy still holds that entry.\n */"}
	if JudgeCase(k11, sentBack).Passed() {
		t.Errorf("the block a reviewer sent back cleared k11")
	}
	named := Return{Summary: PartNone, Note: PartWritten, Block: "/**\n" +
		" * A LiveEntryView stops listing an entry as soon as the book removes it.\n" +
		" * toEntries copies it, so a caller iterating the result still reaches every entry.\n */"}
	if v := JudgeCase(k11, named); !v.Passed() {
		t.Errorf("a block naming the caller's walk failed k11: %v", v.Failures)
	}
}

// A rule the writer reads can be rewritten, and the rewrite of 2026-09-21 reverses the fix that a
// review asked for on the site k07 holds. The case is what keeps the rejected shape from returning,
// so the field it does that with is pinned here.
func TestABlockCarryingTheBarredShapeFails(t *testing.T) {
	c := Case{Name: "k07", Expect: ExpectWritten, Bars: []string{"can choose"}}
	returned := Return{Block: "// The ledger can choose a settlement preferredSettlements leaves out.",
		Summary: PartNone, Note: PartWritten}
	if JudgeCase(c, returned).Passed() {
		t.Errorf("the shape the review rejected passed")
	}
	other := Return{Block: "// preferredSettlements, the ledger's allowed schemes, leaves a source's own scheme out.",
		Summary: PartNone, Note: PartWritten}
	if v := JudgeCase(c, other); !v.Passed() {
		t.Errorf("a block clear of the barred wording failed: %v", v.Failures)
	}
}

// Question 1 reads an exported symbol's callers, and question 3's consequence names what one does to
// the result. The writer here has no repository to grep. The callers reach it through the case and
// the prompt alone.
func TestACaseCarriesItsCallersSeparatelyFromItsCode(t *testing.T) {
	raw := "expect: written\nwhy: a site with callers\n--- code\nexport function f() {}\n" +
		"--- callers\n// helpers/Closing.ts\nf();\n--- facts\nA library drops an entry.\n"
	c, err := ParseCase("k-callers", raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.Code, "Closing.ts") {
		t.Errorf("the callers landed in the code the writer is shown: %q", c.Code)
	}
	if !strings.Contains(c.Callers, "f();") {
		t.Errorf("the callers section went missing: %q", c.Callers)
	}
	if c.Facts != "A library drops an entry." {
		t.Errorf("the facts read %q", c.Facts)
	}
	if !strings.Contains(prompt(t, c), "A grep over the repository finds these call sites") {
		t.Errorf("the prompt shows the writer no callers")
	}
	if _, err := ParseCase("k-typo", strings.Replace(raw, "--- callers", "--- calers", 1)); err == nil {
		t.Errorf("a misspelt section name parsed, and its lines would join the code")
	}
}

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
		body := strings.ToLower(strings.Join([]string{c.Code, c.Facts, c.Why, c.Tests, c.Callers}, " "))
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
	// The site is named, because a return quoting one invented `writer-eval:1` from the run's own
	// working directory when the prompt left it unsaid.
	fmt.Fprintf(&out, "=== the site ===\nThe site is `%s.ts:1`. The file holds this code, with every "+
		"comment block already removed:\n\n"+
		"```ts\n%s\n```\n\nThe facts file for the site holds:\n\n%s\n\n", c.Name, c.Code, c.Facts)
	// The same list the strip writes beside the facts, so the fixture and the lane audit against one
	// thing. A run that withheld it would measure a writer whose audit can classify no noun at all.
	fmt.Fprintf(&out, "=== identifiers.txt ===\nThe audit classifies a noun as `identifier` where it is here:\n\n%s\n\n",
		strings.Join(census.IdentifierWords(strings.Split(c.Code, "\n")), " "))
	if c.Callers != "" {
		fmt.Fprintf(&out, "=== the callers ===\nA grep over the repository finds these call sites and no "+
			"other:\n\n```ts\n%s\n```\n\n", c.Callers)
	}
	if c.Tests != "" {
		fmt.Fprintf(&out, "=== the change set's tests ===\nQuestion 3 greps these for a fact's nouns:\n\n"+
			"```ts\n%s\n```\n\n", c.Tests)
	}
	out.WriteString("You have no tools here, so apply the worker's voice check by reading rather than " +
		"by running it, and report a script you would have run as a finding you read for yourself.\n\n" +
		"Answer with a line `summary: needed` or `summary: none`, a line `note: written` or " +
		"`note: none`, then the block you would write above the declaration, or the single word none, " +
		"or a line `rename: <what to rename>`. The site is none only where both parts are none. Where " +
		"a part was needed and you answer none for it, show the attempts first, one per line, as " +
		"`attempt 1: <part> — <finding>`. Add your audit lines, one per line, as " +
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

// TestWriterEval runs the labelled set through the real writer row and prints the table. It reads
// the bar labelledBar names, which was written down before the first run.
func TestWriterEval(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends one model call per case", evalEnv)
	}
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	// One case at a time, for reading a single case's returns without spending the whole set.
	if only := os.Getenv(caseEnv); only != "" {
		var kept []Case
		for _, c := range cases {
			if strings.HasPrefix(c.Name, only) {
				kept = append(kept, c)
			}
		}
		if len(kept) == 0 {
			t.Fatalf("%s is %q, and no case name starts with it", caseEnv, only)
		}
		cases = kept
	}
	settings := writerSettings(t)
	at := parallelCalls(t)

	rolls := make([][]Verdict, len(cases))
	answers := make([][]string, len(cases))
	for i := range cases {
		rolls[i] = make([]Verdict, evalRolls)
		answers[i] = make([]string, evalRolls)
	}
	gate := make(chan struct{}, at)
	var wait sync.WaitGroup
	for i, c := range cases {
		for roll := 0; roll < evalRolls; roll++ {
			wait.Add(1)
			go func(i, roll int, c Case) {
				defer wait.Done()
				gate <- struct{}{}
				defer func() { <-gate }()
				raw, err := callWriter(settings, prompt(t, c))
				if err != nil {
					rolls[i][roll] = Verdict{Name: c.Name, Want: c.Expect, Got: "error"}
					answers[i][roll] = err.Error()
					return
				}
				answers[i][roll] = strings.TrimSpace(raw)
				rolls[i][roll] = JudgeCase(c, ParseReturn(raw))
			}(i, roll, c)
		}
	}
	wait.Wait()

	var rows []dumped
	for i, c := range cases {
		for roll, answer := range answers[i] {
			rows = append(rows, dumped{Case: c.Name, Roll: roll, Raw: answer})
		}
	}
	dumpReturns(rows)

	var out strings.Builder
	fmt.Fprintf(&out, "\nwriter row: %s %s, %d case(s), %d roll(s) each\n\n",
		settings.Model, settings.Effort, len(cases), evalRolls)
	fmt.Fprintf(&out, "%-46s %-8s %-7s %s\n", "case", "want", "passed", "what came back")
	passed := 0
	cleanByCase := map[string]int{}
	for i, c := range cases {
		clean := 0
		got := map[string]int{}
		failed := map[string]bool{}
		for _, v := range rolls[i] {
			if v.Passed() {
				clean++
			}
			got[string(v.Got)]++
			for _, f := range v.Failures {
				failed[f.Check] = true
			}
		}
		cleanByCase[c.Name] = clean
		if clean >= floorFor(c) {
			passed++
		}
		var classes []string
		for _, class := range []string{"none", "written", "rename", "error"} {
			if got[class] > 0 {
				classes = append(classes, fmt.Sprintf("%s x%d", class, got[class]))
			}
		}
		var checks []string
		for check := range failed {
			checks = append(checks, check)
		}
		sort.Strings(checks)
		fmt.Fprintf(&out, "%-46s %-8s %d of %d (floor %d)  %s %s\n", c.Name, c.Expect, clean, evalRolls,
			floorFor(c), strings.Join(classes, ", "), strings.Join(checks, ", "))
		if clean < floorFor(c) {
			fmt.Fprintf(&out, "    wanted because: %s\n    answered: %s\n", c.Why, oneLine(answers[i][0]))
		}
	}
	fmt.Fprintf(&out, "\n%d of %d cleared their floor. The bar is %s.\n", passed, len(cases), labelledBar)
	if path := os.Getenv(ledgerEnv); path != "" {
		p, err := poolInto(path, cleanByCase)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&out, "\npooled over %d run(s):\n%-46s %-12s %s\n", p.Runs, "case", "clean", "spread")
		for _, c := range cases {
			fmt.Fprintf(&out, "%-46s %3d of %-5d %d\n", c.Name, p.Clean[c.Name], p.Rolls[c.Name], p.Spread[c.Name])
		}
		fmt.Fprintf(&out, "A per-case move is attributable only above the spread this table shows.\n")
	}
	var carried []string
	for _, c := range cases {
		if c.Tests != "" {
			carried = append(carried, c.Name)
		}
	}
	fmt.Fprintf(&out, "%d case(s) carry the change set's tests, so question 3 can reach them: %s.\n",
		len(carried), strings.Join(carried, ", "))
	fmt.Fprintf(&out, "A case whose label says carried by a test and which carries none is unmeasurable, "+
		"because the writer greps nothing and keeps the claim.\n")
	fmt.Fprintf(&out, "k05 is a judgement case: the coined-identifier check reads prose, and that "+
		"compound sits only in the identifier.\n")
	fmt.Fprintf(&out, "Watch: k01 has come back as a rename, on a site carrying no compound at all.\n")
	fmt.Fprintf(&out, "Earlier tables understated every carried-by case: the score read a return's "+
		"`carried by` line as block prose until 2026-09-21.\n")
	t.Log(out.String())
	if passed != len(cases) {
		t.Errorf("%d of %d labelled case(s) cleared their floor", passed, len(cases))
	}
}

func oneLine(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if len(flat) > 240 {
		return flat[:240] + "..."
	}
	return flat
}

// ledgerEnv names a file this eval appends each run's per-case counts to. A per-case count at five
// rolls still carries noise. A change is attributable where it is larger than the spread the ledger
// shows across runs. Unset, the run reports itself alone.
// dumpEnv names a file every raw return is appended to. A decline is classified from the writer's
// own words, and a run that keeps only the verdict has thrown those away.
const dumpEnv = "WRITER_EVAL_DUMP"

type dumped struct {
	Case string `json:"case"`
	Roll int    `json:"roll"`
	Raw  string `json:"raw"`
}

// dumpReturns appends this run's returns. It is best effort: a run that cannot write the dump still
// reports its table, because the table is the thing the bar reads.
func dumpReturns(rows []dumped) {
	path := os.Getenv(dumpEnv)
	if path == "" {
		return
	}
	var held []dumped
	if body, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(body, &held)
	}
	held = append(held, rows...)
	if body, err := json.MarshalIndent(held, "", " "); err == nil {
		_ = os.WriteFile(path, body, 0o644)
	}
}

const ledgerEnv = "WRITER_EVAL_LEDGER"

type pooled struct {
	Runs   int            `json:"runs"`
	Clean  map[string]int `json:"clean"`
	Rolls  map[string]int `json:"rolls"`
	Spread map[string]int `json:"spread"`
}

// poolInto adds this run's counts to the ledger and returns what every run so far has seen. Spread is
// the widest gap between one run's clean count for a case and another's, which is the number a reader
// compares a change against.
func poolInto(path string, clean map[string]int) (pooled, error) {
	out := pooled{Clean: map[string]int{}, Rolls: map[string]int{}, Spread: map[string]int{}}
	var runs []map[string]int
	if body, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(body, &runs); err != nil {
			return out, fmt.Errorf("%s does not parse, so the pooled count cannot be trusted: %w", path, err)
		}
	}
	runs = append(runs, clean)
	body, err := json.MarshalIndent(runs, "", " ")
	if err != nil {
		return out, err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return out, err
	}
	out.Runs = len(runs)
	for name := range clean {
		low, high := evalRolls, 0
		for _, run := range runs {
			count, seen := run[name]
			if !seen {
				continue
			}
			out.Clean[name] += count
			out.Rolls[name] += evalRolls
			low, high = min(low, count), max(high, count)
		}
		if high >= low {
			out.Spread[name] = high - low
		}
	}
	return out, nil
}

// plainSetEnv names a directory of reviewed source whose blocks a reviewer left alone. It is
// somebody else's code and this repository is public. So the environment names the set, a case is
// named by its position in the sorted set, and only counts leave the run.
const plainSetEnv = "JUDGE_EVAL_PLAIN"

// plainBlocksEnv bounds how many blocks the plain half reads, because each one spends a model call.
const plainBlocksEnv = "WRITER_EVAL_PLAIN_BLOCKS"
const plainBlockBudget = 20

// plainFailureBound is the share of written blocks that may fail a check. plainWriteFloor is the
// share of the set the writer has to write for at all. The bound alone scored what was written, so a
// writer answering none everywhere cleared it with an empty numerator. Both numbers were written
// down before the run that reads them.
const plainFailureBound = 0.10
const plainWriteFloor = 0.50

var plainExtensions = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".go": true}

// plainCases turns a reviewed file's blocks into sites: the declaration under the block as the code,
// and the block's own text as the facts file. A case's name is its position in the sorted set.
func plainCases(t *testing.T) []Case {
	t.Helper()
	dir := os.Getenv(plainSetEnv)
	if dir == "" {
		t.Skipf("%s names no directory, so the plain half has not measured anything", plainSetEnv)
	}
	var paths []string
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && plainExtensions[strings.ToLower(filepath.Ext(path))] {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("%s names a directory the run could not read: %v", plainSetEnv, err)
	}
	sort.Strings(paths)
	budget := plainBlockBudget
	if set := os.Getenv(plainBlocksEnv); set != "" {
		parsed, err := strconv.Atoi(set)
		if err != nil || parsed < 1 {
			t.Fatalf("%s is %q, which is not a positive count", plainBlocksEnv, set)
		}
		budget = parsed
	}
	var cases []Case
	for at, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("a file in the set could not be read: %v", err)
		}
		lines := strings.Split(string(body), "\n")
		for n, b := range census.Blocks(lines) {
			if !b.OverDecl || len(cases) >= budget {
				continue
			}
			code := declarationUnder(lines, b)
			if code == "" {
				continue
			}
			cases = append(cases, Case{
				Name:   fmt.Sprintf("plain/%03d/%d", at+1, n+1),
				Expect: ExpectWritten,
				Why:    "a block a reviewer left alone",
				Code:   code,
				Facts:  b.Text,
			})
		}
	}
	if len(cases) == 0 {
		t.Fatalf("%s names a directory holding no block over a declaration", plainSetEnv)
	}
	return cases
}

// declarationUnder is the declaration a block sits on and its body, which is what the writer reads
// once the strip has taken the block away.
func declarationUnder(lines []string, b census.Block) string {
	at := b.Line + b.Span
	for at <= len(lines) && strings.TrimSpace(lines[at-1]) == "" {
		at++
	}
	if at > len(lines) {
		return ""
	}
	var out []string
	depth, seenBrace := 0, false
	for i := at; i <= len(lines) && i < at+40; i++ {
		out = append(out, lines[i-1])
		depth += strings.Count(lines[i-1], "{") - strings.Count(lines[i-1], "}")
		if strings.Contains(lines[i-1], "{") {
			seenBrace = true
		}
		if seenBrace && depth <= 0 {
			break
		}
		if !seenBrace && strings.HasSuffix(strings.TrimSpace(lines[i-1]), ";") {
			break
		}
	}
	return strings.Join(out, "\n")
}

// The plain half. A block a reviewer left alone is one the writer may decline or may write again, and
// what it writes has to pass the same checks the labelled set uses. The bound is on the written ones
// alone: a decline is a judgement this half does not score.
func TestWriterEvalOverThePlainSet(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends a model call per block", evalEnv)
	}
	cases := plainCases(t)
	settings := writerSettings(t)
	at := parallelCalls(t)
	verdicts := make([]Verdict, len(cases))
	raws := make([]string, len(cases))
	parts := make([]Return, len(cases))
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
				verdicts[i] = Verdict{Name: c.Name, Got: "error"}
				raws[i] = err.Error()
				return
			}
			raws[i] = strings.TrimSpace(raw)
			parsed := ParseReturn(raw)
			parts[i] = parsed
			verdicts[i] = JudgeCase(c, parsed)
		}(i, c)
	}
	wait.Wait()
	var rows []dumped
	for i, c := range cases {
		rows = append(rows, dumped{Case: c.Name, Raw: raws[i]})
	}
	dumpReturns(rows)

	written, failed, declined := 0, 0, 0
	byCheck := map[string]int{}
	var names []string
	for _, v := range verdicts {
		switch v.Got {
		case ExpectWritten:
			written++
			if len(v.Failures) > 0 {
				failed++
				names = append(names, v.Name)
				for _, f := range v.Failures {
					byCheck[f.Check]++
				}
			}
		case ExpectNone, ExpectRename:
			declined++
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\nplain half: %d block(s) a reviewer left alone, %d written, %d declined\n",
		len(cases), written, declined)
	if written > 0 {
		fmt.Fprintf(&out, "%d of %d written block(s) failed a check, which is %.0f%% against a bound of %.0f%%\n",
			failed, written, float64(failed)/float64(written)*100, plainFailureBound*100)
	}
	rate := float64(written) / float64(len(cases))
	fmt.Fprintf(&out, "the writer wrote for %.0f%% of the set, against a floor of %.0f%%\n",
		rate*100, plainWriteFloor*100)
	// The parts split. A site is none only where both are. This line exists to show a note lost to
	// the summary's verdict.
	summaries, notes := 0, 0
	for _, r := range parts {
		if r.Summary == PartWritten {
			summaries++
		}
		if r.Note == PartWritten {
			notes++
		}
	}
	fmt.Fprintf(&out, "parts: %d summary(ies) and %d note(s) written over %d site(s)\n",
		summaries, notes, len(cases))
	var checks []string
	for check := range byCheck {
		checks = append(checks, fmt.Sprintf("%s %d", check, byCheck[check]))
	}
	sort.Strings(checks)
	fmt.Fprintf(&out, "checks: %s\ncases: %s\n", strings.Join(checks, ", "), strings.Join(names, ", "))
	t.Log(out.String())
	if written > 0 && float64(failed)/float64(written) > plainFailureBound {
		t.Errorf("%d of %d written block(s) failed a check, over the bound of %.0f%%",
			failed, written, plainFailureBound*100)
	}
	if rate < plainWriteFloor {
		t.Errorf("the writer wrote for %d of %d block(s) a reviewer kept, under the floor of %.0f%%",
			written, len(cases), plainWriteFloor*100)
	}
}

// floorFor is how many rolls a case has to clear. A step decides a none and a rename, so those clear
// every roll. The writer judges a written block, so that one clears writtenFloor. A case naming its
// own floor is one no step reaches, and it clears that many.
func floorFor(c Case) int {
	if c.Floor > 0 {
		return c.Floor
	}
	if c.Expect == ExpectWritten {
		return writtenFloor
	}
	return evalRolls
}
