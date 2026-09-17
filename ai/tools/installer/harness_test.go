package installer_test

// The fixtures and assertions the case files beside it share.
//
// Every case gets a home of its own under t.TempDir(), and the run it drives is bounded to that same
// tree. The cases here exercise the real linking logic, so a run leaves $home/.config/nvim pointing at
// the fixture checkout — correct behaviour, and harmless while each case gets its own home. It stops
// being harmless the moment two cases share one: the second case's write at $home/.config/nvim then
// finds a live symlink into a checkout and goes straight through it into a real config file.
//
// That is not hypothetical. The shell harness this replaces once handed every case the same home. It
// overwrote nvim/init.lua and starship/starship.toml in the working tree and left a stray symlink in
// nvim/. The suite reported it, too — a case failed saying something had been written where nothing
// should be — and the report was read as a harness bug without asking what the broken run had already
// written to disk.
//
// So containment is asserted before each write rather than noticed after, on both sides: the fixture
// writers below resolve a parent physically before touching it, and the run itself is built with
// WriteRoot, which makes the package refuse the same way. expectContained is what reads the second
// half back, and it fails the case as a guard rather than as a result.
//
// Fixtures are built with os.MkdirAll, os.WriteFile and os.Symlink, never by shelling out: a process
// costs about 100ms on the machine these are written on, and file I/O costs nothing. Symlinks are the
// substance of this package, so they are real ones on a real filesystem — an in-memory tree does not
// model their semantics.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/installer"
)

// What the calling installer is called, and what the guard therefore looks for under a candidate root.
const scriptName = "bootstrap.sh"

const label = "env bootstrap"

// One case's tree: a checkout, a home, and the account the run printed.
type fixture struct {
	t    *testing.T
	base string
	repo string
	home string
	out  strings.Builder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := newBareFixture(t)
	f.newCheckout(f.repo)
	return f
}

// The same tree with no checkout in it, for a case that never links anything — the region and registry
// cases, which write into files a caller names. Building the sources anyway would cost every one of
// them ten files to create and ten to tear down, and file I/O is what this suite's wall time is made
// of now that nothing spawns.
func newBareFixture(t *testing.T) *fixture {
	t.Helper()
	base := physical(t, t.TempDir())
	f := &fixture{t: t, base: base, repo: base + "/checkout"}
	f.home = base + "/home"
	f.mkdirAll(f.home)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and the sources beside it. A fixture missing the installer would be turned away by the guard's last
// condition and the case would be measuring that rather than what it named.
//
// The sources are the four shapes env/bootstrap.sh mounts: two files at the top of the home, a whole
// directory, and a file whose parent directory does not exist yet.
func (f *fixture) newCheckout(root string) string {
	f.t.Helper()
	f.mkdirAll(root)
	f.mkdirAll(root + "/zsh")
	f.mkdirAll(root + "/git")
	f.mkdirAll(root + "/nvim")
	f.mkdirAll(root + "/starship")
	f.write(root+"/"+scriptName, "#!/usr/bin/env bash\n")
	f.write(root+"/zsh/.zshrc", "the shell config\n")
	f.write(root+"/git/.gitconfig", "the git identity\n")
	f.write(root+"/nvim/init.lua", "the editor config\n")
	f.write(root+"/starship/starship.toml", "the prompt\n")
	return root
}

// The mount table a case drives, declared against whichever checkout it is asked about.
func (f *fixture) declareMounts(run *installer.Run, repo string) {
	run.AddConfig(repo+"/zsh/.zshrc", f.home+"/.zshrc")
	run.AddConfig(repo+"/git/.gitconfig", f.home+"/.gitconfig")
	run.AddConfig(repo+"/nvim", f.home+"/.config/nvim")
	run.AddConfig(repo+"/starship/starship.toml", f.home+"/.config/starship.toml")
}

// The run a case drives, with everything it did not name taken from the fixture. Out and WriteRoot are
// the fixture's whatever a case says: a case that could print elsewhere is a case whose output nothing
// reads, and one that could write elsewhere is the incident in this file's header.
func (f *fixture) newRun(options installer.RunOptions) *installer.Run {
	f.t.Helper()
	if options.Repo == "" {
		options.Repo = f.repo
	}
	if options.ScriptName == "" {
		options.ScriptName = scriptName
	}
	if options.Label == "" {
		options.Label = label
	}
	if options.ConfigHome == "" {
		options.ConfigHome = f.home + "/.config"
	}
	options.Out = &f.out
	options.WriteRoot = f.base
	f.out.Reset()
	return installer.NewRun(options)
}

// Mount the declared table from one checkout, and hand back the run so a case can read its refusals.
func (f *fixture) mountFrom(repo string, options installer.RunOptions) *installer.Run {
	f.t.Helper()
	options.Repo = repo
	run := f.newRun(options)
	f.declareMounts(run, repo)
	run.Mount()
	f.expectContained(run)
	return run
}

// The default drive: this checkout's own table, mounted.
func (f *fixture) mount(options installer.RunOptions) *installer.Run {
	f.t.Helper()
	return f.mountFrom(f.repo, options)
}

// A run over a discovered set: one bulk mount per named skill, and a sweep of the directory they are
// mounted into. The sweep is what drops a mount this checkout no longer has a source for, so a case
// about it needs both halves declared.
func (f *fixture) mountSkills(names []string, options installer.RunOptions) *installer.Run {
	f.t.Helper()
	options.BulkLabel = "skills"
	run := f.newRun(options)
	for _, name := range names {
		run.AddBulk(f.repo+"/skills/"+name, f.skillsMount()+"/"+name)
	}
	run.AddUnmountScan(f.skillsMount(), f.repo+"/skills")
	run.Mount()
	f.expectContained(run)
	return run
}

func (f *fixture) skillsMount() string {
	return f.home + "/.claude/skills"
}

func (f *fixture) newSkill(name string) {
	f.t.Helper()
	f.mkdirAll(f.repo + "/skills/" + name)
	f.write(f.repo+"/skills/"+name+"/SKILL.md", "---\nname: "+name+"\ndescription: a skill\n---\n")
}

func (f *fixture) expectContained(run *installer.Run) {
	f.t.Helper()
	if breaches := run.Breaches(); len(breaches) > 0 {
		f.t.Fatalf("the run went for a path outside %s — %s\n"+
			"this is the containment guard, not a failing case", f.base, strings.Join(breaches, "; "))
	}
}

// --- the fixture writers ----------------------------------------------------------------------

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
	// The parent being contained says nothing about dir itself, and MkdirAll follows a symlink there
	// the same way a file write does — the door the two writers below each shut on their own last
	// component.
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
	if err := os.Symlink(source, target); err != nil {
		f.t.Fatalf("the fixture could not link %s at %s: %v", target, source, err)
	}
}

