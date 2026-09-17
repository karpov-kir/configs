// Cases for the `# --- shared:tool-stub ---` region every skill script carries to reach its Go binary.
// It is one body copied into several files, so it is covered once here rather than per copy; that the
// copies are still identical is the wiring check's shared-region scan, and that each stub's documented
// usage line matches what its binary prints is `ai/tools/stub_usage_test.go`.
//
// Two of these must not be weakened. Reaching the tool from an unrelated cwd is the defect the region
// exists for, and it fails silently: a stub that resolved nothing prints nothing, and a check that
// printed nothing reads exactly like a clean tree. And argv[0] surviving the exec is what decides which
// skill directory a tool writes into.
//
// The offset scan discovers the stubs rather than listing them, so the one written tomorrow is held
// without an edit here, and finding none is a failure — a scan over nothing is green for the wrong
// reason. It walks rather than asking git: a stub written and not yet added is read too, which is the
// moment it is easiest to leave one uncovered, and a walk cannot lose a name the way `git ls-files`
// does, which C-quotes any path holding a non-ASCII byte or a quote and hands back a name reaching no
// file. The walk is this file's own and not shared with stub_usage_test.go's: two scans that agree by
// construction would shrink together.
package reach

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The repository this package sits in, from ai/tools/reach.
const repoRoot = "../../.."

// Both fences, each as a whole line of its own. A file that only mentions the marker inside a string
// carries neither, which is what tells a stub from a file that talks about stubs.
const (
	regionOpen  = "# --- shared:tool-stub ---"
	regionClose = "# --- end shared:tool-stub ---"
	// What a stub declares its depth to be.
	offsetDeclaration = `tools_offset="`
)

// The stub the fixture cases copy, taken from the discovered set by name so that a stub which moved
// fails here rather than being quietly replaced by a fixture of this file's own.
const fixtureStub = "ai/kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh"

// One stub, and the depth it says it sits at.
type stub struct {
	path   string
	offset string
}

// The region resolves exactly one path — `$here/$tools_offset/tools/resolve.sh` — and consults nothing
// else, so a stub's depth is its own property and what is worth holding is that each one declares the
// depth it really has. An upward walk or a list of candidates would reach a tools directory the stub
// does not name, and both earlier shapes of this region did: either runs a stranger's binary at exit 0.
func TestEveryStubDeclaresTheOffsetThatReachesItsOwnToolsDirectory(t *testing.T) {
	t.Parallel()
	stubs := discoverStubs(t)

	// Two controls, because the loop below is satisfied by an empty set and by one that never leaves a
	// single directory — and the stubs sit at four depths in this tree.
	if len(stubs) < 5 {
		t.Fatalf("the walk found %d script(s) carrying %q, so this case asserts almost nothing. Either the "+
			"region was renamed and this scan has to follow it, or the walk is reaching the wrong tree",
			len(stubs), regionOpen)
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
			resolver := filepath.Join(repoRoot, filepath.Dir(found.path), found.offset, "tools", "resolve.sh")
			info, err := os.Stat(resolver)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				t.Errorf("%s declares tools_offset=%q, which resolves to %s — not an executable resolver "+
					"(%v). Every run of this stub exits 2 having run nothing", found.path, found.offset,
					resolver, err)
			}
		})
	}
}

// The defect the region exists for, at two of the depths the stubs sit at: a stub reaches its own tool
// from a directory that has nothing to do with the checkout it lives in. Driven through the real tree
// rather than a fixture, because a fixture proves only that a copy of the region works where this case
// put it.
func TestAStubReachesItsToolFromAnUnrelatedCwd(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		stub string
		args []string
		// Wording only the binary behind the stub writes. A stub that reached nothing prints nothing, and
		// nothing reads exactly like a clean tree.
		marker string
	}{
		{
			name:   "one four levels above the tools directory",
			stub:   fixtureStub,
			args:   []string{"@root@"},
			marker: "rule stated twice",
		},
		{
			name:   "one two levels above it",
			stub:   "ai/kk-flavor/scripts/model-policy.sh",
			args:   []string{"--help"},
			marker: "Emits the requested settings",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			// A root holding one restatement, so the tool has something to find: without content to find, a
			// stub that silently did nothing would satisfy the same assertion a working one does.
			root := newEchoRoot(t)
			arguments := append([]string(nil), scenario.args...)
			for i, argument := range arguments {
				arguments[i] = strings.ReplaceAll(argument, "@root@", root)
			}

			reached := launch(t, newToolLaunch(t, filepath.Join(repoRoot, scenario.stub), arguments...))
			if !reached.said(scenario.marker) {
				t.Errorf("%s did not reach its tool from a cwd with nothing to do with its checkout\n%v",
					scenario.stub, reached)
			}
			if reached.code == 2 {
				t.Errorf("%s exited 2, which means the tool never ran\n%v", scenario.stub, reached)
			}
		})
	}
}

