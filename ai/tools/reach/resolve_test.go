// Cases for resolve.sh. Three of them must not be weakened.
//
// A binary is served only while it was built from the source beside it, and the comparison is over
// content: the binary that has to be caught is one NEWER than the source it disagrees with, which is
// what every downloaded release binary is.
//
// A binary from somewhere else, or one that cannot be compared at all, is served WITH a warning on
// stderr. Silence there is the defect.
//
// And every failure exits 2 and names what did not happen. These tools report findings, so exit 0 with
// none is what a clean tree looks like, and a tool that could not run must never reach a caller as
// silence.
package reach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The wording each arm of the warning uses, read here rather than restated at three call sites.
const (
	builtElsewhere = "may not be the code you are reading"
	notCompared    = "could NOT be compared"
)

// The negative control the whole file rests on: with source and a toolchain, resolve.sh does reach a
// binary. Without it every refusal below would pass against a resolver that only ever exits 2.
//
// The second launch is the reason a release install needs no Go at all, and the third is what keeps the
// first branch a preference rather than the only path.
func TestABinaryIsBuiltIntoBinAndThenServedFromThereWithNoToolchain(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "built")
	binary := filepath.Join(tools, "bin", tool)

	built := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newBuildPath(t, sandbox, "with-go", fakeToolchain), tool))
	expectServed(t, built, binary)
	if !strings.Contains(built.stderr, "fake toolchain") {
		t.Errorf("the build's own output is not on stderr, so it went to the stream the caller execs\n%v", built)
	}
	if info, err := os.Stat(binary); err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not an executable file after the build (%v), so the launches below say nothing "+
			"about serving one", binary, err)
	}

	release := newReleasePath(t, sandbox, "no-go")
	served := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), release, tool))
	expectServed(t, served, binary)
	// The control every warning case rests on: a binary resolve.sh built from this source says nothing at
	// all on the run after. Without it each of those passes against a resolver that warns about every
	// binary it serves.
	if served.stderr != "" {
		t.Errorf("a binary built from the source beside it was served with a warning, so the cases below "+
			"cannot tell a warned binary from any other\nstderr: %s", served.stderr)
	}

	override := newLaunch(t, filepath.Join(tools, "resolve.sh"), release, tool)
	override.Env = append(override.Env, "ECO_TOOLS_BUILD=1")
	skipped := launch(t, override)
	expectRefusal(t, skipped, "go is not installed")
	if !skipped.said("did NOT run") {
		t.Errorf("ECO_TOOLS_BUILD=1 refused without saying the tool did not run, and a caller reads a "+
			"quiet refusal as a clean tree\n%v", skipped)
	}
}

// What this catches: someone edits the Go, and every run afterwards measures the previous build while
// reading exactly like a run against the edit. Asserted on the binary's own bytes, because a resolver
// that served the stale one prints the same path this one does.
func TestAnEditedSourceIsRebuiltRatherThanServedFromTheBuildBeforeIt(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "edited")
	binary := filepath.Join(tools, "bin", tool)
	path := newBuildPath(t, sandbox, "with-go", fakeToolchain)

	expectServed(t, launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), path, tool)), binary)
	first := read(t, binary)

	writeFile(t, filepath.Join(tools, tool, "main.go"), `package main

func main() { _ = "edited" }
`, 0o644)
	expectServed(t, launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), path, tool)), binary)
	if read(t, binary) == first {
		t.Errorf("%s holds the same bytes after an edit to the source it was built from, so the edit is "+
			"being measured through the build that came before it", binary)
	}
}

