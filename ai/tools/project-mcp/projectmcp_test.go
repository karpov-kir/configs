package projectmcp

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/installertest"
	"configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
)

// No case here runs a client, a package manager or git. Every case builds its own declaration under
// its own temporary directory, and git is the port's fake. The ignore cases arrange an answer instead
// of building a repository to get one.

// The declaration this repository actually ships stays out of this package. It is outside the Go
// module, and Go keys a package's test cache on the module. A case here that read it answers
// `ok (cached)` over a file that had changed underneath the run. That was measured on 2026-09-17.

// `ai/tools/shipped_mcp_declaration_test.go` reads the shipped file instead. testing.md puts it
// there, because a suite reads only what its runner's cache keys on.

// A declaration of the shape every case in this file is built from.
const declarationFixture = `// A declaration of this shape, not the one that ships.
{
  "mcpServers": {
    "alpha": {
      "type": "stdio",
      "command": "@CONFIGS@/mcp-env.sh",
      "args": ["npx", "-y", "@example/alpha", "--flag=a b", "$value é"]
    },
    "beta": {
      "type": "stdio",
      "command": "@CONFIGS@/mcp-env.sh",
      "args": ["beta-server"]
    }
  }
}
`

// The servers that fixture declares, in document order.
var fixtureServers = []string{"alpha", "beta"}

// An argument only a managed server carries, so an uninstall case can tell one from the project's own
// settings without naming a file format.
const managedServerArgument = "@example/alpha"

// No code in this package reads the process environment, because `home` is a parameter. No case can
// reach the owner's own home, even with the containment assertion removed.

// The guard stays because the shell this replaces once followed a live symlink out of its sandbox and
// overwrote real config files in this checkout. installertest.Tree is that guard, and it is the
// production one: it refuses a fixture write the way `installer.tree` refuses the run's own.

// A project of its own, with a home of its own, both under this case's temporary directory.
type project struct {
	*installertest.Tree
	t       *testing.T
	root    string
	dir     string
	home    string
	agent   string
	configs string
	git     repo.Git
}

func newProject(t *testing.T, agent string) *project {
	t.Helper()
	tree := installertest.New(t)
	root := tree.Base()
	// Non-ASCII and a space, because a project directory is a name from outside this system.
	dir := filepath.Join(root, "project é "+agent)
	home := filepath.Join(root, "home")
	configs := filepath.Join(root, "configs")
	for _, made := range []string{dir, home, configs} {
		tree.MkdirAll(made)
	}
	tree.Write(filepath.Join(configs, "mcp.jsonc"), declarationFixture)
	return &project{
		Tree: tree,
		t:    t, root: root, dir: dir, home: home, agent: agent,
		configs: configs,
		// No repository by default. Most projects a human points this at are one, but arranging git's
		// answer is the ignore cases' subject, and every other case would key on it by accident.
		git: notARepository(),
	}
}

func notARepository() *repotest.Fake {
	fake := repotest.New("/nowhere")
	fake.Fail["TopLevel"] = errors.New("not a git repository")
	return fake
}

type result struct {
	code   int
	stdout string
	stderr string
}

func (p *project) run(args ...string) result {
	p.t.Helper()
	full := append([]string{"--agent=" + p.agent}, args...)
	full = append(full, p.dir)
	var out, said bytes.Buffer
	code := Run("project-mcp.sh", full, p.configs, p.home, p.git, &out, &said)
	return result{code: code, stdout: out.String(), stderr: said.String()}
}

func (p *project) configFile() string {
	return filepath.Join(p.dir, filepath.FromSlash(configFiles[p.agent]))
}

func (p *project) read() string {
	p.t.Helper()
	return p.Read(p.configFile())
}

func (p *project) writeConfigFile(text string) {
	p.t.Helper()
	p.Write(p.configFile(), text)
}

