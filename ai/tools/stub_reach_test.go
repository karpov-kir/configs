// Cases for the `# --- shared:tool-stub ---` region every skill script carries to reach its Go binary.
// It is one body copied into several files, and this file covers it once. That the copies are still
// identical is the wiring check's shared-region scan, and that each stub's documented usage line
// matches what its binary prints is `stub_usage_test.go`.

// They sit in this package, away from the resolver's own cases in `reach/`, for the reason
// shipped_tree_test.go gives. Each of them reads the checkout: the stubs themselves, the tree they
// walk, and the two real files the ledger case compares. A fixture would hold none of that.

// Two of these must not be weakened. The first is that a stub reaches its tool from an unrelated cwd,
// which is the defect the region exists for. A stub that failed to resolve stays silent, and silence
// reads exactly like a clean tree. The second is argv[0] surviving the exec, and it decides which
// skill directory a tool writes into.

// The offset scan discovers the stubs and holds no list of its own. A stub written tomorrow is
// therefore covered here, with no edit to this file. A scan that finds none fails, because a scan
// over an empty set is green for the wrong reason.

// It asks git, because `git ls-files` stops at a nested repository's edge and a walk keeps going. A
// developer keeping worktrees under their checkout has a copy of every stub in each of them. Each of
// those copies fails this case for a depth that is correct where it really sits.

// `--others`, so a stub written and still untracked is read too. That is the moment it is easiest to
// leave one uncovered. `-z`, because git C-quotes a path holding a quote or a non-ASCII byte, and a
// quoted name reaches no file.

// One scan answers this file and `stub_usage_test.go`, and it runs once per test binary. It costs a git
// process and a read of every script in the repository, and seven cases ask for it. The floors stay one
// per case, so a scan that narrowed is caught by whichever floor it drops under first.
package tools_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Both fences, each as a whole line of its own. A file that only mentions the marker inside a string
// carries neither, which is what tells a stub from a file that talks about stubs.
const (
	stubRegionOpen  = "# --- shared:tool-stub ---"
	stubRegionClose = "# --- end shared:tool-stub ---"
	// What a stub declares its depth to be.
	offsetDeclaration = `tools_offset="`
)

// The resolver the stubs reach, beside this package.
const resolveScript = "resolve.sh"

// The stub the fixture cases copy. It comes from the discovered set by name, so a stub that moved
// fails here. A fixture of this file's own never quietly replaces it.
const fixtureStub = "ai/kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh"

// One stub, and the depth it says it sits at.
type stubDepth struct {
	path   string
	offset string
}

// An upward walk or a list of candidates would reach a tools directory the stub does not name. Both
// earlier shapes of this region did, and either runs a stranger's binary at exit 0.

// The region resolves exactly one path, `$here/$tools_offset/tools/resolve.sh`, and consults no other.
// A stub's depth is therefore its own property, and what is worth holding is that each one declares
// the depth it really has.
func TestEveryStubDeclaresTheOffsetThatReachesItsOwnToolsDirectory(t *testing.T) {
	t.Parallel()
	stubs := stubDepths(t)

	// Two controls, because the loop over `stubs` is satisfied by an empty set and by one that never
	// leaves a single directory. The stubs declare five distinct offsets in this tree.
	if len(stubs) < 5 {
		t.Fatalf("the scan found %d script(s) carrying %q, so this case asserts almost nothing. Either the "+
			"region was renamed and this scan has to follow it, or the listing is reaching the wrong tree",
			len(stubs), stubRegionOpen)
	}
	directories := map[string]bool{}
	for _, found := range stubs {
		directories[filepath.Dir(found.path)] = true
	}
	if len(directories) < 3 {
		t.Fatalf("those stubs sit in %d director(ies), so a scan that only ever read one depth would "+
			"satisfy this case", len(directories))
	}

	for _, found := range stubs {
		t.Run(filepath.Base(found.path), func(t *testing.T) {
			if found.offset == "" {
				t.Fatalf("%s carries the shared region and declares no %s… line, so it resolves nothing",
					found.path, offsetDeclaration)
			}
			resolver := filepath.Join(repoRoot, filepath.Dir(found.path), found.offset, "tools", resolveScript)
			info, err := os.Stat(resolver)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				t.Errorf("%s declares tools_offset=%q, which resolves to %s — not an executable resolver "+
					"(%v). Every run of this stub exits 2 having run nothing", found.path, found.offset,
					resolver, err)
			}
		})
	}
}

