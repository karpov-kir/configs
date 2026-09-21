package modelpolicy

import (
	"slices"
	"testing"
)

// The two tier orders share no model name. A check reading one list and comparing it against the
// other's top reports an empty ceiling and looks green, so this case asserts both clients.

// `ai/tools` runs the shipped tree past the same derivation, since a case there may read
// kk-flavor/skills/. This case is the half that watches the derivation fail.

// The ceiling against a tree that has the defect, which the shipped tree does not.
func TestTheCeilingCatchesAnOrchestratorAtTheTopTier(t *testing.T) {
	policy, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	atTop, err := policy.OrchestratorsAtTheCeiling(map[string]string{"kk-build": "orchestrator"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"claude/kk-build", "codex/kk-build"}; !slices.Equal(atTop, want) {
		t.Fatalf("at the ceiling: %v; want %v", atTop, want)
	}
	// The same row declared for the work it keeps comes back clean, because the ceiling reads the
	// declaration as well as the tier.
	atTop, err = policy.OrchestratorsAtTheCeiling(map[string]string{"kk-build": "holds — converses"})
	if err != nil || len(atTop) != 0 {
		t.Fatalf("a session at the top tier was reported: %v, %v", atTop, err)
	}
}
