package projectmcp

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/repo"
	"kk-flavor/tools/repo/repotest"
)

// Nothing here runs a client, a package manager or git. Every case builds its own declaration under
// its own temporary directory, and git is the port's fake, so the ignore cases arrange an answer
// instead of building a repository to get one.
//
// The declaration this repository actually ships stays out of this package: it is outside the Go
// module, and Go keys a package's test cache on the module, so a case here that read it would answer
// `ok (cached)` over a file that had changed underneath the run — measured on 2026-09-17.
// `ai/tools/shipped_mcp_declaration_test.go` reads it instead. `testing.md` rule 11.
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

// A project of its own, with a home of its own, both under this case's temporary directory.
//
// Nothing in this package reads the process environment — `home` is a parameter — so no case can
// reach the owner's own home even if the containment below were removed. The guard stays because the
// shell this replaces was the file that once followed a live symlink out of its sandbox and
// overwrote real config files in this checkout, and it is asserted before the first write.
type project struct {
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
	root := t.TempDir()
	// Non-ASCII and a space, because a project directory is a name from outside this system.
	dir := filepath.Join(root, "project é "+agent)
	home := filepath.Join(root, "home")
	configs := filepath.Join(root, "configs")
	for _, made := range []string{dir, home, configs} {
		if err := os.MkdirAll(made, 0o755); err != nil {
			t.Fatalf("building the fixture %s: %v", made, err)
		}
		refuseOutside(t, root, made)
	}
	if err := os.WriteFile(filepath.Join(configs, "mcp.jsonc"), []byte(declarationFixture), 0o644); err != nil {
		t.Fatalf("building the declaration fixture: %v", err)
	}
	return &project{
		t: t, root: root, dir: dir, home: home, agent: agent,
		configs: configs,
		// No repository by default: most projects a human points this at are one, but arranging git's
		// answer is the ignore cases' subject and every other case would be keying on it by accident.
		git: notARepository(),
	}
}

func notARepository() *repotest.Fake {
	fake := repotest.New("/nowhere")
	fake.Fail["TopLevel"] = errors.New("not a git repository")
	return fake
}

// The physical parent, so a symlink anywhere in the path cannot route a write out of the sandbox.
func refuseOutside(t *testing.T, root, path string) {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolving the fixture path %s: %v", path, err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolving the sandbox root %s: %v", root, err)
	}
	if real != realRoot && !strings.HasPrefix(real, realRoot+string(os.PathSeparator)) {
		t.Fatalf("the fixture %s resolves to %s, outside this case's sandbox %s — a case allowed to run "+
			"from there writes into whatever the path really names", path, real, realRoot)
	}
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
	text, err := os.ReadFile(p.configFile())
	if err != nil {
		p.t.Fatalf("reading the project config: %v", err)
	}
	return string(text)
}

func (p *project) writeConfigFile(text string) {
	p.t.Helper()
	if err := os.MkdirAll(filepath.Dir(p.configFile()), 0o755); err != nil {
		p.t.Fatalf("making room for the project config: %v", err)
	}
	if err := os.WriteFile(p.configFile(), []byte(text), 0o644); err != nil {
		p.t.Fatalf("writing the project config: %v", err)
	}
}

// Every case below runs for both clients, because the two file formats are two implementations of one
// behaviour and a case written for one is a claim nobody makes about the other.
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
		// Portable means: it names the one path every install of this flavor has, and never this
		// checkout. A project file is committed and shared with everyone who clones the project.
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

// The write replaces the file by rename, so a link there would either write through it — landing
// wherever the link names — or break a hard link the project made deliberately.
func TestAConfigThatIsNotARegularUnlinkedFileIsRefused(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		outside := filepath.Join(p.root, "outside")
		if err := os.WriteFile(outside, []byte("{}"), 0o644); err != nil {
			t.Fatalf("building the fixture: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(p.configFile()), 0o755); err != nil {
			t.Fatalf("making room for the link: %v", err)
		}
		if err := os.Symlink(outside, p.configFile()); err != nil {
			t.Fatalf("linking the config: %v", err)
		}

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
		if err := os.WriteFile(other, []byte("{}"), 0o644); err != nil {
			t.Fatalf("building the fixture: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(p.configFile()), 0o755); err != nil {
			t.Fatalf("making room for the link: %v", err)
		}
		if err := os.Link(other, p.configFile()); err != nil {
			t.Fatalf("hard-linking the config: %v", err)
		}
		if outcome := p.run(); outcome.code == exitDone {
			t.Fatal("a hard-linked config was replaced by rename, which silently breaks a link the " +
				"project made on purpose")
		}
	})
}

// A project's MCP configuration is meant to be committed and reviewed. Written into a path the
// project ignores it reaches nobody else, and silently differs from what every other clone has.
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

// Uninstalling skips that check: removing servers from a file the project ignores is still worth
// doing, and refusing would leave them there with no way to take them out.
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

// git answering neither yes nor no is not a no: a config this tool cannot ask about is one it may be
// writing where nobody will see it.
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

// And however it is spelled. `.` is what a human standing in their home types, and the guard compares
// the project against an absolute home — so a spelling that stays relative matches nothing and the
// run writes a project server list straight into the home directory.
//
// Not parallel, because it is the working directory that makes `.` mean the home directory.
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

// The private declaration is the one carrying credentials and internal hosts. A project file is
// committed, so nothing from it may ever reach one.
func TestThePrivateDeclarationIsNeverReadHere(t *testing.T) {
	t.Parallel()
	forEachAgent(t, func(t *testing.T, p *project) {
		// Not valid JSON, so a run that reads it at all fails loudly rather than quietly copying it.
		if err := os.WriteFile(filepath.Join(p.configs, "mcp.private.jsonc"),
			[]byte("INVALID PRIVATE SECRET"), 0o644); err != nil {
			t.Fatalf("building the fixture: %v", err)
		}

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