// The defect the region exists for. A stub reaches its own tool from a directory unrelated to the
// checkout it lives in. The real tree drives it. A fixture would prove only that a copy of the region
// works where this case put it.
//
// One stub, because the region resolves `$here/$tools_offset/tools/resolve.sh` and consults nothing
// else: a second depth is another value through the same line. That every stub's own depth reaches a
// real resolver is TestEveryStubDeclaresTheOffsetThatReachesItsOwnToolsDirectory's, over all five.
func TestAStubReachesItsToolFromAnUnrelatedCwd(t *testing.T) {
	t.Parallel()
	// A root holding one restatement, which gives the tool something to find. Over an empty root, a
	// silent stub and a working one would satisfy the same assertion.
	root := newEchoRoot(t)

	// Only the binary behind the stub writes this wording. A stub that resolved no tool stays silent,
	// and silence reads exactly like a clean tree.
	const marker = "rule stated twice"
	reached := runStub(t, newStubLaunch(t, filepath.Join(repoRoot, fixtureStub), root))
	if !reached.said(marker) {
		t.Errorf("%s did not reach its tool from a cwd with nothing to do with its checkout\n%v",
			fixtureStub, reached)
	}
	if reached.code == 2 {
		t.Errorf("%s exited 2, which means the tool never ran\n%v", fixtureStub, reached)
	}
}

// Each way the stub cannot reach a resolver. All of them exit 2 and name the fix, because a stub that
// returned quietly would hand the caller silence, and a caller reads silence as a clean tree.
//
// Every checkout here is built under a sandbox, and the stub copied into each one is the real file.
// Its depth is the depth that file declares, so these cases read the checkout like the rest.
func TestAStubThatCannotReachAResolverExitsTwoAndNamesTheFix(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// Builds the checkout around the stub and returns the directory the stub was copied into.
		fixture func(t *testing.T, sandbox string) string
		says    string
		// A second wording, where the refusal has to carry more than the cause.
		alsoSays string
		// Output that would mean the stub resolved past the path it names.
		neverSays string
	}{
		{
			// A checkout that mounts the skill without shipping ai/tools/.
			name: "the checkout ships no tools directory",
			fixture: func(t *testing.T, sandbox string) string {
				return newStubCheckout(t, sandbox, "orphan")
			},
			says:     "does not ship ai/tools/",
			alsoSays: "did NOT run",
		},
		{
			// A resolver that is there and lost its exec bit: a different fix, so a different message.
			name: "the resolver is there and is not executable",
			fixture: func(t *testing.T, sandbox string) string {
				checkout := newStubCheckout(t, sandbox, "not-executable")
				writeFixture(t, filepath.Join(checkout, "tools", resolveScript),
					readFile(t, runnableScript(t, resolveScript)), 0o644)
				return checkout
			},
			says: "chmod",
		},
		{
			// A decoy one level above the checkout root, where two earlier shapes of this region reached.
			// Both ends are asserted: the refusal has to name the resolver it could not find, and the decoy
			// has to remain unreached. Either alone passes for the wrong reason, since a stub that died
			// early satisfies the second and a run that reached the decoy then failed satisfies the first.
			name: "a tools directory sits outside the checkout",
			fixture: func(t *testing.T, sandbox string) string {
				escape, err := os.MkdirTemp(sandbox, "escape-")
				if err != nil {
					t.Fatalf("building the decoy fixture: %v — nothing was measured", err)
				}
				writeFixture(t, filepath.Join(sandboxed(t, sandbox, escape), "tools", resolveScript),
					"#!/usr/bin/env bash\necho \"decoy resolver reached\" >&2\nexit 2\n", 0o755)
				return newStubCheckout(t, escape, "root")
			},
			says:      "no resolver at",
			neverSays: "decoy resolver reached",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			refused := runStub(t, newStubLaunch(t, filepath.Join(scenario.fixture(t, sandbox), stubIn(t)), newEchoRoot(t)))

			expectStubRefusal(t, refused, scenario.says)
			if scenario.alsoSays != "" && !refused.said(scenario.alsoSays) {
				t.Errorf("the refusal does not also say %q, which is the half telling a caller that no "+
					"finding was made\n%v", scenario.alsoSays, refused)
			}
			if scenario.neverSays != "" && refused.said(scenario.neverSays) {
				t.Errorf("the stub resolved past the path it names and ran a resolver outside its own "+
					"checkout\n%v", refused)
			}
		})
	}
}