// The case that decides why the stamp is content and not a timestamp: a binary NEWER than the source it
// disagrees with. Every release install has that shape, because the asset lands long after the checkout
// it lands in, and a timestamp comparison reads it as fresh — silently, on the very machine that has no
// toolchain to rebuild with and find out.
func TestABinaryNewerThanTheSourceItDisagreesWithIsServedWithTheDoubtOnStderr(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "newer")
	binary := filepath.Join(tools, "bin", tool)
	expectServed(t, launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newBuildPath(t, sandbox, "with-go", fakeToolchain), tool)), binary)

	writeFile(t, filepath.Join(tools, tool, "main.go"), `package main

func main() { _ = "edited" }
`, 0o644)
	backdate(t, filepath.Join(tools, tool, "main.go"))
	backdate(t, filepath.Join(tools, "go.mod"))
	now := time.Now()
	if err := os.Chtimes(binary, now, now); err != nil {
		t.Fatalf("touching the fixture binary: %v — the newer-binary case was never set up", err)
	}
	// The control: the binary really is newer than every source beside it. Were it not, a timestamp check
	// would catch this case too and the case would no longer be about content at all.
	newest := modified(t, binary)
	for _, source := range []string{filepath.Join(tools, tool, "main.go"), filepath.Join(tools, "go.mod")} {
		if !modified(t, source).Before(newest) {
			t.Fatalf("%s is not older than the binary, so this case would pass against a resolver comparing "+
				"timestamps and says nothing about content", source)
		}
	}

	served := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newReleasePath(t, sandbox, "no-go"), tool))
	expectServed(t, served, binary)
	if !strings.Contains(served.stderr, builtElsewhere) {
		t.Errorf("a binary built from something other than the source beside it was served without saying "+
			"so, and on a machine with no toolchain nothing else will ever say it\n%v", served)
	}
}

// A binary that cannot be compared at all is served with that said, because a check that did not run is
// not a clean one. Three ways the question goes unanswered, each of which a resolver could serve in
// silence and look identical doing it.
func TestAComparisonThatCouldNotBeMadeIsServedAndSaidSo(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// Builds the fixture and returns the tools directory, the name to resolve and the PATH to run on.
		fixture func(t *testing.T, sandbox string) (tools, name, path string)
		// Wording beyond the shared "could NOT be compared", where this cause names its own reason.
		also string
	}{
		{
			name: "nothing on the machine can hash the source",
			fixture: func(t *testing.T, sandbox string) (string, string, string) {
				tools := newToolsDir(t, sandbox, "no-hasher-tools")
				placeBinary(t, tools, tool, foreignBinary, 0o755)
				return tools, tool, newPathDir(t, sandbox, "no-hasher", resolveCommands...)
			},
			also: "no shasum or sha256sum",
		},
		{
			// A stamper that runs and fails prints no hash, and on its output alone that is byte for byte
			// what a matching stamp looks like: its exit status is the only thing separating them. A shim
			// rather than a broken tree, because what has to be exercised is the stamper answering badly.
			name: "the stamper runs and fails",
			fixture: func(t *testing.T, sandbox string) (string, string, string) {
				tools := newToolsDir(t, sandbox, "bad-stamper-tools")
				placeBinary(t, tools, tool, foreignBinary, 0o755)
				writeFile(t, filepath.Join(tools, "source-stamp.sh"), failingStamper, 0o755)
				return tools, tool, newReleasePath(t, sandbox, "bad-stamper-path")
			},
		},
		{
			// An orphan: a binary from a tool since renamed, or one nothing here put there. bin/ is
			// gitignored, so it appears in no diff and no `git status`, and answering "built from this
			// source" for it serves it at exit 0 in silence — the one way a binary nobody can account for
			// keeps being exec'd over the human's repositories. What says it is an orphan rather than a
			// source-less checkout is the module file, which this fixture has and the case below does not.
			name: "the checkout ships source and none of it is this tool's",
			fixture: func(t *testing.T, sandbox string) (string, string, string) {
				tools := newToolsDir(t, sandbox, "orphan-tools")
				placeBinary(t, tools, "stowaway", foreignBinary, 0o755)
				return tools, "stowaway", newReleasePath(t, sandbox, "orphan-path")
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			tools, name, path := scenario.fixture(t, sandbox)
			served := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), path, name))
			expectServed(t, served, filepath.Join(tools, "bin", name))
			if !strings.Contains(served.stderr, notCompared) {
				t.Errorf("the binary was served as proven when nothing compared it with the source, and a "+
					"check that did not run is not a clean one\n%v", served)
			}
			if scenario.also != "" && !served.said(scenario.also) {
				t.Errorf("the refusal does not name what was missing (%q), so nobody on that machine can "+
					"act on it\n%v", scenario.also, served)
			}
		})
	}
}

