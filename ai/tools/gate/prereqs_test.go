// A unit whose verdict depends on something the machine provides, over and above its input files and
// the toolchain stamp every unit shares.
//
// `models` is the case that exists. It asks each provider about every name models.json holds, and a
// provider missing from PATH is not asked at all: model-check reports that on stderr and still exits
// 0, because a name it could not ask about is neither good nor bad. So a green there means "no
// provider this machine could reach refuses a name", and a key naming no provider answers for a
// machine where a different set is reachable.
package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// This repository, because the claim is about the policy this tree ships: which clients get probed
// comes out of models.json, and a fixture would assert whatever the case itself wrote there.
func thisRepositorysRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, extModels)); err != nil {
		t.Fatalf("%s is not this repository's root, so nothing below reads its policy: %v", root, err)
	}
	return root
}

// The four transitions node_test.go drives over the toolchain stamp, plus the half that stamp cannot
// have: the movement must reach the models unit ALONE. Availability in the shared stamp would retire
// every verdict in the table the day a provider is installed, units that ask no model included, so
// `gofmt` is read on every transition as the control.
func TestTheReachableProvidersChangeTheModelsUnitsKeyAndNoOthers(t *testing.T) {
	root := thisRepositorysRoot(t)
	bin := t.TempDir()
	t.Setenv("PATH", bin)

	install := func(client string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bin, client), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("installing a stub %s: %v", client, err)
		}
	}
	uninstall := func(client string) {
		t.Helper()
		if err := os.Remove(filepath.Join(bin, client)); err != nil {
			t.Fatalf("removing the stub %s: %v", client, err)
		}
	}

	// Both keys off one registration, over a manifest written here: the case is about what the machine
	// contributes to a key, so every file contributing to one is held still.
	read := func() (models, control string) {
		t.Helper()
		said := &strings.Builder{}
		g := &gate{root: root, env: Env{Root: root}, errOut: said, stamp: "toolchain", manifest: []manifestLine{
			{hash: "policy", path: extModels},
			{hash: "source", path: "ai/tools/gate/gate.go"},
		}}
		g.addModelCheck()
		g.addGoChecks()
		for _, u := range g.units {
			key, lines := g.keyMaterial(u)
			// The control on the instrument: a unit resolving to no manifest line at all would agree
			// with every other such unit, and each comparison below would pass over nothing.
			if len(lines) == 0 {
				t.Fatalf("%s resolved to no input line, so its key is built over nothing: %s", u.id, said.String())
			}
			switch u.id {
			case "models":
				models = key
			case "gofmt":
				control = key
			}
		}
		if models == "" || control == "" {
			t.Fatalf("registration produced no models unit or no gofmt unit: %s", said.String())
		}
		return models, control
	}

	neither, control := read()
	if again, _ := read(); again != neither {
		t.Fatal("an unchanged provider set moved the models unit's key, so no verdict would ever be reused")
	}

	install("claude")
	withClaude, controlWithClaude := read()
	if withClaude == neither {
		t.Error("installing claude left the models unit's key unchanged, so a verdict earned where no " +
			"provider could be asked answers for a machine that can ask one")
	}

	install("codex")
	withBoth, controlWithBoth := read()
	if withBoth == withClaude {
		t.Error("installing codex left the models unit's key unchanged, so the verdict earned while its " +
			"names went unasked still answers now that they can be asked")
	}

	uninstall("codex")
	restored, controlRestored := read()
	if restored != withClaude {
		t.Error("removing codex again did not restore the earlier key, so the key carries something " +
			"other than which providers are reachable")
	}

	for _, seen := range []string{controlWithClaude, controlWithBoth, controlRestored} {
		if seen != control {
			t.Fatal("a provider appearing moved the gofmt unit's key. Provider availability belongs to " +
				"the one unit that asks a provider, never to the stamp every unit shares — there it " +
				"retires every verdict in the table the day someone installs a CLI")
		}
	}
}

// A policy the gate cannot read leaves it unable to say which providers are reachable. Registering
// the unit anyway keys it on an empty string, and that is a key component which never changes — the
// shape resolveMachine refuses when it cannot hash the gate's own binary, and the one this whole
// change exists to remove.
func TestAModelsUnitWhosePolicyCannotBeReadRefuses(t *testing.T) {
	said := &strings.Builder{}
	g := &gate{root: t.TempDir(), errOut: said}
	if code := g.addModelCheck(); code != 2 {
		t.Fatalf("addModelCheck exited %d over a root holding no policy, want 2", code)
	}
	if len(g.units) != 0 {
		t.Errorf("it registered %d unit(s) anyway, keyed on a provider set nothing could read", len(g.units))
	}
}

