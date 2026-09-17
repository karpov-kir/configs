// A stub's header documents a usage line and the binary behind it prints one when it refuses a bad
// invocation. Nothing else compares the two: each binary's line is asserted in its own suite and each
// stub's is only scanned for its lowercase prefix by eco-check, so the two flag lists could drift
// apart with both suites green.
//
// The stubs are discovered and never listed, so the one written tomorrow is held here without an edit.
// What no scan can derive is the invocation that makes each tool refuse: a guess either scans the
// human's tree or, for cadence, is accepted and OVERWRITES a date. Those sit in a table, and discovery
// is what keeps the table honest — a stub with no row fails naming itself, rather than dropping out of
// the loop in silence, which is the same hole in a new place.
//
// Each tool's line is taken by driving its refusal rather than by reading the source for a literal: a
// test that greps the constant out of the package would agree with the code however wrong the printed
// text is.
//
// Most of them are driven IN THIS PROCESS. Four cannot be: cite-graph, rule-echo, install-project and
// project-skills keep their main with their code as `package main`, which no test can import, so those
// are built and executed. The first exec of a fresh binary costs four to six seconds under this
// machine's endpoint agent, and twenty-two of them were 30 of the root package's 33 seconds.
//
// A row's `call` is its in-process route, and it hands the tool's Run exactly what that tool's `main`
// hands it. Three properties the exec route had are kept there by hand. argv[0] is the stub's own path,
// which is what `exec -a "$0"` gives the binary and what the model-policy family resolves its
// models.json from. $HOME, $XDG_CONFIG_HOME and the working directory are this test's own, so nothing a
// refusal reads on the way to refusing is the human's. And the refusal still has to exit 2.
//
// What a process proved and a call cannot is that `cmd/<tool>/main.go` wires Run the way the row does.
// That is `stub_reach_test.go`'s, which drives whole stubs end to end. The name drift these cases exist
// for was in cite-graph and rule-echo, and both of those are still processes.
package tools_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	aibootstrap "configs/ai/tools/ai-bootstrap"
	bloatjudge "configs/ai/tools/bloat-judge"
	"configs/ai/tools/cadence"
	density "configs/ai/tools/comment-density"
	duplicates "configs/ai/tools/dup-literals"
	ecocheck "configs/ai/tools/eco-check"
	ecoguide "configs/ai/tools/eco-guide"
	ecoreport "configs/ai/tools/eco-report"
	ecostats "configs/ai/tools/eco-stats"
	envbootstrap "configs/ai/tools/env-bootstrap"
	"configs/ai/tools/gate"
	handoffcheck "configs/ai/tools/handoff-check"
	"configs/ai/tools/machine"
	mcpsync "configs/ai/tools/mcp-sync"
	modelcheck "configs/ai/tools/model-check"
	modelpolicy "configs/ai/tools/model-policy"
	projectmcp "configs/ai/tools/project-mcp"
	"configs/ai/tools/repo"
	repokey "configs/ai/tools/repo-key"
	"configs/ai/tools/shell"
	treefingerprint "configs/ai/tools/tree-fingerprint"
	waitreap "configs/ai/tools/wait-reap"
)

const (
	// A stub carries the shared region AS a region. A file that names the marker inside a string it
	// searches for answers a grep for the name and carries neither of these lines, which is the
	// difference between a stub and a file that talks about stubs. Told apart by that property rather
	// than by name, because a name here is the stale list this suite exists to replace.
	stubRegionOpen  = "# --- shared:tool-stub ---"
	stubRegionClose = "# --- end shared:tool-stub ---"
	// What every stub declares the tool behind it to be, and how this suite finds the package to build.
	toolDeclaration = `tool="`
)

// One stub's cheapest refusal — cheapest meaning the one that is not a complaint about the tree, so
// what comes back is the usage line rather than a message about a path that does not exist. No args is
// itself a refusal, and the one three of these tools take: they want a path they cannot default.
//
// Every row compares, and there is no field for one that does not. A binary printing no usage line when
// refused is a binary to fix rather than a row to narrow: narrowed, its header stays documented and
// held against nothing, which is the state this suite exists to end.
type refusal struct {
	stub string
	args []string
	env  []string
	// call runs this tool in this process, and is what its own `main` runs. Nil builds the tool and
	// execs it, which is the only route into a `package main`.
	call func(inv invocation) int
}

func (r refusal) base() string {
	return path.Base(r.stub)
}

