package installer_test

// The fixtures and assertions the case files beside it share.

// Every case gets a home of its own under t.TempDir(), and the run it drives is bounded to that same
// tree. The cases here exercise the real linking logic, so a run leaves $home/.config/nvim pointing
// at the fixture checkout. That is correct behaviour, and harmless while each case gets its own
// home.

// It stops being harmless the moment two cases share one. The second case's write at
// $home/.config/nvim then finds a live symlink into a checkout and goes straight through it into a
// real config file.

// That is no hypothesis. The shell harness this replaces once handed every case the same home. It
// overwrote nvim/init.lua and starship/starship.toml in the working tree and left a stray symlink in
// nvim/.

// The suite reported it, too. A case failed saying something had been written where it should not
// be, and the report was read as a harness bug. What the broken run had already written to disk went
// unasked.

// So containment is asserted before each write, on both sides. The fixture writers in this file
// resolve a parent physically before touching it, and the run itself is built with WriteRoot, which
// makes the package refuse the same way.

// expectContained, a method on fixture, is what reads the second half back. It fails the case as a
// guard, and never as a result.

// Fixtures are built with os.MkdirAll, os.WriteFile and os.Symlink, with no shell process in
// between. A process costs about 100ms on the machine these are written on, and file I/O costs
// almost none of that. Symlinks are the substance of this package, so they are real ones on a real
// filesystem. An in-memory tree fails to model their semantics.

import (
	"os"
	"strings"
	"testing"

	"configs/ai/tools/installer"
	"configs/ai/tools/installertest"
)

// What the calling installer is called, and what the guard therefore looks for under a candidate root.
const scriptName = "bootstrap.sh"

const label = "env bootstrap"

// One case's tree: a checkout, a home, and the account the run printed.
type fixture struct {
	*installertest.Writer
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

// The same tree, with no checkout in it. A case that makes no link uses it: the region and registry
// cases, which write into files a caller names.

// The sources built anyway would cost every such case ten files to create and ten to tear down. File
// I/O is what this suite's wall time is made of now that no process spawns.
func newBareFixture(t *testing.T) *fixture {
	t.Helper()
	writer := installertest.New(t)
	base := writer.Base()
	f := &fixture{Writer: writer, t: t, base: base, repo: base + "/checkout"}
	f.home = base + "/home"
	f.MkdirAll(f.home)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the
// root, and the sources beside it. A fixture missing the installer would be turned away by the
// guard's last condition, and the case would then measure that instead of what it named.

// The sources are the four shapes env/bootstrap.sh mounts: two files at the top of the home, a whole
// directory, and a file whose parent directory does not exist yet.
func (f *fixture) newCheckout(root string) string {
	f.t.Helper()
	f.MkdirAll(root)
	f.MkdirAll(root + "/zsh")
	f.MkdirAll(root + "/git")
	f.MkdirAll(root + "/nvim")
	f.MkdirAll(root + "/starship")
	f.Write(root+"/"+scriptName, "#!/usr/bin/env bash\n")
	f.Write(root+"/zsh/.zshrc", "the shell config\n")
	f.Write(root+"/git/.gitconfig", "the git identity\n")
	f.Write(root+"/nvim/init.lua", "the editor config\n")
	f.Write(root+"/starship/starship.toml", "the prompt\n")
	return root
}

// The mount table a case drives. It is declared against whichever checkout the case asks about.
func (f *fixture) declareMounts(run *installer.Run, repo string) {
	run.AddConfig(repo+"/zsh/.zshrc", f.home+"/.zshrc")
	run.AddConfig(repo+"/git/.gitconfig", f.home+"/.gitconfig")
	run.AddConfig(repo+"/nvim", f.home+"/.config/nvim")
	run.AddConfig(repo+"/starship/starship.toml", f.home+"/.config/starship.toml")
}

// The run a case drives, with everything it did not name taken from the fixture. Out and WriteRoot
// are the fixture's whatever a case says. A case that could print elsewhere has output no assertion
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
	f.MkdirAll(f.repo + "/skills/" + name)
	f.Write(f.repo+"/skills/"+name+"/SKILL.md", "---\nname: "+name+"\ndescription: a skill\n---\n")
}

func (f *fixture) expectContained(run *installer.Run) {
	f.t.Helper()
	if breaches := run.Breaches(); len(breaches) > 0 {
		f.t.Fatalf("the run went for a path outside %s — %s\n"+
			"this is the containment guard, not a failing case", f.base, strings.Join(breaches, "; "))
	}
}

// --- the fixture writers ----------------------------------------------------------------------

// A fixture link dropped so another can take its place. It goes through the containment guard,
// because RefuseExistingSymlink, one of its checks, deliberately leaves an existing symlink alone.
// That refusal is a rule, and reaching for os directly would work around it.
func (f *fixture) removeLink(path string) {
	f.t.Helper()
	f.ContainedParent(path)
	if err := os.Remove(path); err != nil {
		f.t.Fatalf("the fixture could not drop %s: %v", path, err)
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

// An empty path, with no entry of any kind. Lstat, because Stat follows the link and answers "not
// there" for one that dangles. That is the shape a half-done removal leaves behind, and these cases
// assert about it.
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
