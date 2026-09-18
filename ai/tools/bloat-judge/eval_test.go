// The eval: what a judge configuration deletes from a corpus whose units a human has labelled. It
// exists because capping the model's thinking and changing which model rolls both change what the
// judge deletes — the one thing it may not get wrong — and no timing run tells a fast configuration
// from one that eats a load-bearing paragraph.
package bloatjudge

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	modelpolicy "kk-flavor/tools/model-policy"
	"kk-flavor/tools/shell"
)

// evalCase is one labelled text. The units are resolved at parse time from the same Split the judge
// runs, so a label naming a unit the judge would not offer is a broken case rather than a silent
// miss. Labels are a human's reading of the judge's own prompt, and a genuinely arguable unit carries
// none: a forced label turns a measurement into noise, so no case labels all of its own units.
type evalCase struct {
	name  string
	kind  string
	cut   []int
	keep  []int
	text  string
	units []Unit
	// verdicts is the labelled answer for a kind that labels every block, read from one label line
	// per verdict. A block with no line is labelled `keep`, so a case names only what it flags.
	verdicts map[int]string
}

const caseSeparator = "---"

// parseCase reads `kind:`, `cut:` and `keep:` header lines, then the text below a `---` line.
func parseCase(name, raw string) (evalCase, error) {
	header, text, found := strings.Cut(raw, "\n"+caseSeparator+"\n")
	if !found {
		return evalCase{}, fmt.Errorf("%s has no %q line separating its labels from its text", name, caseSeparator)
	}
	parsed := evalCase{name: name, text: text}
	for _, line := range shell.SplitLines(header) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return evalCase{}, fmt.Errorf("%s: %q is not a `key: value` label line", name, line)
		}
		switch key {
		case "kind":
			parsed.kind = strings.TrimSpace(value)
		case "carried", "obvious", "stale", "padded", "coined", "unclear":
			if !kinds[parsed.kind].Verdicts {
				return evalCase{}, fmt.Errorf("%s: %q labels a verdict, and kind %q does not answer verdicts", name, key, parsed.kind)
			}
			numbers, err := unitNumbers(value, key)
			if err != nil {
				return evalCase{}, fmt.Errorf("%s: %w", name, err)
			}
			if parsed.verdicts == nil {
				parsed.verdicts = map[int]string{}
			}
			for _, n := range numbers {
				if was, twice := parsed.verdicts[n]; twice {
					return evalCase{}, fmt.Errorf("%s labels block %d both %s and %s", name, n, was, key)
				}
				parsed.verdicts[n] = key
			}
		case "cut", "keep":
			numbers, err := unitNumbers(value, key)
			if err != nil {
				return evalCase{}, fmt.Errorf("%s: %w", name, err)
			}
			if key == "cut" {
				parsed.cut = numbers
			} else {
				parsed.keep = numbers
			}
		default:
			return evalCase{}, fmt.Errorf("%s: unknown label %q", name, key)
		}
	}
	if _, known := kinds[parsed.kind]; !known {
		return evalCase{}, fmt.Errorf("%s names kind %q, which the judge does not have", name, parsed.kind)
	}
	parsed.units, _ = parsed.split()
	for _, label := range [][]int{parsed.cut, parsed.keep} {
		for _, n := range label {
			if n < 1 || n > len(parsed.units) {
				return evalCase{}, fmt.Errorf("%s labels unit %d, but it offers %d", name, n, len(parsed.units))
			}
		}
	}
	for _, n := range parsed.cut {
		if slices.Contains(parsed.keep, n) {
			return evalCase{}, fmt.Errorf("%s labels unit %d both cut and keep", name, n)
		}
	}
	return parsed, nil
}

func unitNumbers(value, label string) ([]int, error) {
	var numbers []int
	for _, field := range shell.SplitFields(value) {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("%q in the %s label is not a unit number", field, label)
		}
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	return numbers, nil
}

// split runs the judge's own Split over the case, so a case's numbering is the numbering a real run
// produces — a commit's withheld trailers included.
func (c evalCase) split() ([]Unit, string) {
	lines := shell.SplitLines(c.text)
	kind := kinds[c.kind]
	return Split(lines, kind.candidates(lines), offerFor(lines, kind))
}

