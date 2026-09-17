// A mistyped invocation has to be answered with the grammar on a machine that can reach no model.
//
// The command resolved its provider before it parsed its arguments, so `bloat-judge.sh` with no
// arguments refused with "no provider" wherever no CLI was installed, and printed the grammar
// wherever one was. That makes the answer a property of the machine rather than of the code:
// TestEveryStubDocumentsTheUsageItsBinaryPrints passed on a developer's laptop carrying `claude` and
// failed in CI, which carries neither — and the reader of the refusal was sent to install a provider
// rather than to fix the command they had typed.
//
// The subject is an ORDER — the grammar asked before a provider is resolved — so the case has to
// drive something that does both. `bloatjudge.Main` is that whole body, which is why it lives in the
// package and `main()` is three lines around it.

package bloatjudge_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	bloatjudge "kk-flavor/tools/bloat-judge"
)

func TestBloatJudgeNamesItsGrammarWithNoProviderReachable(t *testing.T) {
	// The control, and the whole point of the case: a PATH holding no provider at all. Without it this
	// passes on any machine that happens to carry one, which is exactly how the defect survived.
	bare := t.TempDir()
	for _, provider := range []string{"claude", "codex"} {
		if found, err := exec.LookPath(provider); err == nil {
			t.Logf("%s is on this machine's PATH at %s; the run below hides it", provider, found)
		}
	}
	t.Setenv("PATH", bare)
	t.Setenv("JUDGE_PROVIDER", "claude")
	// A malformed deadline override on the developer's own machine refuses BEFORE the grammar is
	// reached, which would redden this case over something it is not about.
	t.Setenv("XDG_CONFIG_HOME", bare)

	var out, errOut strings.Builder
	code := bloatjudge.Main(filepath.Join(bare, "bloat-judge"), nil, strings.NewReader(""), &out, &errOut)
	said := out.String() + errOut.String()
	if code == 0 {
		t.Fatalf("a bare invocation exited 0, so it judged something it was never given: %s", said)
	}
	if !strings.Contains(said, "usage: bloat-judge.sh ") {
		t.Errorf("with no provider on PATH, a bare invocation never named its grammar, so the refusal "+
			"points at the machine rather than at the invocation:\n%s", said)
	}
}
