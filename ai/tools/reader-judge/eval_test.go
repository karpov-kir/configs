// The eval: what a judge configuration deletes from a corpus whose units a human has labelled. It
// exists because capping the model's thinking and changing which model rolls both change what the
// judge deletes — the one thing it may not get wrong — and no timing run tells a fast configuration
// from one that eats a load-bearing paragraph.
package readerjudge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/shell"
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
	// verdicts is the labelled answer for a kind that labels every block. One label line names the
	// blocks carrying that verdict. A block with no line is `keep`, so a case names only what it flags.
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
		case "obvious", "stale", "padded", "coined", "unclear":
			if !specificationFor(parsed.kind).Verdicts {
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
	_, live := kinds[parsed.kind]
	_, recorded := deletedKinds[parsed.kind]
	if !live && !recorded {
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
	kind := specificationFor(c.kind)
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

// `comment-verdict` labelled every block from a closed set, against bars fixed before the run. It
// missed both halves, and a second configuration at the reader's tier with five rolls moved one
// block. The plain half decided it, and the labels it failed are the reason not to build it again in
// this shape.
const deletedVerdictRecord = "comment-verdict: obvious 2/4, coined 0/2, unclear 1/3, plain set ~30% against a bound of 5%"

// `comment` asked a reader which comment blocks to delete. That is the same question about absence
// that sank `comment-verdict`, put to the same reader, and its survivors were the complaint that
// opened this campaign. The lane replacing it writes each block again and bounds it three ways, with
// a model voting on none of the three.
const deletedCommentRecord = "comment: replaced by strip, write and voice, none of which is a vote"

// The records above name kinds, and a kind that came back without its eval would make them lies. The
// corpus keeps the cases either way, so re-adding a kind runs it against the bars it missed.
func TestTheDeletedKindsStayDeleted(t *testing.T) {
	for name, record := range map[string]string{
		"comment-read":    deletedKindRecord,
		"comment-verdict": deletedVerdictRecord,
		"comment":         deletedCommentRecord,
	} {
		if _, present := kinds[name]; present {
			t.Fatalf("%s is a kind again, and %q is now false — clear its bars before restoring it", name, record)
		}
	}
}

// deletedKinds are the kinds this corpus measured and the tree no longer ships, each with the shape
// it had. A case naming one still parses and still splits the way it did, so the fixture that decided
// a kind outlives the kind and is there to re-run against.
var deletedKinds = map[string]Kind{
	"comment-verdict": {Reader: "an engineer in your first year, new to this codebase and not a native English speaker, " +
		"reading quickly to change something near this line", Source: true, Verdicts: true},
	"comment": {Reader: "an engineer opening this file for the first time to change something near this line, " +
		"who has not read the rest of the file and does not know the change that introduced it", Source: true},
}

// withTheRecordedVerdictKind puts a deleted kind back for one case's duration. The label machinery
// stays in the tree for the next attempt, so a test is the only caller left that can drive it.
// TestTheDeletedKindsStayDeleted is what guards the shipped set, in place of this absence.
func withTheRecordedVerdictKind(t *testing.T) string {
	t.Helper()
	withTheRecordedKinds(t)
	return "comment-verdict"
}

// withTheRecordedKinds puts every deleted kind back for one case's duration, for the cases that read
// the whole corpus and would otherwise skip its older half.
func withTheRecordedKinds(t *testing.T) {
	t.Helper()
	for name, kind := range deletedKinds {
		if _, shipped := kinds[name]; shipped {
			t.Fatalf("%s ships again, so this fixture would shadow the real one", name)
		}
		kinds[name] = kind
		t.Cleanup(func() { delete(kinds, name) })
	}
}

// specificationFor is a case's kind, live or recorded as deleted. Every reader of a case goes through
// it, so a deleted kind's fixture splits into the blocks it was labelled against.
func specificationFor(name string) Kind {
	if kind, live := kinds[name]; live {
		return kind
	}
	return deletedKinds[name]
}

func TestEveryCorpusCaseParsesAndLabelsARealUnit(t *testing.T) {
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus {
		if len(c.cut) == 0 && len(c.keep) == 0 && len(c.verdicts) == 0 {
			t.Errorf("%s labels nothing, so no configuration can pass or fail it", c.name)
		}
	}
}

// How a label is checked by eye, since a case's numbering comes from the judge's own Split and not
// from counting lines: `JUDGE_EVAL_SHOW=1 go test ./reader-judge/ -run TestShowCorpus -v`.
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
		t.Logf("\n=== %s (%s) cut %v keep %v verdicts %v ===\n%s", c.name, c.kind, c.cut, c.keep, c.verdicts, view)
	}
}

