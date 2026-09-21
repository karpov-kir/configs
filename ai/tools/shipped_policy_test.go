// The shipped kk-flavor/models.json, held to what the tools resolving it depend on. These live here
// rather than under `ai/tools/model-policy/` for the reason shipped_tree_test.go's do.
//
// The parser's own cases stay beside it and read a fixture document. What they cannot say is anything
// about the file this repository actually ships.
package tools_test

import (
	"maps"
	"testing"

	modelpolicy "configs/ai/tools/model-policy"
)

// The shipped policy is the cost control surface, so every task in it must resolve for both clients.
// Resolving is the whole assertion: two checks here once asked whether the settings that came back
// named a model, and Parse refuses a row that does not, so they became assertions no document could
// reach. TestARowNamingAnEffortAndNoModelIsRefused pins that at the parser, which is the cheaper level.
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

// The judge is the only task a tool resolves on its own, so its assignment is the one the shipped
// policy cannot lose without a gate going quiet.
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
// session row one tier down — see Resolve. A dozen live-looking names sit below, so the refusal is the
// thing under test, not an edge case.
func TestAnUnlistedTaskIsRefusedRatherThanInherited(t *testing.T) {
	policy := loadShippedPolicy(t)
	own, err := policy.Resolve(modelpolicy.Request{Client: "claude", Task: "build/explore"})
	if err != nil || own.Requested.Model != "sonnet" || own.Kind != "worker" {
		t.Fatalf("a site with its own row = %+v, %v", own, err)
	}
	// Each of these has a listed ancestor and no row of its own: a phase of a real skill, and the retired
	// names of renamed workers — every one of them a skill prefix the resolver would have answered from.
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

// The rows that name another worker's prompt, pinned. The field is the first mechanism that can run a
// protected lane's contract at a cheap row's tier, and nothing in the policy orders the tiers — so a
// downgrade added as a new row reads like an ordinary cheap site rather than an edit to the protected
// one. Pinning the set is what makes adding one a decision somebody had to write down; the ceiling
// that would compare the two tiers needs an order the file does not carry yet.
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
// contract: its own tier, and the name of the worker whose prompt the spawn is handed.
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

// The policy's names as a set: every check that uses it asks whether one name is among them rather
// than walking the list.
func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func loadShippedPolicy(t *testing.T) *modelpolicy.Policy {
	t.Helper()
	policy, err := modelpolicy.Load(shippedPolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}
