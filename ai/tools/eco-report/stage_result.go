package ecoreport

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

type resultContext struct {
	Version  int    `json:"version"`
	Attempt  string `json:"attempt"`
	Head     string `json:"head"`
	Tree     string `json:"tree"`
	Worktree string `json:"worktree"`
}

type stageResult struct {
	resultContext
	Id      string        `json:"id"`
	Stage   resultStage   `json:"stage"`
	Status  resultStatus  `json:"status"`
	Outcome resultOutcome `json:"outcome"`
	Items   []resultItem  `json:"items"`
}

type resultStage string

const (
	resultCodeReview     resultStage = "code-review"
	resultSecurityReview resultStage = "security-review"
	resultEdit           resultStage = "edit"
	resultRefactor       resultStage = "refactor"
)

type resultStatus string

const resultCompleted resultStatus = "complete"

type resultOutcome string

const (
	outcomeComplete          resultOutcome = "complete"
	outcomePartialTurnaround resultOutcome = "partial(turnaround)"
	outcomePartialCap        resultOutcome = "partial(cap)"
)

type findingKind string

const (
	findingFalsified       findingKind = "Falsified"
	findingFork            findingKind = "Fork"
	findingPendingEvidence findingKind = "Pending evidence"
)

type findingSeverity string

const (
	severityCritical findingSeverity = "critical"
	severityHigh     findingSeverity = "high"
	severityMedium   findingSeverity = "medium"
	severityLow      findingSeverity = "low"
	severityInfo     findingSeverity = "info"
)

type resultItem struct {
	Id             string          `json:"id"`
	Kind           findingKind     `json:"kind"`
	Severity       findingSeverity `json:"severity"`
	Action         string          `json:"action"`
	Evidence       string          `json:"evidence"`
	Recommendation string          `json:"recommendation"`
}

type storedResult struct {
	Result     stageResult `json:"result"`
	IsAccepted bool        `json:"accepted"`
	Before     string      `json:"before"`
}

type resultManifest struct {
	Version int            `json:"version"`
	Report  string         `json:"report"`
	Context resultContext  `json:"context"`
	Results []storedResult `json:"results"`
}

func (r *run) candidateResultContext(attempt string) resultContext {
	tree, ok := r.currentTreeCached(r.errOut)
	if !ok {
		r.exit(2)
	}
	head, status := r.memoGit(r.errOut, "rev-parse", "--verify", "HEAD")
	if status != 0 || head == "" {
		r.refuse("error: could not identify qualification HEAD")
	}
	worktree, ok := r.worktreeToken()
	if !ok {
		r.refuse("error: could not establish which worktree this pass ran in — result context is unavailable")
	}
	return resultContext{Version: 1, Attempt: attempt, Head: head, Tree: tree, Worktree: worktree}
}

func (r *run) cmdResultContext() {
	r.requireReport(r.arg(1))
	manifest := r.requireResultManifest()
	r.assertResultProjection(manifest)
	context := r.currentResultContext(manifest)
	r.printResultContext(context)
}

func (r *run) printResultContext(context resultContext) {
	content, err := json.Marshal(context)
	if err != nil {
		r.refuse("error: could not encode result context: " + err.Error())
	}
	r.line("%s", content)
}

func (r *run) currentResultContext(manifest resultManifest) resultContext {
	if manifest.Context.Attempt == "" || r.reviewedTree() != "pending" {
		r.refuse("error: no active qualification attempt — run report.sh invalidate")
	}
	context := r.candidateResultContext(manifest.Context.Attempt)
	if context.Head != manifest.Context.Head || context.Worktree != manifest.Context.Worktree {
		r.refuse("error: stale qualification attempt — HEAD or worktree changed; run report.sh invalidate")
	}
	return context
}

func (r *run) beginResultAttempt() {
	manifest := r.readResultManifest()
	if manifest != nil {
		r.assertResultProjection(*manifest)
	} else {
		manifest = &resultManifest{Version: 1, Report: r.canonicalReportPath(), Results: []storedResult{}}
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		r.refuse("error: could not create qualification attempt: " + err.Error())
	}
	manifest.Context = r.candidateResultContext(hex.EncodeToString(raw))
	r.writeResultManifest(*manifest)
	if err := r.rewriteReport("attempt exists but protocol marker was not written", "attempt exists but protocol marker was not written", func(lines []string) []string {
		return mapFrontmatter(lines, func(inFrontmatter bool, line string) []string {
			if inFrontmatter && strings.HasPrefix(line, "result-protocol:") {
				return nil
			}
			if inFrontmatter && strings.HasPrefix(line, "reviewed-tree:") {
				return []string{line, "result-protocol: 1"}
			}
			return []string{line}
		})
	}); err != nil {
		r.exit(2)
	}
	r.printResultContext(manifest.Context)
}

func (r *run) cmdStageResult() {
	if r.arg(1) == "" || len(r.args) > 3 {
		r.refuse(stageResultUsage)
	}
	r.requireReport(r.arg(2))
	content, err := readResultFile(r.absPath(r.arg(1)))
	if err != nil {
		r.refuse("error: could not read stage result: " + err.Error())
	}
	var result stageResult
	if err := decodeResultJson(content, &result); err != nil {
		r.refuse("error: invalid stage result: " + err.Error())
	}
	if err := validateStageResult(result); err != nil {
		r.refuse("error: invalid stage result: " + err.Error())
	}
	manifest := r.requireResultManifest()
	context := r.currentResultContext(manifest)
	if result.resultContext != context {
		r.refuse("error: stale stage result — attempt, HEAD, tree or worktree differs from the current candidate")
	}
	index := r.resultSubmissionIndex(&manifest, result)
	r.finishResultProjection(&manifest, index)
	r.line("accepted %s result %s (%d finding(s))", result.Stage, result.Id, len(result.Items))
}

