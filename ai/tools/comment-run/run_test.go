package commentrun

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"configs/ai/tools/repo"
)

// rulesHome is a home directory holding the rules a block is written under.
func rulesHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	for _, path := range []string{"standards/code-style.md", "workers/comment-writer.md"} {
		full := filepath.Join(home, ".kk-flavor", path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("rules "+path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
}

// change is a repository holding a base commit, an earlier head and the head, with the tree at the head.
type change struct {
	t                   *testing.T
	top                 string
	base, earlier, head string
}

func newChange(t *testing.T) *change {
	t.Helper()
	rulesHome(t)
	c := &change{t: t, top: t.TempDir()}
	c.git("init", "-q", "-b", "main")
	c.write("ledger.ts", "export function other() {}\n")
	c.base = c.commit("base")
	c.write("ledger.ts", "export function other() {}\n\n// canPost throws when its this binding is not the object that owns it.\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n")
	c.earlier = c.commit("earlier head")
	c.write("ledger.ts", "export function other() {}\n\n// A ledger build answers for its own scheme only, so this function asks it once per scheme.\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n")
	c.head = c.commit("head")
	return c
}

func (c *change) git(args ...string) string {
	c.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", c.top}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (c *change) write(name, body string) {
	c.t.Helper()
	if err := os.WriteFile(filepath.Join(c.top, name), []byte(body), 0o644); err != nil {
		c.t.Fatal(err)
	}
}

func (c *change) commit(message string) string {
	c.git("add", "-A")
	c.git("commit", "-q", "-m", message)
	return c.git("rev-parse", "HEAD")
}

// outcome is what one stage exited with and printed.
type outcome struct {
	code           int
	stdout, stderr string
}

func (c *change) run(args ...string) outcome {
	c.t.Helper()
	var out, errOut strings.Builder
	code := Run("comment-run.sh", args, c.top, repo.Exec{}, &out, &errOut)
	return outcome{code, out.String(), errOut.String()}
}

// seed strips the change set at the head after seeding the archive from the earlier head, and records
// a contradiction by the record id the trial strip names. The worktrees it made are gone after.
func TestSeedStripsTheChangeSetAgainstASeededArchive(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	tsv := filepath.Join(t.TempDir(), "claims.tsv")
	if err := os.WriteFile(tsv, []byte("ledger.ts\tcanPost throws when its this binding is not the object that owns it\trun12\tcanPost is static, so no this binding applies\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("seed", "--run-dir="+runDir, "--archive="+archive, "--range="+c.base+".."+c.head,
		"--heads="+c.earlier, "--contradictions="+tsv)
	if said.code != exitClean || !strings.Contains(said.stdout, "ledger.ts:3 ") {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", said.code, said.stdout, said.stderr)
	}
	if !strings.Contains(said.stderr, "1 contradiction(s) recorded by record id") {
		t.Fatalf("the claim was not recorded by id: %s", said.stderr)
	}
	stripped, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts"))
	if strings.Contains(string(stripped), "//") {
		t.Fatalf("the head still holds a block:\n%s", stripped)
	}
	facts, _ := filepath.Glob(filepath.Join(runDir, "facts", "ledger.ts", "*.facts"))
	var offered string
	for _, f := range facts {
		body, _ := os.ReadFile(f)
		offered += string(body)
	}
	if !strings.Contains(offered, "contradicted: run12 canPost is static") {
		t.Fatalf("the live strip offers no contradiction:\n%s", offered)
	}
	if list := c.git("worktree", "list"); strings.Count(list, "\n") != 0 {
		t.Fatalf("a worktree stayed:\n%s", list)
	}
}

// seed refuses a tree standing elsewhere than the head, or holding changes of its own.
func TestSeedRefusesATreeThatIsNotTheHead(t *testing.T) {
	c := newChange(t)
	args := []string{"seed", "--run-dir=" + t.TempDir(), "--archive=" + t.TempDir(), "--range=" + c.base + ".." + c.earlier}
	if said := c.run(args...); said.code != exitDidNotRun || !strings.Contains(said.stderr, "the tree stands at") {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
	c.write("stray.ts", "x\n")
	args[3] = "--range=" + c.base + ".." + c.head
	if said := c.run(args...); said.code != exitDidNotRun || !strings.Contains(said.stderr, "changes of its own") {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
}

// prompts partitions the sites over writers and quotes only the human's words as the emphasis.
func TestPromptsQuoteTheHumansWordsAndNoApproval(t *testing.T) {
	c := newChange(t)
	runDir := filepath.Join(t.TempDir(), "run")
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+t.TempDir(), "--range="+c.base+".."+c.head); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
	if err := os.WriteFile(filepath.Join(runDir, "licence.txt"), []byte(`"The tooling decides every comment."`), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("prompts", "--run-dir="+runDir, "--workers=3")
	if said.code != exitClean || !strings.HasPrefix(said.stdout, "A 1 site(s) in 1 file(s)") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	body, _ := os.ReadFile(filepath.Join(runDir, "spawn-writer-A.md"))
	prompt := string(body)
	for _, want := range []string{"`ledger.ts:3`", `"The tooling decides every comment."`, "comment-writer-A-queue.md"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, prompt)
		}
	}
	if regexp.MustCompile(`(?i)\bapprov`).MatchString(prompt) {
		t.Errorf("the prompt carries an approval:\n%s", prompt)
	}
}

// archive-written finds each block a writer wrote and archives it with its record, and the dry keep
// test then reads it as standing.
func TestArchiveWrittenArchivesWhatTheKeepTestReadsAsStanding(t *testing.T) {
	c := newChange(t)
	archive := filepath.Join(t.TempDir(), "archive")
	ret := filepath.Join(t.TempDir(), "writer-A.md")
	if err := os.WriteFile(ret, []byte("Block 1/1 ledger.ts:4 | OK\n```ts\n"+
		"// A ledger build answers for its own scheme only, so this function asks it once per scheme.\n"+
		"export function claimFor(scheme: string): boolean {\n```\nsummary: none\nnote: written\n"+
		"fact: a ledger build answers for its own scheme only\nbears_on: claimFor\ndoes: returns keys.canPost(scheme)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("archive-written", "--run=run14", "--archive="+archive, ret); said.code != exitClean ||
		!strings.Contains(said.stdout, "ledger.ts:3 archived") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	said := c.run("keep-test", "--archive="+archive, "ledger.ts")
	if said.code != exitClean || !strings.Contains(said.stdout, "ledger.ts:3 kept as run14 wrote it") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	c.write("ledger.ts", strings.Replace(c.git("show", "HEAD:ledger.ts")+"\n", "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
	if said := c.run("keep-test", "--archive="+archive, "ledger.ts"); said.code != exitFindings ||
		!strings.Contains(said.stdout, "rewrite: the code under it changed") {
		t.Fatalf("exit %d: %s", said.code, said.stdout)
	}
}

// transcript writes a writer's JSONL transcript from tool calls and their results.
func transcript(t *testing.T, calls ...[3]string) string {
	t.Helper()
	var lines []string
	for n, c := range calls {
		id := "t" + string(rune('a'+n))
		lines = append(lines, `{"message":{"content":[{"type":"tool_use","id":"`+id+`","name":"`+c[0]+`","input":`+c[1]+`}]}}`)
		lines = append(lines, `{"message":{"content":[{"type":"tool_result","tool_use_id":"`+id+`","content":`+c[2]+`}]}}`)
	}
	path := filepath.Join(t.TempDir(), "writer.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// taint counts a history read of a writer's file before its last write, reads the writes from the
// ledger, and prints a chained command for a hand read. A lint run naming the file, the results file
// and a read after the ledger entry are no taint. Runs 12 and 13 counted all three.
func TestTaintReadsWritesFromTheLedger(t *testing.T) {
	c := newChange(t)
	ledger := filepath.Join(t.TempDir(), "comment-writer-A-queue.md")
	if err := os.WriteFile(ledger, []byte("Block 1/1 ledger.ts:3 | OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := transcript(t,
		[3]string{"Bash", `{"command":"git show HEAD:ledger.ts"}`, `"// canPost throws\nexport function claimFor"`},
		[3]string{"Bash", `{"command":"git diff main -- ledger.ts; sed -n 1,5p other.ts"}`, `"// a comment\n"`},
		[3]string{"Edit", `{"file_path":"/tree/ledger.ts","old_string":"a","new_string":"b"}`, `"ok"`},
		[3]string{"Bash", `{"command":"npx eslint ledger.ts 2>&1 | tail -3"}`, `"clean"`},
		[3]string{"Write", `{"file_path":"/scratch/results.md","content":"ledger.ts done"}`, `"ok"`},
		[3]string{"Bash", `{"command":"echo 'Block 1/1 ledger.ts:3 | OK' >> ` + ledger + `"}`, `""`},
		[3]string{"Bash", `{"command":"git show HEAD:ledger.ts"}`, `"// canPost throws\n"`},
	)
	said := c.run("taint", "--ledger="+ledger, path)
	if said.code != exitFindings || strings.Count(said.stdout, "tainted:") != 1 || !strings.Contains(said.stdout, "tainted: call 1 ") ||
		strings.Count(said.stdout, "read by hand:") != 1 || !strings.Contains(said.stdout, "read by hand: call 2,") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// Every stage refuses what it cannot read, and says so on stderr.
func TestAStageRefusesWhatItCannotRun(t *testing.T) {
	c := newChange(t)
	for _, args := range [][]string{
		{},
		{"stitch"},
		{"seed", "--range=" + c.base},
		{"prompts", "--run-dir=" + t.TempDir(), "--workers=2"},
		{"archive-written", "--run=run14"},
		{"taint", filepath.Join(t.TempDir(), "writer.jsonl")},
		{"keep-test", "--archive=" + t.TempDir()},
	} {
		if said := c.run(args...); said.code != exitDidNotRun || !strings.Contains(said.stderr, "did NOT run") {
			t.Errorf("%v: exit %d: %s", args, said.code, said.stderr)
		}
	}
}
