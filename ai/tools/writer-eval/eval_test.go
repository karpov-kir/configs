package writereval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	census "configs/ai/tools/comment-census"
	modelpolicy "configs/ai/tools/model-policy"
	readerjudge "configs/ai/tools/reader-judge"
)

const casesDir = "testdata/cases"

// policyPath is the checkout's own policy. The eval runs the row this checkout assigns, so a row
// changed in the same branch is the row the table reports.
const policyPath = "../../kk-flavor/configs/models.json"

// writerRow is the policy row whose model writes comment blocks. The eval runs the row the pipeline
// runs, so a table here says what a change to the rule does to the real writer.
const writerRow = "comment-writer"

// evalEnv asks for the run. Every case spends a model call, so a suite that ran it by default would
// bill a full sweep on every push.
const evalEnv = "WRITER_EVAL"

// fullEnv runs every roll of every case and keeps the table as its rules' column. A bar reads later
// tables against that column, and it is measured once per rule set.
const fullEnv = "WRITER_EVAL_FULL"

// caseEnv narrows a run to the cases whose name starts with one of its comma-separated prefixes. A
// change is read on the cases it touches.
const caseEnv = "WRITER_EVAL_CASE"

// parallelEnv bounds how many model calls are in flight at once. Both halves of the eval read it, so
// a sweep is one setting instead of two that can drift.
const parallelEnv = "WRITER_EVAL_PARALLEL"

