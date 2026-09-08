package modelpolicy

import (
	"strings"
	"testing"
)

const sample = `{"version":1,"profiles":{"task":{"source":"task-origin"},"judge":{"codex":{"model":"helper","effort":"low"},"claude":{"model":"haiku"}}},"roles":{"implement":{"profile":"task","uses":["build"]},"correctness":{"profile":"task","uses":["review"]},"security":{"profile":"task","uses":["security"]},"judge":{"profile":"judge","uses":["judge"]}}}`

func policyForTest(t *testing.T) *Policy {
	t.Helper()
	p, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestProtectedWorkUsesOriginalTask(t *testing.T) {
	p := policyForTest(t)
	for _, role := range []string{"implement", "correctness", "security"} {
		got, err := p.Resolve(Request{Client: "codex", Role: role, Transport: "native", Origin: &Origin{Client: "codex", Model: "original-strong", Effort: "high"}})
		if err != nil || got.Requested.Model != "original-strong" || got.Requested.Effort != "high" || got.Source != "task-origin" || len(got.PolicyDigest) != 64 {
			t.Fatalf("%s lost original-task settings: %+v, %v", role, got, err)
		}
	}
}
func TestPolicyRejectsMalformedAndProtectedDowngrades(t *testing.T) {
	for name, raw := range map[string]string{
		"wrong key case":   strings.Replace(sample, `"version":1`, `"Version":1`, 1),
		"unknown field":    strings.Replace(sample, `"version":1`, `"version":1,"typo":true`, 1),
		"duplicate field":  strings.Replace(sample, `"version":1`, `"version":1,"version":1`, 1),
		"nested duplicate": strings.Replace(sample, `"model":"helper"`, `"model":"helper","model":"other"`, 1),
		"unknown client":   strings.Replace(sample, `"codex":`, `"other":`, 1),
		"version":          strings.Replace(sample, `"version":1`, `"version":2`, 1),
		"trailing value":   sample + ` {}`,
		"null":             `null`,
		"downgrade":        strings.Replace(sample, `"implement":{"profile":"task"`, `"implement":{"profile":"judge"`, 1),
		"mixed source":     strings.Replace(sample, `"source":"task-origin"`, `"source":"task-origin","codex":{"model":"helper"}`, 1),
		"unknown profile":  strings.Replace(sample, `"profile":"task"`, `"profile":"absent"`, 1),
		"duplicate use":    strings.Replace(sample, `"uses":["review"]`, `"uses":["build"]`, 1),
		"unknown effort":   strings.Replace(sample, `"effort":"low"`, `"effort":"turbo"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(raw)); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}
func TestInheritanceRequiresOriginalTaskNativeDispatch(t *testing.T) {
	p := policyForTest(t)
	for _, r := range []Request{
		{Client: "codex", Role: "implement", Transport: "cli", FromOriginalTask: true},
		{Client: "codex", Role: "implement", Transport: "native"},
		{Client: "codex", Role: "implement", Transport: "native", Origin: &Origin{Client: "claude", Model: "other", Effort: "high"}},
		{Client: "unknown", Role: "judge", Transport: "cli"},
		{Client: "codex", Role: "unknown", Transport: "cli"},
		{Client: "codex", Role: "judge", Transport: "unknown"},
		{Client: "codex", Role: "judge", Transport: "cli", PolicyDigest: "stale"},
	} {
		if got, err := p.Resolve(r); err == nil {
			t.Fatalf("unsafe dispatch accepted: %+v -> %+v", r, got)
		}
	}
	got, err := p.Resolve(Request{Client: "codex", Role: "implement", Transport: "native", FromOriginalTask: true})
	if err != nil || got.Source != "original-task-native-inheritance" || got.Requested.Model != "" {
		t.Fatalf("original native inheritance = %+v, %v", got, err)
	}
}
func TestJudgeResolvesWithoutOriginAndDoesNotInventEffort(t *testing.T) {
	p := policyForTest(t)
	got, err := p.Resolve(Request{Client: "claude", Role: "judge", Transport: "cli"})
	if err != nil || got.Requested.Model != "haiku" || got.Requested.Effort != "" || got.Source != "profile" {
		t.Fatalf("judge = %+v, %v", got, err)
	}
}

func TestUnknownOriginEffortRequiresOriginalNativeParent(t *testing.T) {
	p := policyForTest(t)
	request := Request{Client: "codex", Role: "implement", Transport: "cli", Origin: &Origin{Client: "codex", Model: "original"}}
	if _, err := p.Resolve(request); err == nil {
		t.Fatal("CLI accepted unknown origin effort")
	}
	request.Transport = "native"
	request.FromOriginalTask = true
	if got, err := p.Resolve(request); err != nil || got.Source != "original-task-native-inheritance" {
		t.Fatalf("native fallback: %+v %v", got, err)
	}
}
