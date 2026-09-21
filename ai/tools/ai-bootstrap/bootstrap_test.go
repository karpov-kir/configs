// The cases for the machine-wide agent install: the tiers, the instruction file, the Codex migration
// and the four steps reaching outside this process. ai/tools/installer's own suite drives the rest.
//
// Every case gets its own home under t.TempDir(), and WriteRoot bounds the run to that tree. The
// shell suite this replaces shared one home: a fixture write followed a live symlink into the
// checkout and overwrote nvim/init.lua and starship.toml in the working tree. So the bound is read
// back after every run, and a breach fails the case as a guard.
//
// No process is spawned. brew, rtk, gh, the client CLIs, the tools installer and the gate arrive
// through the machine port, which lets a case drive an exit code this machine could not produce.
package aibootstrap_test

import (
	"testing"

	aibootstrap "configs/ai/tools/ai-bootstrap"
	"configs/ai/tools/installertest"
	"configs/ai/tools/runtest"
)

// What the second-checkout guard looks for under a candidate root, and what a refusal leads with. The
// stub's own basename, which is what `exec -a "$0"` hands the binary.
const scriptName = "bootstrap.sh"

// The skills the fixture checkout ships. Two audiences, because the tier cases are only comparing
// something on a tree that holds both.
var (
	publicSkills     = []string{"kk-build", "kk-qualify"}
	maintainerSkills = []string{"kk-ecosystem"}
)

// One case's tree: a checkout in ai/'s shape, a home, a machine, and everything the run printed.
type fixture struct {
	*installertest.Tree
	*runtest.Output
	t    *testing.T
	base string
	repo string
	home string
	// Where this fixture's Codex profile is. Its own field, so a case can point it somewhere else. An
	// alias of the shared discovery directory is a shape the migration has to survive.
	codexHome string
	// Whether this run is one another run's verify step started. A field so the case about the marker
	// can set it without a second run.
	isInsideVerify bool
	machine        *installertest.BrewMachine
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	tree := installertest.New(t)
	base := tree.Base()
	f := &fixture{Tree: tree, Output: runtest.NewOutput(t), t: t, base: base,
		repo: base + "/checkout/ai", home: base + "/home", machine: newMachine()}
	f.codexHome = f.home + "/.codex"
	f.MkdirAll(f.home)
	f.newCheckout(f.repo)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and everything a run reaches for beside it.
func (f *fixture) newCheckout(root string) {
	f.t.Helper()
	f.Write(root+"/"+scriptName, "#!/usr/bin/env bash\n")
	f.Write(root+"/owner-instructions.md", ownerTemplate)
	f.Write(root+"/tools/install.sh", "#!/usr/bin/env bash\n")
	f.Write(root+"/mcp-sync.sh", "#!/usr/bin/env bash\n")
	f.Write(root+"/gate.sh", "#!/usr/bin/env bash\n")
	// The three scripts the steps reach through the machine port, declared here. A run whose
	// installer, sync or gate could not start is a machine fault, and a step's decision is a
	// different thing. The cases about that take one away again.
	f.machine.Add(root+"/tools/install.sh", root+"/mcp-sync.sh", root+"/gate.sh")
	for _, name := range publicSkills {
		f.NewSkill(root, name, "")
	}
	for _, name := range maintainerSkills {
		f.NewSkill(root, name, "audience: maintainer\n")
	}
}

// The owner template a case installs. It carries the region body, which is the only property of the
// real file these cases depend on. ai/owner-instructions.md itself is held against the body by the
// case in owner_test.go.
const ownerTemplate = "# Owner\n\n### KK Flavor\n\n" +
	"Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc.\n"

// The run a case drives, with everything it did not name taken from the fixture. Out, Err and
// WriteRoot stay the fixture's whatever a case says. A case printing elsewhere leaves output no
// reader sees, and one writing elsewhere is the incident in this file's header.
func (f *fixture) run(args ...string) int {
	f.t.Helper()
	return f.runFrom(f.repo, args...)
}

// The flags every case that is not about one of those four steps passes. Named once, so a step that
// grows a flag cannot pick it up at some of the cases here and reach a real command at the rest.
var skipSteps = []string{"--skip-brew", "--skip-tools", "--skip-mcp", "--skip-rtk", "--skip-verify"}

// A run of the ordinary shape: one agent, and every step that reaches outside this process skipped.
func (f *fixture) install(args ...string) int {
	f.t.Helper()
	return f.run(append(append([]string{}, skipSteps...), args...)...)
}

func (f *fixture) runFrom(repo string, args ...string) int {
	f.t.Helper()
	f.Reset()
	run, code := aibootstrap.Perform(aibootstrap.Options{
		Self:           scriptName,
		Args:           args,
		Repo:           repo,
		Home:           f.home,
		CodexHome:      f.codexHome,
		ConfigHome:     f.home + "/.config",
		IsInsideVerify: f.isInsideVerify,
		Machine:        f.machine,
		Out:            &f.Out,
		Err:            &f.Err,
		WriteRoot:      f.base,
	})
	if run != nil {
		f.ExpectNoBreach(run.Breaches())
	}
	return code
}

func (f *fixture) skillsMount(agent string) string {
	if agent == "codex" {
		return f.home + "/.agents/skills"
	}
	return f.home + "/.claude/skills"
}

// --- the machine, as a working fake -----------------------------------------------------------------

// The shared brew machine, plus the commands this installer's steps reach for. The clients and the
// release tool are here so the mcp and tools steps get that far; a case about either refusal takes
// its command away again.
func newMachine() *installertest.BrewMachine {
	host := installertest.NewBrewMachine()
	host.Add("claude", "codex", "gh", "go", "rtk")
	return host
}
