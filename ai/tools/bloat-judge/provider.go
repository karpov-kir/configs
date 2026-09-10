package bloatjudge

import (
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
		return Configured{}, fmt.Errorf("JUDGE_MODEL is retired; set the bloat-judge task in models.json or use --config for an evaluation policy")
	}
	provider, err := resolveProvider()
	if err != nil {
		return Configured{}, err
	}
	policy, err := modelpolicy.Load(configuration.PolicyPath)
	if err != nil {
		return Configured{}, err
	}
	decision, err := policy.Resolve(modelpolicy.Request{Client: provider, Task: "bloat-judge"})
	if err != nil {
		return Configured{}, err
	}
	// A vote needs a roll to count; the policy owns how many, so an unset count is its error, not a
	// default this tool supplies.
	if decision.Rolls < 1 {
		return Configured{}, fmt.Errorf("the bloat-judge task sets no roll count, so there is no vote to take")
	}
	call := ClaudeCaller(configuration.Deadline, decision.Requested)
	if provider == "codex" {
		call = CodexCaller(configuration.Deadline, decision.Requested)
	}
	return Configured{Call: call, Decision: decision, CacheIdentity: decision.PolicyDigest + "/" + decision.Client + "/" + decision.Requested.Model + "/" + decision.Requested.Effort}, nil
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
			stdin: prompt + "\n\n" + view,
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

// An effort-only row means "keep the model, lower the effort", so the flag is omitted rather than
// passed empty — `--model ""` asks the CLI for a model with no name instead of asking for none.
func codexArgs(answer string, settings modelpolicy.Settings) []string {
	args := []string{
		"exec", "--ignore-user-config", "--ignore-rules", "--ephemeral",
		"--skip-git-repo-check", "--sandbox", "read-only", "--color", "never",
		"--output-last-message", answer,
		"-c", "approval_policy=\"never\"", "-c", "project_doc_max_bytes=0",
		"-c", "web_search=\"disabled\"", "-c", "tools.update_plan.enabled=false",
		"-c", "suppress_unstable_features_warning=true",
	}
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
