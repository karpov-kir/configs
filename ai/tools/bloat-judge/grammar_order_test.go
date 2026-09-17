// A mistyped invocation has to be answered with the grammar on a machine that can reach no model.
//
// The command resolved its provider before it parsed its arguments, so `bloat-judge.sh` with no
// arguments refused with "no provider" wherever no CLI was installed, and printed the grammar
// wherever one was. That makes the answer a property of the machine rather than of the code:
// TestEveryStubDocumentsTheUsageItsBinaryPrints passed on a developer's laptop carrying `claude` and
// failed in CI, which carries neither — and the reader of the refusal was sent to install a provider
// rather than to fix the command they had typed.

package bloatjudge_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBloatJudgeNamesItsGrammarWithNoProviderReachable(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bloat-judge")
	build := exec.Command("go", "build", "-o", binary, "../cmd/bloat-judge")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building bloat-judge: %v\n%s", err, out)
	}

	// The control, and the whole point of the case: a PATH holding no provider at all. Without it this
	// passes on any machine that happens to carry one, which is exactly how the defect survived.
	bare := t.TempDir()
	for _, provider := range []string{"claude", "codex"} {
		if found, err := exec.LookPath(provider); err == nil {
			t.Logf("%s is on this machine's PATH at %s; the run below hides it", provider, found)
		}
	}

	run := exec.Command(binary)
	run.Dir = t.TempDir()
	run.Env = append(os.Environ(), "PATH="+bare, "JUDGE_PROVIDER=claude")
	said, err := run.CombinedOutput()
	if err == nil {
		t.Fatalf("a bare invocation exited 0, so it judged something it was never given: %s", said)
	}
	if !strings.Contains(string(said), "usage: bloat-judge.sh ") {
		t.Errorf("with no provider on PATH, a bare invocation never named its grammar, so the refusal "+
			"points at the machine rather than at the invocation:\n%s", said)
	}
}
