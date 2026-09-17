// The judge as a command.
//
//	usage: reader-judge.sh [--config <policy.json>] [--numbers] [--changed[=<revisions>]] <kind> [<path>]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	modelpolicy "kk-flavor/tools/model-policy"
	readerjudge "kk-flavor/tools/reader-judge"
)

func main() {
	self := filepath.Base(os.Args[0])
	// An unreadable home leaves nowhere for an override to sit, so it reads as no override at all.
	home, _ := os.UserHomeDir()
	deadline, overridePath, ok := readerjudge.ResolveRollDeadline(self, os.Getenv("XDG_CONFIG_HOME"), home, os.Stderr)
	if !ok {
		os.Exit(2)
	}
	args := os.Args[1:]
	policyPath := ""
	if len(args) > 0 && args[0] == "--config" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "reader-judge: --config needs a policy file")
			os.Exit(2)
		}
		policyPath = args[1]
		args = args[2:]
	}
	// Before the policy and the provider, so an invocation error is answered with the grammar on a
	// machine that can reach no model at all. The earlier order resolved the provider first, so a bare
	// `reader-judge.sh` refused with "no provider" on a machine missing the CLI. It passed here and
	// failed in CI, and it sent the reader to install software when their command was the thing to fix.
	if readerjudge.RefuseIfNotTheGrammar(self, args, os.Stderr) {
		os.Exit(2)
	}
	if policyPath == "" {
		var err error
		policyPath, err = modelpolicy.InstalledPath(os.Args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "reader-judge:", err)
			os.Exit(2)
		}
	}
	configured, err := readerjudge.Configure(readerjudge.Configuration{
		Deadline: deadline, PolicyPath: policyPath, OverridePath: overridePath, Progress: os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v — the judge did NOT run\n", self, err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "reader-judge: requested client=%s model=%s effort=%q rolls=%d policy=%s\n", configured.Decision.Client, configured.Decision.Requested.Model, configured.Decision.Requested.Effort, configured.Decision.Rolls, configured.Decision.PolicyDigest)
	memo := readerjudge.DefaultMemo(configured.CacheIdentity)
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	os.Exit(readerjudge.Run(self, args, os.Stdin, os.Stdout, os.Stderr,
		readerjudge.Voting(configured.Call, configured.Decision.Rolls), memo))
}