// One refusal's context: what a process would have been given, handed to a Run instead. Every field is
// this test's own, so nothing a refusal reads on the way to refusing is the human's.
type invocation struct {
	// stub is the absolute path to the stub, which is argv[0] — what `exec -a "$0"` gives the binary.
	stub string
	// stubDir is the directory the stub really sits in, symlinks resolved, which is where four of these
	// tools read their own sources, declarations and launcher from. Each of those mains resolves it the
	// same way, off argv[0].
	stubDir string
	args    []string
	cwd     string
	home    string
	// out takes both streams, the way CombinedOutput did: a refusal writing to stderr with nothing on
	// stdout would otherwise leave the usage line unread.
	out io.Writer
}

// The program name a refusal prints, which is the stub's basename and not this binary's.
func (i invocation) self() string {
	return filepath.Base(i.stub)
}

func (i invocation) configHome() string {
	return filepath.Join(i.home, ".config")
}

func (i invocation) arg(at int) string {
	if at < len(i.args) {
		return i.args[at]
	}
	return ""
}

var refusals = []refusal{
	// An unknown option, refused in argument parsing before the first link is surveyed. Not a bare
	// invocation: for either of these that one mounts this machine's own configuration.
	{stub: "ai/bootstrap.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return aibootstrap.Run(aibootstrap.Options{
			Self: i.self(), Args: i.args, Repo: i.stubDir, Home: i.home,
			CodexHome: filepath.Join(i.home, ".codex"), ConfigHome: i.configHome(),
			Machine: machine.New(), Out: i.out, Err: i.out,
		})
	}},
	{stub: "env/bootstrap.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return envbootstrap.Run(envbootstrap.Options{
			Self: i.self(), Args: i.args, Repo: i.stubDir, Home: i.home, ConfigHome: i.configHome(),
			Machine: machine.New(), Out: i.out, Err: i.out,
		})
	}},
	// An unknown flag, refused in argument parsing before anything reads the machine or writes a cache.
	{stub: "ai/gate.sh", args: []string{"--nope"}, call: func(i invocation) int {
		// The repository is the parent of the directory holding the stub, which is how the command
		// resolves it with GATE_ROOT unset.
		return gate.Run(i.args, gate.Env{Root: filepath.Dir(i.stubDir)}, i.out, i.out)
	}},
	{stub: "ai/guide.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return ecoguide.Run(i.stub, i.args, i.out, i.out)
	}},
	// Its main names a provider and loads a policy before the argument count is looked at, and refuses
	// without one — so this row reaches the usage line only with a provider named. No provider is
	// called: what refuses is the missing argument.
	{stub: "ai/kk-flavor/scripts/bloat-judge.sh", env: []string{"JUDGE_PROVIDER=claude"}, call: func(i invocation) int {
		return bloatjudge.Main(i.stub, i.args, strings.NewReader(""), i.out, i.out)
	}},
	// An unknown argument, refused before the client is looked for and long before a registry is
	// written. Not a bare invocation: that one is also refused, but only after this machine has been
	// asked whether it has the client's CLI.
	{stub: "ai/mcp-sync.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return mcpsync.Run(i.self(), i.args, i.stubDir, mcpsync.NewCLIClient(i.out, i.out), i.out, i.out)
	}},
	// An unknown option, refused in argument parsing before any project is read or written.
	{stub: "ai/install-project.sh", args: []string{"--nope"}},
	{stub: "ai/project-mcp.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return projectmcp.Run(i.self(), i.args, i.stubDir, i.home, repo.Exec{}, i.out, i.out)
	}},
	// No arguments at all, which is the one refusal this entry point has: it takes a worktree it cannot
	// default, and any path it were handed here would be one on the machine running the suite.
	{stub: "ai/project-skills.sh"},
	{stub: "ai/kk-flavor/scripts/model-check.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return modelcheck.Run(modelcheck.Command{Args: i.args, Invocation: i.stub, Stdout: i.out, Stderr: i.out})
	}},
	{stub: "ai/kk-flavor/scripts/model-policy.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return modelpolicy.Run(modelpolicy.Command{Args: i.args, Invocation: i.stub, Stdout: i.out, Stderr: i.out})
	}},
	// Two roots where the tool takes one path.
	{stub: "ai/kk-flavor/scripts/repo-key.sh", args: []string{"one", "two"}, call: func(i invocation) int {
		return repokey.Run(i.args, repokey.CommandGit(), i.out, i.out)
	}},
	{stub: "ai/kk-flavor/scripts/tree-fingerprint.sh", args: []string{"one", "two"}, call: func(i invocation) int {
		return treefingerprint.Run(i.args, i.out, i.out)
	}},
	// An unknown flag, which the tool refuses before it reads the process listing. Not `--kill`, which
	// is valid and ends processes.
	{stub: "ai/kk-flavor/scripts/wait-reap.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return waitreap.Run(i.args, i.out, i.out)
	}},
	{stub: "ai/kk-flavor/skills/idsd-qualify/scripts/report.sh", args: []string{"nope"}, call: func(i invocation) int {
		// The one tool here that resolves a repository before it dispatches, so the working directory it
		// is handed has to be the empty one this suite built.
		return ecoreport.Invocation{
			Args: i.args, Dir: i.cwd, Self: i.stub, Home: i.home, ConfigHome: i.configHome(),
			Out: i.out, Err: i.out,
		}.Exec()
	}},
	// An unknown flag, which the dispatch refuses. Not `audit asked`, which is a valid subcommand that
	// OVERWRITES the recorded date and is undone by nothing.
	{stub: "ai/kk-flavor/skills/idsd-ship/scripts/cadence.sh", args: []string{"--nope"}, call: func(i invocation) int {
		return cadence.Run(i.self(), i.args, i.cwd, repo.Exec{}, time.Now, i.out, i.out)
	}},
	{stub: "ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh", args: []string{"--agent=claude", "one", "two"}, call: func(i invocation) int {
		return ecocheck.Run(i.args, repo.Exec{}, ecocheck.InstalledBash{}, i.out, i.out)
	}},
	{stub: "ai/kk-flavor/skills/kk-ecosystem/scripts/cite-graph.sh"},
	{stub: "ai/kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh"},
	// An unknown option, refused in argument parsing before either scanner asks git anything. Not a
	// revision git cannot resolve, which is git's complaint about the tree and carries no grammar.
	{stub: "ai/kk-flavor/skills/kk-edit/scripts/comment-density.sh", args: []string{"--nope"}, call: func(i invocation) int {
		cfg, err := density.ConfigFromEnv(os.LookupEnv)
		if err != nil {
			fmt.Fprintf(i.out, "%s: %s\n", i.self(), err)
			return 2
		}
		return density.Run(i.self(), i.args, i.cwd, repo.Exec{}, cfg, i.out, i.out)
	}},
	{stub: "ai/kk-flavor/skills/kk-handoff/scripts/handoff-check.sh", call: func(i invocation) int {
		// An environment no GIT_DIR can redirect, which is what the command hands it: a session drafting
		// a handoff is often standing in a linked worktree.
		git := repo.Exec{Env: repo.WithoutGitLocation(os.Environ())}
		return handoffcheck.Run(i.self(), i.arg(0), i.arg(1), git, i.out, i.out)
	}},
	{stub: "ai/kk-flavor/skills/kk-reduce/scripts/stats.sh", args: []string{"--agent=claude", "one", "two"}, call: func(i invocation) int {
		return ecostats.Run(i.stub, i.args, i.out, i.out)
	}},
	{stub: "ai/kk-flavor/workers/refactor/dup-literals.sh", args: []string{"--nope"}, call: func(i invocation) int {
		cfg, err := duplicates.ConfigFromEnv(os.LookupEnv)
		if err != nil {
			fmt.Fprintf(i.out, "%s: %s\n", i.self(), err)
			return 2
		}
		return duplicates.Run(i.self(), i.args, i.cwd, repo.Exec{}, cfg, i.out, i.out)
	}},
}

