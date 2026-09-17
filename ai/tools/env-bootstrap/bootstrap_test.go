package envbootstrap_test

// The cases for the shell and editor install. What the mounting machinery does with a table —
// refusing a real file, repointing a stale link, stopping at a second checkout — is driven in
// ai/tools/installer's own suite against the same code; what is driven here is this installer's own
// decisions: the table it declares, the flags it accepts, and the packages step.
//
// Every case gets a home of its own under t.TempDir(), and the run it drives is bounded to that same
// tree with WriteRoot. The suite this replaces once handed every case the same home: a fixture write
// followed a live symlink into the checkout and overwrote nvim/init.lua and starship/starship.toml in
// the working tree. So the bound is read back after every run, and a breach fails the case as a guard
// rather than as a result.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	envbootstrap "kk-flavor/tools/env-bootstrap"
	"kk-flavor/tools/machine"
	"kk-flavor/tools/machine/fake"
)

// What the guard looks for under a candidate root, and what a refusal leads with. The stub's own
// basename, which is what `exec -a "$0"` hands the binary.
const scriptName = "bootstrap.sh"

// One case's tree: a checkout in env/'s shape, a home, and everything the run printed.
type fixture struct {
	t    *testing.T
	base string
	repo string
	home string
	brew *brewMachine
	out  strings.Builder
	err  strings.Builder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := physical(t, t.TempDir())
	f := &fixture{t: t, base: base, repo: base + "/checkout/env", home: base + "/home", brew: newBrewMachine()}
	f.mkdirAll(f.home)
	f.newCheckout(f.repo)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and the sources beside it. A fixture missing the installer would be turned away by the guard's last
// condition and the case would be measuring that rather than what it named.
func (f *fixture) newCheckout(root string) {
	f.t.Helper()
	f.write(root+"/"+scriptName, "#!/usr/bin/env bash\n")
	f.write(root+"/zsh/.zpreztorc", "the prezto config\n")
	f.write(root+"/zsh/.zshrc", "the shell config\n")
	f.write(root+"/git/.gitconfig", "the git identity\n")
	f.write(root+"/ghostty/config", "the terminal\n")
	f.write(root+"/nvim/init.lua", "the editor config\n")
	f.write(root+"/starship/starship.toml", "the prompt\n")
}

// The run a case drives, with everything it did not name taken from the fixture. Out, Err and
// WriteRoot are the fixture's whatever a case says: a case that could print elsewhere is a case whose
// output nothing reads, and one that could write elsewhere is the incident in this file's header.
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

// --- the fixture writers ------------------------------------------------------------------------

// The nearest existing directory above what a fixture is about to write, resolved physically.
// Anything landing outside the case's own tree fails the case as a guard rather than as a result.
//
// Climbed rather than asked of the immediate parent, because a fixture creates the missing
// directories under it — so that ancestor is the deepest thing a symlink could still redirect.
func (f *fixture) containedParent(path string) {
	f.t.Helper()
	dir := filepath.Dir(path)
	for {
		parent, err := filepath.EvalSymlinks(dir)
		if err == nil {
			if parent != f.base && !strings.HasPrefix(parent, f.base+"/") {
				f.t.Fatalf("refusing to write %s — its nearest existing parent resolves to %s, outside %s\n"+
					"this is the containment guard, not a failing case", path, parent, f.base)
			}
			return
		}
		next := filepath.Dir(dir)
		if next == dir {
			f.t.Fatalf("refusing to write %s — no directory above it resolves\n"+
				"this is the containment guard, not a failing case", path)
		}
		dir = next
	}
}

func (f *fixture) mkdirAll(dir string) {
	f.t.Helper()
	f.containedParent(dir)
	f.refuseExistingSymlink(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("the fixture could not create %s: %v", dir, err)
	}
}

func (f *fixture) write(path, body string) {
	f.t.Helper()
	f.containedParent(path)
	// The parent being contained says nothing about the last component. A write follows a symlink, and
	// the links these cases produce point into a checkout — a run leaves $home/.zshrc pointing at
	// $repo/zsh/.zshrc, and a fixture write at that path afterwards lands in the real file. That is the
	// incident in this file's header, reached by the one door the parent check does not cover.
	f.refuseExistingSymlink(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatalf("the fixture could not create the parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatalf("the fixture could not write %s: %v", path, err)
	}
}

func (f *fixture) symlink(source, target string) {
	f.t.Helper()
	f.containedParent(target)
	// `ln -s X Y` where Y already exists as a symlink to a directory creates the link INSIDE Y rather
	// than replacing it, which is how a stray link ends up in a checkout. Refused rather than forced:
	// every fixture link here is meant to be the first thing at its path.
	f.refuseExistingSymlink(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		f.t.Fatalf("the fixture could not create the parent of %s: %v", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		f.t.Fatalf("the fixture could not link %s at %s: %v", target, source, err)
	}
}

func (f *fixture) refuseExistingSymlink(path string) {
	f.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		f.t.Fatalf("refusing to write %s — it already exists as a symlink to %s\n"+
			"this is the containment guard, not a failing case", path, value)
	}
}

// t.TempDir hands back /var/folders/… on macOS while /var is itself a symlink to /private/var.
// Comparing an unresolved root against resolved paths would make every containment check refuse
// everything, and a guard that always fires gets deleted.
func physical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("the case's own temp directory does not resolve, so nothing could be contained: %v", err)
	}
	return resolved
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

// The machine that has brew but nothing installed through it, which is every case that does not say
// otherwise.
func (b *brewMachine) without() {
	b.Present["brew"] = false
}
