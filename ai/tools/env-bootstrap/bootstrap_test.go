package envbootstrap_test

// The cases for the shell and editor install. ai/tools/installer's own suite drives what the mounting
// machinery does with a table: refusing a real file, repointing a stale link, stopping at a second
// checkout. This suite drives the decisions this installer makes: the table it declares, the flags it
// accepts, and the packages step.

import (
	"os"
	"strings"
	"testing"

	envbootstrap "configs/ai/tools/env-bootstrap"
	"configs/ai/tools/installertest"
	"configs/ai/tools/machine"
	"configs/ai/tools/machine/fake"
)

// What the guard looks for under a candidate root, and what a refusal leads with. The stub's own
// basename, which is what `exec -a "$0"` hands the binary.
const scriptName = "bootstrap.sh"

// One case's tree: a checkout in env/'s shape, a home, and everything the run printed.
type fixture struct {
	*installertest.Writer
	t    *testing.T
	base string
	repo string
	home string
	brew *brewMachine
	out  strings.Builder
	err  strings.Builder
}

// Every case gets a home of its own under t.TempDir(). The suite this replaced shared one home, where
// a fixture write followed a live symlink into the checkout and overwrote nvim/init.lua and
// starship/starship.toml in the working tree.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	writer := installertest.New(t)
	base := writer.Base()
	f := &fixture{Writer: writer, t: t, base: base, repo: base + "/checkout/env",
		home: base + "/home", brew: newBrewMachine()}
	f.MkdirAll(f.home)
	f.newCheckout(f.repo)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and the sources beside it. A fixture missing the installer is turned away by the guard's last
// condition, and the case then measures that condition.
func (f *fixture) newCheckout(root string) {
	f.t.Helper()
	f.Write(root+"/"+scriptName, "#!/usr/bin/env bash\n")
	f.Write(root+"/zsh/.zpreztorc", "the prezto config\n")
	f.Write(root+"/zsh/.zshrc", "the shell config\n")
	f.Write(root+"/git/.gitconfig", "the git identity\n")
	f.Write(root+"/ghostty/config", "the terminal\n")
	f.Write(root+"/nvim/init.lua", "the editor config\n")
	f.Write(root+"/starship/starship.toml", "the prompt\n")
}

// The run a case drives, with everything it did not name taken from the fixture. Out and Err are the
// fixture's, so every assertion reads the same output. WriteRoot bounds every write to this case's own
// tree, and Breaches is read back afterwards, so a run that reached outside the tree fails as a guard.
func (f *fixture) run(args ...string) int {
	f.t.Helper()
	f.out.Reset()
	f.err.Reset()
	run, code := envbootstrap.Perform(envbootstrap.Options{
		Self:       scriptName,
		Args:       args,
		Repo:       f.repo,
		Home:       f.home,
		ConfigHome: f.home + "/.config",
		Machine:    f.brew,
		Out:        &f.out,
		Err:        &f.err,
		WriteRoot:  f.base,
	})
	if run != nil {
		if breaches := run.Breaches(); len(breaches) > 0 {
			f.t.Fatalf("the run went for a path outside %s — %s\n"+
				"this is the containment guard, not a failing case", f.base, strings.Join(breaches, "; "))
		}
	}
	return code
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

func (f *fixture) expectAbsent(path string) {
	f.t.Helper()
	if info, err := os.Lstat(path); err == nil {
		f.t.Errorf("%s is still there (%s)", path, info.Mode())
	}
}

// --- brew, as a working machine --------------------------------------------------------------------

// The fake machine with brew's own behaviour on it: a `list` answers non-zero until the matching
// `install` has run. Canned codes could not express that, and installed-first is the branch this
// installer spends most of its lines on.
type brewMachine struct {
	*fake.Machine
	installed map[string]bool
	failing   map[string]bool
	installs  []string
}

func newBrewMachine() *brewMachine {
	brew := &brewMachine{Machine: fake.New(), installed: map[string]bool{}, failing: map[string]bool{}}
	brew.Answering("brew", brew.answer)
	return brew
}

func (b *brewMachine) answer(command machine.Command) int {
	if len(command.Args) < 2 {
		return 1
	}
	name := command.Args[len(command.Args)-1]
	kind := "formula"
	for _, arg := range command.Args {
		if arg == "--cask" {
			kind = "cask"
		}
	}
	key := kind + " " + name
	if command.Args[0] == "list" {
		if b.installed[key] {
			return 0
		}
		return 1
	}
	b.installs = append(b.installs, key)
	if b.failing[key] {
		return 1
	}
	b.installed[key] = true
	return 0
}

// Takes brew off this machine. Every other case runs with brew present and an empty install list.
func (b *brewMachine) without() {
	b.Present["brew"] = false
}
