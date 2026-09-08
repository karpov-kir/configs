// The judge as a command.
//
//	usage: bloat-judge.sh [--numbers] [--changed[=<revisions>]] <kind> [<path>]
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
	deadline, ok := bloatjudge.ResolveRollDeadline(self, os.Getenv("XDG_CONFIG_HOME"), home, os.Stderr)
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
	if policyPath == "" {
		var err error
		policyPath, err = modelpolicy.InstalledPath(os.Args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "bloat-judge:", err)
			os.Exit(2)
		}
	}
	configured, err := bloatjudge.Configure(bloatjudge.Configuration{Deadline: deadline, PolicyPath: policyPath})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v — the judge did NOT run\n", self, err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "bloat-judge: requested client=%s model=%s effort=%q policy=%s\n", configured.Decision.Client, configured.Decision.Requested.Model, configured.Decision.Requested.Effort, configured.Decision.PolicyDigest)
	memo := bloatjudge.DefaultMemo(configured.CacheIdentity)
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	os.Exit(bloatjudge.Run(self, args, os.Stdin, os.Stdout, os.Stderr,
		bloatjudge.Voting(configured.Call, 3), memo))
}
