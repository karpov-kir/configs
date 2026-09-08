package ecoroot_test

import (
	ecoroot "kk-flavor/tools/eco-root"
	"testing"
)

func TestAgentIsRequiredAndValidated(t *testing.T) {
	for _, args := range [][]string{nil, {"--agent="}, {"--agent=auto"}, {"--agent=claude", "--agent=codex"}} {
		if _, _, err := ecoroot.AgentArgs(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	agent, rest, err := ecoroot.AgentArgs([]string{"--append", "--agent=claude", "--agent=codex", "ai"})
	if err != nil || agent != "codex" || len(rest) != 3 || rest[1] != "--agent=claude" {
		t.Fatalf("opaque note lost: %s %v %v", agent, rest, err)
	}
}

func TestCodexPathsAndNoClaudeImports(t *testing.T) {
	_, dir := newCheckout(t, "")
	t.Setenv("HOME", dir)
	root, ok := ecoroot.New(dir, "codex")
	if !ok {
		t.Fatal("no root")
	}
	assertEquals(t, "skills mount", root.SkillsMount(), dir+"/.agents/skills")
	assertEquals(t, "instructions", root.InstructionFile(), dir+"/AGENTS.md")
	imports := root.ResolveImports(ecoroot.ImportScan{Read: func(string) ([]string, error) { t.Fatal("Codex must not scan Claude imports"); return nil, nil }})
	if len(imports) != 0 {
		t.Fatal(imports)
	}
	if _, ok := ecoroot.New(dir, ""); ok {
		t.Fatal("missing provider accepted")
	}
}

func TestStructuralRootCannotChooseAProviderImplicitly(t *testing.T) {
	_, dir := newCheckout(t, "")
	root, ok := ecoroot.Checkout(dir)
	if !ok {
		t.Fatal("no root")
	}
	for name, call := range map[string]func(){
		"mount":        func() { root.SkillsMount() },
		"instructions": func() { root.InstructionFile() },
		"budget":       func() { root.BudgetScope() },
		"agent":        func() { root.Agent() },
		"imports":      func() { root.ResolveImports(ecoroot.ImportScan{}) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("missing provider did not fail loudly")
				}
			}()
			call()
		})
	}
}
