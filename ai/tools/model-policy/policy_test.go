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

// Two things at once: it lists what the file asserts, and it lists nothing else. A `task-origin`
// profile names no model — its selection arrives at run time from the invoking task — so a name
// invented for it here would send the check asking a provider about a string this file never chose.
func TestProfileModelsListsWhatTheFileAssertsAndNothingElse(t *testing.T) {
	p := policyForTest(t)
	var got []string
	for _, named := range p.ProfileModels() {
		got = append(got, named.Profile+"/"+named.Client+"/"+named.Model+"/"+named.Effort)
	}
	want := "judge/codex/helper/low judge/claude/haiku/"
	if strings.Join(got, " ") != want {
		t.Fatalf("ProfileModels = %v; want %q", got, want)
	}
}

// sample plus a twin profile at the codex effort given, and a role using it. The two cases below
// differ only in that effort: at "low" the twin repeats judge's selection exactly, at "high" it is a
// second question about one model.
func policyWithTwinProfile(t *testing.T, twinCodexEffort string) *Policy {
	t.Helper()
	raw := strings.Replace(sample,
		`"judge":{"profile":"judge","uses":["judge"]}`,
		`"judge":{"profile":"judge","uses":["judge"]},"edit":{"profile":"twin","uses":["edit"]}`, 1)
	raw = strings.Replace(raw,
		`"judge":{"codex":{"model":"helper","effort":"low"},"claude":{"model":"haiku"}}`,
		`"judge":{"codex":{"model":"helper","effort":"low"},"claude":{"model":"haiku"}},`+
			`"twin":{"codex":{"model":"helper","effort":"`+twinCodexEffort+`"},"claude":{"model":"haiku"}}`, 1)
	p, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("fixture did not parse, so this case measures nothing: %v", err)
	}
	return p
}

// One model named by two profiles at one effort is one question, so the second mention is dropped
// rather than probed again — a duplicate costs a provider call every run of the check and settles
// nothing.
func TestAModelTwoProfilesShareIsListedOnce(t *testing.T) {
	if named := policyWithTwinProfile(t, "low").ProfileModels(); len(named) != 2 {
		t.Errorf("ProfileModels = %v; want the two distinct pairs once each", named)
	}
}

// One model at two efforts is two questions, not a duplicate: a model can refuse an effort it does not
// offer, as ClientModel's measurement shows. A pair collapsed here would have the check ask about one
// of them and report a verdict for both.
func TestOneModelAtTwoEffortsIsTwoQuestions(t *testing.T) {
	efforts := map[string]bool{}
	for _, named := range policyWithTwinProfile(t, "high").ProfileModels() {
		if named.Client == "codex" && named.Model == "helper" {
			efforts[named.Effort] = true
		}
	}
	if !efforts["low"] || !efforts["high"] {
		t.Errorf("helper was asked about at %v; want both low and high", efforts)
	}
}
