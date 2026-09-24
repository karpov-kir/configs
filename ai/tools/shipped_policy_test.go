// This file holds the shipped kk-flavor/configs/models.json to what the tools resolving it depend on. These
// cases live in this package with the others that read the shipped checkout, for the reason
// shipped_tree_test.go gives.
//
// The parser's own cases stay beside it and read a fixture document. What they cannot say is anything
// about the file this repository actually ships.
package tools_test

import (
	"maps"
	"sync"
	"testing"

	modelpolicy "configs/ai/tools/model-policy"
)

// The shipped policy is the cost control surface, so every task in it must resolve for both clients.
// The assertion is the resolution itself. Two checks once asked whether the settings that came back
// named a model, and Parse already refuses any row missing one, so no document can reach those
// assertions. TestARowNamingAnEffortAndNoModelIsRefused pins that at the parser, the cheaper level.
func TestEveryShippedTaskResolvesForBothClients(t *testing.T) {
	policy := loadShippedPolicy(t)
	names := policy.TaskNames()
	if len(names) < 2 {
		t.Fatalf("shipped policy covers %d tasks", len(names))
	}
	for _, name := range names {
		for _, client := range modelpolicy.DispatchClients() {
			if _, err := policy.Resolve(modelpolicy.Request{Client: client, Task: name}); err != nil {
				t.Fatalf("%s/%s: %v", client, name, err)
			}
		}
	}
}

// The judge is the only task a tool resolves on its own. Lose its assignment from the shipped policy
// and a gate goes quiet.
func TestShippedPolicyKeepsTheJudgeCheapAndVoting(t *testing.T) {
	policy := loadShippedPolicy(t)
	for client, want := range map[string]modelpolicy.Settings{
		"claude": {Model: "haiku"},
		"codex":  {Model: "gpt-5.6-luna", Effort: "low"},
	} {
		decision, err := policy.Resolve(modelpolicy.Request{Client: client, Task: "reader-judge"})
		if err != nil || decision.Requested != want {
			t.Fatalf("judge %s = %+v, %v; want %+v", client, decision.Requested, err, want)
		}
		if decision.Rolls < 1 {
			t.Fatalf("judge %s votes with %d rolls", client, decision.Rolls)
		}
	}
}

// A task the policy does not list is refused, including one whose path starts with a skill that has a
// row. That ancestor fallback used to answer, which is what let a renamed worker keep resolving to a
// session row one tier down — see Resolve. A dozen live-looking names sit below, so what is under
// test is the refusal itself.
func TestAnUnlistedTaskIsRefusedRatherThanInherited(t *testing.T) {
	policy := loadShippedPolicy(t)
	own, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: "build/explore"})
	if err != nil || own.Requested.Model != "sonnet" || own.Kind != "worker" {
		t.Fatalf("a site with its own row = %+v, %v", own, err)
	}
	// Each of these has a listed ancestor while holding no row of its own. They are a phase of a real
	// skill, and the retired names of renamed workers, every one of them a skill prefix the resolver
	// would have answered from.
	for _, task := range []string{
		"kk-build/plan-the-change", "kk-invented/phase",
		"kk-patrol/scout", "kk-patrol/fixer", "kk-build/explore", "kk-grill/facts",
		"kk-reduce/over-cut", "kk-reduce/arbitrate", "kk-reduce/fan-out",
		"kk-reduce/reconcile", "kk-reduce/converge", "kk-reduce/repair",
		"idsd-qualify/reconcile",
	} {
		if decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: task}); err == nil {
			t.Errorf("%s resolved to %+v instead of being refused", task, decision)
		}
	}
}

// The field is the first mechanism that can run a protected lane's contract at a cheap row's tier,
// and the policy puts the tiers in no order. A downgrade added as a new row therefore reads like an
// ordinary cheap site, and the edit it really makes is to the protected one.

// The rows that name another worker's prompt, pinned. A pinned set makes adding one a decision
// somebody had to write down. The ceiling that would compare the two tiers needs an order the file
// does not carry yet.
var shippedPromptOwners = map[string]string{
	"reduce/fan-out": "kk-ecosystem",
	"reduce/repair":  "kk-edit",
}

func TestOnlyThePinnedRowsNameAnotherWorkersPrompt(t *testing.T) {
	if named := loadShippedPolicy(t).PromptOwners(); !maps.Equal(named, shippedPromptOwners) {
		t.Errorf("rows naming another worker's prompt = %v; want %v — add it here with its reason, or drop the field", named, shippedPromptOwners)
	}
}

// A row naming another worker's prompt owns none itself, so what it resolves to is the whole of its
// contract. That contract is its own tier, and the name of the worker whose prompt the spawn is
// handed.
func TestARowNamingAnotherWorkersPromptResolvesToBoth(t *testing.T) {
	policy := loadShippedPolicy(t)
	owners := policy.PromptOwners()
	if len(owners) == 0 {
		t.Fatal("no row names another worker's prompt, so this proved nothing")
	}
	for name, target := range owners {
		decision, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if decision.Kind != "worker" || decision.Worker != target {
			t.Errorf("%s resolved as %s dispatching %q; want worker dispatching %q", name, decision.Kind, decision.Worker, target)
		}
		if _, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: target}); err != nil {
			t.Errorf("%s names %s's prompt, which the policy does not assign: %v", name, target, err)
		}
	}
}

// The policy's names as a set. Every check that uses it asks whether one name is among them, so none
// of them walks the list.
func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// The shipped policy, parsed once. Seven cases across this package read it, and none of them writes
// to it.
func loadShippedPolicy(t *testing.T) *modelpolicy.Policy {
	t.Helper()
	policyOnce.Do(func() { shippedPolicy, policyErr = modelpolicy.Load(shippedPolicyPath) })
	if policyErr != nil {
		t.Fatal(policyErr)
	}
	return shippedPolicy
}

var (
	policyOnce    sync.Once
	shippedPolicy *modelpolicy.Policy
	policyErr     error
)