func TestEveryStubDocumentsTheUsageItsBinaryPrints(t *testing.T) {
	stubs := discoverStubs(t)
	tools := declaredTools(t, stubs)
	rows := rowPerStub(t, stubs)
	binaries := buildTools(t, stubs, tools, rows)
	cwd := emptyRepository(t)

	for _, stub := range stubs {
		row, listed := rows[stub]
		if !listed {
			continue
		}
		t.Run(row.base(), func(t *testing.T) {
			printed := refusedUsage(t, row, filepath.Join(binaries, tools[stub]), cwd)
			if printed == "" {
				t.Errorf("%s printed no lowercase `usage:` line when refused, so this case would pass against "+
					"any stub at all. A capitalised `Usage:` reads the same to a human and is invisible to this "+
					"scan and to eco-check's.", row.base())
				return
			}
			if documented := documentedUsage(t, stub); documented != printed {
				t.Errorf("%s documents a usage line its tool does not print\n  stub: %q\n  tool: %q\n"+
					"one of the two grew a flag the other did not", row.base(), documented, printed)
			}
		})
	}
}

// Every stub in the repository, by repo-relative path, sorted. A walk rather than git's listing, so a
// stub written and not yet added is checked too — the moment it is easiest to leave one uncovered.
func discoverStubs(t *testing.T) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(repoRoot, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".sh") {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if !carriesStubRegion(string(body)) {
			return nil
		}
		relative, err := filepath.Rel(repoRoot, name)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s for stubs: %v", repoRoot, err)
	}
	if len(found) < 2 {
		t.Fatalf("found %d file(s) carrying the %q region, so the loop below would assert almost nothing. "+
			"Either the region was renamed and this suite has to follow it, or the walk is reaching the "+
			"wrong tree.", len(found), stubRegionOpen)
	}
	sort.Strings(found)
	return found
}

