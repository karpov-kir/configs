package bloatjudge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/repo"
)

// Main is the whole of the command, held here rather than in `cmd/bloat-judge` because the ORDER of
// the steps below is itself a behaviour and a `main()` is the one shape no case can call. A mistyped
// invocation must be answered with the grammar on a machine carrying no provider CLI, and that holds
// only while the grammar is asked before a provider is resolved — an ordering no function that skips
// provider resolution can exhibit, which is why asserting on the pieces separately cannot replace
// TestBloatJudgeNamesItsGrammarWithNoProviderReachable driving this.
//
// invocation is argv[0]: it names the tool in its own messages, and names the install the model
// policy is found beside.
func Main(invocation string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	self := filepath.Base(invocation)
	// An unreadable home leaves nowhere for an override to sit, which reads as no override rather
	// than as a broken one.
	home, _ := os.UserHomeDir()
	deadline, overridePath, ok := ResolveRollDeadline(self, os.Getenv("XDG_CONFIG_HOME"), home, stderr)
	if !ok {
		return exitDidNotRun
	}
	policyPath := ""
	if len(args) > 0 && args[0] == "--config" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "bloat-judge: --config needs a policy file")
			return exitDidNotRun
		}
		policyPath = args[1]
		args = args[2:]
	}
	// Before the policy and the provider, so an invocation error is answered with the grammar on a
	// machine that can reach no model at all. Resolving first made `bloat-judge.sh` with no arguments
	// refuse with "no provider" wherever no CLI is installed — green here, red in CI, and the reader
	// sent to install something rather than to fix the command they typed.
	if RefuseIfNotTheGrammar(self, args, stderr) {
		return exitDidNotRun
	}
	if policyPath == "" {
		var err error
		policyPath, err = modelpolicy.InstalledPath(invocation)
		if err != nil {
			fmt.Fprintln(stderr, "bloat-judge:", err)
			return exitDidNotRun
		}
	}
	configured, err := Configure(Configuration{
		Deadline: deadline, PolicyPath: policyPath, OverridePath: overridePath, Progress: stderr,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
		return exitDidNotRun
	}
	fmt.Fprintf(stderr, "bloat-judge: requested client=%s model=%s effort=%q rolls=%d policy=%s\n",
		configured.Decision.Client, configured.Decision.Requested.Model,
		configured.Decision.Requested.Effort, configured.Decision.Rolls, configured.Decision.PolicyDigest)
	memo := DefaultMemo(configured.CacheIdentity)
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	return Run(self, args, repo.Exec{}, stdin, stdout, stderr,
		Voting(configured.Call, configured.Decision.Rolls), memo)
}