// A label names a unit number, and that number means nothing unless it is the number a real run puts
// in front of the model. A recording caller reads the view RunIn built and holds it against the
// case's own, so the two cannot drift apart while both still look reasonable on their own.
func TestTheEvalOffersEachCaseTheUnitsARunDoes(t *testing.T) {
	withTheRecordedKinds(t)
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus {
		t.Run(c.name, func(t *testing.T) {
			seen := ""
			recording := func(_, view string) (string, error) {
				seen = view
				if !specificationFor(c.kind).Verdicts {
					return "none", nil
				}
				// "none" is the delete kind's empty answer and is not a verdict. A verdict kind
				// refuses a reply that leaves a block unanswered, so the stub answers every one.
				lines := make([]string, 0, unitsInView(view))
				for n := 1; n <= unitsInView(view); n++ {
					lines = append(lines, strconv.Itoa(n)+" keep")
				}
				return strings.Join(lines, "\n"), nil
			}
			var out, errs strings.Builder
			if code := RunIn("j", []string{c.kind, write(t, c.text)}, ".", noRepository, nil, &out, &errs, recording, nil); code != exitClean {
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
	// rolls overrides evalRolls for this row, so a configuration that changes the roll count is a row
	// of its own and never a second sweep. Zero takes the shipped count.
	rolls int
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
	// What models.json now ships for the verdict kind: its own sub-row at the reader's tier, and five
	// rolls because three disagreed with themselves across two runs of the same corpus.
	{name: "verdict-reader", client: "claude", settings: modelpolicy.Settings{Model: "sonnet"}, rolls: 5},
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
	// answered and wanted are the verdict kind's half: one label per block, and what the case says
	// each should be. Both carry every block, `keep` included, because the false-flag half of the bar
	// is counted over the blocks a case did not label.
	answered map[int]string
	wanted   map[int]string
}

// evalRolls is the shipped roll count. Held here rather than read from models.json: the eval compares
// configurations against each other, and a roll count that moved under it would move every row.
const evalRolls = 3

// rollCount is this row's roll count, and the shipped count for a row that sets zero.
func (v variant) rollCount() int {
	if v.rolls > 0 {
		return v.rolls
	}
	return evalRolls
}

func (v variant) run(c evalCase) trial {
	units, view := c.split()
	started := time.Now()
	reply, err := Voting(v.caller(), v.rollCount())(Prompt(specificationFor(c.kind)), view)
	result := trial{name: c.name, elapsed: time.Since(started)}
	if err != nil {
		result.err = err
		return result
	}
	if specificationFor(c.kind).Verdicts {
		answered, err := ParseLabels(reply, len(units))
		if err != nil {
			result.err = err
			return result
		}
		result.answered = answered
		result.wanted = c.wantedLabels(len(units))
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

// wantedLabels is the case's answer for every block it offers. A block the case does not label is an
// ordinary one, so it is `keep`, and a case therefore names only what it flags.
func (c evalCase) wantedLabels(count int) map[int]string {
	wanted := map[int]string{}
	for n := 1; n <= count; n++ {
		if label, labelled := c.verdicts[n]; labelled {
			wanted[n] = label
			continue
		}
		wanted[n] = "keep"
	}
	return wanted
}

// Spends a model call per roll per case per variant, so it runs only when JUDGE_EVAL names variants —
// `JUDGE_EVAL=baseline,codex go test ./reader-judge/ -run TestJudgeEval -v -timeout 2h`, the names
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
	plainCases, plainInSet, err := loadPlainSet()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range strings.Split(asked, ",") {
		name = strings.TrimSpace(name)
		index := slices.IndexFunc(variants, func(v variant) bool { return v.name == name })
		if index < 0 {
			t.Fatalf("no variant %q; the corpus knows %s", name, variantNames())
		}
		v := variants[index]
		t.Run(v.name, func(t *testing.T) { report(t, v, runAll(v, corpus, at), runPlainSet(v, plainCases, plainInSet, at)) })
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
func report(t *testing.T, v variant, trials []trial, plain plainResult) {
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
		if result.wanted != nil {
			// A verdict trial leaves both cut fields empty, so the delete kind's three columns would
			// print blank for every such case and say none of what it answered.
			t.Logf("%-20s %-28s %7.1f %s", v.name, result.name, result.elapsed.Seconds(), disagreements(result))
			continue
		}
		falseCuts += len(result.falseCuts)
		missedCuts += len(result.missedCuts)
		t.Logf("%-20s %-28s %7.1f cut %v | FALSE CUT %v | missed %v",
			v.name, result.name, result.elapsed.Seconds(), result.gone, result.falseCuts, result.missedCuts)
	}
	t.Logf("SUMMARY %s: false cuts %d, missed cuts %d, did not run %d, %.0fs of roll time over %d cases",
		v.name, falseCuts, missedCuts, failures, total.Seconds(), len(trials))
	for _, line := range labelTable(trials, plain) {
		t.Logf("%s", line)
	}
}

// plainSetEnv names a directory of source files whose comment blocks are all ordinary, the
// denominator for the false-flag half of the bar. A judge labelling a plain block spends a reader's
// attention on text that was fine. Every block wants `keep`, so the set needs no case file. Its
// files are somebody else's code and stay outside this tree, and only counts are reported.
const plainSetEnv = "JUDGE_EVAL_PLAIN"

// plainBlocksEnv bounds how many blocks the plain half reads, because every block costs a roll per
// variant and the set is larger than a sweep can pay for. Files are read in sorted order, so a
// smaller budget reads a prefix of the same set and never a different sample each run.
const plainBlocksEnv = "JUDGE_EVAL_PLAIN_BLOCKS"

const plainBlockBudget = 120

// The verdict kind the plain set is judged by, named here. A plain set is a denominator for a single
// kind's false flags. A lookup over whichever kind answers verdicts would move that denominator on
// its own, the day a second such kind landed.
const plainSetKind = "comment-verdict"

// loadPlainSet reads the source files the environment names, or answers that none were named. A
// directory that is named and unreadable is a failure, since a run handed a bad path has measured no
// block at all and "unset" would hide the typo. It answers the blocks in the whole set beside the
// ones it took, so a sampled run says what fraction of the set it read.
func loadPlainSet() (cases []evalCase, inSet int, err error) {
	dir := os.Getenv(plainSetEnv)
	if dir == "" {
		return nil, 0, nil
	}
	var paths []string
	walked := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && plainSetExtensions[filepath.Ext(path)] {
			paths = append(paths, path)
		}
		return nil
	})
	if walked != nil {
		return nil, 0, fmt.Errorf("%s names %q, and the plain set could not be read there: %w", plainSetEnv, dir, walked)
	}
	if len(paths) == 0 {
		return nil, 0, fmt.Errorf("%s names %q, which holds no source file the scan reads", plainSetEnv, dir)
	}
	sort.Strings(paths)
	budget := plainBlockBudget
	if set := os.Getenv(plainBlocksEnv); set != "" {
		if budget, err = strconv.Atoi(set); err != nil || budget < 1 {
			return nil, 0, fmt.Errorf("%s is %q, which is not a positive count", plainBlocksEnv, set)
		}
	}
	taken := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, 0, err
		}
		// The name is the file's position in the sorted set. A path from this set names somebody
		// else's tree, and a run's log gets read and pasted by people.
		one := evalCase{name: fmt.Sprintf("plain-%03d", len(paths)), kind: plainSetKind, text: string(raw)}
		units, _ := one.split()
		inSet += len(units)
		if len(units) == 0 || taken >= budget {
			continue
		}
		one.name = fmt.Sprintf("plain-%03d", len(cases)+1)
		cases = append(cases, one)
		taken += len(units)
	}
	return cases, inSet, nil
}

// The extensions the plain set is read from. A set of another language joins by adding one here, and
// a file the scan does not read stays out of the denominator instead of counting as clean.
var plainSetExtensions = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".go": true}

// runPlainSet judges the plain cases for one variant and counts what it flagged. Run per variant,
// because the number it produces belongs to the configuration that produced it.
func runPlainSet(v variant, cases []evalCase, inSet, at int) plainResult {
	if len(cases) == 0 {
		return plainResult{}
	}
	result := plainResult{measured: true, cases: len(cases), inSet: inSet}
	for _, one := range runAll(v, cases, at) {
		// A case that did not run leaves its blocks out of the denominator, and the rate would then
		// cover a smaller set than the report names. Counted and reported.
		if one.err != nil {
			result.failed++
			if result.why == "" {
				result.why = one.err.Error()
			}
			continue
		}
		for n := range one.wanted {
			result.blocks++
			if one.answered[n] != "keep" {
				result.flagged++
			}
		}
	}
	return result
}

// plainResult is what the plain half measured, and whether it ran at all. Unset is NOT a pass. An
// unmeasured bound is a bound the run skipped, and reporting it as clean would claim a measurement
// no run took.
type plainResult struct {
	measured bool
	cases    int
	blocks   int
	flagged  int
	// inSet is every block the named set holds, which is the denominator a sampled run did not read.
	inSet int
	// failed is the plain cases that did not run. Their blocks reach the denominator nowhere, so a
	// rate quoted without them covers less than the report says it read.
	failed int
	// why is the first failure's own words. A count says a run was partial. The reason says whether
	// the set is unreadable or the judge is.
	why string
}

// labelTable is the catch-and-miss table: one row per verdict the corpus labels, then the false-flag
// row the plain set answers. It returns lines for its caller to log, so a case can drive it on its
// own, with a model and a corpus both absent.
func labelTable(trials []trial, plain plainResult) []string {
	caught, labelled := map[string]int{}, map[string]int{}
	instead := map[string]map[string]int{}
	falseFlags, plainInCorpus := 0, 0
	for _, result := range trials {
		if result.err != nil || result.wanted == nil {
			continue
		}
		for n, want := range result.wanted {
			got := result.answered[n]
			if want == "keep" {
				plainInCorpus++
				if got != "keep" {
					falseFlags++
				}
				continue
			}
			labelled[want]++
			if got == want {
				caught[want]++
				continue
			}
			if instead[want] == nil {
				instead[want] = map[string]int{}
			}
			instead[want][got]++
		}
	}
	lines := []string{fmt.Sprintf("%-10s %7s %7s  %s", "verdict", "caught", "of", "answered instead")}
	for _, name := range verdictOrder {
		if name == "keep" || labelled[name] == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-10s %7d %7d  %s", name, caught[name], labelled[name], missesAsText(instead[name])))
	}
	lines = append(lines, fmt.Sprintf("false flags on the corpus's own plain blocks: %d of %d", falseFlags, plainInCorpus))
	if !plain.measured {
		lines = append(lines, "plain set: NOT MEASURED — "+plainSetEnv+" names no directory, so the false-flag bound went unmeasured and this run does not clear it")
		return lines
	}
	read := fmt.Sprintf("plain set: %d flagged of %d block(s) read over %d file(s), out of %d block(s) in the set",
		plain.flagged, plain.blocks, plain.cases, plain.inSet)
	if plain.failed > 0 {
		read += fmt.Sprintf(" — %d file(s) did not run, and their blocks are in none of these counts. First: %s",
			plain.failed, plain.why)
	}
	lines = append(lines, read)
	return lines
}

// disagreements names every block whose verdict differs from its label, in block order. A row of the
// table can then be taken back to the block that produced it.
func disagreements(result trial) string {
	var parts []string
	for n := 1; n <= len(result.wanted); n++ {
		want, got := result.wanted[n], result.answered[n]
		if want == got {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d wanted %s got %s", n, want, got))
	}
	if len(parts) == 0 {
		return "every block as labelled"
	}
	return strings.Join(parts, " | ")
}

// missesAsText is what a miss was answered instead, in verdict order so two runs print one order.
func missesAsText(instead map[string]int) string {
	if len(instead) == 0 {
		return "—"
	}
	var parts []string
	for _, name := range verdictOrder {
		if n := instead[name]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", name, n))
		}
	}
	return strings.Join(parts, ", ")
}

func variantNames() string {
	names := make([]string, len(variants))
	for i, v := range variants {
		names[i] = v.name
	}
	return strings.Join(names, ",")
}

// An unmeasured bound is a bound the run skipped. A plain half printed as clean where the caller
// named no set claims a measurement somebody skipped, and the bar this kind ships against has a
// false-flag half.
func TestAnUnmeasuredPlainSetIsReportedAsUnmeasured(t *testing.T) {
	lines := strings.Join(labelTable(nil, plainResult{}), "\n")
	if !strings.Contains(lines, "NOT MEASURED") {
		t.Errorf("an unset plain set was reported as though it had run:\n%s", lines)
	}
	measured := strings.Join(labelTable(nil, plainResult{measured: true, cases: 2, blocks: 40, flagged: 1, inSet: 550}), "\n")
	if !strings.Contains(measured, "1 flagged of 40 block(s) read over 2 file(s), out of 550 block(s) in the set") {
		t.Errorf("a measured plain set did not report its counts:\n%s", measured)
	}
}

// A path that was named and cannot be read is a failure. A typo read as "unset" silently drops the
// half of the bar the caller asked for.
func TestAPlainSetPathThatDoesNotReadIsAFailure(t *testing.T) {
	t.Setenv(plainSetEnv, filepath.Join(t.TempDir(), "nowhere"))
	if _, _, err := loadPlainSet(); err == nil {
		t.Error("a plain set path naming no directory was taken as no plain set at all")
	}
	t.Setenv(plainSetEnv, t.TempDir())
	if _, _, err := loadPlainSet(); err == nil {
		t.Error("a directory holding no source file was taken as a plain set")
	}
	t.Setenv(plainSetEnv, "")
	cases, inSet, err := loadPlainSet()
	if err != nil || cases != nil || inSet != 0 {
		t.Errorf("an unset variable is no plain set and no error, got %d case(s), %d block(s) and %v", len(cases), inSet, err)
	}
}

// A sampled run says what it read and what it did not. The budget takes files in sorted order, so a
// second run at the same budget reads the same blocks, and the report states the denominator it
// quotes a rate over.
func TestAPlainSetSampleNamesWhatItLeftUnread(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.ts", "b.ts", "c.ts"} {
		body := "// Lists every entry in the book.\nexport function list() {}\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(plainSetEnv, dir)
	t.Setenv(plainBlocksEnv, "2")
	cases, inSet, err := loadPlainSet()
	if err != nil {
		t.Fatal(err)
	}
	if inSet != 3 {
		t.Errorf("the set holds 3 blocks and the run counted %d", inSet)
	}
	if len(cases) != 2 {
		t.Errorf("a budget of 2 blocks took %d file(s) of one block each", len(cases))
	}
	lines := strings.Join(labelTable(nil, plainResult{measured: true, cases: 2, blocks: 2, flagged: 0, inSet: 3}), "\n")
	if !strings.Contains(lines, "out of 3 block(s) in the set") {
		t.Errorf("the report does not say what it left unread:\n%s", lines)
	}
}

// No path from the plain set reaches a log. The set is somebody else's tree, and a run's output gets
// pasted around. A case carries its position in place of where it came from.
func TestAPlainSetCaseIsNamedByPositionAndNotByPath(t *testing.T) {
	dir := t.TempDir()
	body := "// Lists every entry in the book.\nexport function list() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "SecretlyNamed.ts"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(plainSetEnv, dir)
	cases, _, err := loadPlainSet()
	if err != nil || len(cases) != 1 {
		t.Fatalf("%d case(s), %v", len(cases), err)
	}
	if strings.Contains(cases[0].name, "SecretlyNamed") || strings.Contains(cases[0].name, dir) {
		t.Errorf("a case carries its path into the report: %q", cases[0].name)
	}
}

// The table is the figure the kind ships or dies by, so a case drives it here instead of reading it
// off a model run. A miss has to name what was answered instead. A label the judge confuses with
// another is a label to merge, and a label it never reaches is a different decision.
func TestTheLabelTableCountsCatchesAndNamesWhatAMissAnswered(t *testing.T) {
	trials := []trial{{
		wanted:   map[int]string{1: "obvious", 2: "obvious", 3: "coined", 4: "keep", 5: "keep"},
		answered: map[int]string{1: "obvious", 2: "padded", 3: "coined", 4: "keep", 5: "stale"},
	}}
	lines := strings.Join(labelTable(trials, plainResult{}), "\n")
	for _, want := range []string{
		"obvious          1       2  padded 1",
		"coined           1       1  —",
		"false flags on the corpus's own plain blocks: 1 of 2",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the table does not carry %q:\n%s", want, lines)
		}
	}
}

// The two kinds are measured over one text. comment-ts.case and comment-ts-verdict.case carry the
// same blocks under different labels. Edit a block in one alone and the two kinds are scored on
// different material, under a report that says one corpus.
func TestTheTwoKindsShareOneTextForTheSharedCase(t *testing.T) {
	corpus, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]string{}
	for _, c := range corpus {
		texts[c.name] = c.text
	}
	deleting, labelling := texts["comment-ts"], texts["comment-ts-verdict"]
	if deleting == "" || labelling == "" {
		t.Fatalf("one of the paired cases is missing: comment-ts %d bytes, comment-ts-verdict %d bytes",
			len(deleting), len(labelling))
	}
	if deleting != labelling {
		t.Error("comment-ts and comment-ts-verdict have drifted, so the two kinds are scored on different blocks")
	}
}
