package commentrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"configs/ai/tools/repo"
	treefingerprint "configs/ai/tools/tree-fingerprint"
)

// rulesHome is a home directory holding the rules a block is written under.
func rulesHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	for _, path := range writerRuleFiles {
		full := filepath.Join(home, ".kk-flavor", path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("## Comments\n\nrules "+path+"\n\n## Next\n\nout of the section\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	template, err := os.ReadFile("../../kk-flavor/templates/spawn-prompt.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".kk-flavor", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, spawnTemplate), template, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
}

const claimForDecl = "export function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n"

const headLedger = "export function other() {}\n\n// A ledger build answers for its own scheme only, so this function asks it once per scheme.\n" + claimForDecl

const carriedClaimFor = "export function claimFor(scheme: string): boolean {\n  return ASKS_ONCE_PER_SCHEME && keys.canPost(scheme);\n}\n"

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
	c.write("ledger.ts", "export function other() {}\n\n// canPost throws when its this binding is not the object that owns it.\n"+claimForDecl)
	c.earlier = c.commit("earlier head")
	c.write("ledger.ts", headLedger)
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

// seeded seeds a run over the change from its base to its head, and fails the case where seed refuses.
func (c *change) seeded(runDir, archive string, extra ...string) {
	c.t.Helper()
	if said := c.run(append([]string{"seed", "--run-dir=" + runDir, "--archive=" + archive, "--range=" + c.base + ".." + c.head}, extra...)...); said.code != exitClean {
		c.t.Fatalf("seed: %s", said.stderr)
	}
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
	c.seeded(runDir, t.TempDir())
	if err := os.WriteFile(filepath.Join(runDir, "licence.txt"), []byte(`"The tooling decides every comment."`), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("prompts", "--run-dir="+runDir)
	// The test home mounts no comment-writer agent, and the stage says so.
	if !strings.Contains(said.stderr, "no comment-writer agent at ~/.claude/agents/comment-writer.md") {
		t.Errorf("a home with no writer agent went unsaid:\n%s", said.stderr)
	}
	if said.code != exitClean || !strings.HasPrefix(said.stdout, "A batch 1, 1 site(s) in 1 file(s)") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	body, _ := os.ReadFile(filepath.Join(runDir, "spawn-writer-A.md"))
	prompt := string(body)
	for _, want := range []string{"ledger.ts:3 1.facts", "the root being `" + filepath.Join(runDir, "facts") + "`",
		`"The tooling decides every comment."`, "comment-writer-A-queue.md"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, prompt)
		}
	}
	// A 135-site prompt named each site three times. The strip's stdout names it once.
	if strings.Count(prompt, "ledger.ts:3") != 1 {
		t.Errorf("the prompt names its site other than once:\n%s", prompt)
	}
	if !strings.HasSuffix(said.stdout, dispatchLine+"\n") {
		t.Errorf("the stage does not say how a prompt reaches its writer:\n%s", said.stdout)
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
		[3]string{"Bash", `{"command":"git log --graph --oneline -3"}`, `"* 1a2b3c4 head\n* 5d6e7f8 earlier head"`},
		[3]string{"Bash", `{"command":"git diff main -- ledger.ts; sed -n 1,5p other.ts"}`, `"// a comment\n"`},
		[3]string{"Edit", `{"file_path":"/tree/ledger.ts","old_string":"a","new_string":"b"}`, `"ok"`},
		[3]string{"Bash", `{"command":"npx eslint ledger.ts 2>&1 | tail -3"}`, `"clean"`},
		[3]string{"Write", `{"file_path":"/scratch/results.md","content":"ledger.ts done"}`, `"ok"`},
		[3]string{"Bash", `{"command":"echo 'Block 1/1 ledger.ts:3 | OK' >> ` + ledger + `"}`, `""`},
		[3]string{"Bash", `{"command":"git show HEAD:ledger.ts"}`, `"// canPost throws\n"`},
	)
	said := c.run("taint", "--ledger="+ledger, path)
	if said.code != exitFindings || strings.Count(said.stdout, "tainted:") != 1 || !strings.Contains(said.stdout, "tainted: call 1 ") ||
		strings.Count(said.stdout, "read by hand:") != 1 || !strings.Contains(said.stdout, "read by hand: call 3,") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// A diff with no range prints HEAD's blocks under a `-`, and run 18 tainted two writers that way while
// this stage stayed quiet. A write by shell script is reported too, since writes go through the Edit tool.
// A script that writes scratch files and hands the source file to a check as `--file=` writes no source.
func TestTaintReadsARangelessDiffAndAScriptWrite(t *testing.T) {
	c := newChange(t)
	ledger := filepath.Join(t.TempDir(), "comment-writer-A-queue.md")
	if err := os.WriteFile(ledger, []byte("Block 1/1 ledger.ts:3 | OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := transcript(t,
		[3]string{"Bash", `{"command":"git diff -- ledger.ts"}`, `"@@ -2,3 +2,2 @@\n-// canPost throws for another scheme\n export function claimFor"`},
		[3]string{"Bash", `{"command":"sed -i '' '3i\\\\// a note' ledger.ts"}`, `""`},
		[3]string{"Bash", `{"command":"printf 'fact: x' | voice-check.sh --source --record --file=ledger.ts - 2>&1 | tail -3"}`, `"clean"`},
		[3]string{"Bash", `{"command":"cat new.ts > ledger.ts"}`, `""`},
		[3]string{"Bash", `{"command":"python3 - <<'PY'\nopen('vc2.txt','w').write(open('vc.txt').read())\nPY\nvoice-check.sh --profile=comment --source --record --file=ledger.ts - < vc2.txt"}`, `"clean"`},
		[3]string{"Bash", `{"command":"sed -i '' 's/yet/and/' vc2.txt && voice-check.sh --profile=comment --source --record --file=ledger.ts - < vc2.txt"}`, `"clean"`},
		[3]string{"Edit", `{"file_path":"/tree/ledger.ts","old_string":"a","new_string":"b"}`, `"ok"`},
	)
	said := c.run("taint", "--ledger="+ledger, path)
	if said.code != exitFindings || !strings.Contains(said.stdout, "tainted: call 1 ") || !strings.Contains(said.stdout, "script write: call 2 ") ||
		strings.Contains(said.stdout, "script write: call 6 ") ||
		strings.Contains(said.stdout, "script write: call 3 ") || !strings.Contains(said.stdout, "script write: call 4 ") ||
		strings.Contains(said.stdout, "script write: call 5 ") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// usage prints a writer's context at its first tool call and at its end, the tokens it wrote, its calls
// and its wall time. Run 18's report estimated a writer's start-up from totals.
func TestUsageReadsATranscriptsFigures(t *testing.T) {
	c := newChange(t)
	path := filepath.Join(t.TempDir(), "writer-A.jsonl")
	lines := []string{
		`{"timestamp":"2026-09-29T10:00:00Z","message":{"id":"m1","content":[{"type":"text"}],"usage":{"input_tokens":2,"cache_read_input_tokens":100,"cache_creation_input_tokens":40000,"output_tokens":10}}}`,
		`{"timestamp":"2026-09-29T10:00:05Z","message":{"id":"m2","content":[{"type":"text"}],"usage":{"input_tokens":2,"cache_read_input_tokens":40100,"cache_creation_input_tokens":900,"output_tokens":20}}}`,
		`{"timestamp":"2026-09-29T10:00:05Z","message":{"id":"m2","content":[{"type":"tool_use","id":"a","name":"Read","input":{}}],"usage":{"input_tokens":2,"cache_read_input_tokens":40100,"cache_creation_input_tokens":900,"output_tokens":20}}}`,
		`{"timestamp":"2026-09-29T10:00:09Z","message":{"content":[{"type":"tool_result","tool_use_id":"a","content":"x"}]}}`,
		`{"timestamp":"2026-09-29T10:01:40Z","message":{"id":"m3","content":[{"type":"tool_use","id":"b","name":"Edit","input":{}}],"usage":{"input_tokens":2,"cache_read_input_tokens":41000,"cache_creation_input_tokens":3000,"output_tokens":30}}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("usage", path, path)
	// Turns count a message once, though the transcript writes it on two lines.
	if said.code != exitClean || !strings.Contains(said.stdout, "writer-A.jsonl | 3 | 40102 | 41002 | 44002 | 81200 | 43900 | 60 | 2 | 100 s") ||
		!strings.Contains(said.stdout, "total | 6 | | | | 162400 | 87800 | 120 | 4 | 200 s") {
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
		{"prompts", "--run-dir=" + t.TempDir()},
		{"prompts", "--run-dir=" + t.TempDir(), "--workers=2"},
		{"archive-written", "--run=run14"},
		{"taint", filepath.Join(t.TempDir(), "writer.jsonl")},
		{"keep-test", "--archive=" + t.TempDir()},
		{"usage"},
		{"revert", "--run=run18"},
		{"revert", "--run=run18", "--run-dir=" + t.TempDir(), "--archive=" + t.TempDir(), "../outside.ts"},
	} {
		if said := c.run(args...); said.code != exitDidNotRun || !strings.Contains(said.stderr, "did NOT run") {
			t.Errorf("%v: exit %d: %s", args, said.code, said.stderr)
		}
	}
}

// The prompt is the ecosystem's spawn template with every slot it names filled or, where empty,
// omitted. A copy of the template in the tool drifted from the template every other dispatch fills.
func TestPromptsFillEverySlotTheTemplateNames(t *testing.T) {
	c := newChange(t)
	runDir := filepath.Join(t.TempDir(), "run")
	c.seeded(runDir, t.TempDir())
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitClean {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
	body, _ := os.ReadFile(filepath.Join(runDir, "spawn-writer-A.md"))
	template, _ := os.ReadFile("../../kk-flavor/templates/spawn-prompt.md")
	for _, paragraph := range strings.Split(string(template), "\n\n") {
		if strings.HasPrefix(paragraph, "You are spawned") {
			continue
		}
		if at := strings.Index(paragraph, ": <"); at >= 0 && strings.Contains(string(body), paragraph[at+2:]) {
			t.Errorf("a slot is left as its placeholder: %s", paragraph[at+2:])
		}
	}
	for _, want := range []string{"Apply the `" + writerContract + "` contract", "User-stated emphasis", "none", "ledger.ts:3 1.facts",
		verdictSentence, "return-writer-A.md", "Edit calls issued together", "Work a file at a time", readSentence} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the prompt lacks %q:\n%s", want, body)
		}
	}
	// The prompt names the rules and quotes neither file.
	if strings.Contains(string(body), "rules standards/comments.md") || strings.Contains(string(body), "rules workers/comment-writer.md") {
		t.Errorf("the prompt quotes a rule file:\n%s", body)
	}
	if strings.Contains(string(body), "Patch queue") || strings.Contains(string(body), "Reached by a handoff") {
		t.Errorf("an empty slot stayed:\n%s", body)
	}
	// A slot the template gains and the tool does not know refuses the prompt.
	home, _ := os.LookupEnv("HOME")
	grown := string(template) + "\n\nBudget: <the calls this spawn may spend>\n"
	if err := os.WriteFile(filepath.Join(home, spawnTemplate), []byte(grown), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitDidNotRun ||
		!strings.Contains(said.stderr, "a slot this tool does not fill: Budget") {
		t.Fatalf("a grown template ran: exit %d: %s", said.code, said.stderr)
	}
}

// archive-written reads the verdict shapes run 14's writers returned: a note after the line, the site
// in brackets or parentheses, and a path dropped after the first verdict. A verdict it cannot place is
// refused by writer and line, with the shape it wants.
func TestArchiveWrittenReadsTheVerdictShapesWritersReturn(t *testing.T) {
	c := newChange(t)
	c.write("book.ts", "// A ledger closes a book at midnight, so this constant holds the hour.\nexport const CLOSE_HOUR = 0;\n\n"+
		"// A ledger rounds to cents, so this constant holds two places.\nexport const CENT_PLACES = 2;\n\n"+
		"// A ledger names its schemes in lower case, so this constant holds the casing.\nexport const SCHEME_CASE = 'lower';\n")
	block := func(comment, code string) string {
		return "```ts\n" + comment + "\n" + code + "\n```\nfact: " + strings.TrimPrefix(comment, "// ") + "\nbears_on: x\ndoes: none\n"
	}
	ret := filepath.Join(t.TempDir(), "writer-C-results.md")
	body := "Block 1/4 book.ts:1 (landed on line 1) | OK\n" + block("// A ledger closes a book at midnight, so this constant holds the hour.", "export const CLOSE_HOUR = 0;") +
		// Run 26's loop writer fenced a block together with the code line that precedes it.
		"Block 2/4 :3 (landed on line 4) | OK\n" + strings.Replace(block("// A ledger rounds to cents, so this constant holds two places.", "export const CENT_PLACES = 2;"),
		"```ts\n", "```ts\nexport const CLOSE_HOUR = 0;\n\n", 1) +
		"Block 3/4 book.ts:7 [site :6] | OK\n" + block("// A ledger names its schemes in lower case, so this constant holds the casing.", "export const SCHEME_CASE = 'lower';") +
		"Block 4/4 the header | OK\n"
	if err := os.WriteFile(ret, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("archive-written", "--run=run15", "--archive="+t.TempDir(), ret)
	if said.code != exitFindings || strings.Count(said.stdout, " archived\n") != 3 {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
	if !strings.Contains(said.stdout, "writer-C-results.md:") || !strings.Contains(said.stdout, "names no <path>:<line>; the shape is `Block N/M <path>:<offered line> | OK`") {
		t.Fatalf("the unplaced verdict was not refused by writer, line and shape:\n%s", said.stdout)
	}
}

// loop records the contradiction, strips that site, adds the review's sentence to its facts file and
// fills the loop writer's prompt with the run tree's fingerprint. Run 14's runner computed the
// fingerprint in its session's own worktree, so the stage runs from elsewhere here.
func TestLoopSendsOneSiteBackWithTheRunTreesFingerprint(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	// The writer wrote a block back at the site, and review found its claim false.
	c.write("ledger.ts", "export function other() {}\n\n// A ledger build answers for every scheme, so this function asks it once.\n"+claimForDecl)
	var out, errOut strings.Builder
	code := Run("comment-run.sh", []string{"loop", "--run-dir=" + runDir, "--archive=" + archive, "--run=run15",
		"--contradict=canPost answers for its own scheme only", "ledger.ts:4", "canPost answers for its own scheme only"},
		t.TempDir(), repo.Exec{}, &out, &errOut)
	if code != exitClean {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	prompt, err := os.ReadFile(strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatalf("no prompt at %q: %v", out.String(), err)
	}
	fingerprint, err := treefingerprint.Fingerprint(c.top)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), verdictSentence) {
		t.Errorf("the loop prompt asks for no verdict shape:\n%s", prompt)
	}
	if !strings.Contains(string(prompt), fingerprint) {
		t.Errorf("the prompt names no fingerprint of the run's tree %s:\n%s", fingerprint, prompt)
	}
	facts, _ := filepath.Glob(filepath.Join(runDir, "review-loop", "*", "*", "*", "*.facts"))
	if len(facts) != 1 {
		t.Fatalf("want one facts file, got %v", facts)
	}
	body, _ := os.ReadFile(facts[0])
	for _, want := range []string{"contradicted: run15 canPost answers for its own scheme only", "# code review:\ncanPost answers for its own scheme only"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the facts file lacks %q:\n%s", want, body)
		}
	}
	if file, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts")); strings.Contains(string(file), "//") {
		t.Errorf("the site still holds its block:\n%s", file)
	}
}

// loop at a site whose block the writers deleted sends the archive's claims back with the review's
// sentence. Run 14's runner built those facts files by hand from the seed's.
func TestLoopAtASiteWithNoBlockOffersTheArchivedRecord(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive, "--heads="+c.earlier)
	// The writers wrote no block at claimFor, whose declaration is line 3 of the stripped file.
	said := c.run("loop", "--run-dir="+runDir, "--archive="+archive, "--run=run15", "ledger.ts:3", "claimFor asks each scheme once")
	if said.code != exitClean {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
	facts, _ := filepath.Glob(filepath.Join(runDir, "review-loop", "*", "*", "*", "*.facts"))
	if len(facts) != 1 {
		t.Fatalf("want one facts file, got %v", facts)
	}
	body, _ := os.ReadFile(facts[0])
	for _, want := range []string{"# claimed at this site by an earlier run:", "canPost throws when its this binding",
		"# code review:\nclaimFor asks each scheme once"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the facts file lacks %q:\n%s", want, body)
		}
	}
}

// loop takes every site of one file in one call. The prompt names each site at its line in the tree the
// writer opens: run 16's second call named a line its first strip had moved.
func TestLoopTakesTwoSitesOfOneFileAtTheirFinalLines(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	c.write("ledger.ts", "// A ledger lists other postings first.\nexport function other() {}\n\n"+
		"// A ledger build answers for every scheme.\n"+claimForDecl)
	said := c.run("loop", "--run-dir="+runDir, "--archive="+archive, "--run=run17",
		"ledger.ts:2", "other lists nothing first",
		"--contradict=canPost answers for its own scheme only", "ledger.ts:5", "canPost answers for its own scheme only")
	if said.code != exitClean {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
	prompt, err := os.ReadFile(strings.TrimSpace(said.stdout))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(prompt), "ledger.ts:1 ledger.ts/") != 1 || strings.Count(string(prompt), "ledger.ts:3 ledger.ts/") != 1 ||
		strings.Contains(string(prompt), "ledger.ts:4") {
		t.Fatalf("the prompt names the wrong lines:\n%s", prompt)
	}
	stripped, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts"))
	if lines := strings.Split(string(stripped), "\n"); !strings.HasPrefix(lines[0], "export function other") ||
		!strings.HasPrefix(lines[2], "export function claimFor") {
		t.Fatalf("the file does not stand as the prompt numbers it:\n%s", stripped)
	}
	facts, _ := filepath.Glob(filepath.Join(runDir, "review-loop", "*", "*", "*", "*.facts"))
	var all string
	for _, f := range facts {
		body, _ := os.ReadFile(f)
		all += string(body)
	}
	for _, want := range []string{"# code review:\nother lists nothing first", "contradicted: run17 canPost answers"} {
		if !strings.Contains(all, want) {
			t.Errorf("the facts lack %q:\n%s", want, all)
		}
	}
}

// One round is one writer: loop takes the sites of every file of the round in one call, and names the
// round. Run 16 dispatched a loop writer per site.
func TestLoopTakesTheSitesOfTwoFilesAsOneRound(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	c.write("ledger.ts", "// A ledger build answers for every scheme.\n"+claimForDecl)
	c.write("book.ts", "// A book closes at midnight.\nexport const CLOSE_HOUR = 0;\n")
	said := c.run("loop", "--run-dir="+runDir, "--archive="+archive, "--run=run19",
		"ledger.ts:2", "claimFor asks each scheme once", "book.ts:2", "the book closes at the ledger's midnight")
	if said.code != exitClean || !strings.HasSuffix(strings.TrimSpace(said.stdout), "spawn-writer-loop-round-1.md") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	prompt, _ := os.ReadFile(strings.TrimSpace(said.stdout))
	for _, want := range []string{"ledger.ts:1 ledger.ts/2/1.facts", "book.ts:1 book.ts/2/1.facts", "return-writer-loop-round-1.md", readSentence,
		"keep what it already states correctly", "the voice-check input among them, goes in `" + runDir + "`",
		"the code line under a block keeps its spacing"} {
		if !strings.Contains(string(prompt), want) {
			t.Errorf("the round's prompt lacks %q:\n%s", want, prompt)
		}
	}
}

// refusedRound says the loop refused its round and stripped none of it.
func refusedRound(said outcome) bool {
	return said.code == exitDidNotRun && strings.Contains(said.stderr, "no file of the round was stripped")
}

// everySchemeLedger is the ledger file with one block, which the loop cases send back to a writer.
const everySchemeLedger = "// A ledger build answers for every scheme.\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n"

// A round strips its files together or none of them. A site the strip refuses leaves every file of the
// round as it stood, so the round can run again as it was given.
func TestALoopRoundThatRefusesStripsNothing(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	ledger := "// A ledger build answers for every scheme.\n" + claimForDecl
	c.write("ledger.ts", ledger)
	c.write("book.ts", "export const CLOSE_HOUR = 0;\n")
	said := c.run("loop", "--run-dir="+runDir, "--archive="+archive, "--run=run19",
		"ledger.ts:2", "claimFor asks each scheme once", "book.ts:1", "the book closes at the ledger's midnight")
	if !refusedRound(said) {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	if body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts")); string(body) != ledger {
		t.Fatalf("a file of the refused round was stripped:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(runDir, "review-loop", "loop-round-1")); err == nil {
		t.Fatal("the refused round left its facts behind")
	}
}

// A round whose prompt cannot be written leaves its files as they stood, and a round run again records
// each contradiction once. The first cut restored files only for a strip's own refusal.
func TestALoopRoundRefusedAtItsPromptStripsNothingAndRerunsClean(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	ledger := everySchemeLedger
	c.write("ledger.ts", ledger)
	home, _ := os.LookupEnv("HOME")
	template, _ := os.ReadFile(filepath.Join(home, spawnTemplate))
	if err := os.Remove(filepath.Join(home, spawnTemplate)); err != nil {
		t.Fatal(err)
	}
	args := []string{"loop", "--run-dir=" + runDir, "--archive=" + archive, "--run=run19",
		"--contradict=canPost answers for its own scheme only", "ledger.ts:2", "canPost answers for its own scheme only"}
	if said := c.run(args...); said.code != exitDidNotRun {
		t.Fatalf("a round with no template ran: exit %d %s", said.code, said.stderr)
	}
	if body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts")); string(body) != ledger {
		t.Fatalf("a round refused at its prompt stripped the file:\n%s", body)
	}
	if recorded, _ := filepath.Glob(filepath.Join(archive, "*.contradicted")); len(recorded) != 0 {
		t.Fatalf("a round refused at its prompt recorded a contradiction: %v", recorded)
	}
	if err := os.WriteFile(filepath.Join(home, spawnTemplate), template, 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run(args...); said.code != exitClean {
		t.Fatalf("the rerun: exit %d %s", said.code, said.stderr)
	}
	if said := c.run(args...); said.code == exitClean {
		t.Log("a third run found its site stripped, as expected")
	}
	contradicted, _ := filepath.Glob(filepath.Join(archive, "*.contradicted"))
	for _, path := range contradicted {
		body, _ := os.ReadFile(path)
		if strings.Count(string(body), "canPost answers for its own scheme only") > 1 {
			t.Fatalf("a rerun recorded its contradiction twice:\n%s", body)
		}
	}
}

// A round refused where its prompt is written restores its files and removes its facts. The spawn file's
// place holds a directory here, so the write fails after every strip.
func TestALoopRoundRefusedAtItsWriteRestoresItsFiles(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	ledger := everySchemeLedger
	c.write("ledger.ts", ledger)
	// The glob counts this directory as one earlier round, so the round is named after it and its write fails.
	if err := os.MkdirAll(spawnFile(runDir, "loop-round-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	said := c.run("loop", "--run-dir="+runDir, "--archive="+archive, "--run=run19", "ledger.ts:2", "claimFor asks each scheme once")
	if !refusedRound(said) {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	if body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts")); string(body) != ledger {
		t.Fatalf("a round refused at its write left the file stripped:\n%s", body)
	}
}

// carried refuses a lane's verdict naming a path outside the tree.
func TestCarriedRefusesAPathOutsideTheTree(t *testing.T) {
	c := newChange(t)
	runDir := filepath.Join(t.TempDir(), "run")
	c.seeded(runDir, t.TempDir())
	lane := filepath.Join(t.TempDir(), "refactor.md")
	if err := os.WriteFile(lane, []byte("Comment 1/1 ../outside.ts:3 | carried by `X`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("carried", "--run=run18", "--run-dir="+runDir, "--archive="+t.TempDir(), lane); said.code != exitFindings ||
		!strings.Contains(said.stdout, "no path inside the tree") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
}

// The stage decides the writers from the sites: one up to 50, two up to 100, and three above. A file
// stays whole, and past three writers' reach the batches follow.
func TestPlanDecidesTheWritersFromTheSites(t *testing.T) {
	sites := func(counts map[string]int) ([]string, map[string][]string) {
		var order []string
		byFile := map[string][]string{}
		for file, n := range counts {
			order = append(order, file)
			for i := 0; i < n; i++ {
				byFile[file] = append(byFile[file], file+":1")
			}
		}
		sort.Strings(order)
		return order, byFile
	}
	for _, tc := range []struct {
		counts  map[string]int
		writers []int
	}{
		{map[string]int{"a/x.ts": 30, "a/y.ts": 20}, []int{1}},
		{map[string]int{"a/x.ts": 30, "b/y.ts": 21}, []int{2}},
		{map[string]int{"a/x.ts": 40, "b/y.ts": 40, "b/z.ts": 20}, []int{2}},
		{map[string]int{"a/x.ts": 40, "b/y.ts": 40, "c/z.ts": 41}, []int{3}},
		{map[string]int{"a/x.ts": 400, "b/y.ts": 400, "c/z.ts": 400, "d/w.ts": 100}, []int{3, 1}},
	} {
		order, byFile := sites(tc.counts)
		batches := plan(order, byFile)
		var got []int
		for _, b := range batches {
			got = append(got, len(b))
			for _, group := range b {
				seen := map[string]bool{}
				for _, file := range group {
					if seen[file] {
						t.Errorf("%v: a file split within a writer", tc.counts)
					}
					seen[file] = true
				}
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.writers) {
			t.Errorf("%v: writers per batch %v, want %v", tc.counts, got, tc.writers)
		}
	}
	// No writer comes back empty where one file holds most of the sites.
	order, byFile := sites(map[string]int{"a/x.ts": 300, "b/y.ts": 20, "b/z.ts": 20})
	for _, group := range plan(order, byFile)[0] {
		if len(group) == 0 {
			t.Error("a writer holds no file")
		}
	}
	// A directory's files stay with one writer where its size allows.
	order, byFile = sites(map[string]int{"a/x.ts": 25, "a/y.ts": 20, "b/z.ts": 25, "b/w.ts": 20})
	for _, group := range plan(order, byFile)[0] {
		dirs := map[string]bool{}
		for _, file := range group {
			dirs[filepath.Dir(file)] = true
		}
		if len(dirs) != 1 {
			t.Errorf("a writer holds files of %d directories: %v", len(dirs), group)
		}
	}
}

// A lane never edits a comment line, and carried refuses a file where it did. revert restores a file
// as the writers left it and withdraws the run's carried record for it. Run 18's lane shortened a note
// and changed code logic to carry a fact, and neither had a stage.
func TestCarriedRefusesALaneCommentEditAndRevertWithdraws(t *testing.T) {
	c := newChange(t)
	archive := filepath.Join(t.TempDir(), "archive")
	runDir := filepath.Join(t.TempDir(), "run18")
	c.seeded(runDir, archive, "--heads="+c.earlier)
	written := headLedger
	c.write("ledger.ts", written)
	ret := filepath.Join(t.TempDir(), "return-writer-A.md")
	if err := os.WriteFile(ret, []byte("Block 1/1 ledger.ts:3 | OK\n```ts\n"+
		"// A ledger build answers for its own scheme only, so this function asks it once per scheme.\n```\n"+
		"fact: a ledger build answers for its own scheme only\nbears_on: claimFor\ndoes: returns keys.canPost(scheme)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(ret, filepath.Join(runDir, "return-writer-A.md")); err != nil {
		t.Fatal(err)
	}
	// With no return named, archive-written reads the writers' return files in the run directory.
	if said := c.run("archive-written", "--run=run18", "--archive="+archive, "--run-dir="+runDir); said.code != exitClean ||
		!strings.Contains(said.stderr+said.stdout, "1 block(s) archived") {
		t.Fatalf("archive-written: %s%s", said.stdout, said.stderr)
	}
	lane := filepath.Join(t.TempDir(), "refactor.md")
	if err := os.WriteFile(lane, []byte("Comment 1/1 ledger.ts:3 | carried by `ASKS_ONCE_PER_SCHEME`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The lane shortens the note while it carries the fact.
	c.write("ledger.ts", "const ASKS_ONCE_PER_SCHEME = true;\n\nexport function other() {}\n\n// A ledger build asks once.\n"+
		carriedClaimFor)
	if said := c.run("carried", "--run=run18", "--run-dir="+runDir, "--archive="+archive, lane); said.code != exitFindings ||
		!strings.Contains(said.stdout, "a lane never edits a comment line") {
		t.Fatalf("a lane's comment edit was taken: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	// Carried cleanly, then reverted by a ruling: the file stands as the writers left it, and the record goes.
	c.write("ledger.ts", "const ASKS_ONCE_PER_SCHEME = true;\n\nexport function other() {}\n\n"+
		carriedClaimFor)
	if said := c.run("carried", "--run=run18", "--run-dir="+runDir, "--archive="+archive, lane); said.code != exitClean {
		t.Fatalf("carried: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	said := c.run("revert", "--run=run18", "--run-dir="+runDir, "--archive="+archive, "./sub/../ledger.ts")
	if said.code != exitClean || !strings.Contains(said.stdout, "ledger.ts restored as the writers left it, 1 carried record(s) withdrawn") {
		t.Fatalf("revert: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	if body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts")); string(body) != written {
		t.Fatalf("the file does not stand as the writers left it:\n%s", body)
	}
}

// A block the refactor lane carried into code stays carried. The next seed offers no site for it while
// the carrier stands, keep-test counts it kept, and removing the carrier reopens it. Runs 14 and 16
// carried one block away twice, and the writers wrote it back from the archive each time.
func TestACarriedBlockStaysCarriedWhileItsCarrierStands(t *testing.T) {
	c := newChange(t)
	archive := filepath.Join(t.TempDir(), "archive")
	first := filepath.Join(t.TempDir(), "run16")
	c.seeded(first, archive, "--heads="+c.earlier)
	written := headLedger
	c.write("ledger.ts", written)
	ret := filepath.Join(t.TempDir(), "writer-A.md")
	if err := os.WriteFile(ret, []byte("Block 1/1 ledger.ts:3 | OK\n```ts\n"+
		"// A ledger build answers for its own scheme only, so this function asks it once per scheme.\n```\n"+
		"fact: a ledger build answers for its own scheme only\nbears_on: claimFor\ndoes: returns keys.canPost(scheme)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("archive-written", "--run=run16", "--archive="+archive, "--run-dir="+first, ret); said.code != exitClean {
		t.Fatalf("archive-written: %s%s", said.stdout, said.stderr)
	}
	// The lane carries the block into a constant the function reads, and removes it.
	carriedTree := "const ASKS_ONCE_PER_SCHEME = true;\n\nexport function other() {}\n\n" +
		carriedClaimFor
	c.write("ledger.ts", carriedTree)
	lane := filepath.Join(t.TempDir(), "refactor.md")
	if err := os.WriteFile(lane, []byte("Comment 1/1 ledger.ts:3 | carried by `ASKS_ONCE_PER_SCHEME`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("carried", "--run=run16", "--run-dir="+first, "--archive="+archive, lane); said.code != exitClean ||
		!strings.Contains(said.stdout, "ledger.ts:3 carried by `ASKS_ONCE_PER_SCHEME`") {
		t.Fatalf("carried: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	head := c.commit("run 16")
	next := filepath.Join(t.TempDir(), "run17")
	said := c.run("seed", "--run-dir="+next, "--archive="+archive, "--range="+c.base+".."+head)
	if said.code != exitClean || strings.Contains(said.stdout, "ledger.ts:5 ") {
		t.Fatalf("the carried site was offered again: exit %d\n%s%s", said.code, said.stdout, said.stderr)
	}
	if said := c.run("keep-test", "--archive="+archive, "ledger.ts"); said.code != exitClean ||
		!strings.Contains(said.stdout, "carried by `ASKS_ONCE_PER_SCHEME` as run16 left it") ||
		!strings.Contains(said.stderr, "0 of 0 block(s) kept, 0 to rewrite, 1 carried into code") {
		t.Fatalf("keep-test: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	// With the carrier gone, the site is offered again.
	c.git("checkout", "--", "ledger.ts")
	c.write("ledger.ts", "export function other() {}\n\n"+claimForDecl)
	head = c.commit("carrier removed")
	again := filepath.Join(t.TempDir(), "run18")
	if said := c.run("seed", "--run-dir="+again, "--archive="+archive, "--range="+c.base+".."+head); !strings.Contains(said.stdout, "ledger.ts:3 ") {
		t.Fatalf("the site stayed closed with its carrier gone: exit %d\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// A prompt names the facts root once. A site line with its facts file anywhere else refuses the prompt.
// That prompt would name a file the writer cannot find.
func TestFactsRootRebuildsEverySiteLinesFactsFile(t *testing.T) {
	root, err := factsRoot([]string{"src/a/ledger.ts:3 /run/facts/src_a_ledger.ts/1.facts", "book.ts:9 /run/facts/book.ts/2.facts"})
	if err != nil || root != "/run/facts" {
		t.Fatalf("root %q, err %v", root, err)
	}
	for _, lines := range [][]string{
		{"src/a/ledger.ts:3 /run/facts/ledger.ts/1.facts"},
		{"book.ts:9 /run/facts/book.ts/2.facts", "ledger.ts:3 /other/facts/ledger.ts/1.facts"},
		{"book.ts /run/facts/book.ts/2.facts"},
	} {
		if _, err := factsRoot(lines); err == nil {
			t.Errorf("%v was taken", lines)
		}
	}
}

// A flag's value names a file a check reads, and a script's own string holding a flag is no such
// value. A one-liner that replaced `--mode=old` in its text and wrote the source file once passed.
func TestAFlagValueIsNoWriteAndAQuotedFlagHidesNone(t *testing.T) {
	for _, tc := range []struct {
		command string
		writes  bool
	}{
		{`sed -i '' 's/a/b/' vc2.txt && voice-check.sh --file=x.ts - < vc2.txt`, false},
		{`python3 -c "s=src.replace('--mode=old','--mode=new');open('x.ts','w').write(s)"`, true},
		{`node -e "s=s.replace('--a=1','--a=2');fs.writeFileSync('x.ts',s)"`, true},
		{`sed -i '' 's/a/b/' x.ts --file=x.ts`, true},
		// Run 26's writer appended its ledger from a script that named source files in strings.
		{"python3 - \"$S/comment-writer-A-queue.md\" <<'EOF'\nF=\"x.ts\"\nopen(sys.argv[1],'a').write(F)\nEOF", false},
		{"python3 - <<'EOF'\nF=\"x.ts\"\nopen(F,'w').write('')\nEOF", true},
		{"python3 - \"$S/comment-writer-A-queue.md\" <<'EOF'\nopen('x.ts','w').write('')\nEOF", true},
		{`python3 -c "print(open('x.ts').read())"`, false},
		// Run 26's writer rewrote a scratch file named by its argument, the source path in its text.
		{"python3 - \"$S/vc.txt\" <<'EOF'\np=sys.argv[1]; t=open(p).read()\nt=t.replace('in `x.ts` a','in `x.ts` b')\nopen(p,'w').write(t)\nEOF", false},
		{"python3 - \"x.ts\" <<'EOF'\np=sys.argv[1]; t=open(p).read()\nopen(p,'w').write(t)\nEOF", true},
		// Each write the review found missed, and the argument vectors of node and ruby.
		{"node - x.ts <<'EOF'\nconst p = process.argv[2]\nfs.writeFileSync(p, s)\nEOF", true},
		{"ruby - x.ts <<'EOF'\np = ARGV[0]; File.open(p,'w') { |f| f.write(s) }\nEOF", true},
		{"python3 -u - x.ts <<'EOF'\np=sys.argv[1]\nopen(p,'w').write('')\nEOF", true},
		{`python3 -c "open('x.ts', mode='w').write('')"`, true},
		{`python3 -c "open(file='x.ts', mode='w').write('')"`, true},
		{`python3 -c "open('x.ts','r+').write('')"`, true},
		{"python3 - <<'EOF'\np=Path('x.ts'); p.write_text('')\nEOF", true},
		{"python3 - <<'EOF'\nfrom pathlib import Path as P\nP('x.ts').write_text('')\nEOF", true},
		{"python3 - <<'EOF'\nfor p in ['x.ts']: open(p,'w').write('')\nEOF", true},
		{"python3 - a.ts x.ts <<'EOF'\nfor p in sys.argv[1:]: open(p,'w').write('')\nEOF", true},
		{`ruby -e "File.write('x.ts', s)"`, true},
		{`node -e "fs.writeFileSync(path.join('src','x.ts'), s)"`, true},
		{"python3 - <<'EOF'\nout=Path('/tmp/x.ts.txt')\nout.write_text(open('x.ts').read())\nEOF", false},
		{`python3 -c "open('/tmp/x.ts.txt','w').write(open('x.ts').read())"`, false},
		{"node - x.ts <<'EOF'\nconst p = process.argv[1]\nfs.writeFileSync(p, s)\nEOF", false},
	} {
		if got := scriptWrites(tc.command, "x.ts"); got != tc.writes {
			t.Errorf("%s: writes %v, want %v", tc.command, got, tc.writes)
		}
	}
}

// A writer writes comment lines only. Run 26's writers changed the spacing of code lines under their
// blocks, and archive-written refuses that against the file as the writer received it.
func TestArchiveWrittenRefusesAChangedCodeLine(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		refused    bool
	}{
		{"kept", "export function claimFor(scheme: string): boolean {", false},
		{"respaced", "export function claimFor(scheme: string):  boolean {", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newChange(t)
			runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
			c.seeded(runDir, archive)
			body, err := os.ReadFile(filepath.Join(c.top, "ledger.ts"))
			if err != nil {
				t.Fatal(err)
			}
			note := "// A ledger build answers for its own scheme only, so this function asks it once per scheme."
			c.write("ledger.ts", strings.Replace(string(body), "export function claimFor(scheme: string): boolean {", note+"\n"+tc.code, 1))
			if err := os.WriteFile(returnFile(runDir, "A"), []byte("Block 1/1 ledger.ts:4 | OK\n```ts\n"+note+"\n"+tc.code+"\n```\n"+
				"summary: none\nnote: written\nfact: a ledger build answers for its own scheme only\nbears_on: claimFor\n"+
				"does: returns keys.canPost(scheme)\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			said := c.run("archive-written", "--run=run26", "--archive="+archive, "--run-dir="+runDir)
			if got := strings.Contains(said.stdout, "code changed"); got != tc.refused || (tc.refused && said.code != exitFindings) {
				t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
			}
			if tc.refused {
				return
			}
			// The refactor lane changes code, and the loop's archive-written reads every return again.
			body, _ = os.ReadFile(filepath.Join(c.top, "ledger.ts"))
			c.write("ledger.ts", strings.Replace(string(body), "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
			lane := filepath.Join(t.TempDir(), "refactor.md")
			if err := os.WriteFile(lane, []byte("Comment 1/1 ledger.ts:4 | stays: the code says it\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if said := c.run("carried", "--run=run26", "--run-dir="+runDir, "--archive="+archive, lane); said.code != exitClean {
				t.Fatalf("carried: exit %d:\n%s%s", said.code, said.stdout, said.stderr)
			}
			if said := c.run("archive-written", "--run=run26", "--archive="+archive, "--run-dir="+runDir); strings.Contains(said.stdout, "code changed") || !strings.Contains(said.stdout, "archived") {
				t.Fatalf("the lane's code reads as a writer's:\n%s%s", said.stdout, said.stderr)
			}
		})
	}
}

// The code check reads comment lines with the strip's grammar. A `#` header is comment, and so is a
// `/* */` body line.
func TestCodeLinesReadCommentsAsTheStripDoes(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, received, left string
		changed              bool
	}{
		{"hash header", "set -e\necho hi\n", "set -e\n# Prints the greeting.\necho hi\n", false},
		{"star body", "const x = 1;\n", "/*\n  A ledger rounds.\n*/\nconst x = 1;\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runDir := filepath.Join(dir, tc.name)
			for path, body := range map[string]string{filepath.Join(runDir, "dispatched", "f.ts"): tc.received, filepath.Join(runDir, "tree", "f.ts"): tc.left} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			at, err := codeChanged(runDir, filepath.Join(runDir, "tree"), "f.ts")
			if err != nil || (at > 0) != tc.changed {
				t.Fatalf("changed at %d, err %v, want changed %v", at, err, tc.changed)
			}
		})
	}
}

// A loop round's writer receives the tree after the refactor lane and the round's strip, and its
// code is read against that. Here the code changed after the seed, as the lane changes it.
func TestALoopRoundsWriterIsReadAgainstItsOwnStrip(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	changed := strings.Replace(claimForDecl, "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1)
	c.write("ledger.ts", "export function other() {}\n\n// A ledger build answers for every scheme, so this function asks it once.\n"+changed)
	var out, errOut strings.Builder
	if code := Run("comment-run.sh", []string{"loop", "--run-dir=" + runDir, "--archive=" + archive, "--run=run26",
		"ledger.ts:4", "canPost answers for its own scheme only"}, t.TempDir(), repo.Exec{}, &out, &errOut); code != exitClean {
		t.Fatalf("loop: exit %d: %s", code, errOut.String())
	}
	note := "// A ledger build answers for its own scheme only, so this function asks it once per scheme."
	c.write("ledger.ts", "export function other() {}\n\n"+note+"\n"+changed)
	if err := os.WriteFile(returnFile(runDir, "loop-round-1"), []byte("Block 1/1 ledger.ts:4 | OK\n```ts\n"+note+"\n"+
		strings.SplitN(claimForDecl, "\n", 2)[0]+"\n```\nsummary: none\nnote: written\nfact: a ledger build answers for its own scheme only\n"+
		"bears_on: claimFor\ndoes: returns keys.canPost(scheme)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("archive-written", "--run=run26", "--archive="+archive, "--run-dir="+runDir); strings.Contains(said.stdout, "code changed") || !strings.Contains(said.stdout, "archived") {
		t.Fatalf("the code before the round reads as the writer's:\n%s%s", said.stdout, said.stderr)
	}
}

// A fence can open on a code line that opens a body. Its first comment then sits inside the body, and
// archive-written refuses it.
func TestArchiveWrittenRefusesACommentInsideAFencedBody(t *testing.T) {
	c := newChange(t)
	c.write("book.ts", "export function f() {\n  // A ledger rounds to cents.\n  return 1;\n}\n")
	ret := filepath.Join(t.TempDir(), "writer-A.md")
	if err := os.WriteFile(ret, []byte("Block 1/1 book.ts:1 | OK\n```ts\nexport function f() {\n  // A ledger rounds to cents.\n  return 1;\n}\n```\n"+
		"fact: a ledger rounds to cents\nbears_on: f\ndoes: returns 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("archive-written", "--run=run26", "--archive="+t.TempDir(), ret)
	if said.code != exitFindings || !strings.Contains(said.stdout, "no comment line before the code") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// Run 26's writer A answered a type's site as written and put the block on a constant. The site kept
// no record of its own, and archive-written refuses that verdict with the shape a move takes.
func TestArchiveWrittenRefusesABlockMovedOffItsSite(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitClean {
		t.Fatalf("prompts: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	sites, _ := os.ReadFile(filepath.Join(runDir, "sites.txt"))
	site := regexp.MustCompile(`ledger\.ts:(\d+)`).FindStringSubmatch(string(sites))
	if site == nil {
		t.Fatalf("no ledger.ts site in %s", sites)
	}
	note := "// A ledger build answers for its own scheme only, so the other entry asks it once per scheme."
	body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts"))
	c.write("ledger.ts", strings.Replace(string(body), "export function other() {}", note+"\nexport function other() {}", 1))
	if err := os.WriteFile(returnFile(runDir, "A"), []byte("Block 1/1 ledger.ts:"+site[1]+" | OK\n```ts\n"+note+"\nexport function other() {}\n```\n"+
		"summary: none\nnote: written\nfact: a ledger build answers for its own scheme only\nbears_on: other\ndoes: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("archive-written", "--run=run26", "--archive="+archive, "--run-dir="+runDir)
	if said.code != exitFindings || !strings.Contains(said.stdout, "not on the declaration offered") ||
		!strings.Contains(said.stdout, "moved to ledger.ts:1") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// A record the strip placed by its name holds its old declaration text, and a block written on the line
// it was offered at stands on the declaration offered.
func TestArchiveWrittenAcceptsABlockOnASitePlacedByName(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	if said := c.run("prompts", "--run-dir="+runDir); said.code != exitClean {
		t.Fatalf("prompts: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	sites, _ := os.ReadFile(filepath.Join(runDir, "sites.txt"))
	site := regexp.MustCompile(`ledger\.ts:(\d+)`).FindStringSubmatch(string(sites))
	if site == nil {
		t.Fatalf("no ledger.ts site in %s", sites)
	}
	offered, _ := filepath.Glob(filepath.Join(runDir, "facts", "*", "*.offered"))
	for _, path := range offered {
		body, _ := os.ReadFile(path)
		body = regexp.MustCompile(`"decl": "[^"]*"`).ReplaceAll(body, []byte(`"decl": "export interface ClaimFor {"`))
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	note := "// A ledger build answers for its own scheme only, so this function asks it once per scheme."
	body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts"))
	c.write("ledger.ts", strings.Replace(string(body), "export function claimFor(", note+"\nexport function claimFor(", 1))
	if err := os.WriteFile(returnFile(runDir, "A"), []byte("Block 1/1 ledger.ts:"+site[1]+" | OK\n```ts\n"+note+"\n"+
		"export function claimFor(scheme: string): boolean {\n```\nsummary: none\nnote: written\n"+
		"fact: a ledger build answers for its own scheme only\nbears_on: claimFor\ndoes: returns keys.canPost(scheme)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("archive-written", "--run=run26", "--archive="+archive, "--run-dir="+runDir); strings.Contains(said.stdout, "not on the declaration offered") {
		t.Fatalf("a block on the offered line was refused:\n%s%s", said.stdout, said.stderr)
	}
}

// The refactor lane's edits are settled by the carried stage. The next seed lists them for the lane's
// prompt, and the carried stage refuses a later lane's edit to one while its code holds.
func TestTheCarriedStageSettlesTheLanesEdits(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	c.seeded(runDir, archive)
	body, _ := os.ReadFile(filepath.Join(c.top, "ledger.ts"))
	c.write("ledger.ts", strings.Replace(string(body), "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
	lane := filepath.Join(t.TempDir(), "refactor.md")
	if err := os.WriteFile(lane, []byte("Comment 1/1 ledger.ts:3 | stays: the code says it\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := c.run("carried", "--run=run26", "--run-dir="+runDir, "--archive="+archive, lane); said.code != exitClean {
		t.Fatalf("carried: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	head := c.commit("run 26")
	next := filepath.Join(t.TempDir(), "run27")
	if said := c.run("seed", "--run-dir="+next, "--archive="+archive, "--range="+c.base+".."+head); said.code != exitClean && said.code != exitFindings {
		t.Fatalf("seed: exit %d %s", said.code, said.stderr)
	}
	listed, _ := os.ReadFile(filepath.Join(next, "settled.txt"))
	if !strings.Contains(string(listed), "export function claimFor(scheme: string): boolean {, as run26 settled it") {
		t.Fatalf("settled.txt:\n%s", listed)
	}
	body, _ = os.ReadFile(filepath.Join(c.top, "ledger.ts"))
	c.write("ledger.ts", strings.Replace(string(body), "keys.canPost(scheme) === true", "Boolean(keys.canPost(scheme))", 1))
	said := c.run("carried", "--run=run27", "--run-dir="+next, "--archive="+archive, lane)
	if said.code != exitFindings || !strings.Contains(said.stdout, "the lane edited `export function claimFor(scheme: string): boolean {`, which run26 settled") {
		t.Fatalf("a later lane's edit: exit %d\n%s%s", said.code, said.stdout, said.stderr)
	}
}
