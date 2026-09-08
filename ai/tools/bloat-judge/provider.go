package bloatjudge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func ConfiguredCaller(deadline time.Duration) (Caller, error) {
	provider, err := resolveProvider()
	if err != nil {
		return nil, err
	}
	if provider == "codex" {
		return CodexCaller(deadline), nil
	}
	return ClaudeCaller(deadline), nil
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

func judgeModel(fallback string) string {
	if model := os.Getenv("JUDGE_MODEL"); model != "" {
		return model
	}
	return fallback
}

func CodexCaller(deadline time.Duration) Caller {
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
			name: "codex", args: codexArgs(answer), dir: dir,
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

func codexArgs(answer string) []string {
	args := []string{
		"exec", "--ignore-user-config", "--ignore-rules", "--ephemeral",
		"--skip-git-repo-check", "--sandbox", "read-only", "--color", "never",
		"--model", judgeModel("gpt-5.4-mini"), "--output-last-message", answer,
		"-c", "approval_policy=\"never\"", "-c", "project_doc_max_bytes=0",
		"-c", "web_search=\"disabled\"", "-c", "tools.update_plan.enabled=false",
		"-c", "model_reasoning_effort=\"low\"",
		"-c", "suppress_unstable_features_warning=true",
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
