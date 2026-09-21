package readerjudge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	modelpolicy "configs/ai/tools/model-policy"
	"configs/ai/tools/repo"
)

// Main is the whole of the command, held here instead of in a `cmd/` package. The ORDER of the steps
// in this function is itself a behaviour, and a `main()` is a shape no case can call.

// A mistyped invocation must be answered with the grammar on a machine carrying no provider CLI. That
// holds only while the grammar is asked before a provider is resolved, and no function skipping
// provider resolution can exhibit that ordering.

// That is why TestReaderJudgeNamesItsGrammarWithNoProviderReachable drives Main, and cases over the
// pieces separately cannot replace it.

// invocation is argv[0]: it names the tool in its own messages, and names the install the model
// policy is found beside.
func Main(invocation string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	self := filepath.Base(invocation)
	// An unreadable home leaves nowhere for an override to sit. That reads as an absent override, and
	// never as a broken one.
	home, _ := os.UserHomeDir()
	deadline, overridePath, ok := ResolveRollDeadline(self, os.Getenv("XDG_CONFIG_HOME"), home, stderr)
	if !ok {
		return exitDidNotRun
	}
	policyPath := ""
	if len(args) > 0 && args[0] == "--config" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "reader-judge: --config needs a policy file")
			return exitDidNotRun
		}
		policyPath = args[1]
		args = args[2:]
	}
	// Before the policy and the provider, so an invocation error is answered with the grammar on a
	// machine that can reach no model at all. With resolution first, an argument-less invocation
	// refused with "no provider" wherever a CLI is missing. It was green here and red in CI, and it
	// sent the reader to install something instead of fixing the command they typed.
	if RefuseIfNotTheGrammar(self, args, stderr) {
		return exitDidNotRun
	}
	if policyPath == "" {
		var err error
		policyPath, err = modelpolicy.InstalledPath(invocation)
		if err != nil {
			fmt.Fprintln(stderr, "reader-judge:", err)
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
	fmt.Fprintf(stderr, "reader-judge: requested client=%s model=%s effort=%q rolls=%d policy=%s\n",
		configured.Decision.Client, configured.Decision.Requested.Model,
		configured.Decision.Requested.Effort, configured.Decision.Rolls, configured.Decision.PolicyDigest)
	memo := DefaultMemo(configured.CacheIdentity)
	if os.Getenv("JUDGE_NO_CACHE") != "" {
		memo = nil
	}
	return Run(self, args, repo.Exec{}, stdin, stdout, stderr,
		Voting(configured.Call, configured.Decision.Rolls), memo)
}
