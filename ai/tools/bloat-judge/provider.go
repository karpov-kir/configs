package bloatjudge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	modelpolicy "kk-flavor/tools/model-policy"
)

type Configuration struct {
	Deadline   time.Duration
	PolicyPath string
}
type Configured struct {
	Call          Caller
	Decision      modelpolicy.Decision
	CacheIdentity string
}

func Configure(configuration Configuration) (Configured, error) {
	if _, present := os.LookupEnv("JUDGE_MODEL"); present {
		return Configured{}, fmt.Errorf("JUDGE_MODEL is retired; select the judge profile in models.json or use --config for an evaluation policy")
	}
	provider, err := resolveProvider()
	if err != nil {
		return Configured{}, err
	}
	policy, err := modelpolicy.Load(configuration.PolicyPath)
	if err != nil {
		return Configured{}, err
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: provider, Role: "judge", Transport: "cli"})
	if err != nil {
		return Configured{}, err
	}
	call := ClaudeCaller(configuration.Deadline, decision.Requested)
	if provider == "codex" {
		call = CodexCaller(configuration.Deadline, decision.Requested)
	}
	call = namingWhatChoseTheModel(call, configuration.PolicyPath)
	return Configured{Call: call, Decision: decision, CacheIdentity: decision.PolicyDigest + "/" + decision.Client + "/" + decision.Requested.Model + "/" + decision.Requested.Effort}, nil
}

// namingWhatChoseTheModel puts the policy file into a refusal, since that is the one roll failure
// whose repair is an edit to a file. A wrapper and not the caller constructors: they are handed a
// model and never its source, and --config means the path is no constant to hard-code either.
func namingWhatChoseTheModel(call Caller, policyPath string) Caller {
	return func(prompt, view string) (string, error) {
		reply, err := call(prompt, view)
		var refused *ModelRefused
		if errors.As(err, &refused) {
			return "", fmt.Errorf("%w, which the judge profile in %s names", err, policyPath)
		}
		return reply, err
	}
}

func resolveProvider() (string, error) {
	provider := os.Getenv("JUDGE_PROVIDER")
	if provider == "" {
		return "", fmt.Errorf("JUDGE_PROVIDER is required: set JUDGE_PROVIDER=claude or JUDGE_PROVIDER=codex")
	}
	if provider != "claude" && provider != "codex" {
		return "", fmt.Errorf("JUDGE_PROVIDER must be claude or codex, got %s", echoable(provider))
	}
	if _, err := exec.LookPath(provider); err != nil {
		return "", fmt.Errorf("JUDGE_PROVIDER=%s requires %s on PATH", provider, provider)
	}
	return provider, nil
}

// ClaudeCaller is the real one: `claude -p` on the CLI's own login, so no key is needed locally. Each
// roll is bounded — deadline.go carries the figure and why an unbounded one was the wrong shape.
func ClaudeCaller(deadline time.Duration, settings modelpolicy.Settings) Caller {
	return func(prompt, view string) (string, error) {
		return runBounded(deadline, modelCommand{name: "claude", args: claudeArgs(prompt, settings), stdin: view, model: settings.Model})
	}
}

// claudeArgs gives the model nothing but the reply: no tools, no MCP servers, and no settings from the
// repository it runs in. The view is whatever the judged text says, and `-p` skips the workspace trust
// dialog. Without these flags a checked-out branch's `.claude/settings.json` would apply, allow rules
// and hooks and all, and a comment telling the model to run a command would be obeyed before the
// numbers came back. `--tools` is variadic, so an option follows it, never the prompt.
func claudeArgs(prompt string, settings modelpolicy.Settings) []string {
	args := []string{
		"-p", "--model", settings.Model, "--output-format", "text",
		"--tools", "", "--strict-mcp-config", "--setting-sources", "user",
	}
	if settings.Effort != "" {
		args = append(args, "--effort", settings.Effort)
	}
	return append(args, prompt)
}

func CodexCaller(deadline time.Duration, settings modelpolicy.Settings) Caller {
	return func(prompt, view string) (string, error) {
		dir, err := os.MkdirTemp("", "bloat-judge-")
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
		"--model", settings.Model, "--output-last-message", answer,
		"-c", "approval_policy=\"never\"", "-c", "project_doc_max_bytes=0",
		"-c", "web_search=\"disabled\"", "-c", "tools.update_plan.enabled=false",
		"-c", "suppress_unstable_features_warning=true",
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
