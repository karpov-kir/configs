// Cases for source-stamp.sh. The coverage set is what must hold: a stamp blind to a file the tool
// compiles is a binary reported as current after that file changed.
//
// The set is every Go file in the module that is not a test, plus go.mod. A directory that cmd/ holds
// a main for looks like that tool's private source and can still be a library this tool imports. A
// subset that guesses wrong goes blind in silence, and covering too much costs only a rebuild the next
// run would have made anyway.
//
// Each row edits its own copy of the fixture. The stamp names its files relatively, so two copies of
// one module stamp alike wherever they sit, and the rows run at once with no edit to undo.
package reach

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The module shape this repository has. A tool with its own main, a tool whose main sits under cmd/
// with its library beside it, and a package both of them compile against.
const (
	ownMain   = tool
	cmdMain   = "gadget"
	sharedPkg = "common"
)

// The length of a stamp, so a case can tell one from the empty output of a script that failed.
const stampLength = 64

func TestTheStampCoversEveryNonTestSourceFileInTheModule(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	baseline := stampOf(t, newModule(t, sandbox, "baseline"), ownMain)
	if len(baseline) != stampLength {
		t.Fatalf("the fixture stamps to %q, which is not a digest — every comparison below would be "+
			"between two things that are not stamps", baseline)
	}

	for _, scenario := range []struct {
		name string
		// The edit this row makes to its own copy of the module.
		edit func(t *testing.T, module string)
		// Whether the stamp has to move. A row that leaves it alone is as much a claim as one that moves it.
		moves bool
	}{
		{
			name:  "an edit to the tool's own source moves the stamp",
			edit:  func(t *testing.T, module string) { appendLine(t, filepath.Join(toolsIn(module), ownMain, "main.go")) },
			moves: true,
		},
		{
			// A library that cmd/ backs is still one this tool may import. eco-report imports
			// tree-fingerprint, which cmd/tree-fingerprint also backs. A stamp that skipped it would serve
			// the old binary in silence.
			name: "an edit to a package outside the tool's own directory moves it",
			edit: func(t *testing.T, module string) {
				appendLine(t, filepath.Join(toolsIn(module), sharedPkg, sharedPkg+".go"))
			},
			moves: true,
		},
		{
			name:  "an edit to go.mod moves it",
			edit:  func(t *testing.T, module string) { appendLine(t, filepath.Join(module, "go.mod")) },
			moves: true,
		},
		{
			name: "an edit to another tool's main under cmd/ moves it",
			edit: func(t *testing.T, module string) {
				appendLine(t, filepath.Join(toolsIn(module), "cmd", cmdMain, "main.go"))
			},
			moves: true,
		},
		{
			// A digest over file contents alone would miss this, so the stamp folds in the names too.
			name: "a source file added to the tool moves it",
			edit: func(t *testing.T, module string) {
				writeFile(t, filepath.Join(toolsIn(module), ownMain, "added.go"), "package main\n\nvar Added = 1\n", 0o644)
			},
			moves: true,
		},
		{
			name: "a test file leaves it alone, since none of them reaches a binary",
			edit: func(t *testing.T, module string) {
				appendLine(t, filepath.Join(toolsIn(module), ownMain, ownMain+"_test.go"))
			},
			moves: false,
		},
		{
			// The control the other six rows rest on: an untouched copy stamps the same as the baseline.
			// A script answering a fresh number each run passes every "moves the stamp" row, and a script
			// printing an empty stamp passes every "leaves it alone" row.
			name:  "control: an untouched copy of the same source stamps the same",
			edit:  func(t *testing.T, module string) {},
			moves: false,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			module := newModule(t, newSandbox(t), "edited")
			scenario.edit(t, module)
			stamped := stampOf(t, module, ownMain)
			if moved := stamped != baseline; moved != scenario.moves {
				t.Errorf("this edit %s. A file the stamp cannot see is one a binary is reported as current "+
					"after it changed, and a file it sees that reaches no binary is a rebuild on every test "+
					"edit.\nbaseline %q\n   after %q", movement(moved), baseline, stamped)
			}
		})
	}
}

// A checkout kept inside the module, a linked worktree or a vendored clone, holds Go source that is
// not this module's. A walk of the filesystem cannot tell the two apart. This repository is developed
// on a machine carrying eight such worktrees, where that walk saw 758 source files against the 243 the
// repository tracks.