// Keying the unit stops a stale green; it does not tell anyone the green is narrow. Both halves of the
// run have to say so: the run that earns the verdict, where model-check's own "not resolved" lines are
// held back because the unit passed, and the run that answers from the record, where the command never
// executes and there is no output to tail at all. The second is sayable only because the first half
// landed — a cache hit now implies the provider set the verdict was earned under.
func TestTheGateNamesTheProviderItCouldNotAskWhetherItRunsOrAnswersFromCache(t *testing.T) {
	root := thisRepositorysRoot(t)
	bin := t.TempDir()
	// `sh` among them: the gate hands every unit's command to a shell, and a PATH without one turns
	// the stood-down command below into a FAILED line about nothing.
	for _, name := range []string{"go", "git", "sh"} {
		binary, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(binary, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	// One provider reachable and one not, which is the partial skip itself. Left to the real machine
	// this case asserts nothing wherever both CLIs happen to be installed.
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cache := t.TempDir()

	run := func() string {
		t.Helper()
		said := &strings.Builder{}
		g := &gate{env: Env{Root: root, Cache: cache, SelfDigest: "test-digest"}, out: said, errOut: said}
		if code := g.resolveMachine(); code != 0 {
			t.Fatalf("machine resolution exited %d: %s", code, said.String())
		}
		if code := g.addModelCheck(); code != 0 {
			t.Fatalf("registering the models unit exited %d: %s", code, said.String())
		}
		// The real registration with its command stood down. What is under test is what the gate says
		// about a provider nothing could ask, and the real command spends a model call per name.
		g.units[0].cmd = "true"
		if code := g.assignStems(); code != 0 {
			t.Fatalf("naming the record exited %d: %s", code, said.String())
		}
		if code := g.buildManifest(); code != 0 {
			t.Fatalf("building the manifest exited %d: %s", code, said.String())
		}
		said.Reset()
		if code := g.runUnits(modeFast, time.Now()); code != 0 {
			t.Fatalf("the run exited %d: %s", code, said.String())
		}
		return said.String()
	}

	earned := run()
	if !strings.Contains(earned, "ran ok") {
		t.Fatalf("the unit did not run, so nothing below is about a verdict being earned:\n%s", earned)
	}
	for _, want := range []string{"codex", "unasked"} {
		if !strings.Contains(earned, want) {
			t.Errorf("the run that earned the verdict never says %q. model-check skipped every codex name "+
				"and said so on stderr; the gate held that back because the unit passed:\n%s", want, earned)
		}
	}

	cached := run()
	if !strings.Contains(cached, "fresh") {
		t.Fatalf("the second run did not answer from cache, so nothing below is about a cache hit:\n%s", cached)
	}
	for _, want := range []string{"codex", "unasked"} {
		if !strings.Contains(cached, want) {
			t.Errorf("the cache hit never says %q, so a reader cannot tell a verdict earned over every "+
				"provider from one earned over half of them:\n%s", want, cached)
		}
	}
}

// A unit that could not measure has said something about the machine, not about one tree — so every
// verdict it holds goes, whatever key it was taken under. `prerequisite` is PATH presence, so a client
// that is installed and has stopped answering leaves the key exactly where it was: without this, the
// record a sibling worktree wrote while it still answered stays fresh, and the next warm run in any
// tree serves a check that cannot currently run at all as a pass.
func TestAnUnmeasuredMachineDependentUnitKeepsNoVerdict(t *testing.T) {
	cache := t.TempDir()
	seed := func(names ...string) {
		t.Helper()
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(cache, name), nil, 0o644); err != nil {
				t.Fatalf("seeding %s: %v", name, err)
			}
		}
	}
	held := func() []string {
		t.Helper()
		entries, err := os.ReadDir(cache)
		if err != nil {
			t.Fatalf("reading the store: %v", err)
		}
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		return names
	}

	seed("models.aaa", "models.bbb", "models"+sidecarSuffix, "gofmt.aaa")
	g := &gate{cache: cache}

	// A unit with no prerequisite keeps its siblings: its key is its whole question.
	g.forgetVerdictsIfMachineDependent(unit{id: "gofmt", stem: "gofmt"})
	if got := held(); len(got) != 4 {
		t.Errorf("a unit with no prerequisite dropped a verdict: %v", got)
	}

	g.forgetVerdictsIfMachineDependent(unit{id: "models", stem: "models", prerequisite: "claude present"})
	want := []string{"gofmt.aaa", "models" + sidecarSuffix}
	if got := held(); !slices.Equal(got, want) {
		t.Errorf("after an unmeasured models run the store holds %v, wanted %v — a verdict under any key "+
			"is one a warm run can still serve, and the sidecar is not a verdict", got, want)
	}
}
