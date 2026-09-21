// Cases for resolve.sh. Three things here must hold.

// A binary is served only while it was built from the source beside it, and the comparison is over
// content. The binary that has to be caught is one NEWER than the source it disagrees with, and every
// downloaded release binary is.

// A binary from somewhere else, or one that cannot be compared at all, is served WITH a warning on
// stderr. Silence there is the defect.

// Every failure exits 2 and names what did not happen. These tools report findings, so exit 0 with
// none is what a clean tree looks like. A tool that failed to run must never reach a caller as
// silence.
package reach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The wording each arm of the warning uses. Three call sites read it from here.
const (
	builtElsewhere = "may not be the code you are reading"
	notCompared    = "could NOT be compared"
)

// The negative control the whole file rests on: with source and a toolchain, resolve.sh does reach a
// binary. Without it every refusal below would pass against a resolver that only ever exits 2.
//
// The second launch is the reason a release install needs no Go at all. The third shows that the
// build path works too, so the first branch is a preference.
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
	// The control every warning case rests on: a binary resolve.sh built from this source draws no
	// warning on the run after. Without it each of those passes against a resolver that warns about every
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
// reading exactly like a run against the edit. The assertion is on the binary's own bytes, because a
// resolver that served the stale one prints the same path this one does.
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

// The case that decides why the stamp is content, and why a timestamp will not do: a binary NEWER than
// the source it disagrees with. Every release install has that shape, because the asset lands long
// after the checkout it lands in. A timestamp comparison reads it as fresh, in silence, on the machine
// that has no toolchain to rebuild with and find out.
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
	backdate(t, filepath.Join(moduleIn(tools), "go.mod"))
	now := time.Now()
	if err := os.Chtimes(binary, now, now); err != nil {
		t.Fatalf("touching the fixture binary: %v — the newer-binary case was never set up", err)
	}
	// The control: the binary really is newer than every source beside it. An older binary would be caught
	// by a timestamp check too, and the case would stop being about content.
	newest := modified(t, binary)
	for _, source := range []string{filepath.Join(tools, tool, "main.go"), filepath.Join(moduleIn(tools), "go.mod")} {
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

// A binary that cannot be compared at all is served with that said. A check that did not run is a
// different thing from a clean one. Three ways the question goes unanswered, each of which a resolver
// could serve in silence and look identical doing it.
func TestAComparisonThatCouldNotBeMadeIsServedAndSaidSo(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// Builds the fixture and returns the tools directory, the name to resolve and the PATH to run on.
		fixture func(t *testing.T, sandbox string) (tools, name, path string)
		// The wording beyond the shared "could NOT be compared", where this cause names its own reason.
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
			// A stamper that runs and fails prints no hash. On its output alone that is byte for byte what a
			// matching stamp looks like, and its exit status is the only thing separating them. The fixture
			// uses a shim, because what has to be exercised is the stamper answering badly, and a broken tree
			// would exercise something else.
			name: "the stamper runs and fails",
			fixture: func(t *testing.T, sandbox string) (string, string, string) {
				tools := newToolsDir(t, sandbox, "bad-stamper-tools")
				placeBinary(t, tools, tool, foreignBinary, 0o755)
				writeFile(t, filepath.Join(tools, "source-stamp.sh"), failingStamper, 0o755)
				return tools, tool, newReleasePath(t, sandbox, "bad-stamper-path")
			},
		},
		{
			// An orphan: a binary from a tool since renamed, or one that no part of this repository put
			// there. bin/ is gitignored, so a diff and a `git status` both pass over it.

			// A resolver answering "built from this source" for it serves it at exit 0 in silence. That is
			// how a binary no human can account for keeps being exec'd over their repositories. The module
			// file is what tells an orphan from a checkout that ships no Go source. This fixture has one, and
			// the other case has none.
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

// A checkout that ships a binary and no Go source at all. There is no source to compare it against,
// and a warning on every run would be noise the machine's owner cannot act on. The orphan row is the
// other half of the pair: that case has a go.mod, and this one has none.
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

// A stale stamp beside new bytes is the defect this whole scheme exists to end, one layer in. The next
// run reads it as "built from this source" and serves without rebuilding. A stamp that cannot be
// written is removed, and the run after reports the gap.
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
			// A row of its own, because the fix differs. This is also the case that would otherwise read as
			// clean on every machine without Go.
			name: "the source is here and the machine has no toolchain",
			fixture: func(t *testing.T, sandbox string) (string, string) {
				return newToolsDir(t, sandbox, "no-toolchain"), newReleasePath(t, sandbox, "no-go")
			},
			says:     "go is not installed",
			alsoSays: "unchecked, not clean",
		},
		{
			// A half-finished install: the file arrived without its exec bit. resolve.sh reports it, because
			// building over it needs Go, and papering over it works on a developer's machine and fails on
			// the install's.
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
			// `-x` alone is true for a directory. Without the regular-file test this path reaches the caller
			// to exec.
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

// A tool name is a directory name here. Anything that can reach outside the tools directory, or name
// something other than a plain entry, is refused before it reaches a path. Each of these also fails to
// be a tool that exists, so the refusal has to be the name check's own. With that check deleted, the
// resolver still exits 2 on both, saying the checkout ships no source or binary.
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

// `--run` is the spelling every stub takes, and it owns what the stub region used to do line by line.
// Three properties at once, because a resolver that dropped any of them would still look like it
// worked.

// The binary is REPLACED into, so its own exit status is what a caller sees. Its arguments arrive whole
// and in order. stdout carries exactly what the tool printed, which is why the whole stream is
// asserted. A search would pass over an extra line. The fourth property, argv[0], is
// stub_reach_test.go's, for the reason reportingBinary, the fake tool const, states.
func TestRunExecsTheBinaryAndLeavesItsOutputAlone(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newSourcelessDir(t, sandbox, "run")
	placeBinary(t, tools, tool, reportingBinary, 0o755)

	ran := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newReleasePath(t, sandbox, "no-go"), "--run", tool, "/somewhere/else/widget.sh", "--flag", "a value"))

	if ran.code != reportingExit {
		t.Errorf("the caller saw exit %d where the binary exits %d, so resolve.sh answered instead of being "+
			"replaced by it\n%v", ran.code, reportingExit, ran)
	}
	if want := reportingMark + "argument=--flag\nargument=a value\n"; ran.stdout != want {
		t.Errorf("stdout is not the tool's own output alone — it belongs to the tool from the exec on, and a "+
			"path or a warning there is a line every caller of every stub has to learn to drop\nwant: %s%v",
			want, ran)
	}
}

