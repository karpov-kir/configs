package readerjudge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/shell"
)

// judgeTask is the tool's own models.json row, and the fallback for a kind with no row of its own.
const judgeTask = "reader-judge"

type Configuration struct {
	Deadline   time.Duration
	PolicyPath string
	// Task is the models.json row to ask for, empty for the tool's own.
	Task string
	// OverridePath is where this machine's roll bound is set, from ResolveRollDeadline. Carried so a
	// roll that times out can name it; empty when there is nowhere for an override to sit.
	OverridePath string
	// Progress is where a roll says it is still waiting. Nil is silent, which is what the suite and
	// the eval want; the command passes stderr.
	Progress io.Writer
}
type Configured struct {
	Call          Caller
	Decision      modelpolicy.Decision
	CacheIdentity string
	// Task is the models.json row that answered. It is the sub-row wherever one exists.
	Task string
}

func Configure(configuration Configuration) (Configured, error) {
	if _, present := os.LookupEnv("JUDGE_MODEL"); present {
		return Configured{}, fmt.Errorf("JUDGE_MODEL is retired; set the reader-judge task in models.json or use --config for an evaluation policy")
	}
	provider, err := resolveProvider()
	if err != nil {
		return Configured{}, err
	}
	policy, err := modelpolicy.Load(configuration.PolicyPath)
	if err != nil {
		return Configured{}, err
	}
	// A kind with a row of its own takes it. One kind can then be read by a different tier while the
	// rest keep the price a delete vote is worth. The row is optional, and a kind without one falls
	// back to the tool's. Which row answered is on the line the command prints.
	task := configuration.Task
	if task == "" {
		task = judgeTask
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: provider, Task: task})
	if err != nil && task != judgeTask {
		task = judgeTask
		decision, err = policy.Resolve(modelpolicy.Request{Client: provider, Task: task})
	}
	if err != nil {
		return Configured{}, err
	}
	// A vote needs a roll to count; the policy owns how many, so an unset count is its error, not a
	// default this tool supplies.
	if decision.Rolls < 1 {
		return Configured{}, fmt.Errorf("the reader-judge task sets no roll count, so there is no vote to take")
	}
	call := ClaudeCaller(configuration.Deadline, decision.Requested)
	if provider == "codex" {
		call = CodexCaller(configuration.Deadline, decision.Requested)
	}
	call = namingTheFileThatDecides(call, configuration.PolicyPath, configuration.OverridePath)
	call = announcingASlowRoll(call, configuration.Deadline, rollSilence, configuration.Progress)
	return Configured{Call: call, Decision: decision, Task: task, CacheIdentity: decision.PolicyDigest + "/" + task + "/" + decision.Client + "/" + decision.Requested.Model + "/" + decision.Requested.Effort}, nil
}

// namingTheFileThatDecides puts a file into the two roll failures whose repair is an edit to one: the
// name the provider would not run is in the policy, and the bound that cut a roll off is in the
// override. Neither error can name its own file — the deadline reaches runBounded as a duration and
// the model reaches it as a string in argv — so the naming happens once, here, where both paths are
// in hand. A wrapper and not the caller constructors: they are handed a model and never its source,
// and --config means neither path is a constant to hard-code.
func namingTheFileThatDecides(call Caller, policyPath, overridePath string) Caller {
	return func(prompt, view string) (string, error) {
		reply, err := call(prompt, view)
		var refused *ModelRefused
		if errors.As(err, &refused) {
			return "", fmt.Errorf("%w, which the reader-judge task in %s names", err, policyPath)
		}
		// Only when there is a path to name. A machine with no absolute config home has nowhere an
		// override could sit, and pointing at the empty string would be worse than the bare bound.
		var cutOff *RollTimedOut
		if errors.As(err, &cutOff) && overridePath != "" {
			return "", fmt.Errorf("%w — raise it with a `%s <seconds>` line in %s, or run the judge over less at once",
				err, rollTimeoutKey, overridePath)
		}
		return reply, err
	}
}

// rollSilence is how long a roll may say nothing before it says it is still there. Measured
// 2026-09-16: two runs stalled at about 343 seconds with nothing on either stream, which reads from
// the outside exactly like the hang the deadline exists to end. A minute is well past the 19 to 45
// seconds a roll takes when the provider is answering, so a healthy run stays silent.
const rollSilence = time.Minute

// announcingASlowRoll makes a stall visible while it happens rather than only in the error that ends
// it, and names the bound, since a reader watching this is deciding whether to wait or to kill the
// run. The interval is a parameter so a case need not wait out a real minute to see one line.
func announcingASlowRoll(call Caller, deadline, silence time.Duration, progress io.Writer) Caller {
	return announcingOnEachTick(call, deadline, progress, func() (<-chan time.Time, func()) {
		ticker := time.NewTicker(silence)
		return ticker.C, ticker.Stop
	})
}

