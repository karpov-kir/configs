package ecoreport_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypedStageResultCapIncludesPendingManifestNewline(t *testing.T) {
	f := newShip(t, "001-size-boundary")
	f.runReport("invalidate")
	result := f.newTypedResult("security-review", "boundary-result")
	items := make([]map[string]any, 300)
	for index := range items {
		items[index] = typedFinding()
		items[index]["id"] = fmt.Sprintf("finding-%03d", index)
		items[index]["evidence"] = "e"
	}
	result["items"] = items
	paths, _ := filepath.Glob(f.repo + "/.git/idsd-stage-results/*.json")
	if len(paths) != 1 {
		t.Fatal("initial manifest not found")
	}
	beforeManifest := f.read(paths[0])
	beforeReport := f.read(f.reportPath("001-size-boundary"))
	var manifest map[string]any
	if err := json.Unmarshal([]byte(beforeManifest), &manifest); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(beforeReport))
	manifest["results"] = []any{map[string]any{"result": result, "accepted": false, "before": hex.EncodeToString(sum[:])}}
	encoded, _ := json.Marshal(manifest)
	const limit = 4 << 20
	remaining := limit - len(encoded)
	for _, item := range items {
		padding := min(remaining, 16383)
		item["evidence"] = "e" + strings.Repeat("a", padding)
		remaining -= padding
	}
	encoded, _ = json.Marshal(manifest)
	if remaining != 0 || len(encoded) != limit {
		t.Fatalf("fixture must hit the exact pending JSON limit: %d, remaining %d", len(encoded), remaining)
	}
	submitTypedResult(f, result)
	if f.status == 0 || !strings.Contains(f.out, "manifest exceeds") {
		t.Errorf("accepted a pending manifest that exceeds its reader's limit after newline: %s", f.out)
	}
	if f.read(paths[0]) != beforeManifest || f.read(f.reportPath("001-size-boundary")) != beforeReport {
		t.Error("oversized pending manifest changed report or receipt")
	}
}

func TestTypedStageResultCWhitespaceMatchesTodoGate(t *testing.T) {
	f := newShip(t, "001-fence-whitespace")
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
	f.runReport("decisions-reviewed")
	f.runReport("stamp", allStagesStampedAs)
	if f.status != 0 {
		t.Fatal(f.out)
	}
	report := f.reportPath("001-fence-whitespace")
	body := strings.Replace(f.read(report), "\n- [ ]", "\n\u00a0```\n```\n- [ ]", 1)
	f.write(report, body+"\n\u00a0```\n```\n")
	f.runReport("gate")
	if f.status == 0 {
		t.Error("gate accepted an open finding hidden by mismatched Unicode fence whitespace")
	}
	f.runReport("close")
	if f.status == 0 || !f.exists(report) {
		t.Error("close discarded an open finding hidden by mismatched Unicode fence whitespace")
	}
}
