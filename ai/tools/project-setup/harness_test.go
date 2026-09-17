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

	"kk-flavor/tools/machine/fake"
	projectsetup "kk-flavor/tools/project-setup"
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
	t       *testing.T
	base    string
	repo    string
	home    string
	project string
	machine *fake.Machine
	git     *fakeGit
	mcp     *fakeMcp
	out     strings.Builder
	err     strings.Builder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := physical(t, t.TempDir())
	f := &fixture{
		t: t, base: base, repo: base + "/checkout/ai", home: base + "/home", project: base + "/project",
		machine: fake.New().Add("mise", "brew"), git: newFakeGit(), mcp: &fakeMcp{},
	}
	f.mkdirAll(f.home)
	f.newCheckout(f.repo)
	f.newProject(f.project)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and the skills beside it.
func (f *fixture) newCheckout(root string) {
	f.t.Helper()
	f.write(root+"/"+guardScriptName, "#!/usr/bin/env bash\n")
	for _, name := range publicSkills {
		f.newSkill(root, name, "")
	}
	for _, name := range maintainerSkills {
		f.newSkill(root, name, "audience: maintainer\n")
	}
}

func (f *fixture) newSkill(root, name, audience string) {
	f.t.Helper()
	f.write(root+"/kk-flavor/skills/"+name+"/SKILL.md",
		"---\nname: "+name+"\ndescription: a skill\n"+audience+"---\n")
}

// A project with something of its own in both files this installer writes, so every case asserting
// that the project's own content survives is comparing against something.
func (f *fixture) newProject(project string) {
	f.t.Helper()
	f.write(project+"/CLAUDE.md", "# project\n\nHow this project works.\n")
	f.write(project+"/.gitignore", "node_modules/\n")
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

// The nearest existing directory above what a fixture is about to write, resolved physically.
// Anything landing outside the case's own tree fails the case as a guard rather than as a result.
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
	// the links these cases produce point into a checkout, so a fixture write at one of those paths
	// afterwards lands in the real file.
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
	f.refuseExistingSymlink(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		f.t.Fatalf("the fixture could not create the parent of %s: %v", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		f.t.Fatalf("the fixture could not link %s at %s: %v", target, source, err)
	}
}

// A second name for one file, which is neither a symlink nor a missing file — so nothing but a link
// count catches a write about to land in a file this run was never told about.
func (f *fixture) hardlink(source, target string) {
	f.t.Helper()
	f.containedParent(target)
	f.refuseExistingSymlink(target)
	if err := os.Link(source, target); err != nil {
		f.t.Fatalf("the fixture could not hard link %s at %s: %v", target, source, err)
	}
}

func (f *fixture) rewrite(path, body string) {
	f.t.Helper()
	f.containedParent(path)
	f.refuseExistingSymlink(path)
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
	f.containedParent(directory)
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

// t.TempDir hands back /var/folders/… on macOS while /var is itself a symlink to /private/var.
func physical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("the case's own temp directory does not resolve, so nothing could be contained: %v", err)
	}
	return resolved
}

// --- git, as a working fake -----------------------------------------------------------------------

// A repository this fixture describes rather than builds. Working, not canned: the worktree listing,
// the shared git directory and each tree's own root are held as data a case sets, and every answer is
// derived from them the way git derives its own.
//
// A project with no entry here is not a repository at all, which is what a directory someone is trying
// this out in looks like.
type fakeGit struct {
	// commonDir is the shared git directory every worktree of the clone answers with.
	commonDir string
	// roots maps a directory to the worktree root git would answer for it, and commons to the shared git
	// directory it belongs to. A directory with no entry is not a worktree. Two maps rather than one
	// answer for every root, because a forged `.git/worktrees/` entry naming an unrelated repository is
	// exactly a root git answers for with a DIFFERENT shared directory, and that difference is the tell.
	roots   map[string]string
	commons map[string]string
	// worktrees is the listing, keyed by the root asked from.
	worktrees   []projectsetup.Worktree
	hooksPath   string
	isHooksPath bool
	// listingError makes `worktree list` fail, which is the one way this installer learns it cannot see
	// the clone at all.
	listingError error
}

func newFakeGit() *fakeGit {
	return &fakeGit{roots: map[string]string{}, commons: map[string]string{}}
}

// Declare one worktree: the directory, the root git answers for it, and the shared git dir.
func (g *fakeGit) worktreeAt(path string) *fakeGit {
	g.roots[path] = path
	g.commons[path] = g.commonDir
	g.worktrees = append(g.worktrees, projectsetup.Worktree{Path: path})
	return g
}

// A directory git answers for, belonging to a different clone. What a forged `.git/worktrees/` entry
// naming an unrelated repository looks like from here.
func (g *fakeGit) foreignWorktreeAt(path, common string) *fakeGit {
	g.roots[path] = path
	g.commons[path] = common
	g.worktrees = append(g.worktrees, projectsetup.Worktree{Path: path})
	return g
}

func (g *fakeGit) TopLevel(dir string) (string, error) {
	root, isWorktree := g.roots[dir]
	if !isWorktree {
		return "", errNotARepository
	}
	return root, nil
}

func (g *fakeGit) CommonDir(dir string) (string, error) {
	common, isWorktree := g.commons[dir]
	if !isWorktree || common == "" {
		return "", errNotARepository
	}
	return common, nil
}

func (g *fakeGit) HooksPath(string) (string, bool) {
	return g.hooksPath, g.isHooksPath
}

func (g *fakeGit) Worktrees(string) ([]projectsetup.Worktree, error) {
	return g.worktrees, g.listingError
}

type gitError string

func (e gitError) Error() string { return string(e) }

const errNotARepository = gitError("not a git repository")

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