// argv[0] survives the exec. The tools derive their skill directory from it, and a stub that let the
// binary's own path through would send every write to the tools directory. The ledger proves it. Its
// write is the only one with a visible destination. Both ends are asserted, and a run that wrote
// nowhere would satisfy the first half on its own.
func TestTheLedgerWriteLandsUnderTheSkillDirectoryTheStubWasInvokedBy(t *testing.T) {
	t.Parallel()
	before := readFile(t, liveLedger)

	// Mirrors the real layout, because stats.sh's declared offset is counted from
	// `kk-flavor/skills/<skill>/scripts/`. A shallower fixture puts the resolver out of its reach, and
	// the case then fails having tested the fixture and never the ledger path.
	sandbox := newSandbox(t)
	fake := sandboxed(t, sandbox, filepath.Join(sandbox, "checkout"))
	writeFixture(t, filepath.Join(fake, "tools", resolveScript),
		readFile(t, runnableScript(t, resolveScript)), 0o755)
	stats := filepath.Join("kk-flavor", "skills", "kk-reduce", "scripts", "stats.sh")
	writeFixture(t, filepath.Join(fake, stats),
		readFile(t, runnableScript(t, filepath.Join(repoRoot, "ai", stats))), 0o755)
	buildTool(t, "eco-stats", filepath.Join(fake, "tools", "bin", "eco-stats"))

	// The real ai/ as the root to measure, named absolutely: the launch runs from a directory of its own,
	// and the tool would read a relative root against that one.
	root, err := filepath.Abs(filepath.Join(repoRoot, "ai"))
	if err != nil {
		t.Fatalf("resolving the tree to measure: %v — nothing was measured", err)
	}
	appended := runStub(t, newStubLaunch(t, filepath.Join(fake, stats),
		"--agent=claude", "--append", "a row from the suite's own fixture", root))
	fixtureLedger := filepath.Join(fake, "kk-flavor", "skills", "kk-reduce", filepath.Base(liveLedger))
	if appended.code != 0 || !appended.said(fixtureLedger) {
		t.Errorf("the append did not land under the skill directory the stub was invoked by, which is where "+
			"a skill reached through its mount symlink keeps its own ledger\n%v", appended)
	}
	if body, err := os.ReadFile(fixtureLedger); err != nil || len(body) == 0 {
		t.Errorf("nothing was written to %s (%v), so the comparison below would pass against a run that "+
			"wrote nowhere at all", fixtureLedger, err)
	}
	if readFile(t, liveLedger) != before {
		t.Errorf("%s changed: the run wrote into the checkout it was reading, which is the incident every "+
			"fixture here is built under a sandbox to stop", liveLedger)
	}
}

// Every script in the repository carrying the shared region, with the offset each one declares. Scanned
// once and handed to every caller after that, because the scan is a git process and a read of every
// script in the repository.
func stubDepths(t *testing.T) []stubDepth {
	t.Helper()
	scanOnce.Do(func() { scanned, scanErr = scanStubs() })
	if scanErr != nil {
		t.Fatalf("%v — nothing was measured", scanErr)
	}
	return scanned
}

var (
	scanOnce sync.Once
	scanned  []stubDepth
	scanErr  error
)

func scanStubs() ([]stubDepth, error) {
	listed, err := exec.Command("git", "-C", repoRoot, "ls-files", "--cached", "--others",
		"--exclude-standard", "-z", "--", "*.sh").Output()
	if err != nil {
		return nil, fmt.Errorf("asking git for %s's scripts: %w", repoRoot, err)
	}
	// git lists the index and then what is untracked, so the two runs are each sorted and the join is not.
	scripts := strings.Split(strings.TrimSuffix(string(listed), "\x00"), "\x00")
	sort.Strings(scripts)

	var found []stubDepth
	for _, script := range scripts {
		if script == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(repoRoot, script))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", script, err)
		}
		if !carriesStubRegion(string(body)) {
			continue
		}
		found = append(found, stubDepth{path: script, offset: declaredOffset(string(body))})
	}
	return found, nil
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

