package modelpolicy

import (
	"slices"
	"testing"
)

// The ceiling against a tree that has the defect, which the shipped one does not. Both clients are
// asserted: the two orders share no model name, so a check that read one list and compared against the
// other's top would report nothing and look green.
//
// The shipped tree is run past the same derivation by `ai/tools`, which is where a case may read
// kk-flavor/skills/. This is the half that watches it fail.
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
	// The same row declared for the work it keeps is no finding at all — the ceiling reads the
	// declaration, never the tier alone.
	atTop, err = policy.OrchestratorsAtTheCeiling(map[string]string{"kk-build": "holds — converses"})
	if err != nil || len(atTop) != 0 {
		t.Fatalf("a session at the top tier was reported: %v, %v", atTop, err)
	}
}
