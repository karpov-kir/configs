package ecoreport_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	ecoreport "kk-flavor/tools/eco-report"
)

func (f *fixture) newTypedResult(stage, id string) map[string]any {
	f.t.Helper()
	f.runReport("result-context")
	if f.status != 0 {
		f.t.Fatalf("result-context failed: %s", f.out)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(f.out), &result); err != nil {
		f.t.Fatalf("context is not JSON: %s: %v", f.out, err)
	}
	result["id"] = id
	result["stage"] = stage
	result["status"] = "complete"
	result["outcome"] = "complete"
	result["items"] = []any{}
	return result
}

func submitTypedResult(f *fixture, result map[string]any) {
	f.t.Helper()
	content, err := json.Marshal(result)
	if err != nil {
		f.t.Fatal(err)
	}
	path := f.base + "/typed-result.json"
	f.write(path, string(content))
	f.runReport("stage-result", path)
}

func typedFinding() map[string]any {
	return map[string]any{"id": "security.auth-1", "kind": "Pending evidence", "severity": "high", "action": "Confirm authorization", "evidence": "The owner of café is unknown", "recommendation": "Ask the owner"}
}

func TestTypedStageResultRetainsFindingsAcrossPasses(t *testing.T) {
	f := newShip(t, "001-typed")
	f.runReport("invalidate")
	result := f.newTypedResult("security-review", "security-1")
	result["items"] = []any{typedFinding()}
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatalf("typed result not accepted: %s", f.out)
	}
	report := f.reportPath("001-typed")
	body := f.read(report)
	for _, field := range []string{"security.auth-1", "Pending evidence", "high", "Confirm authorization", "The owner of café is unknown", "Ask the owner"} {
		if !strings.Contains(body, field) {
			t.Fatalf("finding field %q lost in report: %s", field, body)
		}
	}
	f.runReport("invalidate")
	if f.status != 0 {
		t.Fatalf("new pass lost accepted obligations: %s", f.out)
	}
	f.write(report, strings.ReplaceAll(f.read(report), "The owner of café is unknown", "evidence omitted"))
	f.runReport("invalidate")
	if f.status == 0 || !strings.Contains(f.out, "finding") {
		t.Fatalf("invalidate accepted lost finding evidence: %s", f.out)
	}
}

func TestTypedStageResultRejectsDuplicateAndStaleDelivery(t *testing.T) {
	f := newShip(t, "001-delivery")
	f.runReport("invalidate")
	result := f.newTypedResult("code-review", "code-1")
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatalf("first delivery failed: %s", f.out)
	}
	submitTypedResult(f, result)
	if f.status == 0 || !strings.Contains(f.out, "duplicate") {
		t.Fatalf("duplicate accepted: %s", f.out)
	}
	f.runReport("invalidate")
	result["id"] = "code-2"
	submitTypedResult(f, result)
	if f.status == 0 || !strings.Contains(f.out, "stale") {
		t.Fatalf("prior-attempt result accepted: %s", f.out)
	}
}

func TestTypedStageResultCannotHideAnOpenFinding(t *testing.T) {
	f := newShip(t, "001-hidden")
	f.runReport("invalidate")
	result := f.newTypedResult("security-review", "security-1")
	result["items"] = []any{typedFinding()}
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	report := f.reportPath("001-hidden")
	f.write(report, f.read(report)+"\n<!--\n")
	f.runReport("invalidate")
	if f.status == 0 {
		t.Fatalf("unclosed comment accepted: %s", f.out)
	}
}

func TestTypedStageResultRejectsMalformedOrUnfinishedReturns(t *testing.T) {
	cases := map[string]func(map[string]any){
		"failed":              func(result map[string]any) { result["status"] = "failed" },
		"incomplete":          func(result map[string]any) { delete(result, "status") },
		"null items":          func(result map[string]any) { result["items"] = nil },
		"unknown field":       func(result map[string]any) { result["surprise"] = true },
		"field alias":         func(result map[string]any) { result["STATUS"] = "complete" },
		"Unicode field alias": func(result map[string]any) { result["ſtatus"] = "complete" },
		"unknown stage":       func(result map[string]any) { result["stage"] = "another-stage" },
		"missing outcome":     func(result map[string]any) { delete(result, "outcome") },
		"partial correctness": func(result map[string]any) { result["outcome"] = "partial(cap)" },
		"duplicate findings":  func(result map[string]any) { result["items"] = []any{typedFinding(), typedFinding()} },
		"multiline injection": func(result map[string]any) {
			item := typedFinding()
			item["action"] = "Do this\n<!--"
			result["items"] = []any{item}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newShip(t, "001-invalid")
			f.runReport("invalidate")
			result := f.newTypedResult("code-review", "code-1")
			change(result)
			before := f.read(f.reportPath("001-invalid"))
			submitTypedResult(f, result)
			if f.status == 0 || before != f.read(f.reportPath("001-invalid")) {
				t.Fatalf("invalid result accepted or report changed: %s", f.out)
			}
		})
	}
	t.Run("duplicate JSON key", func(t *testing.T) {
		f := newShip(t, "001-json-key")
		f.runReport("invalidate")
		content, _ := json.Marshal(f.newTypedResult("code-review", "code-1"))
		content = append([]byte(`{"status":"failed",`), content[1:]...)
		path := f.base + "/duplicate-key.json"
		f.write(path, string(content))
		f.runReport("stage-result", path)
		if f.status == 0 || !strings.Contains(f.out, "duplicate") {
			t.Fatalf("duplicate key accepted: %s", f.out)
		}
	})
}

