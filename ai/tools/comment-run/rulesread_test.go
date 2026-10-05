package commentrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A writer's first turn reads the brief and the comments standard whole, after its prompt. Its later
// calls keep out of the rule directories, and a script run there passes. The prompt names the files
// and quotes them nowhere, so a writer that skipped them wrote without its rules.
func TestRuleReadsHoldTheWriterToItsTwoFiles(t *testing.T) {
	home := t.TempDir()
	prompted := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	after := prompted.Add(time.Minute)
	read := func(at int, message, path string, extra map[string]any) call {
		input := map[string]any{"file_path": path}
		for k, v := range extra {
			input[k] = v
		}
		return call{at: at, tool: "Read", input: input, message: message, time: after}
	}
	bash := func(at int, command string) call {
		return call{at: at, tool: "Bash", input: map[string]any{"command": command}, message: "m2", time: after}
	}
	brief, standard := "~/.kk-flavor/workers/comment-writer.md", filepath.Join(home, ".kk-flavor/standards/comments.md")
	given := []string{"ai/kk-flavor/standards/architecture/Logger.ts"}
	clean := []call{
		read(1, "m1", brief, nil), read(2, "m1", standard, nil),
		bash(3, "~/.kk-flavor/skills/kk-edit/scripts/voice-check.sh --profile=comment --source --record - <<'EOF'"),
		bash(4, "~/.kk-flavor/workers/refactor/dup-literals.sh ledger.ts"),
		read(5, "m3", "/src/configs/ai/kk-flavor/standards/architecture/Logger.ts", nil),
		{at: 6, tool: "Edit", input: map[string]any{"file_path": "/src/configs/ai/kk-flavor/standards/architecture/Logger.ts"}, message: "m4"},
		read(7, "m5", "~/.kk-flavor/skills/kk-edit/SKILL.md", nil),
	}
	if got := ruleReads(clean, given, home, prompted); len(got) != 0 {
		t.Errorf("a clean writer drew findings: %v", got)
	}
	for _, tc := range []struct {
		name  string
		calls []call
		want  string
	}{
		{"a read in part", []call{read(1, "m1", brief, map[string]any{"limit": 40}), read(2, "m1", standard, nil)}, "in part"},
		{"one file", []call{read(1, "m1", brief, nil), read(2, "m2", standard, nil)}, "reads 1 of the 2"},
		{"another call first", []call{read(1, "m1", brief, nil), read(2, "m1", standard, nil), read(3, "m1", "/tree/ledger.ts", nil)}, "call 3: the first turn"},
		{"before the prompt", []call{read(1, "m1", brief, nil), {at: 2, tool: "Read", input: map[string]any{"file_path": standard}, message: "m1", time: prompted.Add(-time.Second)}}, "before its prompt"},
		{"a third rule file", append(clean[:2:2], read(3, "m2", "~/.kk-flavor/standards/skill-protocol.md", nil)), "call 3: reads"},
		{"by shell", append(clean[:2:2], bash(3, "cat ~/.kk-flavor/standards/code-style.md")), "call 3: reads"},
		{"by the checkout", append(clean[:2:2], read(3, "m2", "/src/configs/ai/kk-flavor/workers/refactor.md", nil)), "call 3: reads"},
		{"by grep", append(clean[:2:2], call{at: 3, tool: "Grep", input: map[string]any{"pattern": "note", "path": "~/.kk-flavor/standards"}, message: "m2"}), "call 3: reads"},
		{"by a glob from the root", append(clean[:2:2], call{at: 3, tool: "Grep", input: map[string]any{"pattern": "x", "path": "~/.kk-flavor", "glob": "standards/*.md"}, message: "m2"}), "call 3: reads"},
		{"by find", append(clean[:2:2], bash(3, "find ~/.kk-flavor -name '*.md'")), "call 3: reads"},
		{"from the root by cd", append(clean[:2:2], bash(3, "cd ~/.kk-flavor && cat standards/writing.md")), "call 3: reads"},
		{"a script fed a file", append(clean[:2:2], bash(3, "bash ~/.kk-flavor/workers/x.sh<~/.kk-flavor/standards/git.md")), "call 3: reads"},
	} {
		got := strings.Join(ruleReads(tc.calls, given, home, prompted), "\n")
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: findings %q lack %q", tc.name, got, tc.want)
		}
	}
	if got := ruleReads(clean, given, home, time.Time{}); len(got) == 0 {
		t.Error("reads with no prompt time drew no finding")
	}
}

