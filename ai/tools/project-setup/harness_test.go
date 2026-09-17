package projectsetup_test

// The cases for the project install. What the mounting machinery does with a table — refusing a real
// file, repointing a stale link, sweeping a mount whose source is gone, stopping at a second checkout —
// is driven in ai/tools/installer's own suite against the same code. What is driven here is this
// installer's own decisions: the two instruction files, the ignore region, the bucket the skills reach
// through, the worktree setup, and the prerequisite.
//
// Every case gets a home and a project of its own under t.TempDir(), and the run it drives is bounded
// to that same tree with WriteRoot. The shell suite this replaces once handed every case the same home:
// a fixture write followed a live symlink into the checkout and overwrote real config files in the
// working tree. So the bound is read back after every run, and a breach fails the case as a guard
// rather than as a result.
//
// Nothing here spawns a process — not git, not mise, not the MCP tool. The shell suites this replaces
// built a real repository per case, and on this machine a process costs about 100ms: they ran for 819
// and 246 seconds.

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kk-flavor/tools/installertest"
	"kk-flavor/tools/machine/fake"
	projectsetup "kk-flavor/tools/project-setup"
	"kk-flavor/tools/repo/repotest"
)

// What the second-checkout guard looks for under a candidate root. The installer's own name, and the
// name the sync entry point has to look for too — the mounts were written by the installer.
const guardScriptName = "install-project.sh"

// The skills the fixture checkout ships. Two audiences, because the tier cases are only comparing
// something on a tree that holds both.
var (
	publicSkills     = []string{"kk-build", "kk-edit"}
	maintainerSkills = []string{"kk-ecosystem"}
)

