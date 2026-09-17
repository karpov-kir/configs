package tools_test

// The live kk-reduce ledger, held against the seed a fresh checkout is given.
//
// Here rather than in `ai/tools/eco-stats/` because the ledger is outside the module: Go keys a
// package's test cache on the module, so a case there would answer `ok (cached)` over a ledger whose
// opening rules had changed underneath the run. That a first run writes the seed verbatim is that
// package's own case — the two together are what hold the pair.

import (
	"os"
	"strings"
	"testing"

	ecostats "kk-flavor/tools/eco-stats"
)

const liveLedger = repoRoot + "/ai/kk-flavor/skills/kk-reduce/stats.md"

// The seed and the live ledger are a .md/source pair, which the shared-region scan cannot cover — it
// reads `*.sh` only. Drift between them costs a fresh install the rules the real file owns, and
// nothing but this case notices: the seed path runs only when there is no ledger, never on the tree
// that would show it.
func TestTheSeededLedgerSaysWhatTheLiveOneSays(t *testing.T) {
	live, err := os.ReadFile(liveLedger)
	if err != nil {
		t.Fatalf("the live ledger is what this case compares against: %v", err)
	}
	if !strings.HasPrefix(string(live), ecostats.LedgerSeed) {
		t.Errorf("%s no longer opens with the seed a fresh install is given, so the two have drifted and a "+
			"new checkout begins life under different rules from this one:\nseed:\n%s\nlive:\n%s",
			liveLedger, ecostats.LedgerSeed, firstBytes(string(live), len(ecostats.LedgerSeed)))
	}
}

// As much of the live file as the seed is long, so a failure shows the two ends that differ rather
// than every row ever appended.
func firstBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}
