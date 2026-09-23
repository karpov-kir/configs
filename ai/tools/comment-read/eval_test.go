package commentread

import (
	"context"
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

const labelledPath = "testdata/labelled.txt"

// labelledCount is how many blocks the bar was fixed over before the reader existed.
const labelledCount = 10

const briefPath = "../../kk-flavor/workers/comment-read.md"
const policyPath = "../../kk-flavor/models.json"
const readerRow = "comment-read"

// evalEnv gates every model call. rollsEnv and parallelEnv size the run.
const evalEnv = "COMMENT_READ_EVAL"
const rollsEnv = "COMMENT_READ_ROLLS"
const parallelEnv = "COMMENT_READ_PARALLEL"
const plainBlocksEnv = "COMMENT_READ_PLAIN_BLOCKS"

// plainSetEnv names the reviewed set by the environment and never by a committed path. Its blocks
// are somebody else's code: a block is named by its position in the sorted set, and only counts leave.
const plainSetEnv = "JUDGE_EVAL_PLAIN"

// The bar, fixed before the reader was built. Every labelled block gets `cannot say`, and at most one
// reviewer-standing note in twenty does.
const plainCannotBound = 0.05

const defaultRolls = 15
const defaultPlainBlocks = 60
const callDeadline = 4 * time.Minute

func intFromEnv(t *testing.T, name string, fallback int) int {
	t.Helper()
	set := os.Getenv(name)
	if set == "" {
		return fallback
	}
	n, err := strconv.Atoi(set)
	if err != nil || n < 1 {
		t.Fatalf("%s is %q, which is not a positive count", name, set)
	}
	return n
}

func readerSettings(t *testing.T) modelpolicy.Settings {
	t.Helper()
	policy, err := modelpolicy.Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: readerRow})
	if err != nil {
		t.Fatalf("the policy assigns no row to %q: %v", readerRow, err)
	}
	return decision.Requested
}

func callReader(settings modelpolicy.Settings, text string) (string, error) {
	args := []string{"-p", "--model", settings.Model, "--output-format", "text",
		"--tools", "", "--setting-sources", "", "--strict-mcp-config"}
	if settings.Effort != "" {
		args = append(args, "--effort", settings.Effort)
	}
	ctx, stop := context.WithTimeout(context.Background(), callDeadline)
	defer stop()
	command := exec.CommandContext(ctx, "claude", append(args, text)...)
	command.WaitDelay = 5 * time.Second
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("the reader did not answer: %w", err)
	}
	return string(out), nil
}

// read puts every site to the reader `rolls` times, at most `parallel` calls in flight.
func read(t *testing.T, sites []Site, rolls, parallel int) [][]Answer {
	t.Helper()
	brief, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatal(err)
	}
	settings := readerSettings(t)
	out := make([][]Answer, len(sites))
	for i := range out {
		out[i] = make([]Answer, rolls)
	}
	slots := make(chan struct{}, parallel)
	var wait sync.WaitGroup
	for i, s := range sites {
		text := Prompt(string(brief), s)
		for r := 0; r < rolls; r++ {
			wait.Add(1)
			slots <- struct{}{}
			go func(i, r int) {
				defer wait.Done()
				defer func() { <-slots }()
				raw, err := callReader(settings, text)
				if err == nil {
					out[i][r] = Parse(raw)
				}
			}(i, r)
		}
	}
	wait.Wait()
	return out
}

// firstRolls is the verdict the pipeline's own roll count would give, read off the same answers.
func firstRolls(answers []Answer, n int) bool {
	if n > len(answers) {
		n = len(answers)
	}
	cannot, _ := Verdict(answers[:n])
	return cannot
}

func pipelineRolls(t *testing.T) int {
	t.Helper()
	policy, err := modelpolicy.Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: readerRow})
	if err != nil || decision.Rolls < 1 {
		t.Fatalf("the %q row names no roll count", readerRow)
	}
	return decision.Rolls
}