// A fixture link dropped so another can take its place. Through the containment guard, because
// refuseExistingSymlink deliberately will not let one be overwritten and that refusal is not a rule to
// be worked around by reaching for os directly.
func (f *fixture) removeLink(path string) {
	f.t.Helper()
	f.containedParent(path)
	if err := os.Remove(path); err != nil {
		f.t.Fatalf("the fixture could not drop %s: %v", path, err)
	}
}

// A whole fixture checkout deleted. Through the containment guard too: this is a recursive delete
// built from a variable, and the suite this replaces destroyed files in the working tree once already.
func (f *fixture) removeAll(path string) {
	f.t.Helper()
	f.containedParent(path)
	if err := os.RemoveAll(path); err != nil {
		f.t.Fatalf("the fixture could not remove %s: %v", path, err)
	}
}

func (f *fixture) refuseExistingSymlink(path string) {
	f.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		f.t.Fatalf("refusing to write %s — it already exists as a symlink to %s\n"+
			"this is the containment guard, not a failing case", path, value)
	}
}

// --- what a case asks afterwards ----------------------------------------------------------------

func (f *fixture) said() string {
	return f.out.String()
}

func (f *fixture) expectSaid(want string) {
	f.t.Helper()
	if !strings.Contains(f.out.String(), want) {
		f.t.Errorf("the run never said %q. It said:\n%s", want, f.out.String())
	}
}

func (f *fixture) expectNotSaid(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.out.String(), unwanted) {
		f.t.Errorf("the run said %q, which it must not. It said:\n%s", unwanted, f.out.String())
	}
}

func (f *fixture) expectRefusals(run *installer.Run, want int) {
	f.t.Helper()
	if got := len(run.Refusals()); got != want {
		f.t.Errorf("the run collected %d refusal(s), wanted %d: %v", got, want, run.Refusals())
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

// A symlink at the target, wherever it points — for a case whose whole subject is that a link survived
// a run that removes some of them.
func (f *fixture) expectSymlink(target string) {
	f.t.Helper()
	if _, err := os.Readlink(target); err != nil {
		f.t.Errorf("%s is not a symlink any more, so the run took it: %v", target, err)
	}
}

// Nothing at the path at all. Lstat rather than Stat, because Stat follows the link and answers "not
// there" for one that dangles — which is the shape a removal these cases assert about leaves behind
// when it half happens.
func (f *fixture) expectAbsent(path string) {
	f.t.Helper()
	if info, err := os.Lstat(path); err == nil {
		f.t.Errorf("%s is still there (%s)", path, info.Mode())
	}
}

func (f *fixture) expectDir(path string) {
	f.t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		f.t.Errorf("%s is not a directory, so the run never created it: %v", path, err)
	}
}

func (f *fixture) expectNotSymlink(path string) {
	f.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		f.t.Errorf("%s became a symlink to %s, so what was there was replaced", path, value)
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