// forEachAgent runs the body for both clients. The two file formats are two implementations of one
// behaviour, and a case written for one client makes no claim about the other.
func forEachAgent(t *testing.T, body func(t *testing.T, p *project)) {
	t.Helper()
	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			body(t, newProject(t, agent))
		})
	}
}

func TestADryRunWritesNothing(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		outcome := p.run("--dry-run")
		if outcome.code != exitDone {
			t.Fatalf("exit %d, want 0\n%s", outcome.code, outcome.stderr)
		}
		if !strings.Contains(outcome.stdout, "would update") {
			t.Errorf("a dry run did not say it would have written: %q", outcome.stdout)
		}
		entries, err := os.ReadDir(p.dir)
		if err != nil {
			t.Fatalf("reading the project: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("a dry run left %d entries in the project — the flag exists so a human can see what "+
				"would change before it does", len(entries))
		}
	})
}

func TestInstallExportsEveryPublicServerPortably(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("exit %d, want 0\n%s", outcome.code, outcome.stderr)
		}
		text := p.read()
		for _, name := range fixtureServers {
			if !strings.Contains(text, name) {
				t.Errorf("%s is missing from the project config, which is the whole deliverable\n%s", name, text)
			}
		}
		// Portable means it names a path every install of this flavor has, and leaves this checkout out
		// of it. A project file is committed and shared with everyone who clones the project.
		for _, needle := range []string{"$HOME/.kk-flavor", "../mcp-env.sh"} {
			if !strings.Contains(text, needle) {
				t.Errorf("the project config does not reach the launcher through %s\n%s", needle, text)
			}
		}
		if strings.Contains(text, p.root) {
			t.Errorf("the project config names this machine's own path, so every other clone of the "+
				"project gets a server that cannot start\n%s", text)
		}
	})
}

func TestAReinstallIsByteIdempotent(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("the first install exited %d\n%s", outcome.code, outcome.stderr)
		}
		before := p.read()
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("the second install exited %d\n%s", outcome.code, outcome.stderr)
		}
		if after := p.read(); after != before {
			t.Errorf("a second install rewrote the file, so every reinstall shows up as a diff in the "+
				"project's history\n  before: %q\n   after: %q", before, after)
		}
	})
}

func TestUninstallLeavesWhatThisToolDidNotWrite(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("the install exited %d\n%s", outcome.code, outcome.stderr)
		}
		p.writeConfigFile(withUnrelatedSetting(t, p))

		if outcome := p.run("--uninstall"); outcome.code != exitDone {
			t.Fatalf("the uninstall exited %d\n%s", outcome.code, outcome.stderr)
		}
		text := p.read()
		if !strings.Contains(text, "keep") {
			t.Errorf("uninstalling took the project's own settings with it — this tool owns its servers and "+
				"nothing else in the file\n%s", text)
		}
		if strings.Contains(text, managedServerArgument) {
			t.Errorf("uninstalling left a managed server behind\n%s", text)
		}
		// And back again, because an uninstall that cannot be undone is a one-way door.
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("reinstalling after an uninstall exited %d\n%s", outcome.code, outcome.stderr)
		}
		if !strings.Contains(p.read(), "keep") {
			t.Errorf("reinstalling took the project's own settings with it\n%s", p.read())
		}
	})
}

// The project's own setting plus an MCP server this tool does not manage, in each file's own grammar.
func withUnrelatedSetting(t *testing.T, p *project) string {
	t.Helper()
	text := p.read()
	if p.agent == codexAgent {
		return text + "\n[mcp_servers.other]\ncommand = \"keep\"\n"
	}
	return strings.Replace(text, `"mcpServers": {`,
		"\"custom\": \"keep\",\n  \"mcpServers\": {\n    \"other\": {\"command\": \"keep\"},", 1)
}