// The labelled half: every flagged block has to come back `cannot say`.
func TestReaderOverTheLabelledBlocks(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends a model call per roll", evalEnv)
	}
	raw, err := os.ReadFile(labelledPath)
	if err != nil {
		t.Fatal(err)
	}
	sites, err := ParseSites(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	rolls := intFromEnv(t, rollsEnv, defaultRolls)
	answers := read(t, sites, rolls, intFromEnv(t, parallelEnv, 4))
	short := pipelineRolls(t)
	var report strings.Builder
	missed := 0
	for i, s := range sites {
		cannot, counted := Verdict(answers[i])
		yes := 0
		for _, a := range answers[i] {
			if a.Parsed && a.CannotSay {
				yes++
			}
		}
		mark := "cannot say"
		if !cannot {
			mark, missed = "READ", missed+1
		}
		fmt.Fprintf(&report, "%-4s %-10s %2d of %2d cannot say, first %d: %v\n", s.Name, mark, yes, counted,
			short, firstRolls(answers[i], short))
		for _, a := range answers[i] {
			if a.Parsed && !a.CannotSay {
				fmt.Fprintf(&report, "     reads: %s\n", a.Said)
				break
			}
		}
	}
	t.Logf("\n%s%d of %d labelled blocks read where the bar wants cannot say", report.String(), missed, len(sites))
	if missed > 0 {
		t.Fail()
	}
}

// The plain half: reviewer-standing notes, at most one in twenty `cannot say`. Positions and counts
// leave the run, and no text does.
func TestReaderOverThePlainSet(t *testing.T) {
	if os.Getenv(evalEnv) == "" {
		t.Skipf("%s is unset; this spends a model call per roll", evalEnv)
	}
	dir := os.Getenv(plainSetEnv)
	if dir == "" {
		t.Skipf("%s names no directory, so the plain half has not measured anything", plainSetEnv)
	}
	sites := plainSites(t, dir, intFromEnv(t, plainBlocksEnv, defaultPlainBlocks))
	rolls := intFromEnv(t, rollsEnv, defaultRolls)
	answers := read(t, sites, rolls, intFromEnv(t, parallelEnv, 4))
	short := pipelineRolls(t)
	cannot, shortCannot, unanswered := 0, 0, 0
	var positions []string
	for i, s := range sites {
		verdict, counted := Verdict(answers[i])
		if counted == 0 {
			unanswered++
			continue
		}
		if verdict {
			cannot++
			positions = append(positions, s.Name)
		}
		if firstRolls(answers[i], short) {
			shortCannot++
		}
	}
	read := len(sites) - unanswered
	t.Logf("\n%d of %d reviewer-standing notes cannot say at %d rolls (bound %.0f%%), %d at the first %d; %d unanswered\n%s",
		cannot, read, rolls, plainCannotBound*100, shortCannot, short, unanswered, strings.Join(positions, " "))
	if read == 0 || float64(cannot) > plainCannotBound*float64(read) {
		t.Fail()
	}
}

// plainSites takes each block over a declaration that carries a note sentence, one per file in turn
// until the budget is spent, so no single file fills the sample.
func plainSites(t *testing.T, dir string, budget int) []Site {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ts", ".tsx", ".js", ".go":
			if !entry.IsDir() {
				paths = append(paths, path)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("%s names a directory the run could not read: %v", plainSetEnv, err)
	}
	sort.Strings(paths)
	perFile := make([][]Site, len(paths))
	for at, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(body), "\n")
		for n, b := range census.Blocks(lines) {
			if !b.OverDecl || len(b.Notes()) == 0 {
				continue
			}
			under := b.Line + b.Span
			for under <= len(lines) && strings.TrimSpace(lines[under-1]) == "" {
				under++
			}
			if under > len(lines) {
				continue
			}
			var block []string
			for offset := 0; offset < b.Span; offset++ {
				block = append(block, lines[b.Line-1+offset])
			}
			perFile[at] = append(perFile[at], Site{Name: fmt.Sprintf("%03d/%d", at+1, n+1),
				Block: strings.Join(block, "\n"), Line: lines[under-1]})
		}
	}
	var out []Site
	for round := 0; len(out) < budget; round++ {
		took := false
		for _, sites := range perFile {
			if round < len(sites) && len(out) < budget {
				out = append(out, sites[round])
				took = true
			}
		}
		if !took {
			break
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no note over a declaration", plainSetEnv)
	}
	return out
}