// A checkout that ships a binary and no Go source at all: there is nothing to compare it against, and
// saying so on every run would be noise nobody on that machine could act on. The pair with the orphan
// row above is what separates the two shapes — this one has no go.mod, that one has.
func TestABinaryWithNoSourceBesideItWarnsAboutNothing(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newSourcelessDir(t, sandbox, "sourceless")
	binary := placeBinary(t, tools, tool, foreignBinary, 0o755)

	served := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newReleasePath(t, sandbox, "no-go"), tool))
	expectServed(t, served, binary)
	if served.stderr != "" {
		t.Errorf("a checkout with no source to compare against was warned about, and a warning nobody can "+
			"act on is one that stops being read\nstderr: %s", served.stderr)
	}
}

// A stale stamp beside new bytes is the defect this whole scheme exists to end, reintroduced one layer
// in: the next run reads it as "built from this source" and serves without rebuilding. So a stamp that
// cannot be written is removed rather than left, and the run after reports the gap.
func TestABuildWhoseStamperFailsLeavesNoStampRatherThanThePreviousBuilds(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "stamp-write")
	binary := filepath.Join(tools, "bin", tool)
	build := newBuildPath(t, sandbox, "with-go", fakeToolchain)
	expectServed(t, launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), build, tool)), binary)

	// The control: a build that can stamp does, and what it writes is the stamp of the source it built.
	stamp := binary + ".stamp"
	stamped := launch(t, newLaunch(t, filepath.Join(tools, "source-stamp.sh"), build, tool))
	if stamped.code != 0 || strings.TrimSpace(read(t, stamp)) != strings.TrimSpace(stamped.stdout) {
		t.Fatalf("the build wrote %q where the source stamps to %q, so the case below cannot tell a stamp "+
			"that was removed from one that was never written", read(t, stamp), stamped.stdout)
	}

	writeFile(t, filepath.Join(tools, "source-stamp.sh"), failingStamper, 0o755)
	rebuild := newLaunch(t, filepath.Join(tools, "resolve.sh"), build, tool)
	rebuild.Env = append(rebuild.Env, "ECO_TOOLS_BUILD=1")
	expectServed(t, launch(t, rebuild), binary)
	if _, err := os.Stat(stamp); err == nil {
		t.Errorf("%s still holds %q after a build that could not stamp what it wrote. The next run reads "+
			"that as proof these bytes came from this source and serves them unbuilt", stamp, read(t, stamp))
	}

	served := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newReleasePath(t, sandbox, "no-go"), tool))
	expectServed(t, served, binary)
	if !strings.Contains(served.stderr, notCompared) {
		t.Errorf("the run after an unstamped build served those bytes as proven\n%v", served)
	}
}

