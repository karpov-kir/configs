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
	"os"
	"path/filepath"
	"strings"
	"testing"

	aibootstrap "configs/ai/tools/ai-bootstrap"
	"configs/ai/tools/installertest"
	"configs/ai/tools/machine"
	"configs/ai/tools/machine/fake"
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
	*installertest.Writer
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
	machine        *brewMachine
	out            strings.Builder
	err            strings.Builder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	writer := installertest.New(t)
	base := writer.Base()
	f := &fixture{Writer: writer, t: t, base: base, repo: base + "/checkout/ai",
		home: base + "/home", machine: newBrewMachine()}
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
		f.newSkill(root, name, "")
	}
	for _, name := range maintainerSkills {
		f.newSkill(root, name, "audience: maintainer\n")
	}
}

func (f *fixture) newSkill(root, name, audience string) {
	f.t.Helper()
	f.Write(root+"/kk-flavor/skills/"+name+"/SKILL.md",
		"---\nname: "+name+"\ndescription: a skill\n"+audience+"---\n")
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
	f.out.Reset()
	f.err.Reset()
	run, code := aibootstrap.Perform(aibootstrap.Options{
		Self:           scriptName,
		Args:           args,
		Repo:           repo,
		Home:           f.home,
		CodexHome:      f.codexHome,
		ConfigHome:     f.home + "/.config",
		IsInsideVerify: f.isInsideVerify,
		Machine:        f.machine,
		Out:            &f.out,
		Err:            &f.err,
		WriteRoot:      f.base,
	})
	if run != nil {
		if breaches := run.Breaches(); len(breaches) > 0 {
			f.t.Fatalf("the run went for a path outside %s — %s\n"+
				"this is the containment guard, not a failing case", f.base, strings.Join(breaches, "; "))
		}
	}
	return code
}

func (f *fixture) skillsMount(agent string) string {
	if agent == "codex" {
		return f.home + "/.agents/skills"
	}
	return f.home + "/.claude/skills"
}

func (f *fixture) said() string {
	return f.out.String() + f.err.String()
}

func (f *fixture) expectSaid(want string) {
	f.t.Helper()
	if !strings.Contains(f.said(), want) {
		f.t.Errorf("the run never said %q. It said:\n%s", want, f.said())
	}
}

func (f *fixture) expectNotSaid(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.said(), unwanted) {
		f.t.Errorf("the run said %q, which it must not. It said:\n%s", unwanted, f.said())
	}
}

func (f *fixture) expectCode(got, want int) {
	f.t.Helper()
	if got != want {
		f.t.Errorf("the run exited %d, wanted %d. It said:\n%s", got, want, f.said())
	}
}

func (f *fixture) expectLinkTo(target, want string) {
	f.t.Helper()
	value, err := os.Readlink(target)
	if err != nil {
		f.t.Errorf("%s is not a symlink, so it was never mounted: %v", target, err)
		return
	}
	if value != want {
		f.t.Errorf("%s -> %s, wanted %s", target, value, want)
	}
}

func (f *fixture) expectSymlink(target string) {
	f.t.Helper()
	if _, err := os.Readlink(target); err != nil {
		f.t.Errorf("%s is not a symlink any more, so the run took it: %v", target, err)
	}
}

// No entry at the path at all. Lstat is used, because Stat follows the link and answers "not there"
// for one that dangles. That is the shape a half-finished removal leaves behind.
func (f *fixture) expectAbsent(path string) {
	f.t.Helper()
	if info, err := os.Lstat(path); err == nil {
		f.t.Errorf("%s is still there (%s)", path, info.Mode())
	}
}

func (f *fixture) expectFileBody(path, want string) {
	f.t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		f.t.Errorf("%s could not be read, so what was in it did not survive: %v", path, err)
		return
	}
	if string(got) != want {
		f.t.Errorf("%s holds %q, wanted %q", path, got, want)
	}
}

func (f *fixture) expectFileContains(path, want string) {
	f.t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		f.t.Errorf("%s could not be read: %v", path, err)
		return
	}
	if !strings.Contains(string(got), want) {
		f.t.Errorf("%s does not carry %q. It holds:\n%s", path, want, got)
	}
}

func (f *fixture) expectNotSymlink(path string) {
	f.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		f.t.Errorf("%s is a symlink to %s, so it is not a copy of its own", path, value)
	}
}

// Every symlink directly under a directory, by name.
func (f *fixture) mounted(directory string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if _, err := os.Readlink(filepath.Join(directory, entry.Name())); err == nil {
			names = append(names, entry.Name())
		}
	}
	return names
}

// --- the machine, as a working fake -----------------------------------------------------------------

// brew's own behaviour on top of the shared fake: a `list` answers non-zero until the matching
// `install` has run. Canned codes could not express that, and installed-first is the branch the
// packages step spends all its lines on.
type brewMachine struct {
	*fake.Machine
	installed map[string]bool
	installs  []string
}

func newBrewMachine() *brewMachine {
	host := &brewMachine{Machine: fake.New(), installed: map[string]bool{}}
	host.Answering("brew", host.answer)
	// The clients and the release tool, so the mcp and tools steps reach their commands. A case about
	// either refusal takes its command away again.
	host.Add("claude", "codex", "gh", "go", "rtk")
	return host
}

func (b *brewMachine) answer(command machine.Command) int {
	name := command.Args[len(command.Args)-1]
	if command.Args[0] == "list" {
		if b.installed[name] {
			return 0
		}
		return 1
	}
	b.installs = append(b.installs, name)
	b.installed[name] = true
	return 0
}

func (b *brewMachine) without(names ...string) {
	for _, name := range names {
		b.Present[name] = false
	}
}