// Both fences, each as a whole line of its own. A file that only mentions the marker inside a string
// carries neither.
func carriesStubRegion(body string) bool {
	open, closed := false, false
	for _, line := range strings.Split(body, "\n") {
		switch strings.TrimSpace(line) {
		case stubRegionOpen:
			open = true
		case stubRegionClose:
			closed = true
		}
	}
	return open && closed
}

// The tool each stub declares. Read rather than guessed from the filename: four stubs are named nothing
// like their tool — ruleecho.sh reaches rule-echo, stats.sh reaches eco-stats.
func declaredTools(t *testing.T, stubs []string) map[string]string {
	t.Helper()
	tools := map[string]string{}
	for _, stub := range stubs {
		tools[stub] = declaredTool(t, stub)
	}
	return tools
}

func declaredTool(t *testing.T, stub string) string {
	t.Helper()
	for _, line := range strings.Split(readStub(t, stub), "\n") {
		if after, found := strings.CutPrefix(line, toolDeclaration); found {
			if name, _, ok := strings.Cut(after, `"`); ok && name != "" {
				return name
			}
		}
	}
	t.Fatalf("%s carries the shared stub region and declares no %s… line, so nothing here can name the "+
		"binary it execs", stub, toolDeclaration)
	return ""
}

// The row for each discovered stub, with every gap in either direction reported. A stub with no row is
// the hole this suite exists to close; a row naming no stub is a table still describing a file that
// moved, and the case it was thought to cover has been running against nothing.
func rowPerStub(t *testing.T, stubs []string) map[string]refusal {
	t.Helper()
	rows := map[string]refusal{}
	for _, row := range refusals {
		rows[row.stub] = row
	}
	discovered := map[string]bool{}
	for _, stub := range stubs {
		discovered[stub] = true
		if _, listed := rows[stub]; !listed {
			t.Errorf("%s carries the shared stub region and has no row in this suite, so its usage line is "+
				"held against nothing. %d stubs were discovered and %d rows are written; add one naming the "+
				"invocation that makes its tool refuse.", stub, len(stubs), len(refusals))
		}
	}
	for _, row := range refusals {
		if !discovered[row.stub] {
			t.Errorf("this suite has a row for %s, which carries no shared stub region — it moved, or it was "+
				"retired. %d stubs were discovered. A row pointing at nothing drives nothing and reads exactly "+
				"like a case that passed.", row.stub, len(stubs))
		}
	}
	return rows
}

// The tools that still have to be executed, built once into a directory of this test's own. Only the
// rows with no in-process route: every binary built here is a fresh one, and the first exec of a fresh
// binary costs seconds under this machine's endpoint agent.
//
// `./cmd/<tool>/` where the tool keeps its library apart, `./<tool>/` where its main sits with its
// code — resolve.sh's rule. Today every row left here is of the second shape, which is exactly why it
// is left here: `package main` is what no test can import.
func buildTools(t *testing.T, stubs []string, tools map[string]string, rows map[string]refusal) string {
	t.Helper()
	into := t.TempDir()
	build := []string{"build", "-o", into + string(os.PathSeparator)}
	flagCount := len(build)
	for _, stub := range stubs {
		if row, listed := rows[stub]; !listed || row.call != nil {
			continue
		}
		tool := tools[stub]
		pkg := "./" + tool + "/"
		if info, err := os.Stat(filepath.Join("cmd", tool)); err == nil && info.IsDir() {
			pkg = "./cmd/" + tool + "/"
		}
		build = append(build, pkg)
	}
	// No package named is every tool reached in process, which is where this suite is headed rather
	// than an error. `go build` handed no package would build the directory it stands in.
	if len(build) == flagCount {
		return into
	}
	if output, err := exec.Command("go", build...).CombinedOutput(); err != nil {
		t.Fatalf("building the tools these stubs reach: %v\n%s", err, output)
	}
	return into
}

