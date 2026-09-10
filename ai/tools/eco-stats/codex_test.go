package ecostats_test

import (
	"bytes"
	"strings"
	"testing"

	ecostats "kk-flavor/tools/eco-stats"
)

func TestStatsRequiresAgentAndMeasuresCodexBudget(t *testing.T) {
	var out bytes.Buffer
	if code := ecostats.Run("stats.sh", nil, &out, &out); code != 2 {
		t.Fatal(code)
	}
	f := newRoot(t)
	f.write(f.root+"/AGENTS.md", "codex instructions @ignored.md\n")
	f.write(f.root+"/CLAUDE.md", "claude instructions must not be counted here\n")
	out.Reset()
	if code := ecostats.Run("stats.sh", []string{"--agent=codex", f.root}, &out, &out); code != 0 {
		t.Fatalf("%d: %s", code, out.String())
	}
	for _, expected := range []string{"= 5 router", "excludes global instructions and their referenced files"} {
		if !strings.Contains(out.String(), expected) {
			t.Errorf("missing %q: %s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "uncounted import") {
		t.Fatal(out.String())
	}
}

func TestDescriptionCensusRespectsSelectedAgent(t *testing.T) {
	for _, test := range []struct {
		agent            string
		mount            string
		wantDescriptions string
		wantOutside      string
	}{
		{"codex", ".agents/skills", "5 descriptions across 2 of 2 skills", "mounted outside:   9 words  (2 skill(s)"},
		{"claude", ".claude/skills", "2 descriptions across 1 of 2 skills", "mounted outside:   4 words  (1 skill(s)"},
	} {
		t.Run(test.agent, func(t *testing.T) {
			f := newRoot(t)
			f.newHome()
			mount := f.home + "/" + test.mount
			f.mkdirAll(mount)
			for _, skill := range []struct {
				name   string
				parent string
				header string
			}{
				{"local", f.root + "/kk-flavor/skills", "description: local words\n"},
				{"local-marked", f.root + "/kk-flavor/skills", "description: local marked words\ndisable-model-invocation: true\n"},
				{"outside", f.base, "description: outside description four words\n"},
				{"outside-marked", f.base, "description: outside marked description five words\ndisable-model-invocation: true\n"},
			} {
				dir := skill.parent + "/" + skill.name
				f.mkdirAll(dir)
				f.write(dir+"/SKILL.md", "---\nname: "+skill.name+"\n"+skill.header+"---\n")
				f.symlink(dir, mount+"/"+skill.name)
			}
			f.prepare()
			var out, errOut bytes.Buffer
			if code := ecostats.Run(f.self(), []string{"--agent=" + test.agent, f.root}, &out, &errOut); code != 0 {
				t.Fatalf("status %d: %s", code, errOut.String())
			}
			for _, expected := range []string{test.wantDescriptions, test.wantOutside} {
				if !strings.Contains(out.String(), expected) {
					t.Errorf("missing %q in selected-agent census:\n%s", expected, out.String())
				}
			}
		})
	}
}