func TestTypedStageResultRecoversBothInterruptedWritePositions(t *testing.T) {
	for _, reportWasWritten := range []bool{false, true} {
		t.Run(map[bool]string{false: "before projection", true: "after projection"}[reportWasWritten], func(t *testing.T) {
			f := newShip(t, "001-recovery")
			f.runReport("invalidate")
			result := f.newTypedResult("security-review", "security-1")
			result["items"] = []any{typedFinding()}
			report := f.reportPath("001-recovery")
			before := f.read(report)
			submitTypedResult(f, result)
			if f.status != 0 {
				t.Fatal(f.out)
			}
			manifestPaths, err := filepath.Glob(f.repo + "/.git/idsd-stage-results/*.json")
			if err != nil || len(manifestPaths) != 1 {
				t.Fatalf("manifest unavailable: %v %v", manifestPaths, err)
			}
			var manifest map[string]json.RawMessage
			if err := json.Unmarshal([]byte(f.read(manifestPaths[0])), &manifest); err != nil {
				t.Fatal(err)
			}
			var results []map[string]json.RawMessage
			if err := json.Unmarshal(manifest["results"], &results); err != nil {
				t.Fatal(err)
			}
			results[0]["accepted"] = json.RawMessage(`false`)
			sum := sha256.Sum256([]byte(before))
			results[0]["before"], _ = json.Marshal(hex.EncodeToString(sum[:]))
			manifest["results"], _ = json.Marshal(results)
			encoded, _ := json.Marshal(manifest)
			f.write(manifestPaths[0], string(encoded))
			if !reportWasWritten {
				f.write(report, before)
			}
			f.runReport("invalidate")
			if f.status == 0 {
				t.Fatal("invalidate discarded a pending result")
			}
			submitTypedResult(f, result)
			if f.status != 0 || strings.Count(f.read(report), "Confirm authorization") != 1 {
				t.Fatalf("exact replay failed or duplicated finding: %s", f.out)
			}
			f.write(report, strings.Replace(f.read(report), "- [ ]", "- [x]", 1)+"\nThe owner approved it.\n")
			f.runReport("invalidate")
			if f.status != 0 || !strings.Contains(f.read(report), "The owner approved it.") {
				t.Fatalf("checkbox or human explanation rejected: %s", f.out)
			}
		})
	}
}

func TestTypedStageResultRejectsTreeAndHistoryMovement(t *testing.T) {
	for _, movement := range []string{"tree", "head"} {
		t.Run(movement, func(t *testing.T) {
			f := newShip(t, "001-movement")
			f.runReport("invalidate")
			result := f.newTypedResult("code-review", "code-1")
			if movement == "tree" {
				f.write(f.repo+"/tracked.txt", "changed\n")
			} else if out, status := f.git("commit", "--allow-empty", "-qm", "new head"); status != 0 {
				t.Fatal(out)
			}
			submitTypedResult(f, result)
			if f.status == 0 || !strings.Contains(f.out, "stale") {
				t.Fatalf("moving %s accepted stale result: %s", movement, f.out)
			}
		})
	}
}

func TestTypedStageResultParallelSubmissionsPreserveBothFindings(t *testing.T) {
	f := newShip(t, "001-parallel")
	f.runReport("invalidate")
	var paths []string
	for _, stage := range []string{"code-review", "security-review"} {
		result := f.newTypedResult(stage, stage+"-1")
		item := typedFinding()
		item["id"] = stage + ".finding"
		result["items"] = []any{item}
		content, _ := json.Marshal(result)
		path := f.base + "/" + stage + ".json"
		f.write(path, string(content))
		paths = append(paths, path)
	}
	var wait sync.WaitGroup
	for _, path := range paths {
		wait.Add(1)
		go func() {
			defer wait.Done()
			var output bytes.Buffer
			status := (ecoreport.Invocation{Args: []string{"stage-result", path, "001-parallel"}, Dir: f.repo, Self: f.skill + "/scripts/report.sh", Home: f.home, ConfigHome: f.configHome, Out: &output, Err: &output}).Exec()
			if status != 0 {
				t.Errorf("parallel submission refused: %s", output.String())
			}
		}()
	}
	wait.Wait()
	body, err := os.ReadFile(f.reportPath("001-parallel"))
	if err != nil || strings.Count(string(body), "Confirm authorization") != 2 {
		t.Fatalf("concurrent result lost findings: %s %v", body, err)
	}
}