// archive-written refuses a run whose rules changed after its prompts. A writer read them from the
// mount, and its blocks would be archived under rules it never read.
func TestArchiveWrittenRefusesRulesChangedSinceThePrompts(t *testing.T) {
	c := newChange(t)
	runDir := filepath.Join(t.TempDir(), "run")
	c.seeded(runDir, t.TempDir())
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitClean || rulesHeld(runDir) == "" {
		t.Fatalf("prompts recorded no rules: exit %d %s", said.code, said.stderr)
	}
	home, _ := os.LookupEnv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".kk-flavor", "standards", "comments.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(returnFile(runDir, "A"), []byte("Block 1/1 ledger.ts:3 | OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("archive-written", "--run=run22", "--archive="+t.TempDir(), "--run-dir="+runDir)
	if said.code != exitDidNotRun || !strings.Contains(said.stderr, "the rules changed since this run's prompts") {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitDidNotRun || !strings.Contains(said.stderr, "start a new run") {
		t.Fatalf("a second prompt under other rules was written: exit %d %s", said.code, said.stderr)
	}
}

// taint holds a writer to the reads in a run whose prompts named the rules. Those prompts recorded the
// rules' sum beside the ledger.
func TestTaintHoldsAPromptedRunsWriterToTheRuleReads(t *testing.T) {
	c := newChange(t)
	runDir := filepath.Join(t.TempDir(), "run")
	c.seeded(runDir, t.TempDir())
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitClean {
		t.Fatalf("exit %d %s", said.code, said.stderr)
	}
	ledger := filepath.Join(runDir, "comment-writer-A-queue.md")
	if err := os.WriteFile(ledger, []byte("Block 1/1 ledger.ts:3 | OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := transcript(t, [3]string{"Edit", `{"file_path":"/tree/ledger.ts","old_string":"a","new_string":"b"}`, `"ok"`})
	said := c.run("taint", "--ledger="+ledger, path)
	if said.code != exitFindings || !strings.Contains(said.stdout, "rules: ") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// The backfill reads each round's verdicts through its prompt to the archived claims they weighed. A
// `none` declines them, and a later loop round's written block clears its own claims' decline.
func TestDeclinedRecordsAWritersNoneAndALoopBlockClearsIt(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	if err := os.WriteFile(returnFile(runDir, "A"), []byte("Block 1/1 ledger.ts:3 | none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("declined", "--run=run20", "--run-dir="+runDir, "--archive="+archive, "--tree="+c.top)
	if said.code != exitClean || !strings.Contains(said.stdout, "1 declined site(s) recorded") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	// A loop round writes a block on the same claims, under the prompt format of today.
	facts, _ := filepath.Glob(filepath.Join(runDir, "facts", "*", "1.facts"))
	round := filepath.Join(runDir, "review-loop", "loop-round-1", "ledger.ts", "3")
	if err := os.MkdirAll(round, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(facts[0])
	if err := os.WriteFile(filepath.Join(round, "1.facts"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := "```\nledger.ts:3 ledger.ts/3/1.facts\n```\n"
	if err := os.WriteFile(spawnFile(runDir, "loop-round-1"), []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(returnFile(runDir, "loop-round-1"), []byte("Block 1/1 ledger.ts:3 | OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said = c.run("declined", "--run=run20", "--run-dir="+runDir, "--archive="+archive, "--tree="+c.top)
	if said.code != exitClean || !strings.Contains(said.stdout, "0 declined site(s) recorded, 1 decline(s) cleared") {
		t.Fatalf("the loop round's block did not clear the decline: exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
}
