package readerjudge

import (
	modelpolicy "configs/ai/tools/model-policy"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProviderSelection(t *testing.T) {
	for _, c := range []struct{ name, requested, thread, installed, want string }{
		{"explicit codex", "codex", "", "claude codex", "codex"},
		{"explicit claude", "claude", "thread", "claude codex", "claude"},
		{"missing in Codex task", "", "thread", "claude codex", ""},
		{"missing with both installed", "", "", "claude codex", ""},
		{"auto with Codex only", "auto", "", "codex", ""},
		{"auto with Claude only", "auto", "", "claude", ""},
		{"explicit missing", "codex", "", "claude", ""},
		{"explicit Claude missing", "claude", "thread", "codex", ""},
		{"missing with Claude only", "", "", "claude", ""},
		{"missing with Codex only", "", "thread", "codex", ""},
		{"neither installed", "", "", "", ""},
		{"unknown provider", "typo", "", "claude codex", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range strings.Fields(c.installed) {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir)
			t.Setenv("JUDGE_PROVIDER", c.requested)
			t.Setenv("CODEX_THREAD_ID", c.thread)
			got, err := resolveProvider()
			if c.want == "" {
				if err == nil {
					t.Fatalf("accepted unavailable or invalid provider: %s", got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("provider = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestCodexCallerUsesOnlyTheFinalMessage(t *testing.T) {
	fakeCodex(t, `while [ "$#" -gt 0 ]; do
 if [ "$1" = "--output-last-message" ]; then shift; answer="$1"; fi
 shift
done
printf 'progress, not a verdict\n'
printf 'none\n' > "$answer"`)
	got, err := CodexCaller(notTheSubject, testSettings())("prompt", "view")
	if err != nil || got != "none\n" {
		t.Fatalf("answer = %q, %v; want final message", got, err)
	}
}

func fakeCodex(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCodexCallerRefusesMissingFinalAnswer(t *testing.T) {
	fakeCodex(t, "echo none")
	if _, err := CodexCaller(notTheSubject, testSettings())("prompt", "view"); err == nil || !strings.Contains(err.Error(), "final answer") {
		t.Fatalf("missing answer accepted: %v", err)
	}
}

func TestCodexCallerRefusesFailedProcess(t *testing.T) {
	fakeCodex(t, "exit 7")
	if _, err := CodexCaller(notTheSubject, testSettings())("prompt", "view"); err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("failed process accepted: %v", err)
	}
}

func TestCodexCallerBoundsTheRoll(t *testing.T) {
	fakeCodex(t, "sleep 30")
	if _, err := CodexCaller(100*time.Millisecond, testSettings())("prompt", "view"); err == nil || !strings.Contains(err.Error(), "within 100ms") {
		t.Fatalf("timeout was not reported: %v", err)
	}
}

func TestCodexCallerIsolatesInputAndCleansItsDirectory(t *testing.T) {
	// Baked into the script rather than passed as a variable: a roll's environment is an allow-list,
	// so a test hook handed through it would be dropped the way any other stray variable is.
	record := filepath.Join(t.TempDir(), "cwd")
	fakeCodex(t, `pwd > "`+record+`"
for arg do
 case "$arg" in
  --ignore-user-config) user_config=1;;
  --ignore-rules) rules=1;;
  project_doc_max_bytes=0) instructions=1;;
  read-only) sandbox=1;;
 esac
done
[ "$user_config$rules$instructions$sandbox" = 1111 ] || exit 9
while [ "$#" -gt 0 ]; do
 if [ "$1" = "--output-last-message" ]; then shift; answer="$1"; fi
 shift
done
cat > "$answer"`)
	got, err := CodexCaller(notTheSubject, testSettings())("judge", "résumé $() `command`")
	if err != nil || got != "judge\n\nrésumé $() `command`" {
		t.Fatalf("input = %q, %v", got, err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	dir := strings.TrimSpace(string(raw))
	if !strings.Contains(filepath.Base(dir), "reader-judge-") {
		t.Fatalf("not isolated: %s", dir)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary directory survived: %s, %v", dir, err)
	}
}

func testSettings() modelpolicy.Settings {
	return modelpolicy.Settings{Model: "fixture-model", Effort: "low"}
}

func TestRetiredJudgeModelIsRejected(t *testing.T) {
	t.Setenv("JUDGE_MODEL", "chosen-model")
	if _, err := Configure(Configuration{Deadline: time.Second, PolicyPath: "missing"}); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("retired override not rejected: %v", err)
	}
}

func TestConfiguredJudgeUsesTheCentralPolicy(t *testing.T) {
	fakeCodex(t, "exit 0")
	t.Setenv("JUDGE_PROVIDER", "codex")
	t.Setenv("JUDGE_MODEL", "")
	os.Unsetenv("JUDGE_MODEL")
	configured, err := Configure(Configuration{Deadline: time.Second, PolicyPath: "../../kk-flavor/configs/models.json"})
	if err != nil {
		t.Fatal(err)
	}
	if configured.Decision.Requested.Model != "gpt-5.6-luna" || configured.Decision.Requested.Effort != "low" {
		t.Fatalf("central judge assignment = %+v", configured.Decision)
	}
	if configured.Decision.Rolls != 3 {
		t.Fatalf("central judge roll count = %d", configured.Decision.Rolls)
	}
	for _, args := range [][]string{codexArgs("answer", testSettings()), claudeArgs("prompt", testSettings())} {
		found := false
		for i, arg := range args {
			if arg == "--model" && i+1 < len(args) && args[i+1] == "fixture-model" {
				found = true
			}
		}
		if !found {
			t.Fatalf("configured model was lost: %v", args)
		}
	}
}

func TestJudgeCacheSeparatesClientSelections(t *testing.T) {
	fakeCodex(t, "exit 0")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	identities := map[string]bool{}
	for _, client := range []string{"codex", "claude"} {
		t.Setenv("JUDGE_PROVIDER", client)
		configured, err := Configure(Configuration{Deadline: time.Second, PolicyPath: "../../kk-flavor/configs/models.json"})
		if err != nil {
			t.Fatal(err)
		}
		if configured.CacheIdentity == "" || identities[configured.CacheIdentity] {
			t.Fatalf("judge cache did not distinguish %s selection: %q", client, configured.CacheIdentity)
		}
		identities[configured.CacheIdentity] = true
	}
}

// The two refusals below used to reach the caller as "the model did not answer", the sentence a broken
// CLI and a slow API also produce — which is why ModelRefused exists. Their scripts print what the real
// CLIs were measured saying on 2026-09-15; refusal.go holds both sentences in full.
func TestClaudeRefusingTheModelNameIsToldApartFromNotAnswering(t *testing.T) {
	fakeClaude(t, `printf "There's an issue with the selected model (fixture-model). It may not exist or you may not have access to it.\n"
printf '[claude-code:unrecognized_model] {"model":"fixture-model"}\n' >&2
exit 1`)
	_, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view")
	var refused *ModelRefused
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v; want a ModelRefused", err)
	}
	if refused.Client != "claude" || refused.Model != "fixture-model" {
		t.Errorf("refusal names %s/%s; want claude/fixture-model", refused.Client, refused.Model)
	}
}

func TestCodexRefusingTheModelNameIsToldApartFromNotAnswering(t *testing.T) {
	fakeCodex(t, `printf 'ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The '"'"'fixture-model'"'"' model is not supported when using Codex with a ChatGPT account."}}\n' >&2
exit 1`)
	_, err := CodexCaller(notTheSubject, testSettings())("prompt", "view")
	var refused *ModelRefused
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v; want a ModelRefused", err)
	}
	if refused.Client != "codex" || refused.Model != "fixture-model" {
		t.Errorf("refusal names %s/%s; want codex/fixture-model", refused.Client, refused.Model)
	}
}

// The negative control the two above are read against: a CLI failing for any other reason must still
// come back as "did not answer", or every broken judge would be reported as a bad model name.
func TestAFailureThatIsNotAboutTheModelNameStaysTheOldSentence(t *testing.T) {
	fakeClaude(t, `printf 'segfault\n' >&2; exit 7`)
	_, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view")
	var refused *ModelRefused
	if errors.As(err, &refused) {
		t.Fatalf("a plain failure was read as a refused model: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("error = %v; want the did-not-answer sentence", err)
	}
}

// Driven through the wrapper rather than through Configure, which would need a provider on PATH and a
// policy on disk to reach the same line.
func TestARefusalNamesThePolicyFileThatChoseTheModel(t *testing.T) {
	refuse := func(string, string) (string, error) {
		return "", &ModelRefused{Client: "codex", Model: "gpt-5.4-mini"}
	}
	_, err := namingTheFileThatDecides(refuse, "/somewhere/models.json", "/somewhere/reader-judge.conf")("prompt", "view")
	if err == nil {
		t.Fatal("the wrapper dropped the refusal")
	}
	// "reader-judge task" and not "judge profile": a refusal sends the reader to a key to edit, and
	// v4 deleted `profiles`. A message naming a construct the file no longer has costs the reader the
	// one thing this wrapper exists to give them.
	for _, want := range []string{"codex refused the model gpt-5.4-mini", "reader-judge task", "/somewhere/models.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err.Error(), want)
		}
	}
}

// The wrapper sits on every roll, so it must be invisible to the ones that succeed and to the ones
// that fail some other way.
func TestNamingThePolicyLeavesEveryOtherAnswerAlone(t *testing.T) {
	answered := func(string, string) (string, error) { return "none", nil }
	if reply, err := namingTheFileThatDecides(answered, "/somewhere/models.json", "/somewhere/reader-judge.conf")("prompt", "view"); reply != "none" || err != nil {
		t.Errorf("a good roll came back %q, %v; want none, nil", reply, err)
	}
	broke := func(string, string) (string, error) {
		return "", errors.New("the model did not answer (exit status 7)")
	}
	_, err := namingTheFileThatDecides(broke, "/somewhere/models.json", "/somewhere/reader-judge.conf")("prompt", "view")
	if err == nil || strings.Contains(err.Error(), "models.json") || strings.Contains(err.Error(), "reader-judge.conf") {
		t.Errorf("an unrelated failure was blamed on a config file: %v", err)
	}
}

// The other half of the wrapper. A roll cut off by the bound is the one failure a human repairs by
// raising a number, and the number lives in a file the error is the only place they will be told
// about — nothing else in a gate's output points at it.
func TestACutOffRollNamesTheFileThatSetsTheBound(t *testing.T) {
	cutOff := func(string, string) (string, error) { return "", &RollTimedOut{Deadline: 900 * time.Second} }
	_, err := namingTheFileThatDecides(cutOff, "/somewhere/models.json", "/somewhere/reader-judge.conf")("prompt", "view")
	if err == nil {
		t.Fatal("the wrapper dropped the expiry")
	}
	for _, want := range []string{"did not answer within 15m0s", "roll-timeout", "/somewhere/reader-judge.conf"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expiry %q does not say %q", err.Error(), want)
		}
	}
	// A machine with no absolute config home has nowhere an override could sit, and a message
	// pointing at the empty string is worse than the bare bound.
	_, nowhere := namingTheFileThatDecides(cutOff, "/somewhere/models.json", "")("prompt", "view")
	if nowhere == nil || strings.Contains(nowhere.Error(), "roll-timeout") {
		t.Errorf("with nowhere to set it, the expiry still sent someone to a file: %v", nowhere)
	}
}

// A stall is silent from the outside — the two 343-second rolls this bound was measured against
// printed nothing on either stream — so the point of the announcer is that something arrives while
// the wait is happening rather than only in the error that ends it.
//
// Both halves run off a clock this case holds. A roll made slow by sleeping past a real interval
// asserts instead that 60ms of wall clock outruns 10ms of it. That is false on a loaded machine, and
// it is what turned this case red on a green tree.
func TestASlowRollSaysItIsStillWaitingAndAFastOneSaysNothing(t *testing.T) {
	stalling, said := newHeldClock(), newAnnouncedLines()
	slow := func(string, string) (string, error) {
		stalling.tick(t)
		said.awaitLine(t)
		return "none", nil
	}
	if reply, err := announcingOnEachTick(slow, notTheSubject, said, stalling.ticking)("prompt", "view"); reply != "none" || err != nil {
		t.Fatalf("the announcer changed the answer: %q %v", reply, err)
	}
	if !strings.Contains(said.String(), "still waiting") || !strings.Contains(said.String(), "1h0m0s") {
		t.Errorf("a slow roll did not say it was waiting, or against what: %q", said.String())
	}

	// No tick is sent here at all, and a line could only come from an announcer that speaks on entry or
	// on exit. The case waits for the watcher to let its clock go, and that is what makes the emptiness
	// final instead of merely unobserved.
	answering, quiet := newHeldClock(), newAnnouncedLines()
	quick := func(string, string) (string, error) { return "none", nil }
	if _, err := announcingOnEachTick(quick, notTheSubject, quiet, answering.ticking)("prompt", "view"); err != nil {
		t.Fatal(err)
	}
	answering.awaitStop(t)
	if quiet.String() != "" {
		t.Errorf("a roll that answered at once still announced itself: %q", quiet.String())
	}

	// Nil is the suite's and the eval's setting, and it must not merely go unread: a decorator that
	// wrote to a nil writer would panic on the first tick, in a goroutine, taking the run with it. So
	// what a silent announcer owes is a clock it never starts.
	unwatched := func() (<-chan time.Time, func()) {
		t.Error("a silent announcer started a clock, so a tick would reach a nil writer")
		return nil, func() {}
	}
	if reply, err := announcingOnEachTick(quick, notTheSubject, nil, unwatched)("prompt", "view"); reply != "none" || err != nil {
		t.Fatalf("a silent announcer changed the answer: %q %v", reply, err)
	}
}

// The production wiring of that clock, which TestASlowRollSaysItIsStillWaitingAndAFastOneSaysNothing
// replaces: a real ticker does reach a roll that is still waiting. The roll here ends on the line, and
// never on a duration. Load makes this slower, and never wrong.
func TestTheAnnouncersOwnTickerReachesARollThatIsStillWaiting(t *testing.T) {
	said := newAnnouncedLines()
	waiting := func(string, string) (string, error) {
		said.awaitLine(t)
		return "none", nil
	}
	if reply, err := announcingASlowRoll(waiting, notTheSubject, time.Millisecond, said)("prompt", "view"); reply != "none" || err != nil {
		t.Fatalf("the announcer changed the answer: %q %v", reply, err)
	}
	if !strings.Contains(said.String(), "still waiting") {
		t.Errorf("a ticker of the announcer's own never reached the roll: %q", said.String())
	}
}

// announcerDeadlock bounds a step that is instant when the announcer works: a tick it is already
// selecting on, a write it has already been told to make. Only a broken announcer ever waits this
// long. It is a net under a hang, and never an allowance for a slow machine. That is why it can be
// this coarse without becoming the budget this file's scan refuses.
const announcerDeadlock = 10 * time.Second

// heldClock is the announcer's clock with the case holding it, and no tick happens unless the case
// sends it. `stopped` closes when the roll it belongs to lets its watcher go.
type heldClock struct {
	ticks   chan time.Time
	stopped chan struct{}
}

func newHeldClock() *heldClock {
	return &heldClock{ticks: make(chan time.Time), stopped: make(chan struct{})}
}

func (c *heldClock) ticking() (<-chan time.Time, func()) {
	return c.ticks, sync.OnceFunc(func() { close(c.stopped) })
}

// tick hands the watcher one tick and returns once it has taken it, so what follows is ordered after
// the announcement instead of hoping to outrun it.
func (c *heldClock) tick(t *testing.T) {
	t.Helper()
	select {
	case c.ticks <- time.Now():
	case <-time.After(announcerDeadlock):
		t.Error("the announcer took no tick, so a stalled roll would never say it was waiting")
	}
}

func (c *heldClock) awaitStop(t *testing.T) {
	t.Helper()
	select {
	case <-c.stopped:
	case <-time.After(announcerDeadlock):
		t.Error("the watcher outlived its roll, so a later tick could still announce a finished one")
	}
}

// announcedLines is where a case reads the announcer's output and how it learns a line has landed. The
// announcer writes from a goroutine per roll, so the destination has to be safe for two of them.
// `written` lets a roll end only once the line it asked for is in hand.
type announcedLines struct {
	mu      sync.Mutex
	builder strings.Builder
	written chan struct{}
}

func newAnnouncedLines() *announcedLines {
	return &announcedLines{written: make(chan struct{}, 1)}
}

func (a *announcedLines) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	written, err := a.builder.Write(p)
	select {
	case a.written <- struct{}{}:
	default:
	}
	return written, err
}

func (a *announcedLines) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.builder.String()
}

func (a *announcedLines) awaitLine(t *testing.T) {
	t.Helper()
	select {
	case <-a.written:
	case <-time.After(announcerDeadlock):
		t.Error("the announcer let a tick pass and wrote nothing")
	}
}

// The false positive the subtraction exists to stop, and why it is not hypothetical: judge this very
// file and every marker string refusal.go lists is inside the view.
func TestAMarkerInsideTheJudgedTextIsNotAProviderRefusal(t *testing.T) {
	fakeCodex(t, `cat >&2; exit 9`)
	view := "1 the model is not supported when using Codex with a ChatGPT account\n2 a second line"
	_, err := CodexCaller(notTheSubject, testSettings())("prompt", view)
	var refused *ModelRefused
	if errors.As(err, &refused) {
		t.Fatalf("the judged text was read as the provider refusing a model: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("error = %v; want the did-not-answer sentence", err)
	}
}

// The other half of that subtraction: a CLI that echoes its input and then refuses the model must
// still read as a refusal — the echo goes, the CLI's own line stays.
func TestARefusalSurvivesACliThatEchoesItsInput(t *testing.T) {
	fakeCodex(t, `cat >&2
printf 'ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The '"'"'fixture-model'"'"' model is not supported when using Codex with a ChatGPT account."}}\n' >&2
exit 1`)
	_, err := CodexCaller(notTheSubject, testSettings())("prompt", "1 a line of judged text\n2 another")
	var refused *ModelRefused
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v; want a ModelRefused", err)
	}
	if refused.Model != "fixture-model" {
		t.Errorf("refusal names %q; want fixture-model", refused.Model)
	}
}

func TestARollIsHandedOnlyTheAllowListedEnvironment(t *testing.T) {
	kept := rollEnv([]string{
		"HOME=/h", "PATH=/p", "LC_ALL=C", "ANTHROPIC_API_KEY=k", "CODEX_HOME=/c",
		"ANTHROPIC_BASE_URL=http://attacker", "OPENAI_BASE_URL=http://attacker",
		"HTTPS_PROXY=http://attacker", "MAX_THINKING_TOKENS=0", "NOT_A_PAIR",
	})
	want := []string{"HOME=/h", "PATH=/p", "LC_ALL=C", "ANTHROPIC_API_KEY=k", "CODEX_HOME=/c"}
	if strings.Join(kept, " ") != strings.Join(want, " ") {
		t.Fatalf("kept %q, want %q — a destination must not pass where a credential does", kept, want)
	}
}

// The allow-list at the syscall rather than in a slice: a child really does see only these, and a
// caller's own extra really does reach it.
func TestTheChildProcessSeesTheAllowListAndTheCommandsOwnAdditions(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "http://attacker")
	t.Setenv("JUDGE_TEST_STRAY", "stray")
	out, err := runBounded(notTheSubject, modelCommand{
		name: "/usr/bin/env", model: "none", env: []string{"MAX_THINKING_TOKENS=0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"ANTHROPIC_BASE_URL", "JUDGE_TEST_STRAY"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("%s reached the roll:\n%s", unwanted, out)
		}
	}
	for _, wanted := range []string{"PATH=", "MAX_THINKING_TOKENS=0"} {
		if !strings.Contains(out, wanted) {
			t.Errorf("%s did not reach the roll:\n%s", wanted, out)
		}
	}
}

func TestAnExhaustedLoginIsNotAnAnswer(t *testing.T) {
	fakeCodex(t, `printf "Usage limit reached. Try again later.\n"`)
	_, err := CodexCaller(notTheSubject, testSettings())("judge", "text")
	var exhausted *ProviderExhausted
	if !errors.As(err, &exhausted) {
		t.Fatalf("got %v, want the login reported as exhausted rather than as a verdict", err)
	}
}

// The subtraction refusedTheModel does, for the same reason: a document about rate limits must not
// report the account out of capacity.
func TestJudgedTextAboutRateLimitsIsNotAnExhaustedLogin(t *testing.T) {
	said := []byte("You've hit your session limit · resets 3:50pm\n")
	if !exhausted("claude", "some other text", said) {
		t.Fatal("the measured apology was not recognised")
	}
	if exhausted("claude", "You've hit your session limit · resets 3:50pm", said) {
		t.Fatal("the judged text was read back as the provider's own answer")
	}
}
