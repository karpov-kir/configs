package tools_test

import (
	"os"
	"strings"
	"testing"
)

func TestTheRulesCallTheBuildGate(t *testing.T) {
	for _, c := range []struct{ file, call string }{
		{"ai/kk-flavor/skills/kk-build/SKILL.md", "~/.kk-flavor/scripts/build-gate.sh open "},
		{"ai/kk-flavor/skills/kk-build/SKILL.md", "~/.kk-flavor/scripts/build-gate.sh stamp"},
		{"ai/kk-flavor/standards/git.md", "~/.kk-flavor/scripts/build-gate.sh check"},
	} {
		raw, err := os.ReadFile(repoRoot + "/" + c.file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), c.call) {
			t.Errorf("%s no longer calls `%s`, so the build gate no longer holds", c.file, strings.TrimSpace(c.call))
		}
	}
}