// Each way the stub cannot reach a resolver. All of them exit 2 and name the fix, because a stub that
// returned quietly would hand the caller silence, and a caller reads silence as a clean tree.
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
				writeFile(t, filepath.Join(checkout, "tools", "resolve.sh"),
					read(t, runnable(t, resolveScript)), 0o644)
				return checkout
			},
			says: "chmod",
		},
		{
			// A decoy one level above the checkout root, which is where two earlier shapes of this region
			// reached. Asserted from both ends: the refusal names the resolver it could not find, AND the
			// decoy never ran. Either alone passes for the wrong reason — a stub that died before resolving
			// anything satisfies the second, and one that ran the decoy and then failed satisfies the first.
			name: "a tools directory sits outside the checkout",
			fixture: func(t *testing.T, sandbox string) string {
				escape, err := os.MkdirTemp(sandbox, "escape-")
				if err != nil {
					t.Fatalf("building the decoy fixture: %v — nothing was measured", err)
				}
				writeFile(t, filepath.Join(sandboxed(t, sandbox, escape), "tools", "resolve.sh"),
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
			refused := launch(t, newToolLaunch(t, filepath.Join(scenario.fixture(t, sandbox), stubIn(t)), newEchoRoot(t)))

			expectRefusal(t, refused, scenario.says)
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

// argv[0] survives the exec. The tools derive their skill directory from it, so a stub that let the
// binary's own path through would send every write to the tools directory instead. Proven with the
// ledger, because it is the one write whose destination is visible, and asserted from both ends: a run
// that wrote nowhere would satisfy the first half on its own.
func TestTheLedgerWriteLandsUnderTheSkillDirectoryTheStubWasInvokedBy(t *testing.T) {
	t.Parallel()
	const ledgerName = "stats.md"
	realLedger := filepath.Join(repoRoot, "ai", "kk-flavor", "skills", "kk-reduce", ledgerName)
	before := read(t, realLedger)

	// Mirrors the real layout, because stats.sh's declared offset is counted from
	// `kk-flavor/skills/<skill>/scripts/`. A shallower fixture puts the resolver out of its reach and the
	// case fails having tested the fixture rather than the ledger path.
	sandbox := newSandbox(t)
	fake := sandboxed(t, sandbox, filepath.Join(sandbox, "checkout"))
	writeFile(t, filepath.Join(fake, "tools", "resolve.sh"), read(t, runnable(t, resolveScript)), 0o755)
	stats := filepath.Join("kk-flavor", "skills", "kk-reduce", "scripts", "stats.sh")
	writeFile(t, filepath.Join(fake, stats),
		read(t, runnable(t, filepath.Join(repoRoot, "ai", stats))), 0o755)
	build(t, "eco-stats", filepath.Join(fake, "tools", "bin", "eco-stats"))

	// The real ai/ as the root to measure, named absolutely: the launch runs from a directory of its own,
	// and the tool would read a relative root against that one.
	root, err := filepath.Abs(filepath.Join(repoRoot, "ai"))
	if err != nil {
		t.Fatalf("resolving the tree to measure: %v — nothing was measured", err)
	}
	appended := launch(t, newToolLaunch(t, filepath.Join(fake, stats),
		"--agent=claude", "--append", "a row from the suite's own fixture", root))
	fixtureLedger := filepath.Join(fake, "kk-flavor", "skills", "kk-reduce", ledgerName)
	if appended.code != 0 || !appended.said(fixtureLedger) {
		t.Errorf("the append did not land under the skill directory the stub was invoked by, which is where "+
			"a skill reached through its mount symlink keeps its own ledger\n%v", appended)
	}
	if body, err := os.ReadFile(fixtureLedger); err != nil || len(body) == 0 {
		t.Errorf("nothing was written to %s (%v), so the comparison below would pass against a run that "+
			"wrote nowhere at all", fixtureLedger, err)
	}
	if read(t, realLedger) != before {
		t.Errorf("%s changed: the run wrote into the checkout it was reading, which is the incident every "+
			"fixture here is built under a sandbox to stop", realLedger)
	}
}

// Every script in the repository carrying the shared region, with the offset each one declares.
func discoverStubs(t *testing.T) []stub {
	t.Helper()
	var found []stub
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
		if !carriesRegion(string(body)) {
			return nil
		}
		relative, err := filepath.Rel(repoRoot, name)
		if err != nil {
			return err
		}
		found = append(found, stub{path: filepath.ToSlash(relative), offset: declaredOffset(string(body))})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s for stubs: %v — nothing was measured", repoRoot, err)
	}
	return found
}

func carriesRegion(body string) bool {
	open, closed := false, false
	for _, line := range strings.Split(body, "\n") {
		switch strings.TrimSpace(line) {
		case regionOpen:
			open = true
		case regionClose:
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

// A checkout holding one stub at the depth that stub declares, and nothing else. Built from the declared
// offset rather than from directory names written here: a fixture a level off puts the resolver out of
// the stub's reach, and the case then fails for the fixture's shape rather than for anything the stub
// did.
func newStubCheckout(t *testing.T, sandbox, name string) string {
	t.Helper()
	checkout := filepath.Join(sandbox, name)
	writeFile(t, filepath.Join(checkout, stubIn(t)), read(t, runnable(t, filepath.Join(repoRoot, fixtureStub))), 0o755)
	return checkout
}

// Where the fixture stub sits inside such a checkout: as many levels down as its own offset climbs.
func stubIn(t *testing.T) string {
	t.Helper()
	for _, found := range discoverStubs(t) {
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

// A root holding one restatement in two files, which is what rule-echo has to find.
func newEchoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	const rule = "**a shared rule stating several discriminating words plainly**\n"
	writeFile(t, filepath.Join(root, "a", "one.md"), "# A\n\n"+rule, 0o644)
	writeFile(t, filepath.Join(root, "b", "two.md"), "# B\n\n"+rule, 0o644)
	return root
}

// One tool, built where the caller wants it. The only real build in this package: what the case above
// measures is where the binary writes, so the binary has to be the real one.
//
// `./cmd/<tool>/` where the tool keeps its library apart and `./<tool>/` where its main sits with its
// code, which is resolve.sh's own rule. Build the library package by mistake and `-o` writes an archive
// — a file that is not executable, which the resolver then reports as a half-finished install.
func build(t *testing.T, name, into string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(into), 0o755); err != nil {
		t.Fatalf("building %s: %v — nothing was measured", filepath.Dir(into), err)
	}
	pkg := "./" + name + "/"
	if info, err := os.Stat(filepath.Join("..", "cmd", name)); err == nil && info.IsDir() {
		pkg = "./cmd/" + name + "/"
	}
	command := exec.Command("go", "build", "-o", into, pkg)
	command.Dir = ".."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s — nothing was measured", name, err, output)
	}
}

// A launch of a stub in the real tree, which is the one place here where a real toolchain may run: the
// resolver behind these stubs rebuilds whenever the checkout has moved on since the binary in bin/.
// HOME is still this case's own, and the build cache is still the machine's, because a `go build` given
// an empty cache compiles the standard library before it reaches the tool.
func newToolLaunch(t *testing.T, script string, arguments ...string) *exec.Cmd {
	t.Helper()
	command := newLaunch(t, script, os.Getenv("PATH"), arguments...)
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("finding this machine's cache directory: %v — nothing was measured", err)
	}
	command.Env = append(command.Env, "GOCACHE="+filepath.Join(cache, "go-build"))
	return command
}
