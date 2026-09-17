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
// Each binary's line is taken by driving the refusal rather than by reading the source for a literal:
// a test that greps the constant out of the package would agree with the code however wrong the
// printed text is. Driven as a process rather than called in-process, because cite-graph and rule-echo
// are `package main` and no test can import them — and those two were where the name drift was. argv[0]
// is the stub's own path, which is what `exec -a "$0"` hands the binary, and what the model-policy
// family resolves its models.json from.
package tools_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
}

func (r refusal) base() string {
	return path.Base(r.stub)
}

var refusals = []refusal{
	// An unknown option, refused in argument parsing before the first link is surveyed. Not a bare
	// invocation: for either of these that one mounts this machine's own configuration.
	{stub: "ai/bootstrap.sh", args: []string{"--nope"}},
	{stub: "env/bootstrap.sh", args: []string{"--nope"}},
	// An unknown flag, refused in argument parsing before anything reads the machine or writes a cache.
	{stub: "ai/gate.sh", args: []string{"--nope"}},
	{stub: "ai/guide.sh", args: []string{"--nope"}},
	// Its main names a provider and loads a policy before the argument count is looked at, and refuses
	// without one — so this row reaches the usage line only with a provider named. No provider is
	// called: what refuses is the missing argument.
	{stub: "ai/kk-flavor/scripts/bloat-judge.sh", env: []string{"JUDGE_PROVIDER=claude"}},
	// An unknown argument, refused before the client is looked for and long before a registry is
	// written. Not a bare invocation: that one is also refused, but only after this machine has been
	// asked whether it has the client's CLI.
	{stub: "ai/mcp-sync.sh", args: []string{"--nope"}},
	// An unknown option, refused in argument parsing before any project is read or written.
	{stub: "ai/install-project.sh", args: []string{"--nope"}},
	{stub: "ai/project-mcp.sh", args: []string{"--nope"}},
	// No arguments at all, which is the one refusal this entry point has: it takes a worktree it cannot
	// default, and any path it were handed here would be one on the machine running the suite.
	{stub: "ai/project-skills.sh"},
	{stub: "ai/kk-flavor/scripts/model-check.sh", args: []string{"--nope"}},
	{stub: "ai/kk-flavor/scripts/model-policy.sh", args: []string{"--nope"}},
	// Two roots where the tool takes one path.
	{stub: "ai/kk-flavor/scripts/repo-key.sh", args: []string{"one", "two"}},
	{stub: "ai/kk-flavor/scripts/tree-fingerprint.sh", args: []string{"one", "two"}},
	{stub: "ai/kk-flavor/skills/idsd-qualify/scripts/report.sh", args: []string{"nope"}},
	// An unknown flag, which the dispatch refuses. Not `audit asked`, which is a valid subcommand that
	// OVERWRITES the recorded date and is undone by nothing.
	{stub: "ai/kk-flavor/skills/idsd-ship/scripts/cadence.sh", args: []string{"--nope"}},
	{stub: "ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh", args: []string{"--agent=claude", "one", "two"}},
	{stub: "ai/kk-flavor/skills/kk-ecosystem/scripts/cite-graph.sh"},
	{stub: "ai/kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh"},
	// An unknown option, refused in argument parsing before either scanner asks git anything. Not a
	// revision git cannot resolve, which is git's complaint about the tree and carries no grammar.
	{stub: "ai/kk-flavor/skills/kk-edit/scripts/comment-density.sh", args: []string{"--nope"}},
	{stub: "ai/kk-flavor/skills/kk-handoff/scripts/handoff-check.sh"},
	{stub: "ai/kk-flavor/skills/kk-reduce/scripts/stats.sh", args: []string{"--agent=claude", "one", "two"}},
	{stub: "ai/kk-flavor/workers/refactor/dup-literals.sh", args: []string{"--nope"}},
}

func TestEveryStubDocumentsTheUsageItsBinaryPrints(t *testing.T) {
	stubs := discoverStubs(t)
	tools := declaredTools(t, stubs)
	rows := rowPerStub(t, stubs)
	binaries := buildTools(t, stubs, tools)
	cwd := emptyRepository(t)

	for _, stub := range stubs {
		row, listed := rows[stub]
		if !listed {
			continue
		}
		t.Run(row.base(), func(t *testing.T) {
			printed := refusedUsage(t, filepath.Join(binaries, tools[stub]), cwd, row)
			if printed == "" {
				t.Errorf("%s printed no lowercase `usage:` line when refused, so this case would pass against "+
					"any stub at all. A capitalised `Usage:` reads the same to a human and is invisible to this "+
					"scan and to eco-check's.", row.base())
				return
			}
			if documented := documentedUsage(t, stub); documented != printed {
				t.Errorf("%s documents a usage line its binary does not print\n  stub: %q\nbinary: %q\n"+
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

// Every tool the stubs name, built once into a directory of this test's own. `./cmd/<tool>/` where the
// tool keeps its library apart, `./<tool>/` where its main sits with its code — resolve.sh's rule, and
// the reason two of these tools can be reached only as processes.
func buildTools(t *testing.T, stubs []string, tools map[string]string) string {
	t.Helper()
	into := t.TempDir()
	build := []string{"build", "-o", into + string(os.PathSeparator)}
	for _, stub := range stubs {
		tool := tools[stub]
		pkg := "./" + tool + "/"
		if info, err := os.Stat(filepath.Join("cmd", tool)); err == nil && info.IsDir() {
			pkg = "./cmd/" + tool + "/"
		}
		build = append(build, pkg)
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
func refusedUsage(t *testing.T, binary, cwd string, row refusal) string {
	t.Helper()
	stub, err := filepath.Abs(filepath.Join(repoRoot, row.stub))
	if err != nil {
		t.Fatalf("resolving %s: %v", row.stub, err)
	}
	// A home of this test's own, so nothing a refusal reads on its way to refusing is the human's, and
	// nothing it writes lands there.
	home := t.TempDir()
	run := exec.Command(binary, row.args...)
	// What `exec -a "$0"` gives the binary. Three tools find their models.json from it and one finds its
	// ledger, so a bare basename here would refuse for a reason the stub never produces.
	run.Args[0] = stub
	run.Dir = cwd
	run.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"))
	run.Env = append(run.Env, row.env...)
	var output bytes.Buffer
	run.Stdout, run.Stderr = &output, &output

	var exit *exec.ExitError
	switch err = run.Run(); {
	case err == nil:
		t.Fatalf("expected exit 2 from a refused invocation of %s, it exited 0\n%s", row.base(), output.String())
	case !errors.As(err, &exit):
		t.Fatalf("could not run the binary behind %s: %v\n%s", row.base(), err, output.String())
	case exit.ExitCode() != 2:
		t.Fatalf("expected exit 2 from a refused invocation of %s, got %d\n%s",
			row.base(), exit.ExitCode(), output.String())
	}

	for _, line := range strings.Split(output.String(), "\n") {
		if text := strings.TrimPrefix(line, row.base()+": "); strings.HasPrefix(text, "usage: ") {
			return text
		}
	}
	return ""
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
