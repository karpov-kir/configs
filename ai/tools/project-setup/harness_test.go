package projectsetup_test

// The cases for the project install. ai/tools/installer's own suite drives what the mounting machinery
// does with a table. That covers a refused real file, a repointed stale link, a swept mount whose
// source is gone, and a stop at a second checkout. What is driven here: the two instruction files,
// the ignore region, the bucket the skills reach through, the worktree setup, and the prerequisite.

import (
	"testing"

	"configs/ai/tools/installertest"
	"configs/ai/tools/machine/fake"
	projectsetup "configs/ai/tools/project-setup"
	"configs/ai/tools/repo/repotest"
	"configs/ai/tools/runtest"
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
	*installertest.Tree
	*runtest.Output
	t       *testing.T
	base    string
	repo    string
	home    string
	project string
	machine *fake.Machine
	git     *repotest.Fake
	mcp     *fakeMcp
}

// Every case gets a home and a project of its own under t.TempDir(). No process is spawned here, for
// git, mise or the MCP tool. The shell suites this replaces built a real repository per case, a
// process costs about 100ms on this machine, and they ran for 819 and 246 seconds.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	tree := installertest.New(t)
	base := tree.Base()
	f := &fixture{
		Tree: tree, Output: runtest.NewOutput(t),
		t: t, base: base, repo: base + "/checkout/ai", home: base + "/home", project: base + "/project",
		machine: fake.New().Add("mise", "brew"), mcp: &fakeMcp{},
		// Per directory, so a project a case left undeclared reads as no repository. That is the ordinary
		// shape for a directory someone is merely trying this out in, and a single Root cannot express it.
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
	// because the hook is only written when both are. A fixture checkout missing them would make every
	// worktree case measure the guard instead of what it named.
	f.Write(root+"/project-skills.sh", "#!/usr/bin/env bash\n")
	f.MkdirAll(root + "/tools")
	f.WriteMode(root+"/tools/resolve.sh", "#!/usr/bin/env bash\n", 0o755)
	for _, name := range publicSkills {
		f.NewSkill(root, name, "")
	}
	for _, name := range maintainerSkills {
		f.NewSkill(root, name, "audience: maintainer\n")
	}
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
	f.Reset()
	run, code := projectsetup.Perform(projectsetup.Options{
		Self:       guardScriptName,
		Args:       args,
		Repo:       f.repo,
		Home:       f.home,
		ConfigHome: f.home + "/.config",
		Machine:    f.machine,
		Git:        f.git,
		Mcp:        f.mcp,
		Out:        &f.Out,
		Err:        &f.Err,
		WriteRoot:  f.base,
	})
	// The shell suite this replaces handed every case the same home. A fixture write followed a live
	// symlink into the checkout and overwrote real config files in the working tree. WriteRoot bounds
	// the run to this fixture's own tree, and the bound is read back here after every run.
	if run != nil {
		f.ExpectNoBreach(run.Breaches())
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
	f.Reset()
	return projectsetup.Sync(projectsetup.SyncOptions{
		Self:       "project-skills.sh",
		Args:       args,
		Repo:       f.repo,
		Home:       f.home,
		ConfigHome: f.home + "/.config",
		Git:        f.git,
		Out:        &f.Out,
		Err:        &f.Err,
		WriteRoot:  f.base,
	})
}

func (f *fixture) skillsMount(agent string) string {
	if agent == "codex" {
		return f.project + "/.agents/skills"
	}
	return f.project + "/.claude/skills"
}

// --- the MCP tool, as a fake ------------------------------------------------------------------------

// The project MCP tool is a whole tool with its own suite. This installer decides only what to do
// with the code it answers, so that code is what a case sets.
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