func parallelCalls(t *testing.T) int {
	t.Helper()
	set := os.Getenv(parallelEnv)
	if set == "" {
		return defaultWorkers
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
var evalRolls, rollsProblem = rollsFromEnv(os.Getenv(rollsEnv))

// defaultRolls is fifteen because five cannot read this set. A run on 690b548, with the tree
// untouched, gave l01 6 of 15, l03 5 of 15 and k03 6 of 15. The same three cases had read 5 of 5,
// 3 of 5 and 4 of 5 that day, so five rolls printed a coin as certainty.
const defaultRolls = 15

// rollsEnv sets the roll count for one run. Five rolls cannot separate a rule that moved an answer
// from a case that answers differently twice. l03 came back 0 of 5 and then 4 of 5 over two runs
// whose rule text differed by one sentence, and that case reads neither. A question about what a
// rule did to a case is asked at a roll count that can answer it.
const rollsEnv = "WRITER_EVAL_ROLLS"

// rollsFromEnv reads the roll count, and returns the reason where the value carries no positive
// number. A silent fall back to five prints a table a reader places against the wrong run.
func rollsFromEnv(set string) (int, string) {
	if set == "" {
		return defaultRolls, ""
	}
	count, err := strconv.Atoi(set)
	if err != nil || count < 1 {
		return defaultRolls, fmt.Sprintf("%s is %q, which is not a positive count", rollsEnv, set)
	}
	return count, ""
}

// labelledBar is what every labelled case has to do. A step decides a none and a rename, so those
// clear every roll. The writer judges a written block, so that one clears writtenFloor.
//
// The floors came apart after the steps that fixed every none case collapsed every written one. The
// writer had moved from judging to refusing, and one number could not see that happen.
const labelledBar = "a none case on every roll, a written case on three of five"

// writtenFloor is the share of rolls a case the writer judges has to clear. Three of five was the
// number before floors became shares.
const writtenFloor = 60

// An invariant's worth is the obligation it states. A run on 2026-09-21 turned "must stay in step
// with" into "copies", which leaves a reader of the changed side unaware of what they owe. Every
// part of that return was otherwise right, and the harness had no way to see the loss.
func TestABlockThatDropsTheObligationFails(t *testing.T) {
	c := Case{Name: "k08", Expect: ExpectWritten, Keeps: [][]string{{"must match", "must stay in step"}}}
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

// A run that quietly fell back to five rolls would print a table a reader could not place against
// another run's.
func TestARollCountThatNamesNoNumberIsReported(t *testing.T) {
	if count, problem := rollsFromEnv(""); count != defaultRolls || problem != "" {
		t.Errorf("an unset value gave %d rolls and %q", count, problem)
	}
	if count, problem := rollsFromEnv("15"); count != 15 || problem != "" {
		t.Errorf("15 gave %d rolls and %q", count, problem)
	}
	for _, bad := range []string{"0", "-3", "many"} {
		count, problem := rollsFromEnv(bad)
		if problem == "" {
			t.Errorf("%q passed as a roll count, giving %d", bad, count)
		}
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

// The fixture is the private review translated into the ledger domain. The rule it enforces is
// ai/kk-flavor/standards/ecosystem.md -> No outside names.
// ownNames reads the owner and the organisation off what this machine already holds: the owner in the
// remote's URL, and the organisation the CLI is signed into. The guard spells neither, because a list
// written here would put that name in a public repository.
func ownNames(remote, status string) []string {
	var found []string
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return
		}
		found = append(found, name)
		if joined := strings.Join(strings.Fields(name), "-"); joined != name {
			found = append(found, joined)
		}
	}
	if m := regexp.MustCompile(`[:/]([^/:]+)/[^/]+?(\.git)?\s*$`).FindStringSubmatch(remote); m != nil {
		add(m[1])
	}
	var signed struct {
		Org string `json:"orgName"`
	}
	if json.Unmarshal([]byte(status), &signed) == nil {
		add(signed.Org)
	}
	return found
}

// The names come from a fake remote and a fake status, so the guard is read without the machine's own.
func TestOwnNamesComeFromTheRemoteAndTheSignedInOrganisation(t *testing.T) {
	got := ownNames("git@example.invalid:Acme-Owner/ledger.git\n", `{"orgName":"Acme Corp"}`)
	want := []string{"acme-owner", "acme corp", "acme-corp"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names %q, want %q", got, want)
	}
	if got := ownNames("", "not json"); len(got) != 0 {
		t.Fatalf("names %q from nothing", got)
	}
}

func TestNoCaseCarriesTheReviewedCodebasesWords(t *testing.T) {
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	barred := []string{"codec", "fairplay", "widevine", "playready", "drm", "dash",
		"player", "manifest", "adaptationset", "representation", "mimetype", "cenc", "cbcs"}
	remote, _ := exec.Command("git", "remote", "get-url", "origin").Output()
	status, _ := exec.Command("claude", "auth", "status", "--json").Output()
	names := ownNames(string(remote), string(status))
	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "writer-eval: no remote owner and no signed-in organisation here, so no own name is barred")
	}
	barred = append(barred, names...)
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

// rulePaths are the two files the writer writes to. A run measures the text they held when it
// started.
var rulePaths = []string{
	"../../kk-flavor/standards/code-style.md",
	"../../kk-flavor/workers/comment-writer.md",
}

type ruleFile struct {
	path string
	body string
}

var (
	ruleOnce  sync.Once
	ruleHeld  []ruleFile
	ruleSum   string
	ruleError error
)

// readRules reads the rule files once for the whole run. The prompt read them off disk on every
// roll. An edit made while a run was going then reached the rolls after it, and the table named the
// text it had measured nowhere. A pair of runs on 2026-09-22 took six hours between them, and
// neither rule file could be touched for the whole of it.
func readRules(t *testing.T) []ruleFile {
	t.Helper()
	ruleOnce.Do(func() {
		sum := sha256.New()
		for _, path := range rulePaths {
			body, err := os.ReadFile(path)
			if err != nil {
				ruleError = err
				return
			}
			ruleHeld = append(ruleHeld, ruleFile{path: path, body: string(body)})
			sum.Write(body)
		}
		ruleSum = hex.EncodeToString(sum.Sum(nil))[:12]
	})
	if ruleError != nil {
		t.Fatalf("the eval could not read a rule file: %v", ruleError)
	}
	return ruleHeld
}

// reHyphenatedName is a name spelled with hyphens, such as a string value or a file's name.
var reHyphenatedName = regexp.MustCompile(`[A-Za-z0-9]+(?:-[A-Za-z0-9]+)+`)

// hyphenatedNames is each hyphenated name the code spells, in both cases. The strip writes them into
// identifiers.txt from the tree, and a case's list lacked them until 2026-09-24. Six of k06's rolls
// then returned a string value of the code as a coined word to rename.
func hyphenatedNames(code string) []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range reHyphenatedName.FindAllString(code, -1) {
		for _, spelled := range []string{name, strings.ToLower(name)} {
			if !seen[spelled] {
				seen[spelled] = true
				out = append(out, spelled)
			}
		}
	}
	return out
}

// numbered puts a line number in front of each line. The writer answers with the number of the
// declaration it chose, and a file it reads unnumbered leaves it guessing at one.
func numbered(code string) string {
	var out strings.Builder
	for n, line := range strings.Split(code, "\n") {
		fmt.Fprintf(&out, "%d\t%s\n", n+1, line)
	}
	return strings.TrimRight(out.String(), "\n")
}

// prompt assembles what an isolated writer is given: the rule, the worker and the site. The files
// come from the checkout under test, so a change to either is what the next run measures.
func prompt(t *testing.T, c Case) string {
	t.Helper()
	var out strings.Builder
	for _, held := range readRules(t) {
		fmt.Fprintf(&out, "=== %s ===\n%s\n\n", filepath.Base(held.path), held.body)
	}
	// The site is named, because a return quoting one invented `writer-eval:1` from the run's own
	// working directory when the prompt left it unsaid. The lines are numbered because the writer
	// answers with the line it put the block on.
	fmt.Fprintf(&out, "=== the site ===\nThe strip offers the site `%s.ts:%d`. The file holds this code, "+
		"with every comment block already removed and every line numbered:\n\n```ts\n%s\n```\n\n"+
		"The facts file for the site holds:\n\n%s\n\n", c.Name, c.Site, numbered(c.Code), c.Facts)
	// The same list the strip writes beside the facts, so the fixture and the lane audit against one
	// thing. A run that withheld it would measure a writer whose audit can classify no noun at all. The
	// callers' and tests' names stay out: with them in, k26 fell from 15 of 15 to 9, each miss a rename
	// of a name the list did not hold.
	fmt.Fprintf(&out, "=== identifiers.txt ===\n%s\n\n",
		strings.Join(append(census.IdentifierWords(strings.Split(c.Code, "\n")), hyphenatedNames(c.Code)...), " "))
	if c.Callers != "" {
		fmt.Fprintf(&out, "=== the callers ===\nA grep over the repository finds these call sites and no "+
			"other:\n\n```ts\n%s\n```\n\n", c.Callers)
	}
	if c.Tests != "" {
		fmt.Fprintf(&out, "=== the change set's tests ===\n"+
			"```ts\n%s\n```\n\n", c.Tests)
	}
	// The brief says where a block goes, when a site is none, how attempts are shown and what the check
	// is. The prompt restated each in its own words until 2026-09-24, and a restatement is text the
	// pipeline's writer never reads. What stays is what the brief cannot say: that the harness runs the
	// check, and the shape it parses. Told only that it had no tools, the writer reported the check it
	// could not run as a code-review finding, in four rolls of item 20's table.
	out.WriteString("You have no tools here, and the harness runs the record check over your answer. Answer with a line `summary: needed` or `summary: none`, a line " +
		"`note: written` or `note: none`, a line `at: <line number>` naming the declaration the block sits " +
		"on, then the block, or the single word none, or a line `rename: <what to rename>`. Where you write " +
		"a note, give its record first, one slot per line: `fact:`, `bears_on:` and `does:`. Add the " +
		"attempt lines, the audit lines and the routed lines in the shapes the brief gives. Answer with " +
		"nothing else.")
	return out.String()
}

// writerOf is the writer row as one call per turn.
func writerOf(settings modelpolicy.Settings) writerCall {
	return func(text string) (string, error) { return callWriter(settings, text) }
}

// callWriter puts the text to the row's model through reader-judge's caller, which every programmatic
// call here uses. It tallies which model answered. An account can serve another model than the row
// asks for, and only the call's own report says so.
func callWriter(settings modelpolicy.Settings, text string) (string, error) {
	return readerjudge.ClaudeCallerObserved(callDeadline, settings, servedModels.add)(text, "")
}

// tally counts the models that answered a run's calls, against the model each asked for.
type tally struct {
	mu     sync.Mutex
	counts map[string]int
}

var servedModels = &tally{counts: map[string]int{}}

// reset empties the tally, so a table's header counts only that table's calls.
func (t *tally) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts = map[string]int{}
}