// score reads one verdict against the labels, and the two counts are not symmetric. A FALSE CUT is a
// load-bearing unit the vote deleted — the damage, and what a human notices in their own commit
// message. A MISSED CUT is a worthless one it kept — waste, and the text merely stays as long as it
// was. A configuration is admissible when it makes no false cut. Unlabelled units count in neither.
func (c evalCase) score(gone []int) (falseCuts, missedCuts []int) {
	for _, n := range c.keep {
		if slices.Contains(gone, n) {
			falseCuts = append(falseCuts, n)
		}
	}
	for _, n := range c.cut {
		if !slices.Contains(gone, n) {
			missedCuts = append(missedCuts, n)
		}
	}
	return falseCuts, missedCuts
}

func loadCorpus(dir string) ([]evalCase, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.case"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var corpus []evalCase
	for _, path := range names {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed, err := parseCase(strings.TrimSuffix(filepath.Base(path), ".case"), string(raw))
		if err != nil {
			return nil, err
		}
		corpus = append(corpus, parsed)
	}
	if len(corpus) == 0 {
		return nil, fmt.Errorf("no cases under %s", dir)
	}
	return corpus, nil
}

const corpusDir = "testdata/corpus"

// A kind this corpus measured and the tree does not carry.
//
// `comment-read` put each comment block to the model on its own, with the file withheld, and asked
// whether a cold reader could restate what the declaration did. It shipped or died by a numeric rule
// fixed before it was written: flag at least 10 of 12 blocks a reviewer had labelled, and at most 3 of
// 26 unlabelled ones. Two prompt iterations scored 8 of 12 with 8 false, then 4 of 12 with 2 false, so
// it was deleted before landing and the deterministic voice check stands alone.
//
// Recorded here because the corpus is where a measured configuration belongs, and because the numbers
// are the reason not to build it again: the two bounds are not jointly reachable by that question. A
// third of the labelled blocks were comments a reader could follow perfectly well and that were wanted
// in another form, which is a finding about form and not about clarity. No prompt separates those from
// prose nobody asked to change.
//
// The second bound as measured used 26 blocks of the labelled corpus. The rule as first written named
// a directory of the host repository instead; the denominator differs and the outcome does not.
const deletedKindRecord = "comment-read: 8/12 and 8/26, then 4/12 and 2/26, against a bar of >=10 and <=3"

// The record above names a kind, and a kind that came back without its eval would make it a lie.
func TestTheDeletedKindStaysDeleted(t *testing.T) {
	if _, present := kinds["comment-read"]; present {
		t.Fatalf("comment-read is a kind again, and %q is now false — restore its eval case with it", deletedKindRecord)
	}
}

func TestEveryCorpusCaseParsesAndLabelsARealUnit(t *testing.T) {
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus {
		if len(c.cut) == 0 && len(c.keep) == 0 {
			t.Errorf("%s labels nothing, so no configuration can pass or fail it", c.name)
		}
	}
}

// How a label is checked by eye, since a case's numbering comes from the judge's own Split and not
// from counting lines: `JUDGE_EVAL_SHOW=1 go test ./bloat-judge/ -run TestShowCorpus -v`.
func TestShowCorpus(t *testing.T) {
	if os.Getenv("JUDGE_EVAL_SHOW") == "" {
		t.Skip("JUDGE_EVAL_SHOW is unset")
	}
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus {
		_, view := c.split()
		t.Logf("\n=== %s (%s) cut %v keep %v ===\n%s", c.name, c.kind, c.cut, c.keep, view)
	}
}

// A label names a unit number, and that number means nothing unless it is the number a real run puts
// in front of the model. A recording caller reads the view RunIn built and holds it against the
// case's own, so the two cannot drift apart while both still look reasonable on their own.
func TestTheEvalOffersEachCaseTheUnitsARunDoes(t *testing.T) {
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus {
		t.Run(c.name, func(t *testing.T) {
			seen := ""
			recording := func(_, view string) (string, error) {
				seen = view
				return "none", nil
			}
			var out, errs strings.Builder
			if code := RunIn("j", []string{c.kind, write(t, c.text)}, ".", nil, &out, &errs, recording, nil); code != exitClean {
				t.Fatalf("exit %d, stderr %s", code, errs.String())
			}
			if _, view := c.split(); view != seen {
				t.Fatalf("the eval numbers this case differently from a run:\n--- the eval ---\n%s\n--- a run ---\n%s", view, seen)
			}
		})
	}
}

func TestParseCaseRefusesALabelOutsideTheUnitsOnOffer(t *testing.T) {
	_, err := parseCase("x", "kind: commit\ncut: 9\n---\nSubject\n\nBody.\n")
	if err == nil || !strings.Contains(err.Error(), "labels unit 9") {
		t.Fatalf("got %v, want a refusal naming unit 9", err)
	}
}

