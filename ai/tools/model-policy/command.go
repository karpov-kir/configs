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
	role := flags.String("role", "", "required role from the policy")
	originPath := flags.String("origin", "", "origin JSON FILE with client, model and effort; never inline JSON")
	transport := flags.String("transport", "native", "native or cli; this resolver does not execute either")
	fromOriginal := flags.Bool("from-original-task", false, "attest this dispatch runs directly in the original task; only native inheritance can use it")
	digest := flags.String("policy-digest", "", "expected policy digest; refuse a changed policy")
	flags.Usage = func() {
		fmt.Fprintln(command.Stderr, "Usage: model-policy.sh --client codex|claude --role <role> [options]\nEmits requested settings and policy digest, never observed settings or an availability claim.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(command.Args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(command.Stderr, "model-policy: unexpected positional arguments")
		return 2
	}
	if *config == "" {
		path, err := InstalledPath(command.Invocation)
		if err != nil {
			fmt.Fprintln(command.Stderr, "model-policy:", err)
			return 2
		}
		*config = path
	}
	policy, err := Load(*config)
	if err != nil {
		fmt.Fprintln(command.Stderr, "model-policy:", err)
		return 2
	}
	request := Request{Client: *client, Role: *role, Transport: *transport, FromOriginalTask: *fromOriginal, PolicyDigest: *digest}
	if *originPath != "" {
		raw, err := readJsonFile(*originPath)
		if err != nil {
			fmt.Fprintln(command.Stderr, "model-policy: read origin:", err)
			return 2
		}
		request.Origin, err = ParseOrigin(raw)
		if err != nil {
			fmt.Fprintln(command.Stderr, "model-policy:", err)
			return 2
		}
	}
	decision, err := policy.Resolve(request)
	if err != nil {
		fmt.Fprintln(command.Stderr, "model-policy:", err)
		return 2
	}
	if err := json.NewEncoder(command.Stdout).Encode(decision); err != nil {
		fmt.Fprintln(command.Stderr, "model-policy: write decision:", err)
		return 2
	}
	return 0
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