func TestTypedStageResultPartialRefactorCanFinishWithoutTreeEdits(t *testing.T) {
	f := newShip(t, "001-partial-result")
	f.runReport("invalidate")
	result := f.newTypedResult("refactor", "refactor-partial")
	result["outcome"] = "partial(cap)"
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	for _, stage := range []string{"code-review", "security-review", "edit"} {
		submitTypedResult(f, f.newTypedResult(stage, stage+"-1"))
		if f.status != 0 {
			t.Fatal(f.out)
		}
	}
	f.runReport("decisions-reviewed")
	f.runReport("stamp", allStagesStampedAs)
	if f.status == 0 || !strings.Contains(f.out, "outcome") {
		t.Fatalf("partial result stamped as complete: %s", f.out)
	}
	result["id"] = "refactor-complete"
	result["outcome"] = "complete"
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatalf("resumed refactor refused: %s", f.out)
	}
	f.runReport("stamp", allStagesStampedAs)
	if f.status != 0 {
		t.Fatalf("completed refactor not stampable: %s", f.out)
	}
}

func TestTypedStageResultGateDetectsDroppedAcceptedFindings(t *testing.T) {
	f := newShip(t, "001-post-stamp")
	f.runReport("invalidate")
	for _, stage := range allStages {
		result := f.newTypedResult(stage, stage+"-1")
		if stage == "security-review" {
			result["items"] = []any{typedFinding()}
		}
		submitTypedResult(f, result)
		if f.status != 0 {
			t.Fatal(f.out)
		}
	}
	report := f.reportPath("001-post-stamp")
	f.write(report, strings.Replace(f.read(report), "- [ ]", "- [x]", 1))
	f.runReport("decisions-reviewed")
	f.runReport("stamp", allStagesStampedAs)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	f.runReport("gate")
	if f.status != 0 {
		t.Fatalf("resolved finding blocked: %s", f.out)
	}
	f.write(report, strings.Replace(f.read(report), "The owner of café is unknown", "removed", 1))
	f.runReport("gate")
	if f.status == 0 || !strings.Contains(f.out, "BLOCK (stages)") {
		t.Fatalf("stamped evidence deletion went undetected: %s", f.out)
	}
}

func TestTypedStageResultSiblingInvalidationCannotLoseFindings(t *testing.T) {
	f := newShip(t, "001-sibling-result")
	f.runReport("invalidate")
	result := f.newTypedResult("security-review", "security-1")
	result["items"] = []any{typedFinding()}
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	second := f.base + "/second"
	f.mustGit("worktree", "add", "-q", second, "-b", "second")
	if !f.exists(second + "/.git") {
		t.Skip("git worktree add is unavailable")
	}
	f.runReportIn(second, "invalidate", "001-sibling-result")
	if f.status != 0 {
		t.Fatalf("sibling could not start its own attempt: %s", f.out)
	}
	report := f.reportPath("001-sibling-result")
	f.write(report, strings.Replace(f.read(report), "The owner of café is unknown", "removed", 1))
	f.runReportIn(second, "invalidate", "001-sibling-result")
	if f.status == 0 || !strings.Contains(f.out, "finding") {
		t.Fatalf("sibling erased inherited finding obligation: %s", f.out)
	}
}

func TestTypedStageResultPromotionKeepsFindingObligations(t *testing.T) {
	f := newShip(t, "001-promoted-result")
	f.newIntentFile("001-promoted-result")
	f.runReport("invalidate")
	result := f.newTypedResult("security-review", "security-1")
	result["items"] = []any{typedFinding()}
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	f.runReport("promote")
	if f.status != 0 {
		t.Fatalf("promotion failed: %s", f.out)
	}
	report := f.repo + "/.idsd/intents/001-promoted-result/for-agents/qualify-report.md"
	f.write(report, strings.Replace(f.read(report), "The owner of café is unknown", "removed", 1))
	f.runReport("invalidate", "001-promoted-result")
	if f.status == 0 || !strings.Contains(f.out, "finding") {
		t.Fatalf("promotion lost finding obligation: %s", f.out)
	}
}

func TestTypedStageResultMissingManifestCannotResetObligations(t *testing.T) {
	f := newShip(t, "001-lost-manifest")
	f.runReport("invalidate")
	result := f.newTypedResult("security-review", "security-1")
	result["items"] = []any{typedFinding()}
	submitTypedResult(f, result)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	paths, _ := filepath.Glob(f.repo + "/.git/idsd-stage-results/*.json")
	if len(paths) != 1 {
		t.Fatal("manifest not found")
	}
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	f.runReport("invalidate")
	if f.status == 0 || !strings.Contains(f.out, "missing its stage result manifest") {
		t.Fatalf("lost manifest silently reset evidence: %s", f.out)
	}
}
