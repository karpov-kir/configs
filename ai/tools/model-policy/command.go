package modelpolicy

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	modelserved "configs/ai/tools/model-served"
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
	case "model-policy.sh", "reader-judge.sh", "model-check.sh":
		if filepath.Base(dir) == "scripts" {
			return filepath.Join(dir, "..", "configs", "models.json"), nil
		}
	case "model-policy", "reader-judge", "model-check":
		if filepath.Base(dir) == "bin" {
			return filepath.Join(dir, "..", "..", "kk-flavor", "configs", "models.json"), nil
		}
	}
	return "", fmt.Errorf("cannot locate installed model policy from %q; name --config explicitly", invocation)
}

type Command struct {
	Args       []string
	Invocation string
	Stdout     io.Writer
	Stderr     io.Writer
	// Account names the Claude login a dispatch from this process runs on. Nil means `claude auth status`.
	Account func() string
	// ServedCache is model-check's served set. Empty means the user's cache.
	ServedCache string
	// Now reads the clock the set's age is judged by. Nil means time.Now.
	Now func() time.Time
}

func Run(command Command) int {
	flags := flag.NewFlagSet("model-policy", flag.ContinueOnError)
	flags.SetOutput(command.Stderr)
	config := flags.String("config", "", "policy JSON file; defaults to models.json in the installed flavor's configs directory")
	client := flags.String("client", "", "required: codex or claude")
	task := flags.String("task", "", "task from the policy, such as code-review or patrol/scout")
	limits := flags.Bool("limits", false, "emit the policy's limits instead of a task's settings")
	flags.Usage = func() {
		fmt.Fprintln(command.Stderr, "usage: model-policy.sh --client codex|claude --task <task> [--config <file>] [--limits] [--help]\nEmits the requested settings and the policy digest. It never dispatches, and never reports what actually ran.")
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
	decision.Dispatched = decision.Requested
	if decision.Client == "claude" && decision.Kind == "worker" {
		dispatchServed(command, policy, &decision)
	}
	if err := json.NewEncoder(command.Stdout).Encode(decision); err != nil {
		return refuse(command.Stderr, "write decision:", err)
	}
	return 0
}

// dispatchServed moves a worker's dispatch to the nearest tier its account serves, where model-check
// kept a served set for this login within the day. Without one the row dispatches as written. A set
// that exists and cannot be used says why, so a person knows to run model-check.
func dispatchServed(command Command, policy *Policy, decision *Decision) {
	path := command.ServedCache
	if path == "" {
		path = modelserved.Path(os.LookupEnv)
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	account := command.Account
	if account == nil {
		account = func() string { return modelserved.Account(os.Environ()) }
	}
	now := time.Now
	if command.Now != nil {
		now = command.Now
	}
	entry, why := modelserved.Lookup(path, "claude", account(), now())
	if why != "" {
		fmt.Fprintln(command.Stderr, "model-policy:", why)
		return
	}
	requested := decision.Requested.Model
	decision.Dispatched.Model = modelserved.Nearest(entry, policy.Tiers("claude"), requested)
	if decision.Dispatched.Model != requested {
		fmt.Fprintf(command.Stderr, "model-policy: requested %s, dispatched %s (nearest served)\n", requested,
			decision.Dispatched.Model)
	}
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
