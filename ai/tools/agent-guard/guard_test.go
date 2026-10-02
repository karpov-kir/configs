package agentguard

import (
	"strings"
	"testing"
)

// The guard refuses a dispatch to the general agent with no Needs line. It lets a narrow or built-in
// type run, a Needs line naming a tool or a type the session predates, and every tool but Agent.
func TestTheGuardRefusesOnlyAGeneralDispatchNamingNoNeed(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        int
	}{
		{"general, no need", `{"tool_name":"Agent","tool_input":{"subagent_type":"general-purpose","prompt":"Review the diff."}}`, exitRefuse},
		{"omitted type", `{"tool_name":"Agent","tool_input":{"prompt":"Review the diff."}}`, exitRefuse},
		{"the older tool name", `{"tool_name":"Task","tool_input":{"subagent_type":"general-purpose","prompt":"x"}}`, exitRefuse},
		{"a need mid-sentence", `{"tool_name":"Agent","tool_input":{"prompt":"It Needs: nothing here."}}`, exitRefuse},
		{"general, needs a tool", `{"tool_name":"Agent","tool_input":{"subagent_type":"general-purpose","prompt":"Needs: Skill\n\nRun the lane."}}`, exitAllow},
		{"stale session names the type", `{"tool_name":"Agent","tool_input":{"subagent_type":"general-purpose","prompt":"Needs: edit-worker, which this session predates\n\nApply the contract."}}`, exitAllow},
		{"narrow type", `{"tool_name":"Agent","tool_input":{"subagent_type":"edit-worker","prompt":"x"}}`, exitAllow},
		{"built-in type", `{"tool_name":"Agent","tool_input":{"subagent_type":"Explore","prompt":"x"}}`, exitAllow},
		{"another tool", `{"tool_name":"Bash","tool_input":{"command":"ls"}}`, exitAllow},
		{"unreadable input", `not json`, exitAllow},
		{"codex under Agent, a role", `{"tool_name":"Agent","tool_input":{"agent_type":"light-worker","message":"Review the diff."}}`, exitAllow},
		{"codex under Agent, no role", `{"tool_name":"Agent","tool_input":{"message":"Review the diff."}}`, exitRefuse},
		{"codex under Agent, a need", `{"tool_name":"Agent","tool_input":{"message":"Needs: browser\n\nDrive it."}}`, exitAllow},
		{"codex, no role", `{"tool_name":"spawn_agent","tool_input":{"task_name":"review","message":"Review the diff."}}`, exitRefuse},
		{"codex, a role", `{"tool_name":"spawn_agent","tool_input":{"agent_type":"light-worker","message":"Review the diff."}}`, exitAllow},
		{"codex, no role, a need", `{"tool_name":"spawn_agent","tool_input":{"message":"Needs: browser\n\nDrive the page."}}`, exitAllow},
		{"an empty need", `{"tool_name":"Agent","tool_input":{"prompt":"Needs:\n\nReview the diff."}}`, exitRefuse},
	} {
		var stderr strings.Builder
		got := Run(nil, strings.NewReader(tc.input), &stderr)
		if got != tc.want {
			t.Errorf("%s: exit %d, want %d (%s)", tc.name, got, tc.want, stderr.String())
		}
		if got == exitRefuse && (!strings.Contains(stderr.String(), "read-worker") || !strings.Contains(stderr.String(), "edit-worker")) {
			t.Errorf("%s: the refusal names neither type: %s", tc.name, stderr.String())
		}
	}
}