// Every way the tool fails to be reached, each exiting 2 and naming what did not happen. A resolver
// that returned quietly here would turn every unreachable tool into a clean bill of health.
func TestEveryWayTheToolCannotBeReachedExitsTwoAndSaysItDidNotRun(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		fixture func(t *testing.T, sandbox string) (tools, path string)
		// The wording only this cause produces, then a second one the case would read as a near miss.
		says     string
		alsoSays string
		// What the refusal has to have left behind, where that is part of the claim.
		after func(t *testing.T, tools string)
	}{
		{
			// The shape a skill mounted from an incomplete checkout has. One assertion over both halves,
			// because a refusal naming only the binary reads as a bad install when the fix is a whole
			// checkout.
			name: "the checkout ships neither a binary nor source",
			fixture: func(t *testing.T, sandbox string) (string, string) {
				return newSourcelessDir(t, sandbox, "empty"), newReleasePath(t, sandbox, "no-go")
			},
			says:     "ships neither",
			alsoSays: "no source at",
		},
		{
			// Distinct from the row above because the fix is different, and because this is the one that
			// would otherwise read as clean on every machine without Go.
			name: "the source is here and the machine has no toolchain",
			fixture: func(t *testing.T, sandbox string) (string, string) {
				return newToolsDir(t, sandbox, "no-toolchain"), newReleasePath(t, sandbox, "no-go")
			},
			says:     "go is not installed",
			alsoSays: "unchecked, not clean",
		},
		{
			// A half-finished install: the file arrived without its exec bit. Reported and not built over,
			// because building needs Go — papering over it works on a developer's machine and fails on the
			// install's.
			name: "the binary arrived without its exec bit",
			fixture: func(t *testing.T, sandbox string) (string, string) {
				tools := newToolsDir(t, sandbox, "not-executable")
				placeBinary(t, tools, tool, foreignBinary, 0o644)
				return tools, newBuildPath(t, sandbox, "with-go", fakeToolchain)
			},
			says:     "is not executable",
			alsoSays: "did not complete",
		},
		{
			// `-x` alone is true for a directory, so without the regular-file test this path would be
			// handed to the caller to exec.
			name: "a directory sits where the binary goes",
			fixture: func(t *testing.T, sandbox string) (string, string) {
				tools := newToolsDir(t, sandbox, "directory")
				if err := os.MkdirAll(filepath.Join(tools, "bin", tool), 0o755); err != nil {
					t.Fatalf("building the directory fixture: %v — nothing was measured", err)
				}
				return tools, newBuildPath(t, sandbox, "with-go", fakeToolchain)
			},
			says: "not a regular file",
		},
		{
			// The branch that would otherwise print a path to a binary that was never written.
			name: "the source is here and does not build",
			fixture: func(t *testing.T, sandbox string) (string, string) {
				return newToolsDir(t, sandbox, "broken-source"),
					newBuildPath(t, sandbox, "failing-go", failingToolchain)
			},
			says:     "did not build",
			alsoSays: "did NOT run",
			after: func(t *testing.T, tools string) {
				if _, err := os.Stat(filepath.Join(tools, "bin", tool)); err == nil {
					t.Errorf("a build that failed left %s behind, and the run after would serve it as the "+
						"tool", filepath.Join(tools, "bin", tool))
				}
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			tools, path := scenario.fixture(t, sandbox)
			refused := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), path, tool))
			expectRefusal(t, refused, scenario.says)
			if scenario.alsoSays != "" && !refused.said(scenario.alsoSays) {
				t.Errorf("the refusal does not also say %q, which is the half telling a caller what it "+
					"costs\n%v", scenario.alsoSays, refused)
			}
			if scenario.after != nil {
				scenario.after(t, tools)
			}
		})
	}
}

// A tool name is a directory name here, so anything that could climb out of the tools directory or name
// something other than a plain entry is refused before it reaches a path. Each of these also fails to be
// a tool that exists, so the refusal has to be the name check's own: with that check deleted the
// resolver still exits 2 on both, saying the checkout ships neither source nor binary.
func TestANameThatCouldBecomeSomethingOtherThanADirectoryIsRefusedAsAName(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		asked   []string
		refusal string
	}{
		{name: "a name that climbs out of the tools directory", asked: []string{"../" + tool},
			refusal: "is not a tool name"},
		{name: "an empty name", asked: []string{""}, refusal: "is not a tool name"},
		{name: "no name at all", refusal: "usage: resolve.sh"},
		// The other half of the argument count. Without it, a check loosened to "at least one" keeps every
		// other case here green.
		{name: "a second argument", asked: []string{tool, "extra"}, refusal: "usage: resolve.sh"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			tools := newToolsDir(t, sandbox, "names")
			refused := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
				newBuildPath(t, sandbox, "with-go", fakeToolchain), scenario.asked...))
			expectRefusal(t, refused, scenario.refusal)
		})
	}
}

// Older than anything a case writes, so the binary can be made newer than every source beside it.
func backdate(t *testing.T, path string) {
	t.Helper()
	when := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("backdating %s: %v — the case that needs it was never set up", path, err)
	}
}

func modified(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v — nothing was measured", path, err)
	}
	return info.ModTime()
}