func TestAServerAlreadyThereUnderAnotherDefinitionIsRefused(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		text := `{"mcpServers":{"alpha":{"command":"mine"}}}`
		if p.agent == codexAgent {
			text = "[mcp_servers.alpha]\ncommand = \"mine\"\n"
		}
		p.writeConfigFile(text)

		if outcome := p.run(); outcome.code == exitDone {
			t.Fatalf("a server the human defined themselves was overwritten — this tool cannot tell an " +
				"intentional override from a stale copy, so it has to refuse")
		}
		if after := p.read(); after != text {
			t.Errorf("the refusal changed the file anyway\n  before: %q\n   after: %q", text, after)
		}
	})
}

// The write replaces the file by rename. A followed symlink writes wherever the link names, and a
// followed hard link breaks one the project made deliberately.
func TestAConfigThatIsNotARegularUnlinkedFileIsRefused(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		outside := filepath.Join(p.root, "outside")
		p.Write(outside, "{}")
		p.Symlink(outside, p.configFile())

		if outcome := p.run(); outcome.code == exitDone {
			t.Fatal("a symlinked config was followed, so the write landed outside the project")
		}
		text, err := os.ReadFile(outside)
		if err != nil {
			t.Fatalf("reading what the link named: %v", err)
		}
		if string(text) != "{}" {
			t.Errorf("the file the link named was written: %q", text)
		}
	})
	forEachAgent(t, func(t *testing.T, p *project) {
		other := filepath.Join(p.root, "other")
		p.Write(other, "{}")
		p.Hardlink(other, p.configFile())
		if outcome := p.run(); outcome.code == exitDone {
			t.Fatal("a hard-linked config was replaced by rename, which silently breaks a link the " +
				"project made on purpose")
		}
	})
}

// A project's MCP configuration is meant to be committed and reviewed. A config written into a path
// the project ignores reaches no other clone, and it differs silently from what every clone has.
func TestAnIgnoredConfigIsReportedWithoutRewritingTheProjectsIgnoreRules(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		fake := repotest.New(p.dir)
		fake.Ignore(".gitignore", configFiles[p.agent])
		p.git = fake

		outcome := p.run()
		if outcome.code == exitDone {
			t.Fatal("a config the project ignores was written, so it reaches nobody who clones the project")
		}
		if !strings.Contains(outcome.stderr, "ignored by Git") {
			t.Errorf("the refusal does not say why, and the fix is the project's ignore rules — which this "+
				"tool does not touch\n%s", outcome.stderr)
		}
		if _, err := os.Stat(p.configFile()); err == nil {
			t.Errorf("the config was written anyway")
		}
	})
}

// An uninstall skips that check. A file the project ignores is still worth clearing of servers, and a
// refusal leaves them there with no way to take them out.
func TestUninstallRunsEvenWhereTheConfigIsIgnored(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("the install exited %d\n%s", outcome.code, outcome.stderr)
		}
		fake := repotest.New(p.dir)
		fake.Ignore(".gitignore", configFiles[p.agent])
		p.git = fake

		if outcome := p.run("--uninstall"); outcome.code != exitDone {
			t.Fatalf("uninstalling from an ignored config exited %d\n%s", outcome.code, outcome.stderr)
		}
		if strings.Contains(p.read(), managedServerArgument) {
			t.Errorf("the servers are still there\n%s", p.read())
		}
	})
}

// An ignore question git leaves unanswered stops the write. A config this tool cannot ask about may
// be one it is writing where no other clone will see it.
func TestAnUnanswerableIgnoreQuestionStopsTheWrite(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		fake := repotest.New(p.dir)
		fake.Fail["Ignored"] = errors.New("git exploded")
		p.git = fake

		outcome := p.run()
		if outcome.code == exitDone {
			t.Fatal("the config was written without knowing whether git would track it")
		}
		if !strings.Contains(outcome.stderr, "cannot check whether Git will track") {
			t.Errorf("the refusal does not say what went unanswered\n%s", outcome.stderr)
		}
	})
}

func TestTheHomeDirectoryIsNotAProject(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		p.dir = p.home
		if outcome := p.run(); outcome.code == exitDone {
			t.Fatal("a project file was written into the home directory, where every client standing in " +
				"it reads it as a project of its own")
		}
	})
}