// `git ls-files` stops at a nested repository's edge, and that is what keeps them out. The second half
// of the case is the control. A stamp that read an empty file set would also leave the stamp still.
// The same stray file written into the module itself has to move it.
func TestSourceInsideANestedCheckoutStaysOutOfTheStamp(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	module := newModule(t, sandbox, "nesting")
	newRepository(t, module)
	alone := stampOf(t, module, ownMain)

	nested := filepath.Join(module, "worktrees", "inner")
	writeFile(t, filepath.Join(toolsIn(nested), ownMain, "main.go"), straySource, 0o644)
	newRepository(t, nested)
	if nesting := stampOf(t, module, ownMain); nesting != alone {
		t.Errorf("a checkout kept inside this one moved its stamp, so the source of every worktree a "+
			"developer keeps under their repository is being hashed as this module's\nalone %q\n  now %q",
			alone, nesting)
	}

	writeFile(t, filepath.Join(toolsIn(module), ownMain, "stray.go"), straySource, 0o644)
	if stray := stampOf(t, module, ownMain); stray == alone {
		t.Errorf("a source file added to the module left the stamp where it was, so the assertion above "+
			"holds against a stamp that reads nothing\nalone %q\n  now %q", alone, stray)
	}
}

// A file that is Go source. No case here builds it. It is written into the module and into the
// checkout nested inside it, so its location is the only difference between the two.
const straySource = "package main\n\nvar Stray = 1\n"

// A git repository at this path, with no file added to it. source-stamp.sh lists untracked files as
// well as tracked ones, so an init is all a fixture needs in order to be a checkout.
func newRepository(t *testing.T, dir string) {
	t.Helper()
	if output, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init in %s: %v\n%s — nothing was measured", dir, err, output)
	}
}

// The guard on the coverage set. A per-tool subset would make these two disagree, and such a subset
// goes wrong in silence: it drops a directory the tool really imports and stops noticing edits there.
// Whatever narrows the set again has to fail here first. With this case held, a row asked from one
// tool says the same about every other.
func TestEveryToolInTheModuleStampsTheSameSourceAlike(t *testing.T) {
	t.Parallel()
	module := newModule(t, newSandbox(t), "shared")
	if own, other := stampOf(t, module, ownMain), stampOf(t, module, cmdMain); own != other {
		t.Errorf("%s stamps %q and %s stamps %q, so the stamp is a guess at which files one tool compiles "+
			"— and a guess that drops a directory goes blind in silence", ownMain, own, cmdMain, other)
	}
}

// A release is stamped on Linux and read on macOS, on whichever of the two hashers that machine has. The
// stamp is a digest over their own output, so the whole scheme rests on the two writing a digest and a
// name identically. If that ever stops holding, every install warns about binaries that are perfectly
// current.
func TestShasumAndSha256sumStampTheSameSourceAlike(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"shasum", "sha256sum"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("this machine has no %s, so the two cannot be compared here", name)
		}
	}
	sandbox := newSandbox(t)
	module := newModule(t, sandbox, "hashers")

	withShasum := stampOn(t, module, newPathDir(t, sandbox, "only-shasum", append(stampCommands, "shasum")...))
	withSha256sum := stampOn(t, module, newPathDir(t, sandbox, "only-sha256sum", append(stampCommands, "sha256sum")...))
	if len(withShasum.stdout) == 0 || withShasum.code != 0 {
		t.Fatalf("shasum alone did not stamp, so the comparison below is against nothing\n%v", withShasum)
	}
	if withShasum.stdout != withSha256sum.stdout {
		t.Errorf("shasum stamped %q and sha256sum stamped %q over one source. A release stamped on the "+
			"machine that built it is read on the machine that installs it, so every install would warn "+
			"about binaries that are current", withShasum.stdout, withSha256sum.stdout)
	}
}