// A repository for the tools that read one from their working directory — eco-report refuses before its
// dispatch without one, and would then be reporting on the absent repository rather than on the
// argument. Empty and this test's own, so no refusal here can observe or touch the checkout.
func emptyRepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init for the refusals' working directory: %v\n%s", err, output)
	}
	return dir
}

// What a tool says when it refuses an invocation it cannot act on, taken from the line carrying the
// lowercase `usage:` anchor. Two spellings are accepted because the tools genuinely use both: some
// prefix the line with their own name, some print it bare.
func refusedUsage(t *testing.T, row refusal, binary, cwd string) string {
	t.Helper()
	output, code := drive(t, row, binary, cwd)
	if code != 2 {
		t.Fatalf("expected exit 2 from a refused invocation of %s, got %d\n%s", row.base(), code, output)
	}
	for _, line := range strings.Split(output, "\n") {
		if text := strings.TrimPrefix(line, row.base()+": "); strings.HasPrefix(text, "usage: ") {
			return text
		}
	}
	return ""
}

// One refusal, and the exit code it answered with. In this process where the row carries a route, and
// as a process where the tool is a `package main` nothing can import.
//
// Both routes get the same three things: argv[0] as the stub's own path, a $HOME of this test's own,
// and the empty repository as the working directory.
func drive(t *testing.T, row refusal, binary, cwd string) (string, int) {
	t.Helper()
	stub, err := filepath.Abs(filepath.Join(repoRoot, row.stub))
	if err != nil {
		t.Fatalf("resolving %s: %v", row.stub, err)
	}
	// A home of this test's own, so nothing a refusal reads on its way to refusing is the human's, and
	// nothing it writes lands there.
	home := t.TempDir()
	if row.call == nil {
		return asAProcess(t, row, binary, stub, cwd, home)
	}

	// The environment a process was given, set for this case alone. bloat-judge reads $HOME and
	// $XDG_CONFIG_HOME for a roll-deadline override before it looks at the arguments at all.
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	for _, entry := range row.env {
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}
	// shell.OwnDirectory, because that is the function every main reading its own directory off argv[0]
	// calls, symlinks and all: the checkout is often reached through one.
	stubDir, err := shell.OwnDirectory(stub)
	if err != nil {
		t.Fatalf("resolving %s to the directory its tool reads from: %v", row.stub, err)
	}
	var output bytes.Buffer
	code := row.call(invocation{
		stub: stub, stubDir: stubDir, args: row.args, cwd: cwd, home: home, out: &output,
	})
	return output.String(), code
}

func asAProcess(t *testing.T, row refusal, binary, stub, cwd, home string) (string, int) {
	t.Helper()
	run := exec.Command(binary, row.args...)
	// What `exec -a "$0"` gives the binary. Three tools find their models.json from it and one finds its
	// ledger, so a bare basename here would refuse for a reason the stub never produces.
	run.Args[0] = stub
	run.Dir = cwd
	run.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"))
	run.Env = append(run.Env, row.env...)
	var output bytes.Buffer
	run.Stdout, run.Stderr = &output, &output

	err := run.Run()
	if err == nil {
		return output.String(), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("could not run the binary behind %s: %v\n%s", row.base(), err, output.String())
	}
	return output.String(), exit.ExitCode()
}

// The usage line a stub's header states, with the comment marker and the trailing prose stripped. The
// stub writes it as `#   usage: <line>   # <what the argument means>`, so the run of spaces before the
// second marker is the boundary — a single space cannot be one, since the usage text holds those.
//
// The first such line and no other. Several stubs write an aligned block under theirs, and every line
// of those blocks is prose about one flag rather than more grammar, so a fold across them would hold
// the binary's line against an explanation.
func documentedUsage(t *testing.T, stub string) string {
	t.Helper()
	for _, line := range strings.Split(readStub(t, stub), "\n") {
		trimmed := strings.TrimLeft(strings.TrimPrefix(strings.TrimSpace(line), "#"), " ")
		if !strings.HasPrefix(trimmed, "usage: ") {
			continue
		}
		if cut := strings.Index(trimmed, "   #"); cut >= 0 {
			trimmed = trimmed[:cut]
		}
		return strings.TrimRight(trimmed, " ")
	}
	t.Fatalf("%s states no usage line, so the stub documents nothing to compare", stub)
	return ""
}

func readStub(t *testing.T, stub string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot, stub))
	if err != nil {
		t.Fatalf("read %s: %v", stub, err)
	}
	return string(body)
}