func (t *tally) add(s readerjudge.Served) {
	t.mu.Lock()
	defer t.mu.Unlock()
	answered := s.Answered
	if answered == "" {
		answered = "a model the reply did not name"
	}
	t.counts["requested "+s.Requested+", served "+answered]++
}

// line is the tally as a table header prints it. A table names the model it measured: the model that
// answered.
func (t *tally) line() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var parts []string
	for pair, n := range t.counts {
		parts = append(parts, fmt.Sprintf("%s ×%d", pair, n))
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "no call answered"
	}
	return strings.Join(parts, "; ")
}

// TestWriterEval runs the labelled set through the real writer row and prints the table. It reads
// the bar labelledBar names, which was written down before the first run.
func TestWriterEval(t *testing.T) {
	servedModels.reset()
	if rollsProblem != "" {
		t.Fatal(rollsProblem)
	}
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
			for _, prefix := range strings.Split(only, ",") {
				if prefix = strings.TrimSpace(prefix); prefix != "" && strings.HasPrefix(c.Name, prefix) {
					kept = append(kept, c)
					break
				}
			}
		}
		if len(kept) == 0 {
			t.Fatalf("%s is %q, and no case name starts with it", caseEnv, only)
		}
		cases = kept
	}
	settings := writerSettings(t)
	cases, targeted := changedFirst(cases)
	prof := profile{workers: parallelCalls(t), started: time.Now()}
	full := os.Getenv(fullEnv) != ""
	reference := mainColumn()
	need := needFrom(reference)
	asked := map[string]string{}
	for _, c := range cases {
		asked[c.Name] = askedSum(prompt(t, c))
	}
	resume := &resumed{}
	if os.Getenv(resumeEnv) != "" {
		resume = resumeFrom(os.Getenv(dumpEnv), ruleSum, func(name string) string { return asked[name] })
	}
	results, stopped := runTable(cases, parallelCalls(t), need, regressionFrom(reference), shortFor(reference, targeted), full, func(ctx context.Context, c Case) rollResult {
		if kept, ok := resume.take(c.Name); ok {
			return kept
		}
		return rollOnce(ctx, settings, c, prompt(t, c), &prof)
	})

	var rows []dumped
	for i, c := range cases {
		for roll, res := range results[i] {
			if res.reused {
				continue
			}
			row := dumped{Case: c.Name, Roll: roll, Raw: res.raw, Findings: res.findings}
			if !res.stopped && !res.limit && res.verdict.Got != "error" {
				verdict := res.verdict
				row.Rules, row.Asked, row.Verdict, row.Rounds = ruleSum, asked[c.Name], &verdict, res.rounds
			}
			rows = append(rows, row)
		}
	}
	dumpReturns(rows)
	prof.wall = time.Since(prof.started)

	var out strings.Builder
	fmt.Fprintf(&out, "\nwriter row: %s %s, %d case(s), %d roll(s) each\n\n",
		settings.Model, settings.Effort, len(cases), evalRolls)
	fmt.Fprintf(&out, "rules read once at %s\n", ruleSum)
	fmt.Fprintf(&out, "%s\n", servedModels.line())
	fmt.Fprintf(&out, "%s\n", prof.String())
	if resume.taken > 0 {
		fmt.Fprintf(&out, "resumed %d roll(s) from %s\n", resume.taken, os.Getenv(dumpEnv))
	}
	if stopped != "" {
		fmt.Fprintf(&out, "stopped early: %s cannot reach the count the bar needs, so the rolls after it did not run\n", stopped)
	}
	fmt.Fprintf(&out, "%-46s %-8s %-7s %s\n", "case", "want", "passed", "what came back")
	passed := 0
	cleanByCase := map[string]int{}
	for i, c := range cases {
		row, clean := caseRow(c, results[i])
		cleanByCase[c.Name] = clean
		if clean >= need(c) {
			passed++
		}
		out.WriteString(row)
	}
	fmt.Fprintf(&out, "\n%d of %d cleared their floor. The bar is %s.\n", passed, len(cases), labelledBar)
	if reference == nil {
		fmt.Fprintf(&out, "no column is kept for main's rules, so each case needed its floor alone\n")
	}
	// A column holds every case, so a table narrowed to some of them keeps none.
	if full && stopped == "" && os.Getenv(caseEnv) == "" {
		if err := writeColumn(ruleSum, cleanByCase); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&out, "kept this table as the column for rules %s\n", ruleSum)
	}
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
	// Findings is what the record check printed at each turn it sent the block back, so a roll that
	// recovered still shows what the check refused.
	Findings [][]string `json:"findings,omitempty"`
	// Rules, Asked, Verdict and Rounds let a later run resume from this roll. A roll that landed no
	// verdict carries none of them.
	Rules   string   `json:"rules,omitempty"`
	Asked   string   `json:"asked,omitempty"`
	Verdict *Verdict `json:"verdict,omitempty"`
	Rounds  int      `json:"rounds,omitempty"`
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
// somebody else's code, and ai/kk-flavor/standards/ecosystem.md -> No outside names keeps its
// names out of this tree. So the environment names the set, a case is named by its position in
// the sorted set, and only counts leave the run.
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
	servedModels.reset()
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
			parsed, raw, err := writeChecked(writerOf(settings), recordCheckFor(), prompt(t, c), c.Code)
			if err != nil {
				verdicts[i] = Verdict{Name: c.Name, Got: "error"}
				raws[i] = err.Error()
				return
			}
			raws[i] = strings.TrimSpace(raw)
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
	share := 100
	if c.Floor > 0 {
		share = c.Floor
	} else if c.Expect == ExpectWritten {
		share = writtenFloor
	}
	// Rounded up, so a share never asks for less than it says.
	at := (share*evalRolls + 99) / 100
	if at < 1 {
		at = 1
	}
	return at
}