// Every way the stamp cannot be given, each exiting 2. A wrong stamp is worse than none: it reads as a
// stale binary on every run afterwards, and an empty one reads as a mismatch.
func TestEveryWayTheSourceCannotBeStampedExitsTwoAndNamesIt(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// The tool name asked for, and the PATH to ask it on.
		asked   []string
		path    func(t *testing.T, sandbox string) string
		refusal string
	}{
		{
			// A machine with neither hasher says so, and the refusal keeps an empty stamp off stdout.
			name:  "the machine has no way to hash a file",
			asked: []string{ownMain},
			path: func(t *testing.T, sandbox string) string {
				return newPathDir(t, sandbox, "no-hasher", stampCommands...)
			},
			refusal: "no shasum or sha256sum",
		},
		{
			// A mistyped name is refused, and the module's hash is what it would otherwise get.
			name:    "the name is not a tool in this module",
			asked:   []string{"absent"},
			refusal: "no source for absent",
		},
		{
			name:    "the name climbs out of the tools directory",
			asked:   []string{"../" + ownMain},
			refusal: "is not a tool name",
		},
		{
			name:    "the name is empty",
			asked:   []string{""},
			refusal: "is not a tool name",
		},
		{
			name:    "no name at all",
			refusal: "usage: source-stamp.sh",
		},
		{
			// The other half of the argument count: loosened to "at least one", every other case stays green.
			name:    "a second argument",
			asked:   []string{ownMain, "extra"},
			refusal: "usage: source-stamp.sh",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			module := newModule(t, sandbox, "refused")
			path := newStampPath(t, sandbox)
			if scenario.path != nil {
				path = scenario.path(t, sandbox)
			}
			refused := launch(t, newLaunch(t, filepath.Join(toolsIn(module), "source-stamp.sh"), path, scenario.asked...))
			expectRefusal(t, refused, scenario.refusal)
			if refused.stdout != "" {
				t.Errorf("the refusal printed %q on stdout, which resolve.sh would compare against a stamp",
					refused.stdout)
			}
		})
	}
}

// A module shaped like this repository's: go.mod at the root, every tool's source in the tools
// directory under it, and the stamper under test beside that source. The `_test.go` file is here
// because one row turns on its absence from the stamp.
func newModule(t *testing.T, sandbox, name string) string {
	t.Helper()
	module := sandboxed(t, sandbox, filepath.Join(sandbox, name))
	tools := toolsIn(module)
	writeFile(t, filepath.Join(module, "go.mod"), "module fixture\n\ngo 1.24\n", 0o644)
	writeFile(t, filepath.Join(tools, ownMain, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	writeFile(t, filepath.Join(tools, ownMain, ownMain+"_test.go"), "package main\n", 0o644)
	writeFile(t, filepath.Join(tools, "cmd", cmdMain, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	writeFile(t, filepath.Join(tools, cmdMain, cmdMain+".go"), "package "+cmdMain+"\n\nvar Value = 1\n", 0o644)
	writeFile(t, filepath.Join(tools, sharedPkg, sharedPkg+".go"), "package "+sharedPkg+"\n\nvar Value = 1\n", 0o644)
	writeFile(t, filepath.Join(tools, "source-stamp.sh"), read(t, runnable(t, stampScript)), 0o755)
	return module
}

// The stamp a module answers for one tool. The two ways it can fail to be a stamp are refused here,
// because a stamp read off a failed run is an empty string, and two of those agree.
func stampOf(t *testing.T, module, name string) string {
	t.Helper()
	sandbox := filepath.Dir(module)
	stamped := launch(t, newLaunch(t, filepath.Join(toolsIn(module), "source-stamp.sh"), newStampPath(t, sandbox), name))
	if stamped.code != 0 {
		t.Fatalf("stamping %s exited %d, so nothing this case compares is a stamp\n%v", name, stamped.code, stamped)
	}
	// Exactly one line, or resolve.sh compares a hash with a log line stuck to it.
	if strings.Count(stamped.stdout, "\n") != 1 || !strings.HasSuffix(stamped.stdout, "\n") {
		t.Fatalf("stdout carried %q rather than one line of stamp", stamped.stdout)
	}
	return strings.TrimSpace(stamped.stdout)
}

func stampOn(t *testing.T, module, path string) outcome {
	t.Helper()
	return launch(t, newLaunch(t, filepath.Join(toolsIn(module), "source-stamp.sh"), path, ownMain))
}

func newStampPath(t *testing.T, sandbox string) string {
	t.Helper()
	return newPathDir(t, sandbox, "stamp-path", append(stampCommands, hasher(t))...)
}

func appendLine(t *testing.T, path string) {
	t.Helper()
	writeFile(t, path, read(t, path)+"\nvar scratch = 2\n", 0o644)
}

func movement(moved bool) string {
	if moved {
		return "moved the stamp"
	}
	return "left the stamp where it was"
}
