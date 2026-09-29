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
	for _, path := range []string{"standards/code-style.md", "workers/comment-writer.md"} {
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
	said := c.run("prompts", "--run-dir="+runDir)
	if said.code != exitClean || !strings.HasPrefix(said.stdout, "A batch 1, 1 site(s) in 1 file(s)") {
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

// A diff with no range prints HEAD's blocks under a `-`, and run 18 tainted two writers that way with no
// line from this stage. A write by shell script is reported too, since writes go through the Edit tool.
func TestTaintReadsARangelessDiffAndAScriptWrite(t *testing.T) {
	c := newChange(t)
	ledger := filepath.Join(t.TempDir(), "comment-writer-A-queue.md")
	if err := os.WriteFile(ledger, []byte("Block 1/1 ledger.ts:3 | OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := transcript(t,
		[3]string{"Bash", `{"command":"git diff -- ledger.ts"}`, `"@@ -2,3 +2,2 @@\n-// canPost throws for another scheme\n export function claimFor"`},
		[3]string{"Bash", `{"command":"sed -i '' '3i\\\\// a note' ledger.ts"}`, `""`},
		[3]string{"Edit", `{"file_path":"/tree/ledger.ts","old_string":"a","new_string":"b"}`, `"ok"`},
	)
	said := c.run("taint", "--ledger="+ledger, path)
	if said.code != exitFindings || !strings.Contains(said.stdout, "tainted: call 1 ") || !strings.Contains(said.stdout, "script write: call 2 ") {
		t.Fatalf("exit %d:\n%s%s", said.code, said.stdout, said.stderr)
	}
}

// usage prints a writer's context at its first tool call and at its end, the tokens it wrote, its calls
// and its wall time. Run 18's report estimated a writer's start-up from totals.
func TestUsageReadsATranscriptsFigures(t *testing.T) {
	c := newChange(t)
	path := filepath.Join(t.TempDir(), "writer-A.jsonl")
	lines := []string{
		`{"timestamp":"2026-09-29T10:00:00Z","message":{"content":[{"type":"text"}],"usage":{"input_tokens":2,"cache_read_input_tokens":100,"cache_creation_input_tokens":40000,"output_tokens":10}}}`,
		`{"timestamp":"2026-09-29T10:00:05Z","message":{"content":[{"type":"tool_use","id":"a","name":"Read","input":{}}],"usage":{"input_tokens":2,"cache_read_input_tokens":40100,"cache_creation_input_tokens":900,"output_tokens":20}}}`,
		`{"timestamp":"2026-09-29T10:00:09Z","message":{"content":[{"type":"tool_result","tool_use_id":"a","content":"x"}]}}`,
		`{"timestamp":"2026-09-29T10:01:40Z","message":{"content":[{"type":"tool_use","id":"b","name":"Edit","input":{}}],"usage":{"input_tokens":2,"cache_read_input_tokens":41000,"cache_creation_input_tokens":3000,"output_tokens":30}}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said := c.run("usage", path)
	if said.code != exitClean || !strings.Contains(said.stdout, "writer-A.jsonl | 41002 | 44002 | 60 | 2 | 100 s") {
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
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+t.TempDir(), "--range="+c.base+".."+c.head); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
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
		verdictSentence, "return-writer-A.md", "with the Edit tool", "rules standards/code-style.md", "rules workers/comment-writer.md"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the prompt lacks %q:\n%s", want, body)
		}
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
		"Block 2/4 :3 (landed on line 4) | OK\n" + block("// A ledger rounds to cents, so this constant holds two places.", "export const CENT_PLACES = 2;") +
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
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+archive, "--range="+c.base+".."+c.head); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
	// The writer wrote a block back at the site, and review found its claim false.
	c.write("ledger.ts", "export function other() {}\n\n// A ledger build answers for every scheme, so this function asks it once.\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n")
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
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+archive, "--range="+c.base+".."+c.head,
		"--heads="+c.earlier); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
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
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+archive, "--range="+c.base+".."+c.head); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
	c.write("ledger.ts", "// A ledger lists other postings first.\nexport function other() {}\n\n"+
		"// A ledger build answers for every scheme.\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n")
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
	if !strings.Contains(string(prompt), "`ledger.ts:1`") || !strings.Contains(string(prompt), "`ledger.ts:3`") ||
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
// round, not a file. Run 16 dispatched a loop writer per site.
func TestLoopTakesTheSitesOfTwoFilesAsOneRound(t *testing.T) {
	c := newChange(t)
	runDir, archive := filepath.Join(t.TempDir(), "run"), filepath.Join(t.TempDir(), "archive")
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+archive, "--range="+c.base+".."+c.head); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
	c.write("ledger.ts", "// A ledger build answers for every scheme.\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n")
	c.write("book.ts", "// A book closes at midnight.\nexport const CLOSE_HOUR = 0;\n")
	said := c.run("loop", "--run-dir="+runDir, "--archive="+archive, "--run=run19",
		"ledger.ts:2", "claimFor asks each scheme once", "book.ts:2", "the book closes at the ledger's midnight")
	if said.code != exitClean || !strings.HasSuffix(strings.TrimSpace(said.stdout), "spawn-writer-loop-round-1.md") {
		t.Fatalf("exit %d: %s%s", said.code, said.stdout, said.stderr)
	}
	prompt, _ := os.ReadFile(strings.TrimSpace(said.stdout))
	for _, want := range []string{"`ledger.ts:1`", "`book.ts:1`", "return-writer-loop-round-1.md", "rules workers/comment-writer.md"} {
		if !strings.Contains(string(prompt), want) {
			t.Errorf("the round's prompt lacks %q:\n%s", want, prompt)
		}
	}
}

// The stage decides the writers from the sites: one up to 150, two up to 300, three above, a file never
// split, and more than three writers' reach in batches of three.
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
		{map[string]int{"a/x.ts": 90, "a/y.ts": 60}, []int{1}},
		{map[string]int{"a/x.ts": 100, "b/y.ts": 100, "b/z.ts": 50}, []int{2}},
		{map[string]int{"a/x.ts": 120, "b/y.ts": 120, "c/z.ts": 120}, []int{3}},
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
	// A directory's files stay with one writer where its size allows.
	order, byFile := sites(map[string]int{"a/x.ts": 80, "a/y.ts": 70, "b/z.ts": 100, "b/w.ts": 60})
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
	if said := c.run("seed", "--run-dir="+runDir, "--archive="+archive, "--range="+c.base+".."+c.head, "--heads="+c.earlier); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
	written := "export function other() {}\n\n// A ledger build answers for its own scheme only, so this function asks it once per scheme.\n" +
		"export function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n"
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
		"export function claimFor(scheme: string): boolean {\n  return ASKS_ONCE_PER_SCHEME && keys.canPost(scheme);\n}\n")
	if said := c.run("carried", "--run=run18", "--run-dir="+runDir, "--archive="+archive, lane); said.code != exitFindings ||
		!strings.Contains(said.stdout, "a lane never edits a comment line") {
		t.Fatalf("a lane's comment edit was taken: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	// Carried cleanly, then reverted by a ruling: the file stands as the writers left it, and the record goes.
	c.write("ledger.ts", "const ASKS_ONCE_PER_SCHEME = true;\n\nexport function other() {}\n\n"+
		"export function claimFor(scheme: string): boolean {\n  return ASKS_ONCE_PER_SCHEME && keys.canPost(scheme);\n}\n")
	if said := c.run("carried", "--run=run18", "--run-dir="+runDir, "--archive="+archive, lane); said.code != exitClean {
		t.Fatalf("carried: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	said := c.run("revert", "--run=run18", "--run-dir="+runDir, "--archive="+archive, "ledger.ts")
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
	if said := c.run("seed", "--run-dir="+first, "--archive="+archive, "--range="+c.base+".."+c.head, "--heads="+c.earlier); said.code != exitClean {
		t.Fatalf("seed: %s", said.stderr)
	}
	written := "export function other() {}\n\n// A ledger build answers for its own scheme only, so this function asks it once per scheme.\n" +
		"export function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n"
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
		"export function claimFor(scheme: string): boolean {\n  return ASKS_ONCE_PER_SCHEME && keys.canPost(scheme);\n}\n"
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
		!strings.Contains(said.stdout, "carried by `ASKS_ONCE_PER_SCHEME` as run16 left it") {
		t.Fatalf("keep-test: exit %d %s%s", said.code, said.stdout, said.stderr)
	}
	// With the carrier gone, the site is offered again.
	c.git("checkout", "--", "ledger.ts")
	c.write("ledger.ts", "export function other() {}\n\nexport function claimFor(scheme: string): boolean {\n  return keys.canPost(scheme);\n}\n")
	head = c.commit("carrier removed")
	again := filepath.Join(t.TempDir(), "run18")
	if said := c.run("seed", "--run-dir="+again, "--archive="+archive, "--range="+c.base+".."+head); !strings.Contains(said.stdout, "ledger.ts:3 ") {
		t.Fatalf("the site stayed closed with its carrier gone: exit %d\n%s%s", said.code, said.stdout, said.stderr)
	}
}
