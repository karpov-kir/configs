package agentguard

import (
	"strings"
	"testing"
)

// The guard refuses a general-agent dispatch with no Needs line and lets every other dispatch run: a
// narrow or built-in type, a Needs line naming a tool or a type the session predates, and any tool but
// the Agent tool.
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
	} {
		var stderr strings.Builder
		got := Run(strings.NewReader(tc.input), &stderr)
		if got != tc.want {
			t.Errorf("%s: exit %d, want %d (%s)", tc.name, got, tc.want, stderr.String())
		}
		if got == exitRefuse && (!strings.Contains(stderr.String(), "read-worker") || !strings.Contains(stderr.String(), "edit-worker")) {
			t.Errorf("%s: the refusal names neither type: %s", tc.name, stderr.String())
		}
	}
}