type fixture struct {
	*installertest.Writer
	t       *testing.T
	base    string
	repo    string
	home    string
	project string
	machine *fake.Machine
	git     *repotest.Fake
	mcp     *fakeMcp
	out     strings.Builder
	err     strings.Builder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	writer := installertest.New(t)
	base := writer.Base()
	f := &fixture{
		Writer: writer,
		t:      t, base: base, repo: base + "/checkout/ai", home: base + "/home", project: base + "/project",
		machine: fake.New().Add("mise", "brew"), mcp: &fakeMcp{},
		// Per directory, so a project no case declared is no repository — the ordinary shape for a
		// directory someone is merely trying this out in, and the one a single Root cannot express.
		git: repotest.New(base + "/project").PerDirectory(),
	}
	f.MkdirAll(f.home)
	f.newCheckout(f.repo)
	f.newProject(f.project)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and the skills beside it.
func (f *fixture) newCheckout(root string) {
	f.t.Helper()
	f.Write(root+"/"+guardScriptName, "#!/usr/bin/env bash\n")
	// What the post-checkout hook runs, and what that stub reaches its binary through. Both are here
	// because the hook is only written when both are, and a fixture checkout missing them would make
	// every worktree case measure the guard instead of what it named.
	f.Write(root+"/project-skills.sh", "#!/usr/bin/env bash\n")
	f.MkdirAll(root + "/tools")
	f.rewrite(root+"/tools/resolve.sh", "#!/usr/bin/env bash\n")
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

// A project with something of its own in both files this installer writes, so every case asserting
// that the project's own content survives is comparing against something.
func (f *fixture) newProject(project string) {
	f.t.Helper()
	f.Write(project+"/CLAUDE.md", "# project\n\nHow this project works.\n")
	f.Write(project+"/.gitignore", "node_modules/\n")
}

func (f *fixture) run(args ...string) int {
	f.t.Helper()
	f.out.Reset()
	f.err.Reset()
	run, code := projectsetup.Perform(projectsetup.Options{
		Self:       guardScriptName,
		Args:       args,
		Repo:       f.repo,
		Home:       f.home,
		ConfigHome: f.home + "/.config",
		Machine:    f.machine,
		Git:        f.git,
		Mcp:        f.mcp,
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

// The ordinary drive: one agent, this fixture's project.
func (f *fixture) install(args ...string) int {
	f.t.Helper()
	return f.run(append(args, f.project)...)
}

func (f *fixture) sync(worktree string) int {
	f.t.Helper()
	return f.syncWith("--sync", worktree)
}

func (f *fixture) syncWith(args ...string) int {
	f.t.Helper()
	f.out.Reset()
	f.err.Reset()
	return projectsetup.Sync(projectsetup.SyncOptions{
		Self:       "project-skills.sh",
		Args:       args,
		Repo:       f.repo,
		Home:       f.home,
		ConfigHome: f.home + "/.config",
		Git:        f.git,
		Out:        &f.out,
		Err:        &f.err,
		WriteRoot:  f.base,
	})
}

func (f *fixture) skillsMount(agent string) string {
	if agent == "codex" {
		return f.project + "/.agents/skills"
	}
	return f.project + "/.claude/skills"
}

// --- what a case asks afterwards ----------------------------------------------------------------

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

func (f *fixture) expectFileLacks(path, unwanted string) {
	f.t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if strings.Contains(string(got), unwanted) {
		f.t.Errorf("%s still carries %q. It holds:\n%s", path, unwanted, got)
	}
}

func (f *fixture) read(path string) string {
	f.t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatalf("the case could not read %s: %v", path, err)
	}
	return string(body)
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

// --- the fixture writers ------------------------------------------------------------------------

// A second name for one file, which is neither a symlink nor a missing file — so nothing but a link
// count catches a write about to land in a file this run was never told about.
func (f *fixture) hardlink(source, target string) {
	f.t.Helper()
	f.ContainedParent(target)
	f.RefuseExistingSymlink(target)
	if err := os.Link(source, target); err != nil {
		f.t.Fatalf("the fixture could not hard link %s at %s: %v", target, source, err)
	}
}

func (f *fixture) rewrite(path, body string) {
	f.t.Helper()
	f.ContainedParent(path)
	f.RefuseExistingSymlink(path)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		f.t.Fatalf("the fixture could not rewrite %s: %v", path, err)
	}
}

// A directory this process cannot create a file in. Probed rather than assumed: root ignores the mode
// bits and a filesystem can drop them, and a case that asserted against a directory it can still write
// would pass for a reason other than its name. Skipped with that reason where the probe says so, which
// is what ~/.kk-flavor/standards/testing.md asks of a fixture that denies access.
func (f *fixture) closeToNewFiles(directory string) {
	f.t.Helper()
	f.ContainedParent(directory)
	if err := os.Chmod(directory, 0o555); err != nil {
		f.t.Fatalf("the fixture could not close %s: %v", directory, err)
	}
	f.t.Cleanup(func() { os.Chmod(directory, 0o755) })
	probe := filepath.Join(directory, ".can-this-process-still-write")
	if file, err := os.Create(probe); err == nil {
		file.Close()
		os.Remove(probe)
		f.t.Skip("this process writes into a directory with no write bit — CAP_DAC_OVERRIDE, root, or a " +
			"filesystem that drops the bit — so the refusal this case names could not be built")
	}
}

// --- the MCP tool, as a fake ------------------------------------------------------------------------

// The project MCP tool is a whole tool with its own suite; what this installer decides is only what to
// do with the code it answers, so that code is what a case sets.
type fakeMcp struct {
	code  int
	calls []string
}

func (m *fakeMcp) Configure(project, agent string, isDryRun, isUninstall bool) int {
	mode := "install"
	if isUninstall {
		mode = "uninstall"
	}
	if isDryRun {
		mode += " dry-run"
	}
	m.calls = append(m.calls, agent+" "+mode+" "+project)
	return m.code
}

// --- reading a usage line ---------------------------------------------------------------------------

// The flags a usage line names. `--agent=claude|codex` is a selector rather than a flag, so both of
// its spellings come back.
var flagPattern = regexp.MustCompile(`--[a-z][a-z-]*(?:=[a-z|]+)?`)

func flagsIn(line string) []string {
	var found []string
	for _, match := range flagPattern.FindAllString(line, -1) {
		if name, choices, isSelector := strings.Cut(match, "="); isSelector {
			for _, choice := range strings.Split(choices, "|") {
				found = append(found, name+"="+choice)
			}
			continue
		}
		found = append(found, match)
	}
	slices.Sort(found)
	return slices.Compact(found)
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}