// `ticking` starts one clock per roll and hands back the stop that ends it. One decorator wraps the
// caller a vote then calls once per roll, so the lock is shared and the lines cannot interleave. A nil
// destination starts no clock, since a tick reaching one would panic in a goroutine.

// announcingOnEachTick takes the clock, and never a duration to build one from. A case then releases a
// tick by hand and reads the line it caused instead of sleeping past an interval and racing it.
func announcingOnEachTick(call Caller, deadline time.Duration, progress io.Writer, ticking func() (<-chan time.Time, func())) Caller {
	if progress == nil {
		return call
	}
	var speaking sync.Mutex
	return func(prompt, view string) (string, error) {
		started, done := time.Now(), make(chan struct{})
		ticks, stop := ticking()
		go func() {
			defer stop()
			for {
				select {
				case <-done:
					return
				case <-ticks:
					speaking.Lock()
					fmt.Fprintf(progress, "reader-judge: a roll is still waiting, %s of its %s\n",
						time.Since(started).Round(time.Second), deadline)
					speaking.Unlock()
				}
			}
		}()
		reply, err := call(prompt, view)
		close(done)
		return reply, err
	}
}

func resolveProvider() (string, error) {
	provider := os.Getenv("JUDGE_PROVIDER")
	if provider == "" {
		return "", fmt.Errorf("JUDGE_PROVIDER is required: set JUDGE_PROVIDER=claude or JUDGE_PROVIDER=codex")
	}
	if provider != "claude" && provider != "codex" {
		return "", fmt.Errorf("JUDGE_PROVIDER must be claude or codex, got %s", shell.Echoable(provider))
	}
	if _, err := exec.LookPath(provider); err != nil {
		return "", fmt.Errorf("JUDGE_PROVIDER=%s requires %s on PATH", provider, provider)
	}
	return provider, nil
}

// ClaudeCaller is the real one: `claude -p` on the CLI's own login, so no key is needed locally. Each
// roll is bounded — deadline.go carries the figure and why an unbounded one was the wrong shape.
func ClaudeCaller(deadline time.Duration, settings modelpolicy.Settings) Caller {
	return ClaudeCallerObserved(deadline, settings, nil)
}

// Served is what one Claude call reports about itself: the model it asked for, the model that wrote
// the answer, and what it spent. An account can serve another model than the row asks for, and only
// this report says so. On 2026-09-25 an organisation's team account answered `sonnet` and `haiku` as
// claude-opus-5-5[1m], and every row naming either had run on Opus unnoticed.
type Served struct {
	Requested, Answered                    string
	Input, CacheCreated, CacheRead, Output int
	CostUSD                                float64
}

// Substituted says another model than the requested one wrote the answer.
func (s Served) Substituted() bool {
	return s.Answered != "" && !strings.Contains(strings.ToLower(s.Answered), strings.ToLower(s.Requested))
}

// ClaudeCallerObserved is ClaudeCaller handing each call's Served to observe. A substituted model is
// reported and never refused. The call answers on what the account serves, and model-check is where a
// person reads it.
func ClaudeCallerObserved(deadline time.Duration, settings modelpolicy.Settings, observe func(Served)) Caller {
	return func(prompt, view string) (string, error) {
		out, err := runBounded(deadline, modelCommand{name: "claude", args: claudeArgs(prompt, settings), stdin: view, model: settings.Model})
		var exhausted *ProviderExhausted
		if errors.As(err, &exhausted) {
			exhausted.Account = claudeAccount()
		}
		if err != nil {
			return "", err
		}
		answer, served, err := readClaudeReply(out, settings.Model)
		if err != nil {
			return "", err
		}
		if observe != nil {
			observe(served)
		}
		return answer, nil
	}
}

// readClaudeReply reads `--output-format json`: the answer, and what the call reports about itself.
// The answering model is the entry with the most output tokens, because the CLI makes a small call of
// its own on another model beside it. A reply that is not JSON is taken as the answer it prints, so a
// CLI that changed its output still answers.
func readClaudeReply(out, requested string) (string, Served, error) {
	served := Served{Requested: requested}
	var reply struct {
		Result  string  `json:"result"`
		IsError bool    `json:"is_error"`
		Cost    float64 `json:"total_cost_usd"`
		Usage   struct {
			Input        int `json:"input_tokens"`
			CacheCreated int `json:"cache_creation_input_tokens"`
			CacheRead    int `json:"cache_read_input_tokens"`
			Output       int `json:"output_tokens"`
		} `json:"usage"`
		ModelUsage map[string]struct {
			Output int `json:"outputTokens"`
		} `json:"modelUsage"`
	}
	if json.Unmarshal([]byte(out), &reply) != nil {
		return out, served, nil
	}
	if reply.IsError {
		return "", served, fmt.Errorf("the model answered an error: %s", shell.CutBytesMarked(shell.Oneline(reply.Result), 200))
	}
	served.Input, served.CacheCreated = reply.Usage.Input, reply.Usage.CacheCreated
	served.CacheRead, served.Output, served.CostUSD = reply.Usage.CacheRead, reply.Usage.Output, reply.Cost
	most := -1
	for model, spent := range reply.ModelUsage {
		if spent.Output > most || (spent.Output == most && model < served.Answered) {
			served.Answered, most = model, spent.Output
		}
	}
	return reply.Result, served, nil
}