func declaredOffset(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if after, held := strings.CutPrefix(line, offsetDeclaration); held {
			if offset, _, closed := strings.Cut(after, `"`); closed {
				return offset
			}
		}
	}
	return ""
}

// A checkout holding one stub at the depth that stub declares, and no other file. The declared offset
// builds it, and directory names written here play no part. A fixture a level off puts the resolver
// out of the stub's reach, and the case then fails for the fixture's shape.
func newStubCheckout(t *testing.T, sandbox, name string) string {
	t.Helper()
	checkout := filepath.Join(sandbox, name)
	writeFixture(t, filepath.Join(checkout, stubIn(t)),
		readFile(t, runnableScript(t, filepath.Join(repoRoot, fixtureStub))), 0o755)
	return checkout
}

// Where the fixture stub sits inside such a checkout: as many levels down as its offset has segments.
func stubIn(t *testing.T) string {
	t.Helper()
	for _, found := range stubDepths(t) {
		if found.path != fixtureStub {
			continue
		}
		levels := strings.Split(found.offset, string(filepath.Separator))
		place := make([]string, 0, len(levels)+1)
		for range levels {
			place = append(place, "down")
		}
		return filepath.Join(append(place, filepath.Base(fixtureStub))...)
	}
	t.Fatalf("%s carries no shared stub region, so the fixtures below would be built around a file that is "+
		"not a stub", fixtureStub)
	return ""
}

// A root holding one restatement in two files, which is what `ruleecho.sh` has to find.
func newEchoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	const rule = "**a shared rule stating several discriminating words plainly**\n"
	writeFixture(t, filepath.Join(root, "a", "one.md"), "# A\n\n"+rule, 0o644)
	writeFixture(t, filepath.Join(root, "b", "two.md"), "# B\n\n"+rule, 0o644)
	return root
}

// One tool, built where the caller wants it. The only real build in this file. The ledger case
// measures where the binary writes, and the binary has to be the real one.

// `./cmd/<tool>/` for a tool keeping its library apart, and `./<tool>/` for one whose main sits with
// its code. That is resolve.sh's own rule. Build the library package by mistake and `-o` writes an
// archive, which is not executable, and the resolver then reports a half-finished install.
func buildTool(t *testing.T, name, into string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(into), 0o755); err != nil {
		t.Fatalf("building %s: %v — nothing was measured", filepath.Dir(into), err)
	}
	pkg := "./" + name + "/"
	if info, err := os.Stat(filepath.Join("cmd", name)); err == nil && info.IsDir() {
		pkg = "./cmd/" + name + "/"
	}
	if output, err := exec.Command("go", "build", "-o", into, pkg).CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s — nothing was measured", name, err, output)
	}
}

// What one launch of a stub came back with. stdout and stderr are kept apart, because a refusal has
// to be audible on stderr. stdout carries what a caller reads.
type stubRun struct {
	stdout string
	stderr string
	code   int
}

// Whether either stream holds the wording. A refusal is asserted on what it says, and never on its
// exit code alone. Every refusal the region has exits 2, and the code says one happened without
// saying which. A case reading the code alone passes on whatever the fixture broke first.
func (r stubRun) said(wording string) bool {
	return strings.Contains(r.stdout, wording) || strings.Contains(r.stderr, wording)
}

func (r stubRun) String() string {
	return fmt.Sprintf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
}

// A launch of a stub, with an environment of its own. HOME sits under this case's own temp directory,
// because the tools these stubs reach read and write beneath it. PATH is this machine's real one,
// because the resolver behind them rebuilds whenever the checkout has moved past the binary in bin/.

// The build cache is the machine's too, since a `go build` given an empty one compiles the standard
// library before it reaches the tool.