// A floor written as a count was a share of five rolls that no case wrote down as one. The day the
// roll count went to fifteen, each floor asked for a third of the day before, and the table stayed
// quiet about it.
func TestAFloorAsksTheSameShareAtAnyRollCount(t *testing.T) {
	held := evalRolls
	defer func() { evalRolls = held }()
	written := Case{Expect: ExpectWritten}
	none := Case{Expect: ExpectNone}
	forty := Case{Expect: ExpectWritten, Floor: 40}
	for _, rolls := range []int{5, 15, 30} {
		evalRolls = rolls
		if at, want := floorFor(written), (writtenFloor*rolls+99)/100; at != want {
			t.Errorf("at %d rolls the written floor is %d, want %d", rolls, at, want)
		}
		if at := floorFor(none); at != rolls {
			t.Errorf("at %d rolls a none case clears %d, and the bar is every roll", rolls, at)
		}
		if at, want := floorFor(forty), (40*rolls+99)/100; at != want {
			t.Errorf("at %d rolls a 40%% floor is %d, want %d", rolls, at, want)
		}
	}
	evalRolls = 5
	if at := floorFor(forty); at != 2 {
		t.Errorf("40%% of five rolls is %d, and the cases were written as 2 of 5", at)
	}
	evalRolls = 15
	if at := floorFor(forty); at != 6 {
		t.Errorf("40%% of fifteen rolls is %d, want 6", at)
	}
	// A count left over from the days of five rolls reads as 3% of them here, which is no floor at all.
	for _, bad := range []string{"0", "101", "150", "none"} {
		if _, err := ParseCase("k-floor", "expect: written\nwhy: a floor\nfloor: "+bad+
			"\n--- code\nexport function f() {}\n--- facts\nA library drops an entry.\n"); err == nil {
			t.Errorf("a floor of %q parsed, and a share runs from 1 to 100", bad)
		}
	}
}