// claudeAccount names the login a roll runs on, for a message a person reads. Someone with several
// accounts can switch the app and leave the CLI on another, and a usage limit then reads as the app's.
func claudeAccount() string { return accountIn(rollEnv(os.Environ())) }

// ClaudeAccounts names both logins a call here can reach. Inside the desktop app a call inheriting its
// variables runs on the app's sign-in, and a roll, which keeps only the allow-list, runs on the CLI's
// own login. The app can switch one and leave the other. Outside the app the two are one login.
func ClaudeAccounts() (inherited, own string) {
	return accountIn(os.Environ()), accountIn(rollEnv(os.Environ()))
}

func accountIn(env []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), accountDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", "auth", "status", "--json")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var status struct {
		LoggedIn     bool   `json:"loggedIn"`
		Email        string `json:"email"`
		Org          string `json:"orgName"`
		Subscription string `json:"subscriptionType"`
	}
	if json.Unmarshal(out, &status) != nil || !status.LoggedIn {
		return ""
	}
	return shell.CutBytesMarked(shell.Oneline(fmt.Sprintf("%s (%s, %s)", status.Email, status.Org, status.Subscription)), 120)
}

// accountDeadline bounds the call that names the login. It runs only after a usage limit or in
// model-check.
const accountDeadline = 20 * time.Second

// claudeArgs gives the model nothing but the reply: no tools, no MCP servers, and no settings from
// anywhere. An untrusted branch's `.claude/settings.json` would otherwise bring its hooks and allow
// rules, and with the `user` source the operator's own `CLAUDE.md` made a roll answer in prose where
// a verdict belongs. Every empty-valued flag is followed by another, and the prompt comes last.
func claudeArgs(prompt string, settings modelpolicy.Settings) []string {
	args := []string{
		"-p", "--model", settings.Model, "--output-format", "json",
		"--tools", "", "--setting-sources", "", "--strict-mcp-config",
	}
	if settings.Effort != "" {
		args = append(args, "--effort", settings.Effort)
	}
	return append(args, prompt)
}

func CodexCaller(deadline time.Duration, settings modelpolicy.Settings) Caller {
	return func(prompt, view string) (string, error) {
		dir, err := os.MkdirTemp("", "reader-judge-")
		if err != nil {
			return "", fmt.Errorf("could not isolate Codex: %w", err)
		}
		defer os.RemoveAll(dir)
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
			return "", fmt.Errorf("could not mark the isolated Codex root: %w", err)
		}
		answer := filepath.Join(dir, "answer")
		_, err = runBounded(deadline, modelCommand{
			name: "codex", args: codexArgs(answer, settings), dir: dir,
			stdin: prompt + "\n\n" + view, model: settings.Model,
		})
		if err != nil {
			return "", err
		}
		raw, err := os.ReadFile(answer)
		if err != nil {
			return "", fmt.Errorf("Codex did not write its final answer: %w", err)
		}
		return string(raw), nil
	}
}

func codexArgs(answer string, settings modelpolicy.Settings) []string {
	args := []string{
		"exec", "--ignore-user-config", "--ignore-rules", "--ephemeral",
		"--skip-git-repo-check", "--sandbox", "read-only", "--color", "never",
		"--output-last-message", answer,
		"-c", "approval_policy=\"never\"", "-c", "project_doc_max_bytes=0",
		"-c", "web_search=\"disabled\"", "-c", "tools.update_plan.enabled=false",
		"-c", "suppress_unstable_features_warning=true",
	}
	// Omitted rather than passed empty: `--model ""` asks the CLI for a model with no name instead of
	// asking for none. A parsed policy names one on every row, so this guards a Settings built here.
	if settings.Model != "" {
		args = append(args, "--model", settings.Model)
	}
	if settings.Effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(settings.Effort))
	}
	for _, feature := range []string{
		"shell_tool", "unified_exec", "apps", "plugins", "browser_use", "computer_use",
		"in_app_browser", "image_generation", "view_image", "multi_agent", "hooks",
		"memories", "skill_search", "tool_suggest", "shell_snapshot", "goals", "sleep_tool",
	} {
		args = append(args, "--disable", feature)
	}
	return append(args, "--enable", "skip_host_skill_discovery", "-")
}
