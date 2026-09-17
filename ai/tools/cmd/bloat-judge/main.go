// The judge as a command.
//
//	usage: bloat-judge.sh [--config <policy.json>] [--numbers | --strip=<dir>] [--changed[=<revisions>]] <kind> [<path>]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	bloatjudge "kk-flavor/tools/bloat-judge"
	modelpolicy "kk-flavor/tools/model-policy"
)

func main() {
	self := filepath.Base(os.Args[0])
	// An unreadable home leaves nowhere for an override to sit, which reads as no override rather
	// than as a broken one.
	home, _ := os.UserHomeDir()
	deadline, overridePath, ok := bloatjudge.ResolveRollDeadline(self, os.Getenv("XDG_CONFIG_HOME"), home, os.Stderr)
	if !ok {
		os.Exit(2)
	}
	args := os.Args[1:]
	policyPath := ""
	if len(args) > 0 && args[0] == "--config" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "bloat-judge: --config needs a policy file")
			os.Exit(2)
		}
		policyPath = args[1]
		args = args[2:]
	}
	// Before the policy and the provider, so an invocation error is answered with the grammar on a
	// machine that can reach no model at all. Resolving first made `bloat-judge.sh` with no arguments
	// refuse with "no provider" wherever no CLI is installed — green here, red in CI, and the reader
	// sent to install something rather than to fix the command they typed.
	// The strip calls no model, so it needs no policy and no provider, and its grammar is its own.
	if bloatjudge.StripRequested(args) {
		os.Exit(bloatjudge.Strip(self, args, ".", os.Stdout, os.Stderr))
	}
	if bloatjudge.RefuseIfNotTheGrammar(self, args, os.Stderr) {
		os.Exit(2)
	}
	if policyPath == "" {
		var err error
		policyPath, err = modelpolicy.InstalledPath(os.Args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "bloat-judge:", err)
			os.Exit(2)
		}
	}
	configured, err := bloatjudge.Configure(bloatjudge.Configuration{
		Deadline: deadline, PolicyPath: policyPath, OverridePath: overridePath, Progress: os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v — the judge did NOT run\n", self, err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "bloat-judge: requested client=%s model=%s effort=%q rolls=%d policy=%s\n", configured.Decision.Client, configured.Decision.Requested.Model, configured.Decision.Requested.Effort, configured.Decision.Rolls, configured.Decision.PolicyDigest)
	memo := bloatjudge.DefaultMemo(configured.CacheIdentity)
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	os.Exit(bloatjudge.Run(self, args, os.Stdin, os.Stdout, os.Stderr,
		bloatjudge.Voting(configured.Call, configured.Decision.Rolls), memo))
}
