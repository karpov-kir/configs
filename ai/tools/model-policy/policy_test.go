package modelpolicy

import (
	"strings"
	"testing"
)

const sample = `{"version":3,"limits":{"intents-in-flight":10},` +
	`"sessions":{"kk-build":{"codex":{"effort":"high"},"claude":{"model":"opus"}}},` +
	`"workers":{` +
	`"bloat-judge":{"codex":{"model":"helper","effort":"low"},"claude":{"model":"haiku"},"rolls":3},` +
	`"build/explore":{"codex":{"effort":"low"},"claude":{"model":"sonnet"}}}}`

func policyForTest(t *testing.T) *Policy {
	t.Helper()
	p, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEachClientGetsItsOwnSettingsAndRolls(t *testing.T) {
	p := policyForTest(t)
	got, err := p.Resolve(Request{Client: "claude", Task: "bloat-judge"})
	if err != nil || got.Requested.Model != "haiku" || got.Requested.Effort != "" || got.Rolls != 3 {
		t.Fatalf("claude judge = %+v, %v", got, err)
	}
	if len(got.PolicyDigest) != 64 {
		t.Fatalf("digest = %q", got.PolicyDigest)
	}
	got, err = p.Resolve(Request{Client: "codex", Task: "bloat-judge"})
	if err != nil || got.Requested.Model != "helper" || got.Requested.Effort != "low" {
		t.Fatalf("codex judge = %+v, %v", got, err)
	}
}

// An effort with no model is the one lever that keeps a site on the caller's model, so it must survive
// resolution rather than being filled in with a guess.
func TestEffortWithoutModelResolvesAndInventsNoModel(t *testing.T) {
	got, err := policyForTest(t).Resolve(Request{Client: "codex", Task: "kk-build"})
	if err != nil || got.Requested.Effort != "high" || got.Requested.Model != "" {
		t.Fatalf("effort-only row = %+v, %v", got, err)
	}
}

func TestUnassignedTaskFailsRatherThanInheriting(t *testing.T) {
	p := policyForTest(t)
	for _, request := range []Request{
		{Client: "claude", Task: "kk-code-review"},
		{Client: "claude", Task: ""},
		{Client: "unknown", Task: "bloat-judge"},
	} {
		if got, err := p.Resolve(request); err == nil {
			t.Fatalf("unassigned dispatch accepted: %+v -> %+v", request, got)
		}
	}
}

func TestPolicyRejectsMalformedDocuments(t *testing.T) {
	for name, raw := range map[string]string{
		// These three keep the version valid on purpose: Go's decoder matches keys case-insensitively
		// and takes the last of a duplicate pair, so a wrong version would fail them on the version
		// alone and prove nothing about strictness.
		"wrong key case":   strings.Replace(sample, `"version":3`, `"Version":3`, 1),
		"unknown field":    strings.Replace(sample, `"version":3`, `"version":3,"typo":true`, 1),
		"duplicate field":  strings.Replace(sample, `"version":3`, `"version":3,"version":3`, 1),
		"nested duplicate": strings.Replace(sample, `"model":"helper"`, `"model":"helper","model":"other"`, 1),
		"unknown client":   strings.Replace(sample, `"codex":{"model":"helper"`, `"other":{"model":"helper"`, 1),
		"version 1":        strings.Replace(sample, `"version":3`, `"version":1`, 1),
		"trailing value":   sample + ` {}`,
		"null":             `null`,
		"no tasks":         `{"version":3,"limits":{"intents-in-flight":10},"sessions":{},"workers":{}}`,
		"no cap":           strings.Replace(sample, `"intents-in-flight":10`, `"intents-in-flight":0`, 1),
		"one client only":  strings.Replace(sample, `"claude":{"model":"haiku"},`, ``, 1),
		"empty settings":   strings.Replace(sample, `"claude":{"model":"haiku"}`, `"claude":{}`, 1),
		"unknown effort":   strings.Replace(sample, `"effort":"low"`, `"effort":"turbo"`, 1),
		// The other row of the effort table: three effort names are codex's alone, so one on a claude
		// entry reads as a tier and sets nothing. Without this case the two halves of the set can be
		// merged into one and every case here stays green.
		"codex-only effort on claude": strings.Replace(sample, `"claude":{"model":"opus"}`, `"claude":{"model":"opus","effort":"ultra"}`, 1),
		"claude effort alone":         strings.Replace(sample, `"claude":{"model":"opus"}`, `"claude":{"effort":"high"}`, 1),
		"option-shaped model":         strings.Replace(sample, `"model":"helper"`, `"model":"--dangerously-skip-permissions"`, 1),
		"even rolls":                  strings.Replace(sample, `"rolls":3`, `"rolls":4`, 1),
		"rolls over the cap":          strings.Replace(sample, `"rolls":3`, `"rolls":31`, 1),
		"cap over the ceiling":        strings.Replace(sample, `"intents-in-flight":10`, `"intents-in-flight":999999`, 1),
		"task name with space":        strings.Replace(sample, `"bloat-judge":`, `"bloat judge":`, 1),
		"negative rolls":              strings.Replace(sample, `"rolls":3`, `"rolls":-1`, 1),
		// Each way the field naming another row's prompt can name nothing, refused at parse time rather
		// than at the dispatch that would read it.
		"prompt owner is no row": strings.Replace(sample, `"claude":{"model":"sonnet"}`,
			`"claude":{"model":"sonnet"},"worker":"absent"`, 1),
		"prompt owner is a session": strings.Replace(sample, `"claude":{"model":"sonnet"}`,
			`"claude":{"model":"sonnet"},"worker":"kk-build"`, 1),
		"row names itself as its prompt's owner": strings.Replace(sample, `"claude":{"model":"sonnet"}`,
			`"claude":{"model":"sonnet"},"worker":"build/explore"`, 1),
		"session names a prompt owner": strings.Replace(sample, `"claude":{"model":"opus"}`,
			`"claude":{"model":"opus"},"worker":"bloat-judge"`, 1),
		"prompt owner names one in turn": strings.NewReplacer(
			`"claude":{"model":"sonnet"}`, `"claude":{"model":"sonnet"},"worker":"bloat-judge"`,
			`"rolls":3`, `"rolls":3,"worker":"build/explore"`).Replace(sample),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(raw)); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}

// The digest is the judge's cache identity, so a changed assignment has to change it or a stale
// verdict is reused under the new policy.
func TestDigestTracksEveryAssignment(t *testing.T) {
	first := policyForTest(t)
	changed, err := Parse([]byte(strings.Replace(sample, `"model":"haiku"`, `"model":"sonnet"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == changed.Digest() {
		t.Fatal("a changed model left the digest alone")
	}
	rerolled, err := Parse([]byte(strings.Replace(sample, `"rolls":3`, `"rolls":5`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == rerolled.Digest() {
		t.Fatal("a changed roll count left the digest alone")
	}
}

func TestARowNamingAnotherWorkersPromptKeepsItsOwnTier(t *testing.T) {
	raw := strings.Replace(sample, `"claude":{"model":"sonnet"}`,
		`"claude":{"model":"sonnet"},"worker":"bloat-judge"`, 1)
	p, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Resolve(Request{Client: "claude", Task: "build/explore"})
	if err != nil || got.Requested.Model != "sonnet" || got.Worker != "bloat-judge" || got.Kind != "worker" {
		t.Fatalf("a row naming another's prompt = %+v, %v", got, err)
	}
	if named := p.PromptOwners(); len(named) != 1 || named["build/explore"] != "bloat-judge" {
		t.Fatalf("PromptOwners() = %v", named)
	}
	// The rows that own their prompt say nothing, so a caller reads the field as "someone else's".
	own, err := p.Resolve(Request{Client: "claude", Task: "bloat-judge"})
	if err != nil || own.Worker != "" {
		t.Fatalf("a row owning its prompt = %+v, %v", own, err)
	}
}

// The self-reference refusal is observed only by its message: the chain refusal below it catches a
// one-row cycle too. A row naming itself declares no prompt at all, and being told it names a further
// row's prompt in turn sends its author hunting for a second row that was never there.
func TestARowNamingItselfIsToldThatRatherThanToldOfAChain(t *testing.T) {
	raw := strings.Replace(sample, `"claude":{"model":"sonnet"}`,
		`"claude":{"model":"sonnet"},"worker":"build/explore"`, 1)
	_, err := Parse([]byte(raw))
	if err == nil {
		t.Fatal("a row naming itself as the prompt it dispatches was accepted")
	}
	if !strings.Contains(err.Error(), "names itself") {
		t.Fatalf("the refusal does not say the row names itself: %v", err)
	}
}