// The prompt read both rule files off disk every time. An edit made while a run was going then
// reached the rolls after it. A run at fifteen rolls takes long enough that somebody will want to edit a rule
// beside it.
func TestTheRuleFilesAreReadOnceForTheWholeRun(t *testing.T) {
	first := readRules(t)
	if len(first) != len(rulePaths) {
		t.Fatalf("%d rule file(s), want %d", len(first), len(rulePaths))
	}
	if ruleSum == "" {
		t.Errorf("the run names no rule text, so a table cannot say which it measured")
	}
	held := first[0].body
	on := first[0].path
	body, err := os.ReadFile(on)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(on, append(body, []byte("\nan edit made while the run is going\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(on, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}()
	again := readRules(t)
	if again[0].body != held {
		t.Errorf("the edit reached the run, so a roll after it read a rule the table does not name")
	}
	if strings.Contains(prompt(t, Case{Name: "k", Code: "export function f() {}", Facts: "A library drops an entry.",
		Expect: ExpectWritten, Why: "a site"}), "an edit made while the run is going") {
		t.Errorf("the prompt carried an edit made after the run started")
	}
}

// A strip offers the declaration the archive recorded, and the fact is often about something further
// down the file. Three of four blocks a reviewer sent back on 2026-09-22 sat where the archive had
// put them. One was a file header carrying a claim about a function, one a constant carrying a claim
// about two functions.
func TestACaseSaysWhichDeclarationTheBlockBelongsOn(t *testing.T) {
	raw := "expect: written\nnote: written\nwhy: a claim about a function, offered at the header\n" +
		"site: 1\nlands: export function postRow\n--- code\nimport { LedgerRow } from './rows';\n\n" +
		"export const SOURCE = 'accrual-eu';\n\nexport function postRow(row: LedgerRow): void {}\n" +
		"--- facts\nA posting server rejects a row it has already recorded.\n"
	c, err := ParseCase("k-lands", raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Site != 1 {
		t.Errorf("the offered site reads %d, want 1", c.Site)
	}
	if at := DeclaredAt(c.Code, c.Lands); at != 5 {
		t.Errorf("postRow is declared on line %d, want 5", at)
	}
	written := Return{Summary: PartNone, Note: PartWritten, Block: "// A posting server rejects a row it has already recorded."}
	atTheHeader := written
	atTheHeader.At = 1
	if JudgeCase(c, atTheHeader).Passed() {
		t.Errorf("a block left at the offered site passed a case that names another declaration")
	}
	atTheFunction := written
	atTheFunction.At = 5
	if v := JudgeCase(c, atTheFunction); !v.Passed() {
		t.Errorf("a block on the named declaration failed: %v", v.Failures)
	}
	// A case naming a declaration its own code lacks is broken. The parse refuses it, and no run scores it.
	if _, err := ParseCase("k-typo", strings.Replace(raw, "lands: export function postRow",
		"lands: export function postRowe", 1)); err == nil {
		t.Errorf("a case naming a declaration the code lacks parsed")
	}
	if !strings.Contains(prompt(t, c), "5\texport function postRow") {
		t.Errorf("the prompt shows the writer no line numbers to answer with")
	}
}

// A bar is a pattern. k35's false claim reads with any words between the throw and the failure, and a
// fixed phrase barred one wording of it.
func TestABarIsAPattern(t *testing.T) {
	c := Case{Name: "k", Expect: ExpectWritten, Bars: []string{"throw[^,.]*makes[^,.]*fail"}}
	barred := Return{Summary: PartNone, Note: PartWritten,
		Block: "// A throw from the preprocessing function makes the service fail the load."}
	if v := JudgeCase(c, barred); v.Passed() {
		t.Fatal("a throw said to fail the load passed the bar")
	}
	kept := Return{Summary: PartNone, Note: PartWritten,
		Block: "// The service catches a throw, and only a rejected promise fails the load."}
	if v := JudgeCase(c, kept); !v.Passed() {
		t.Fatalf("the corrected claim failed: %v", v.Failures)
	}
}

// rescoreEnv names a dump to score again against the case files as they are now. A table reads each
// label at scoring. A label ruled mid-table scores the rolls already taken, with no roll taken again.
// It reads rolls taken with the check off: a roll the loop ended at none keeps a note in its answer.
const rescoreEnv = "WRITER_EVAL_RESCORE"

func TestRescoreADump(t *testing.T) {
	path := os.Getenv(rescoreEnv)
	if path == "" {
		t.Skipf("%s is unset", rescoreEnv)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []dumped
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	cases, err := LoadCases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Case{}
	for _, c := range cases {
		byName[c.Name] = c
	}
	clean, rolls := map[string]int{}, map[string]int{}
	var names []string
	for _, row := range rows {
		c, known := byName[row.Case]
		if !known || !strings.HasPrefix(row.Case, os.Getenv(caseEnv)) {
			continue
		}
		if rolls[row.Case] == 0 {
			names = append(names, row.Case)
		}
		rolls[row.Case]++
		if JudgeCase(c, ParseReturn(row.Raw)).Passed() {
			clean[row.Case]++
		}
	}
	for _, name := range names {
		t.Logf("%-46s %d of %d (floor %d)", name, clean[name], rolls[name], floorFor(byName[name]))
	}
}