// The working directory is an empty one, and no part of the fixture knows it. Every stub finds its own
// directory from `BASH_SOURCE`, and reaching one from cwd is the defect the region exists to stop. No
// case here is given a cwd that might hide it.
// The script is bash's argument, and bash is what this execs. Linux refuses to exec a file any process
// holds open for writing, with ETXTBSY. These cases write their fixture scripts and run them, and one
// case's open descriptor reaches another case's fork. That failed the go job on a push to main, and
// macOS has no such rule, so it passed here. bash opens the script to read.
func newStubLaunch(t *testing.T, script string, arguments ...string) *exec.Cmd {
	t.Helper()
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("finding this machine's cache directory: %v — nothing was measured", err)
	}
	home := t.TempDir()
	command := exec.Command(bashOnThisMachine(t), append([]string{runnableScript(t, script)}, arguments...)...)
	command.Dir = t.TempDir()
	command.Env = []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + os.Getenv("PATH"),
		"GOCACHE=" + filepath.Join(cache, "go-build"),
	}
	return command
}

// One launch. A stub that could not be started at all is fatal. Every case here is a launch, and a
// failed case would otherwise report a reason unrelated to the guard it names.
func runStub(t *testing.T, command *exec.Cmd) stubRun {
	t.Helper()
	var out, errOut strings.Builder
	command.Stdout, command.Stderr = &out, &errOut
	result := stubRun{}
	var exit *exec.ExitError
	switch runErr := command.Run(); {
	case runErr == nil:
	case errors.As(runErr, &exit):
		result.code = exit.ExitCode()
	default:
		t.Fatalf("could not run %s: %v — nothing was measured", command.Path, runErr)
	}
	result.stdout, result.stderr = out.String(), errOut.String()
	return result
}

// This holds a refusal to the wording only its own cause produces. Every refusal the region has exits
// 2, and the code says one happened without saying which. A case asserting the code alone passes on
// whatever the fixture broke first, while its name claims the cause. `command not found` is a PATH
// short of something the stub calls, which no case here ever means.
func expectStubRefusal(t *testing.T, got stubRun, wording string) {
	t.Helper()
	if got.code != 2 {
		t.Errorf("wanted exit 2 and the refusal %q\n%v", wording, got)
		return
	}
	if got.said("command not found") || got.said(": not found") {
		t.Errorf("a missing command produced this refusal, not %q — the PATH is short of something the stub "+
			"calls, so this case measured that instead\n%v", wording, got)
		return
	}
	if !got.said(wording) {
		t.Errorf("the refusal does not say %q, so a caller cannot tell this cause from the others that also "+
			"exit 2\n%v", wording, got)
	}
}

// The path to a script, refused loudly where it cannot be run.
func runnableScript(t *testing.T, script string) string {
	t.Helper()
	path, err := filepath.Abs(script)
	if err != nil {
		t.Fatalf("resolving %s: %v — nothing was measured", script, err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not an executable file (%v) — nothing was measured, and every case reaching for it "+
			"would fail for that reason rather than for its own", path, err)
	}
	return path
}

// macOS reaches a temp directory through a symlinked /var, and these stubs write executables. The
// incident behind this: a harness bug once handed every case the same HOME. It followed a live
// symlink into the checkout and overwrote config files in the working tree.

// A directory every fixture is built under, resolved physically. The resolution here is what lets
// sandboxed() refuse a path before anything is written to it.
func newSandbox(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving this case's temp directory: %v — nothing was measured", err)
	}
	return dir
}

// A fixture path, refused unless it really lies inside the sandbox. Every caller reaches it before the
// directory is built and before any script writes into it. Afterwards the write has already landed,
// and what these scripts write is executable files.
func sandboxed(t *testing.T, sandbox, path string) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		t.Fatalf("resolving the parent of %s: %v — a fixture whose path cannot be checked is one that "+
			"could be written anywhere", path, err)
	}
	if parent != sandbox && !strings.HasPrefix(parent, sandbox+string(os.PathSeparator)) {
		t.Fatalf("%s resolves to %s, which is outside this case's sandbox at %s — nothing was run, because "+
			"what runs next writes executables", path, parent, sandbox)
	}
	return path
}

func writeFixture(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("building %s: %v — nothing was measured", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("writing %s: %v — nothing was measured", path, err)
	}
	// WriteFile leaves an existing file's mode alone, and one case rewrites a file it already wrote.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("setting the mode of %s: %v — nothing was measured", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v — nothing was measured", path, err)
	}
	return string(body)
}

// bash itself, found on the PATH this process was started with.
func bashOnThisMachine(t *testing.T) string {
	t.Helper()
	found, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("no bash on this machine (%v) — every script here is one, so nothing was measured", err)
	}
	return found
}
