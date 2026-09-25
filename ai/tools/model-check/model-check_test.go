package modelcheck

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modelpolicy "configs/ai/tools/model-policy"
	modelserved "configs/ai/tools/model-served"
	readerjudge "configs/ai/tools/reader-judge"
)

// Two rows and one model per client that the order ranks and no row names, so a case can refuse
// exactly one of six selections and still read which part of the file held it. Deduplication and ordering are
// model-policy's, and its suite covers them — Selections is what these cases are handed.
const fixturePolicy = `{
  "version": 4,
  "limits": { "intents-in-flight": 10 },
  "tiers": {
    "codex":  ["cheap-codex", "dear-codex", "unrun-codex"],
    "claude": ["cheap-claude", "dear-claude", "unrun-claude"]
  },
  "sessions": {
    "review": { "codex": { "model": "dear-codex", "effort": "high" }, "claude": { "model": "dear-claude" } }
  },
  "workers": {
    "judge": { "codex": { "model": "cheap-codex", "effort": "low" }, "claude": { "model": "cheap-claude" }, "rolls": 3 }
  }
}`

func policyFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(path, []byte(fixturePolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// oneAccount is a machine where the app and the CLI run on the same login.
func oneAccount() (string, string) {
	return "a@example.invalid (Org, team)", "a@example.invalid (Org, team)"
}

func run(t *testing.T, agent string, probe Probe, path string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status := Run(Command{Args: []string{"--agent=" + agent, "--config", path}, Stdout: &out, Stderr: &errOut,
		Probe: probe, Accounts: oneAccount, ServedCache: filepath.Join(t.TempDir(), "served.json")})
	return status, out.String(), errOut.String()
}

// served answers every selection as requested.
func served(selection modelpolicy.Selection) (readerjudge.Served, error) {
	return readerjudge.Served{Requested: selection.Model, Answered: "claude-" + selection.Model}, nil
}

// Each selection the file holds for the named client is asked about once, rows and tier order alike.
// A name invented here would send the check asking a provider about a string this file never chose.
//
// It is also the clean run every refusal case below is read against. A clean file exits 0 and prints
// no REFUSED line, and a report that fires on every input passes those refusal cases too.
func TestEverySelectionTheFileHoldsIsAskedAboutOnce(t *testing.T) {
	var asked []string
	status, out, errOut := run(t, "codex", func(selection modelpolicy.Selection) (readerjudge.Served, error) {
		asked = append(asked, selection.Client+"/"+selection.Model+"/"+selection.Effort)
		return served(selection)
	}, policyFile(t))
	if status != 0 || strings.Contains(out, "REFUSED") {
		t.Fatalf("status = %d, want 0\n%s%s", status, out, errOut)
	}
	want := "codex/cheap-codex/low codex/dear-codex/high codex/unrun-codex/"
	if got := strings.Join(asked, " "); got != want {
		t.Errorf("asked about %q; want %q", got, want)
	}
}

// The case the whole unit exists for: a name the provider will not run fails at the file, and the
// message names the file rather than leaving a human to read the next judge failure.
func TestARefusedNameFailsAndNamesTheConfig(t *testing.T) {
	path := policyFile(t)
	status, out, errOut := run(t, "codex", func(selection modelpolicy.Selection) (readerjudge.Served, error) {
		if selection.Model == "cheap-codex" {
			return readerjudge.Served{}, &readerjudge.ModelRefused{Client: selection.Client, Model: selection.Model}
		}
		return served(selection)
	}, path)
	if status != 1 {
		t.Fatalf("status = %d, want 1\n%s%s", status, out, errOut)
	}
	if !strings.Contains(out, "judge codex cheap-codex at low — REFUSED") {
		t.Errorf("the refused name is not reported with the row that holds it:\n%s", out)
	}
	if !strings.Contains(errOut, path) {
		t.Errorf("the summary does not name %s:\n%s", path, errOut)
	}
}

// A CLI that is not installed, a network that is down and a deadline are all the same answer here:
// nobody asked. That is neither a pass for the name nor a failure of the file, so it goes to stderr
// and leaves the status to the names that were asked about.
func TestANameNothingCouldAskAboutIsNeitherPassedNorFailed(t *testing.T) {
	status, out, errOut := run(t, "claude", func(selection modelpolicy.Selection) (readerjudge.Served, error) {
		if selection.Model == "unrun-claude" {
			return readerjudge.Served{}, errors.New("unrun-claude is not served here")
		}
		return served(selection)
	}, policyFile(t))
	if status != 0 {
		t.Fatalf("status = %d, want 0 — the other claude names resolved\n%s%s", status, out, errOut)
	}
	if strings.Contains(out, "claude unrun-claude — served") {
		t.Errorf("a name nothing asked about was reported as good:\n%s", out)
	}
	if !strings.Contains(errOut, "claude unrun-claude — not resolved") {
		t.Errorf("the unresolved names are not reported:\n%s", errOut)
	}
}

// Nothing reachable at all is exit 2, not exit 0. A pass here would be a claim about every name in
// the file made by a run that measured none of them.
func TestNoProviderReachableIsNotACleanRun(t *testing.T) {
	path := policyFile(t)
	status, out, errOut := run(t, "codex", func(modelpolicy.Selection) (readerjudge.Served, error) {
		return readerjudge.Served{}, errors.New("not on PATH here")
	}, path)
	if status != 2 {
		t.Fatalf("status = %d, want 2\n%s%s", status, out, errOut)
	}
	if !strings.Contains(errOut, "unchecked, not clean") || !strings.Contains(errOut, path) {
		t.Errorf("the refusal does not say what was not measured:\n%s", errOut)
	}
}

func TestAPolicyItCannotReadIsExitTwo(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Run(Command{Args: []string{"--agent=claude", "--config", filepath.Join(t.TempDir(), "absent.json")}, ServedCache: filepath.Join(t.TempDir(), "served.json"),
		Stdout: &out, Stderr: &errOut, Probe: func(modelpolicy.Selection) (readerjudge.Served, error) { return readerjudge.Served{}, nil }})
	if status != 2 {
		t.Fatalf("status = %d, want 2\n%s%s", status, out.String(), errOut.String())
	}
}

// The file is whatever the gated branch put there, so one far past any real size is refused before a
// call is spent rather than paid for one selection at a time.
func TestAFilePastTheProbeCeilingIsRefusedRatherThanPaidFor(t *testing.T) {
	var workers, codexTiers, claudeTiers []string
	for i := 0; i <= maxSelections; i++ {
		workers = append(workers, fmt.Sprintf(
			`"w%d": { "codex": { "model": "m%d", "effort": "low" }, "claude": { "model": "c%d" } }`, i, i, i))
		codexTiers = append(codexTiers, fmt.Sprintf(`"m%d"`, i))
		claudeTiers = append(claudeTiers, fmt.Sprintf(`"c%d"`, i))
	}
	path := filepath.Join(t.TempDir(), "models.json")
	body := fmt.Sprintf(`{"version":4,"limits":{"intents-in-flight":1},`+
		`"tiers":{"codex":[%s],"claude":[%s]},`+
		`"sessions":{"s":{"codex":{"model":"m0","effort":"low"},"claude":{"model":"c0"}}},`+
		`"workers":{%s}}`,
		strings.Join(codexTiers, ","), strings.Join(claudeTiers, ","), strings.Join(workers, ","))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	asked := 0
	status, out, errOut := run(t, "codex", func(modelpolicy.Selection) (readerjudge.Served, error) { asked++; return readerjudge.Served{}, nil }, path)
	if status != 2 {
		t.Fatalf("status = %d, want 2\n%s%s", status, out, errOut)
	}
	if asked != 0 {
		t.Errorf("%d provider call(s) were spent before the ceiling refused the file", asked)
	}
	if !strings.Contains(errOut, "past the") {
		t.Errorf("the refusal does not say what it refused:\n%s", errOut)
	}
}

// liveProbe is the one path the Probe seam hides, and it picks which CLI to run. A codex selection
// sent through the claude caller would give a wrong verdict for the whole file with the suite green.
func TestLiveProbeRunsTheClientsOwnBinary(t *testing.T) {
	dir := t.TempDir()
	ran := filepath.Join(dir, "ran")
	for _, name := range []string{"claude", "codex"} {
		script := "#!/bin/sh\nprintf '%s\\n' " + name + " >> " + ran + "\n"
		if name == "claude" {
			// ClaudeCaller asks for JSON, and anything else is a fault in the CLI.
			script += `echo '{"result":".","modelUsage":{"claude-fixture-model":{"outputTokens":1}}}'` + "\n"
		}
		if name == "codex" {
			// CodexCaller reads the file named by --output-last-message, so the stub has to write one.
			script += `while [ "$#" -gt 0 ]; do
 if [ "$1" = "--output-last-message" ]; then shift; printf '.\n' > "$1"; fi
 shift
done
`
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, client := range []string{"claude", "codex"} {
		if _, err := liveProbe(modelpolicy.Selection{Client: client, Model: "fixture-model", Effort: "low"}); err != nil {
			t.Fatalf("%s probe: %v", client, err)
		}
	}
	got, err := os.ReadFile(ran)
	if err != nil {
		t.Fatal(err)
	}
	if order := strings.Fields(string(got)); strings.Join(order, " ") != "claude codex" {
		t.Errorf("the binaries run were %q; want claude then codex", order)
	}
}

// isRefusal is what separates the two, so the mapping a missing CLI lands on is asserted here.
func TestLiveProbeCallsAMissingCliUnresolvedRatherThanARefusal(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := liveProbe(modelpolicy.Selection{Client: "codex", Model: "fixture-model", Effort: "low"})
	if err == nil {
		t.Fatal("a missing CLI answered as though it had run the model")
	}
	if isRefusal(err) {
		t.Errorf("a missing CLI was read as the provider refusing the name: %v", err)
	}
	if !strings.Contains(err.Error(), "not on PATH") {
		t.Errorf("error = %v; want it to say the CLI is absent", err)
	}
}

// The bound is reachable by a legitimate file, not only a hostile one: validName accepts 200 bytes of
// task name and 200 more of model name. Without a case, deleting the shaping leaves the suite green.
func TestALongSelectionIsCutAndMarkedRatherThanPrintedWhole(t *testing.T) {
	task := strings.Repeat("p", 150)
	model := strings.Repeat("m", 150)
	body := fmt.Sprintf(`{
  "version": 4,
  "limits": { "intents-in-flight": 1 },
  "tiers": { "codex": ["%s"], "claude": ["c"] },
  "sessions": { "s": { "codex": { "model": "%s", "effort": "low" }, "claude": { "model": "c" } } },
  "workers": { "%s": { "codex": { "model": "%s", "effort": "low" }, "claude": { "model": "c" } } }
}`, model, model, task, model)
	path := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, out, errOut := run(t, "codex", func(modelpolicy.Selection) (readerjudge.Served, error) { return readerjudge.Served{}, nil }, path)
	for _, line := range strings.Split(out, "\n") {
		if len(line) > len("model-check: ")+maxReportedSelectionBytes+len(" — the provider will run it; it reports no served model") {
			t.Fatalf("a reported line ran past the bound (%d bytes):\n%s%s", len(line), out, errOut)
		}
	}
	if strings.Contains(out, task+" codex "+model) {
		t.Errorf("the whole untrimmed selection reached the report:\n%s", out)
	}
}

// A substituted model is reported on its line and in one summary. The run still passes, and the row
// keeps the model it intends.
func TestASubstitutedModelIsReportedAndPasses(t *testing.T) {
	status, out, errOut := run(t, "claude", func(selection modelpolicy.Selection) (readerjudge.Served, error) {
		return readerjudge.Served{Requested: selection.Model, Answered: "claude-opus-5-5[1m]"}, nil
	}, policyFile(t))
	if status != 0 {
		t.Fatalf("status = %d, want 0\n%s%s", status, out, errOut)
	}
	if !strings.Contains(out, "judge claude cheap-claude — SUBSTITUTED: claude-opus-5-5[1m] answered") ||
		!strings.Contains(out, "WARNING: optimal model use is not possible on this account: 3 selection(s)") {
		t.Fatalf("the substitution is not reported:\n%s", out)
	}
}

// The app and the CLI can be signed into different accounts, and the report says so.
func TestTwoAccountsAreNamedWhenTheyDiffer(t *testing.T) {
	var out, errOut bytes.Buffer
	Run(Command{Args: []string{"--agent=claude", "--config", policyFile(t)}, Stdout: &out, Stderr: &errOut,
		Probe: served, Accounts: func() (string, string) { return "app@example.invalid (A, team)", "cli@example.invalid (B, max)" },
		ServedCache: filepath.Join(t.TempDir(), "served.json")})
	if !strings.Contains(out.String(), "the app runs on app@example.invalid (A, team) and the CLI on cli@example.invalid (B, max)") {
		t.Fatalf("the differing accounts are not named:\n%s", out.String())
	}
}

// The client is named, since each probe spends a call on that client's account.
func TestTheClientIsNamed(t *testing.T) {
	var out, errOut bytes.Buffer
	if status := Run(Command{Args: []string{"--config", policyFile(t)}, Stdout: &out, Stderr: &errOut, Probe: served,
		ServedCache: filepath.Join(t.TempDir(), "served.json")}); status != 2 {
		t.Fatalf("status = %d, want 2 with no --agent\n%s", status, errOut.String())
	}
}

// The probed set is kept for the resolver, under the account a dispatch runs on.
func TestTheProbedSetIsKeptForTheResolver(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "served.json")
	var out, errOut bytes.Buffer
	Run(Command{Args: []string{"--agent=claude", "--config", policyFile(t)}, Stdout: &out, Stderr: &errOut,
		Probe: func(selection modelpolicy.Selection) (readerjudge.Served, error) {
			return readerjudge.Served{Requested: selection.Model, Answered: "claude-opus-5-5[1m]"}, nil
		}, Accounts: oneAccount, ServedCache: cache})
	entry, why := modelserved.Lookup(cache, "claude", "a@example.invalid (Org, team)", time.Now())
	if why != "" || entry.Served["cheap-claude"] != "claude-opus-5-5[1m]" {
		t.Fatalf("kept %+v (%s)\n%s", entry, why, out.String())
	}
}