func TestScoreSeparatesDamageFromWaste(t *testing.T) {
	c, err := parseCase("x", "kind: report\ncut: 2\nkeep: 1\n---\nload-bearing\n\nworthless\n\nunlabelled\n")
	if err != nil {
		t.Fatal(err)
	}
	if falseCuts, missed := c.score([]int{1}); len(falseCuts) != 1 || len(missed) != 1 {
		t.Fatalf("deleting the kept unit scored %v/%v, want one false cut and one missed cut", falseCuts, missed)
	}
	if falseCuts, missed := c.score([]int{2, 3}); len(falseCuts) != 0 || len(missed) != 0 {
		t.Fatalf("the labelled verdict scored %v/%v, want nothing on either count", falseCuts, missed)
	}
}

// variant is one judge configuration to measure. Everything but the delta named here is the shipped
// path, so what a row measures is production plus one change.
type variant struct {
	name     string
	client   string
	settings modelpolicy.Settings
	// settingSources replaces the empty list claudeArgs ships, for the row that measures what
	// inheriting the operator's configuration did.
	settingSources string
	// thinking is what MAX_THINKING_TOKENS is set to for the roll. Handed to the command rather than
	// exported here, because a roll's environment is an allow-list that drops this one on purpose —
	// exported, the variant would measure the baseline while reporting as itself. Empty leaves the
	// model's own budget alone.
	thinking string
}

var variants = []variant{
	{name: "baseline", client: "claude", settings: modelpolicy.Settings{Model: "haiku"}},
	{name: "user-settings", client: "claude", settings: modelpolicy.Settings{Model: "haiku"}, settingSources: "user"},
	// A positive cap is not honoured on this path — measured 2026-09-16, MAX_THINKING_TOKENS=256 over
	// the comment-go view still drew 542 and 434 output tokens, and 1024, 2048 and 4096 each measured
	// the same roll time and the same verdicts as no cap at all. Only zero moves anything, which is
	// why the two rows left are the cap that should bind and the switch that does.
	{name: "think-1024", client: "claude", settings: modelpolicy.Settings{Model: "haiku"}, thinking: "1024"},
	{name: "think-off", client: "claude", settings: modelpolicy.Settings{Model: "haiku"}, thinking: "0"},
	{name: "sonnet", client: "claude", settings: modelpolicy.Settings{Model: "sonnet"}},
	{name: "codex", client: "codex", settings: modelpolicy.Settings{Model: "gpt-5.6-luna", Effort: "low"}},
}

// caller builds the variant's roll out of the shipped one, so a row differs from production by the
// field it names and nothing else. Bounded at notTheSubject by the rule deadline_test.go's scan
// holds — nothing here asks what a roll's bound should be, and an hour still ends a hung one.
func (v variant) caller() Caller {
	if v.client == "codex" {
		return CodexCaller(notTheSubject, v.settings)
	}
	return func(prompt, view string) (string, error) {
		args := claudeArgs(prompt, v.settings)
		if v.settingSources != "" {
			var err error
			if args, err = withSettingSources(args, v.settingSources); err != nil {
				return "", err
			}
		}
		var environment []string
		if v.thinking != "" {
			environment = append(environment, "MAX_THINKING_TOKENS="+v.thinking)
		}
		return runBounded(notTheSubject, modelCommand{
			name: "claude", args: args, stdin: view, model: v.settings.Model, env: environment,
		})
	}
}

// withSettingSources replaces the empty list claudeArgs ships. The case below holds the miss to an
// error rather than a no-op.
func withSettingSources(args []string, sources string) ([]string, error) {
	for i, arg := range args {
		if arg == "--setting-sources" {
			args[i+1] = sources
			return args, nil
		}
	}
	return nil, fmt.Errorf("claudeArgs passes no --setting-sources to set to %q", sources)
}

// The user-settings row is the shipped argv with one flag rewritten, so it only measures anything
// while claudeArgs still passes that flag. A rewrite that quietly matched nothing would roll the
// shipped flags and report the result as a configuration nobody ran.
func TestAVariantFindingNoFlagToRewriteRefusesRatherThanRepeatingBaseline(t *testing.T) {
	args, err := withSettingSources(claudeArgs("p", modelpolicy.Settings{Model: "fixture-model"}), "user")
	if err != nil {
		t.Fatal(err)
	}
	if i := slices.Index(args, "--setting-sources"); i < 0 || args[i+1] != "user" {
		t.Fatalf("the flag was not rewritten: %q", args)
	}
	if _, err := withSettingSources([]string{"-p", "--model", "fixture-model"}, "user"); err == nil {
		t.Fatal("a variant rewrote a --setting-sources that was not there, so its row would repeat baseline")
	}
}