// A tool invoked with no arguments at all is the common case here. An empty array under `set -u` is
// unbound in the bash macOS still ships as /bin/bash. Without the guard for it this is the launch that
// dies before the exec, and every other case passes while a bare stub stays broken.
func TestRunExecsTheBinaryWhenThereIsNothingToForward(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newSourcelessDir(t, sandbox, "run-bare")
	placeBinary(t, tools, tool, reportingBinary, 0o755)

	ran := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
		newReleasePath(t, sandbox, "no-go"), "--run", tool, "/somewhere/else/widget.sh"))

	if ran.code != reportingExit {
		t.Errorf("a tool invoked with no arguments did not reach its binary\n%v", ran)
	}
	if ran.stdout != reportingMark {
		t.Errorf("an argument reached the binary that no caller passed\n%v", ran)
	}
}

// Under `--run` a refusal is all the caller gets, so it has to be the same refusal print mode gives:
// exit 2, naming what did not happen. A resolver that exec'd something on this path, or exited 0 having
// run no tool, would hand every stub's caller a clean tree.
func TestRunRefusesTheSameWayAndNeverExecsAnything(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name  string
		asked []string
		says  string
	}{
		{
			name:  "the checkout ships neither a binary nor source",
			asked: []string{"--run", tool, "/somewhere/else/widget.sh"},
			says:  "ships neither",
		},
		{
			// argv[0] is required. Without it the resolver execs the binary under its own path, and every
			// tool then writes into ai/tools instead of its skill directory. `--run` with no tool either
			// is this same count check, `[ $# -ge 3 ]`, one argument further short.
			name:  "no argv0 to exec under",
			asked: []string{"--run", tool},
			says:  "usage: resolve.sh --run",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			tools := newSourcelessDir(t, sandbox, "refused")
			refused := launch(t, newLaunch(t, filepath.Join(tools, "resolve.sh"),
				newReleasePath(t, sandbox, "no-go"), scenario.asked...))
			expectRefusal(t, refused, scenario.says)
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
