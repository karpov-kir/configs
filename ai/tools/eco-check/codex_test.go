package ecocheck_test

import (
	"bytes"
	ecocheck "kk-flavor/tools/eco-check"
	"strings"
	"testing"
)

func TestCodexBudgetAndInvocationPolicy(t *testing.T) {
	f := newRoot(t)
	f.write(f.root+"/AGENTS.md", "codex instructions @ignored.md\n")
	f.write(f.root+"/CLAUDE.md", "claude instructions must not be counted here\n")
	f.mkdirAll(f.root + "/kk-flavor/skills/manual/agents")
	f.write(f.root+"/kk-flavor/skills/manual/SKILL.md", "---\nname: manual\ndescription: two words\ndisable-model-invocation: true\n---\n")
	var out bytes.Buffer
	ecocheck.Run([]string{"--agent=codex", f.root}, &out, &out)
	for _, expected := range []string{"5 words across 2 files", "excludes global instructions and their referenced files", "2 words of skill description across 1 of 1", "Codex invocation policy mismatch"} {
		if !strings.Contains(out.String(), expected) {
			t.Errorf("missing %q: %s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "uncounted import") {
		t.Fatal(out.String())
	}
	f.write(f.root+"/kk-flavor/skills/manual/agents/openai.yaml", "policy:\n  allow_implicit_invocation: false\n")
	out.Reset()
	ecocheck.Run([]string{"--agent=codex", f.root}, &out, &out)
	if strings.Contains(out.String(), "Codex invocation policy mismatch") {
		t.Fatal(out.String())
	}
}

func TestCheckRequiresExplicitAgent(t *testing.T) {
	var out bytes.Buffer
	if code := ecocheck.Run(nil, &out, &out); code != 2 || !strings.Contains(out.String(), "--agent=claude|codex is required") {
		t.Fatalf("code %d: %s", code, out.String())
	}
}