func (r *run) resultSubmissionIndex(manifest *resultManifest, result stageResult) int {
	for index, saved := range manifest.Results {
		if saved.Result.Id != result.Id {
			continue
		}
		if saved.IsAccepted || !reflect.DeepEqual(saved.Result, result) {
			r.refuse("error: duplicate stage result id " + result.Id)
		}
		return index
	}
	r.assertResultProjection(*manifest)
	ids := map[string]bool{}
	for _, saved := range manifest.Results {
		if !saved.IsAccepted {
			r.refuse("error: stage result " + saved.Result.Id + " is pending — retry its exact payload first")
		}
		if saved.Result.Stage == result.Stage && saved.Result.resultContext == result.resultContext && saved.Result.Outcome == outcomeComplete {
			r.refuse("error: duplicate completed stage for this candidate: " + string(result.Stage))
		}
		for _, item := range saved.Result.Items {
			ids[item.Id] = true
		}
	}
	for _, item := range result.Items {
		if ids[item.Id] {
			r.refuse("error: duplicate retained finding id " + item.Id)
		}
	}
	body := r.readResultReport()
	manifest.Results = append(manifest.Results, storedResult{Result: result, Before: resultDigest(body)})
	r.writeResultManifest(*manifest)
	return len(manifest.Results) - 1
}

func (r *run) finishResultProjection(manifest *resultManifest, index int) {
	saved := manifest.Results[index]
	body := r.readResultReport()
	if err := verifyResultItems(body, saved.Result.Items); err != nil {
		if resultDigest(body) != saved.Before {
			r.refuse("error: pending result cannot recover over changed report; restore its prior report or complete its exact finding projection: " + err.Error())
		}
		if err := visibleResultReport(body); err != nil {
			r.refuse("error: cannot render findings into report: " + err.Error())
		}
		var rendered strings.Builder
		for _, item := range saved.Result.Items {
			rendered.WriteString("\n" + renderResultItem(item) + "\n")
		}
		err := r.rewriteReport("result is pending; retry its exact payload", "result is pending; report was not updated", func(lines []string) []string {
			return append(lines, strings.Split(strings.TrimSuffix(rendered.String(), "\n"), "\n")...)
		})
		if err != nil {
			r.exit(2)
		}
	}
	// The receipt is committed only after reading the actual report, not the intended rendering.
	manifest.Results[index].IsAccepted = true
	r.assertResultProjection(*manifest)
	if err := syncResultProjection(r.report); err != nil {
		r.refuse("error: result remains pending because its report could not be synced: " + err.Error())
	}
	r.writeResultManifest(*manifest)
}

func (r *run) readResultReport() []byte {
	body, err := os.ReadFile(r.report)
	if err != nil {
		r.refuse("error: could not read report findings: " + err.Error())
	}
	return body
}

func (r *run) assertResultProjection(manifest resultManifest) {
	if err := resultProjectionProblem(r.readResultReport(), manifest); err != nil {
		r.refuse("error: stage findings are not durably recorded: " + err.Error())
	}
}

func resultProjectionProblem(body []byte, manifest resultManifest) error {
	if err := visibleResultReport(body); err != nil {
		return err
	}
	for _, saved := range manifest.Results {
		if !saved.IsAccepted {
			return fmt.Errorf("result %s is pending; retry its exact payload", saved.Result.Id)
		}
		if err := verifyResultItems(body, saved.Result.Items); err != nil {
			return err
		}
	}
	return nil
}

func (r *run) acceptedResultProblem(entry string, manifest resultManifest) string {
	stage, _, _ := strings.Cut(entry, ":")
	context := r.candidateResultContext(manifest.Context.Attempt)
	for index := len(manifest.Results) - 1; index >= 0; index-- {
		saved := manifest.Results[index]
		if string(saved.Result.Stage) != stage || saved.Result.resultContext != context {
			continue
		}
		if !saved.IsAccepted {
			return "result is pending"
		}
		expected := string(saved.Result.Stage)
		if saved.Result.Outcome != outcomeComplete {
			expected += ":" + string(saved.Result.Outcome)
		}
		if entry != expected {
			return "result outcome does not match stamp entry"
		}
		return ""
	}
	return "no accepted result for the current attempt and candidate"
}

func (r *run) resultStagesProblems(entries string) []string {
	manifest := r.readResultManifest()
	if manifest == nil {
		return []string{"no typed qualification attempt — re-qualify"}
	}
	var problems []string
	if err := resultProjectionProblem(r.readResultReport(), *manifest); err != nil {
		problems = append(problems, err.Error())
	}
	for _, entry := range strings.Split(entries, ",") {
		stage, _, _ := strings.Cut(entry, ":")
		if strings.Contains(entry, ":skipped(") {
			for _, saved := range manifest.Results {
				if string(saved.Result.Stage) == stage && saved.Result.Attempt == manifest.Context.Attempt {
					problems = append(problems, stage+": returned and cannot be recorded as skipped")
					break
				}
			}
			continue
		}
		if reason := r.acceptedResultProblem(entry, *manifest); reason != "" {
			problems = append(problems, stage+": "+reason)
		}
	}
	return problems
}

func (r *run) hasResultProjectionProblem() bool {
	manifest := r.readResultManifest()
	return manifest != nil && resultProjectionProblem(r.readResultReport(), *manifest) != nil
}