type trial struct {
	name       string
	gone       []int
	falseCuts  []int
	missedCuts []int
	elapsed    time.Duration
	err        error
}

// evalRolls is the shipped roll count. Held here rather than read from models.json: the eval compares
// configurations against each other, and a roll count that moved under it would move every row.
const evalRolls = 3

func (v variant) run(c evalCase) trial {
	units, view := c.split()
	started := time.Now()
	reply, err := Voting(v.caller(), evalRolls)(Prompt(kinds[c.kind]), view)
	result := trial{name: c.name, elapsed: time.Since(started)}
	if err != nil {
		result.err = err
		return result
	}
	gone, err := ParseVerdict(reply, len(units))
	if err != nil {
		result.err = err
		return result
	}
	result.gone = gone
	result.falseCuts, result.missedCuts = c.score(gone)
	return result
}

// Spends a model call per roll per case per variant, so it runs only when JUDGE_EVAL names variants —
// `JUDGE_EVAL=baseline,codex go test ./bloat-judge/ -run TestJudgeEval -v -timeout 2h`, the names
// being the `variants` list below. An unknown one fails the run rather than being skipped, so a stale
// command cannot measure less than it claims.
func TestJudgeEval(t *testing.T) {
	asked := os.Getenv("JUDGE_EVAL")
	if asked == "" {
		t.Skip("JUDGE_EVAL is unset; this spends a model call per roll per case per variant")
	}
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	at := 4
	if set := os.Getenv("JUDGE_EVAL_PARALLEL"); set != "" {
		if at, err = strconv.Atoi(set); err != nil || at < 1 {
			t.Fatalf("JUDGE_EVAL_PARALLEL is %q, which is not a positive count", set)
		}
	}
	for _, name := range strings.Split(asked, ",") {
		name = strings.TrimSpace(name)
		index := slices.IndexFunc(variants, func(v variant) bool { return v.name == name })
		if index < 0 {
			t.Fatalf("no variant %q; the corpus knows %s", name, variantNames())
		}
		v := variants[index]
		t.Run(v.name, func(t *testing.T) { report(t, v, runAll(v, corpus, at)) })
	}
}

// Cases go out together because the sweep is about what is deleted, and a corpus judged one case at a
// time is an hour of wall clock per variant. The per-case seconds below are therefore an upper bound
// under that contention, never a clean timing — measure one case alone for that.
func runAll(v variant, corpus []evalCase, at int) []trial {
	trials := make([]trial, len(corpus))
	slots := make(chan struct{}, at)
	var wg sync.WaitGroup
	for i, c := range corpus {
		wg.Add(1)
		go func(i int, c evalCase) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			trials[i] = v.run(c)
		}(i, c)
	}
	wg.Wait()
	return trials
}

// The counts are reported and never asserted on — what a configuration deletes is the human's to read,
// and a threshold here would only teach the next corpus to clear it. A case that never reached the
// model is the exception and fails the run: an eval that exits 0 over eight cases it could not run is
// silence dressed as a clean result, the same thing ParseVerdict refuses of a roll that said nothing.
func report(t *testing.T, v variant, trials []trial) {
	t.Helper()
	falseCuts, missedCuts, failures := 0, 0, 0
	var total time.Duration
	t.Logf("%-20s %-28s %7s %s", "variant", "case", "seconds", "verdict")
	for _, result := range trials {
		total += result.elapsed
		if result.err != nil {
			failures++
			t.Errorf("%-20s %-28s %7.1f DID NOT RUN: %v", v.name, result.name, result.elapsed.Seconds(), result.err)
			continue
		}
		falseCuts += len(result.falseCuts)
		missedCuts += len(result.missedCuts)
		t.Logf("%-20s %-28s %7.1f cut %v | FALSE CUT %v | missed %v",
			v.name, result.name, result.elapsed.Seconds(), result.gone, result.falseCuts, result.missedCuts)
	}
	t.Logf("SUMMARY %s: false cuts %d, missed cuts %d, did not run %d, %.0fs of roll time over %d cases",
		v.name, falseCuts, missedCuts, failures, total.Seconds(), len(trials))
}

func variantNames() string {
	names := make([]string, len(variants))
	for i, v := range variants {
		names[i] = v.name
	}
	return strings.Join(names, ",")
}
