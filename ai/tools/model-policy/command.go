package modelpolicy

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Load(path string) (*Policy, error) {
	raw, err := readJsonFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model policy: %w", err)
	}
	return Parse(raw)
}

// InstalledPath follows the invoked stub or tools/bin binary, never the target repository.
func InstalledPath(invocation string) (string, error) {
	absolute, err := filepath.Abs(invocation)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("locate model policy from invocation: %w", err)
	}
	dir := filepath.Dir(real)
	switch filepath.Base(real) {
	case "model-policy.sh", "bloat-judge.sh":
		if filepath.Base(dir) == "scripts" {
			return filepath.Join(dir, "..", "models.json"), nil
		}
	case "model-policy", "bloat-judge":
		if filepath.Base(dir) == "bin" {
			return filepath.Join(dir, "..", "..", "kk-flavor", "models.json"), nil
		}
	}
	return "", fmt.Errorf("cannot locate installed model policy from %q; name --config explicitly", invocation)
}

type Command struct {
	Args       []string
	Invocation string
	Stdout     io.Writer
	Stderr     io.Writer
}

func Run(command Command) int {
	flags := flag.NewFlagSet("model-policy", flag.ContinueOnError)
	flags.SetOutput(command.Stderr)
	config := flags.String("config", "", "policy JSON file; defaults to models.json beside the installed flavor scripts")
	client := flags.String("client", "", "required: codex or claude")
	task := flags.String("task", "", "task from the policy, such as kk-code-review or patrol/scout")
	limits := flags.Bool("limits", false, "emit the policy's limits instead of a task's settings")
	flags.Usage = func() {
		fmt.Fprintln(command.Stderr, "Usage: model-policy.sh --client codex|claude --task <task> [--config <file>]\nEmits the requested settings and the policy digest. It never dispatches, and never reports what actually ran.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(command.Args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		return refuse(command.Stderr, "unexpected positional arguments")
	}
	if *config == "" {
		path, err := InstalledPath(command.Invocation)
		if err != nil {
			return refuse(command.Stderr, err)
		}
		*config = path
	}
	policy, err := Load(*config)
	if err != nil {
		return refuse(command.Stderr, err)
	}
	if *limits {
		if err := json.NewEncoder(command.Stdout).Encode(policy.Limits()); err != nil {
			return refuse(command.Stderr, "write limits:", err)
		}
		return 0
	}
	decision, err := policy.Resolve(Request{Client: *client, Task: *task})
	if err != nil {
		return refuse(command.Stderr, err)
	}
	if err := json.NewEncoder(command.Stdout).Encode(decision); err != nil {
		return refuse(command.Stderr, "write decision:", err)
	}
	return 0
}

// Every decline leaves the same two marks: the tool's name ahead of the reason on stderr, and status
// 2, which the shell stubs read as "did not run". One place, so a new failure path cannot differ.
func refuse(stderr io.Writer, reason ...any) int {
	fmt.Fprintln(stderr, append([]any{"model-policy:"}, reason...)...)
	return 2
}

func readJsonFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, fmt.Errorf("JSON file exceeds 1 MiB")
	}
	return raw, nil
}
