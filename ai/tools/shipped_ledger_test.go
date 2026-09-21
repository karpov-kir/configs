package tools_test

// This file compares the live kk-reduce ledger with the seed a fresh checkout is given.
//
// It lives in this package with the other cases that read the shipped checkout, for the reason
// shipped_tree_test.go gives. That a first run writes the seed verbatim is the `ecostats` package's
// own case, and the two together hold the pair.

import (
	"os"
	"strings"
	"testing"

	ecostats "configs/ai/tools/eco-stats"
)

const liveLedger = repoRoot + "/ai/kk-flavor/skills/kk-reduce/stats.md"

// The seed and the live ledger are a .md/source pair, and the shared-region scan reads `*.sh` only,
// so it cannot cover them. A divergence costs a fresh install the rules the real file owns, and this
// case is the only place it shows. The seed path runs when a ledger is absent, and the tree that
// would show a divergence never takes it.
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

// As much of the live file as the seed is long. A failure then shows the two ends that differ, and
// leaves out every row ever appended.
func firstBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}