// `.` is what a human standing in their home types, and the guard compares the project against an
// absolute home. A spelling that stays relative matches none of it, and the run writes a project
// server list straight into the home directory.

// The home directory is no project target, however it is spelled. This case is not parallel, because
// the working directory is what makes `.` mean the home directory.
func TestTheHomeDirectoryIsNotAProjectHoweverItIsSpelled(t *testing.T) {
	for _, agent := range agents {
		p := newProject(t, agent)
		p.dir = "."
		t.Chdir(p.home)

		outcome := p.run()
		if !strings.Contains(outcome.stderr, "the home directory is not a project target") {
			t.Errorf("%s: a project file was written into the home directory, where every client standing "+
				"in it reads it as a project of its own\nexit %d\n%s", agent, outcome.code, outcome.stderr)
		}
		if entries, err := os.ReadDir(p.home); err != nil || len(entries) != 0 {
			t.Errorf("%s: the run left %d entries in the home directory", agent, len(entries))
		}
	}
}

// The private declaration carries credentials and internal hosts. A project file is committed, so no
// part of it may reach one.
func TestThePrivateDeclarationIsNeverReadHere(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		// Not valid JSON, so a run that reads it at all fails loudly instead of quietly copying it.
		p.Write(filepath.Join(p.configs, "mcp.private.jsonc"), "INVALID PRIVATE SECRET")

		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("a private declaration beside the public one stopped the run: exit %d\n%s",
				outcome.code, outcome.stderr)
		}
		if strings.Contains(p.read(), "SECRET") {
			t.Errorf("the private declaration reached a committed project file\n%s", p.read())
		}
	})
}

func TestAGrammarThisToolDoesNotUnderstandStopsIt(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name          string
		args          []string
		namesAProject bool
	}{
		{name: "an unknown option", args: []string{"--agent=claude", "--nope"}, namesAProject: true},
		{name: "a client this tool does not know", args: []string{"--agent=invalid", "--dry-run"}, namesAProject: true},
		{name: "no client at all", args: []string{"--dry-run"}, namesAProject: true},
		{name: "no project", args: []string{"--agent=claude"}},
		{name: "a project directory that is not there", args: []string{"--agent=claude", "/nowhere/at/all"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			p := newProject(t, claudeAgent)
			args := scenario.args
			if scenario.namesAProject {
				args = append(args, p.dir)
			}
			var out, said bytes.Buffer
			code := Run("project-mcp.sh", args, p.configs, p.home, p.git, &out, &said)
			if code != exitBadUsage {
				t.Errorf("exit %d, want %d — %s", code, exitBadUsage, scenario.name)
			}
			if !strings.Contains(said.String(), usage("project-mcp.sh")) {
				t.Errorf("the refusal does not state the grammar it wanted\n%s", said.String())
			}
		})
	}
}

// Two projects at once is an argument this tool cannot act on: it would configure one and leave the
// human believing it configured both.
func TestOnlyOneProjectMayBeNamed(t *testing.T) {
	t.Parallel()
	p := newProject(t, claudeAgent)
	var out, said bytes.Buffer
	code := Run("project-mcp.sh", []string{"--agent=claude", p.dir, p.home}, p.configs, p.home, p.git, &out, &said)
	if code != exitBadUsage {
		t.Errorf("exit %d, want %d\n%s", code, exitBadUsage, said.String())
	}
}

// This tool writes project files. Anything it leaves in the home directory is a user-scope setting it
// had no business creating, which is the other installer's job.
func TestNothingIsWrittenOutsideTheProject(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		if outcome := p.run(); outcome.code != exitDone {
			t.Fatalf("exit %d\n%s", outcome.code, outcome.stderr)
		}
		entries, err := os.ReadDir(p.home)
		if err != nil {
			t.Fatalf("reading the home directory: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("the run left %d entries in the home directory: this tool configures a project and "+
				"never user settings", len(entries))
		}
	})
}
