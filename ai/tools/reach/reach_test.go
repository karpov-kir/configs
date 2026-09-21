// Cases for the layer that puts a Go binary within reach of a checkout holding none: `resolve.sh`,
// `source-stamp.sh` and `install.sh` beside this package. The `# --- shared:tool-stub ---` region every
// skill script carries to call the first of them is covered by `ai/tools/stub_reach_test.go` instead,
// for the reason that file's header gives. No case here reads the checkout at all. Every case builds
// its own fixture, so what one measures is the shape it declared, whatever tree it ran in.

// Those three stay shell. They run before there is a binary to run, and a Go build of them would have
// to execute before it could be executed. What each one measures is what a bash script did on a
// machine shaped a particular way, and Go can answer that only by running the script.

// Their cases live here for the reason `ai/tools/mcp_env_test.go`'s do. So the exec stays, and only the
// exec. Every fixture is built in process, every case runs in parallel, and a fake toolchain stands
// wherever the subject is resolve.sh's decision to build, and never Go's compiler.

// What that replaces: four shell suites, 280 assertions, 247 seconds and 1,887 commands, nearly all of
// it fixture plumbing. A real `go build` per staleness case, a 22-tool release per install case, and a
// process for each of the sourced-function rows. This package and the stub cases answer 134 cases in
// 469 commands, counted through the same PATH shim, and about half a minute.

// The six cases `--run` added cost four commands between them. The stub region they cover shed a `cat`
// and a `git rev-parse` per launch when the resolver took the exec over. The launches here paid for
// most of the new ones.

// The floor under that number is one launch per claim. What a script did on a machine shaped a
// particular way can only be measured by running it there. The rest of each launch's cost is the
// commands the script itself calls.
package reach

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The scripts under test, beside the rest of `ai/tools`.
const (
	resolveScript = "../resolve.sh"
	stampScript   = "../source-stamp.sh"
	installScript = "../install.sh"
)

// The tool every fixture ships source for. A directory name is all a tool is to these scripts, so the
// name itself carries no meaning.
const tool = "widget"

// Exactly what source-stamp.sh calls, and no spare entry. An unused entry would let a new dependency
// land in the script without a case going red. It would also skip a case on a machine the script runs
// fine on.

// `bash` is here because the shebang is `#!/usr/bin/env bash` and env looks it up on PATH. The hasher
// is chosen per machine, so the fixture adds it at build time. Both `git` and `find` belong here,
// because the script asks git for its file list inside a checkout and walks a tree with no checkout.
var stampCommands = []string{"bash", "dirname", "find", "git", "sort", "cut"}

// What resolve.sh calls on top of those. It runs source-stamp.sh, so it needs all of them as well.
// `mkdir`, `rmdir` and `sleep` are the build lock: the directory is the mutex, and a run queued behind
// another build of the same tool sleeps between attempts at it.
var resolveCommands = append([]string{"cat", "mkdir", "mv", "rm", "rmdir", "sleep"}, stampCommands...)

// A toolchain that writes the file `go build -o` names and compiles no source. What the cases here
// turn on is resolve.sh's decision to build and what it does with the result. Whether Go can compile a
// fixture is a separate question, and a real build per case is most of the minute the shell suite took.

// The pid goes into the bytes, so "this binary was rewritten" is readable from the file itself and
// never from its mtime.

// It prints on stdout, because resolve.sh sends the build's own chatter to stderr and keeps stdout for
// the path a caller execs. A toolchain that printed no line would let that redirect be deleted with
// every case still green.
const fakeToolchain = `#!/bin/sh
printf 'fake toolchain: building %s\n' "$*"
out=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "-o" ]; then out="$argument"; fi
  previous="$argument"
done
[ -n "$out" ] || exit 1
printf '#!/bin/sh\nexit 0\n# built by pid %s\n' "$$" >"$out"
chmod 755 "$out"
`

// A toolchain that refuses, standing for source that does not compile. Its message is its own, so the
// case reading it is not reading Go's wording.
const failingToolchain = `#!/bin/sh
echo 'fake toolchain: this source does not compile' >&2
exit 1
`

// A stamper that runs and fails, printing no line. On stdout alone that is byte for byte what a stamp
// that could not be computed looks like, so its exit status is the only thing telling the two apart.
const failingStamper = "#!/bin/sh\nexit 1\n"

// An executable file standing in for a binary somebody else built — a release asset, or a tool since
// renamed. resolve.sh serves any runnable regular file in bin/, so a fixture needs no build to have one.
const foreignBinary = "#!/bin/sh\nexit 0\n"

// A binary that says how it was launched, for the `--run` cases. One line of its own, its arguments
// one per line, and an exit status no success or refusal of resolve.sh's shares.

// It cannot report its argv[0], and no fixture here can. `exec -a NAME file` puts NAME in the execve
// argv, and the kernel then hands a `#!` file to its interpreter as `sh file …`, dropping NAME on the
// way. A real binary does see it. That half is `ai/tools/stub_reach_test.go`'s ledger case: a whole
// stub, a real Go binary, and the write whose destination argv[0] decides.
const reportingBinary = `#!/bin/sh
printf 'the tool ran\n'
for argument in "$@"; do printf 'argument=%s\n' "$argument"; done
exit 7
`

// The first line reportingBinary prints. Cases hold the WHOLE of stdout against this and the arguments.
// Under `--run` that stream is the tool's, and a path or a warning on it is a line every caller of
// every stub would have to learn to drop.
const reportingMark = "the tool ran\n"

// The status reportingBinary leaves. Neither 0 nor 2, so "the binary's own status reached the caller"
// cannot be satisfied by resolve.sh succeeding or refusing.
const reportingExit = 7

// What one launch of a script came back with. stdout and stderr are kept apart, because several cases
// turn on a warning being audible on stderr. stdout carries the path a caller execs, and no other line.
type outcome struct {
	stdout string
	stderr string
	code   int
}

// Whether either stream holds the wording. A refusal is asserted on what it says, and never on its
// exit code alone. Every refusal these scripts have exits 2, so the code says one happened and leaves
// the cause open. A case reading the code alone passes on whatever the fixture broke first.
func (o outcome) said(wording string) bool {
	return strings.Contains(o.stdout, wording) || strings.Contains(o.stderr, wording)
}

func (o outcome) String() string {
	return fmt.Sprintf("exit %d\nstdout: %s\nstderr: %s", o.code, o.stdout, o.stderr)
}

// A command ready to launch, with an environment of its own. HOME sits under this case's own temp
// directory, because these scripts reach tools that read and write beneath it. PATH is named by the
// caller, because half the cases here are about a machine missing `go` or a way to hash a file.

// The working directory is an empty one, unknown to every part of the fixture. Every script here finds
// its own directory from `BASH_SOURCE`, and reaching one from cwd instead is the defect the stubs exist
// to stop. No case is given a cwd that could hide it.
func newLaunch(t *testing.T, script, path string, arguments ...string) *exec.Cmd {
	t.Helper()
	home := t.TempDir()
	command := exec.Command(runnable(t, script), arguments...)
	command.Dir = t.TempDir()
	command.Env = []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + path,
	}
	return command
}

// Asserts a refusal carries the wording only its own cause produces. Every refusal these scripts have
// exits 2, so the code says one happened and leaves the cause open. A case asserting the code alone
// passes on whatever the fixture broke first while its name claims the cause. `command not found` is a
// stripped PATH killing the script before it reaches any check at all, which no case here ever means.
func expectRefusal(t *testing.T, got outcome, wording string) {
	t.Helper()
	if got.code != 2 {
		t.Errorf("wanted exit 2 and the refusal %q\n%v", wording, got)
		return
	}
	if got.said("command not found") || got.said(": not found") {
		t.Errorf("a missing command produced this refusal, not %q — the PATH fixture is short of something "+
			"the script calls, so this case measured that instead\n%v", wording, got)
		return
	}
	if !got.said(wording) {
		t.Errorf("the refusal does not say %q, so a caller cannot tell this cause from the others that also "+
			"exit 2\n%v", wording, got)
	}
}

// A binary served: exit 0, and stdout carrying that path alone. The whole of stdout, because a caller
// execs what it reads, and a build log or a warning on that stream is a string no shell can run.
func expectServed(t *testing.T, got outcome, binary string) {
	t.Helper()
	if got.code != 0 || got.stdout != binary+"\n" {
		t.Errorf("wanted exit 0 and %q alone on stdout, which is what the caller execs\n%v", binary, got)
	}
}

// One launch. A script that could not be started at all is fatal here. Every case here is a launch,
// and a failed case would otherwise report a reason far from the guard it names.
func launch(t *testing.T, command *exec.Cmd) outcome {
	t.Helper()
	var out, err strings.Builder
	command.Stdout, command.Stderr = &out, &err
	result := outcome{}
	var exit *exec.ExitError
	switch runErr := command.Run(); {
	case runErr == nil:
	case errors.As(runErr, &exit):
		result.code = exit.ExitCode()
	default:
		t.Fatalf("could not run %s: %v — nothing was measured", command.Path, runErr)
	}
	result.stdout, result.stderr = out.String(), err.String()
	return result
}

// The path to a script, refused loudly where it cannot be run.
func runnable(t *testing.T, script string) string {
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

// A directory every fixture is built under, resolved physically. macOS reaches a temp directory through
// a symlinked /var, and these scripts write executables.

// The incident behind this: a harness bug once handed every case the same HOME, followed a live symlink
// into the checkout, and overwrote real config files in the working tree. The suite reported it, and
// the report was read as a harness bug. What the run had already written went unasked. The resolution
// happens here, so sandboxed() can refuse a path before anything is written to it.
func newSandbox(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving this case's temp directory: %v — nothing was measured", err)
	}
	return dir
}

// A fixture path, refused unless it really lies inside the sandbox. Callers reach it before the
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

// Where the scripts sit inside a fixture checkout, at the same depth under the module root that
// `ai/tools` has in this repository. The depth is part of the subject here. Both scripts reach go.mod
// by a declared offset. A fixture holding go.mod beside them would measure a shape the repository
// never ships, and stay green over an offset that points nowhere.
func toolsIn(root string) string {
	return filepath.Join(root, "ai", "tools")
}

// The module root above a fixture's tools directory — what that declared offset reaches.
func moduleIn(tools string) string {
	return filepath.Dir(filepath.Dir(tools))
}

// A checkout shaped like the real one: go.mod at its root, and under it a tools directory holding both
// scripts under test and one tool's source. Copied and never linked, because resolve.sh resolves its own
// directory physically — through a link it would find the real ai/tools and build into the checkout's
// own bin/.
func newToolsDir(t *testing.T, sandbox, name string) string {
	t.Helper()
	root := sandboxed(t, sandbox, filepath.Join(sandbox, name))
	writeFile(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.24\n", 0o644)
	dir := toolsIn(root)
	writeFile(t, filepath.Join(dir, tool, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	copyScripts(t, dir)
	return dir
}

// A checkout that ships binaries and no Go source, which is the shape a skill mounted from a
// source-less checkout has. No go.mod above it either, which is what tells that shape apart from a
// checkout holding an orphan binary. The resolver is still here, or there would be no tool to run.
func newSourcelessDir(t *testing.T, sandbox, name string) string {
	t.Helper()
	dir := toolsIn(sandboxed(t, sandbox, filepath.Join(sandbox, name)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("building the source-less fixture: %v — nothing was measured", err)
	}
	copyScripts(t, dir)
	return dir
}

func copyScripts(t *testing.T, dir string) {
	t.Helper()
	for _, script := range []string{resolveScript, stampScript} {
		writeFile(t, filepath.Join(dir, filepath.Base(script)), read(t, runnable(t, script)), 0o755)
	}
}

// A binary in the fixture's bin/, written straight to disk. resolve.sh serves any runnable regular file
// it finds there, so the cases about serving one need no toolchain at all.
func placeBinary(t *testing.T, tools, name, body string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(tools, "bin", name)
	writeFile(t, path, body, mode)
	return path
}

// A PATH holding only the commands named. Symlinks to the real binaries, so what runs is the real thing
// anywhere a case has not put a shim beside them. The directory's name is made unique, because several
// cases build two of these and a collision would leave one case running on the other's PATH.
func newPathDir(t *testing.T, sandbox, name string, commands ...string) string {
	t.Helper()
	dir, err := os.MkdirTemp(sandbox, name+"-")
	if err != nil {
		t.Fatalf("building the PATH fixture under %s: %v — nothing was measured", sandbox, err)
	}
	sandboxed(t, sandbox, dir)
	for _, command := range commands {
		real, err := exec.LookPath(command)
		if err != nil {
			t.Fatalf("this machine has no %s, and both scripts call it — nothing was measured", command)
		}
		if err := os.Symlink(real, filepath.Join(dir, command)); err != nil {
			t.Fatalf("linking %s into the PATH fixture: %v — nothing was measured", command, err)
		}
	}
	return dir
}

// A PATH with everything the staleness check needs and no `go`. This stands for a machine that
// installed a release, and the POSIX utilities are all still there. Strip the hasher too and
// resolve.sh takes the "could not compare" branch instead of the branch most cases here are about.
func newReleasePath(t *testing.T, sandbox, name string) string {
	t.Helper()
	return newPathDir(t, sandbox, name, append(resolveCommands, hasher(t))...)
}

// The same PATH with a toolchain on it. `chmod` comes with it because the fake toolchain marks what it
// writes executable. The real toolchain does that for itself.
func newBuildPath(t *testing.T, sandbox, name, toolchain string) string {
	t.Helper()
	dir := newPathDir(t, sandbox, name, append(resolveCommands, hasher(t), "chmod")...)
	writeFile(t, filepath.Join(dir, "go"), toolchain, 0o755)
	return dir
}

// Whichever SHA-256 tool this machine has, the way source-stamp.sh chooses it.
func hasher(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"shasum", "sha256sum"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	t.Fatalf("this machine has neither shasum nor sha256sum, so no stamp could be computed and nothing " +
		"below was measured")
	return ""
}

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("building %s: %v — nothing was measured", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("writing %s: %v — nothing was measured", path, err)
	}
	// WriteFile leaves an existing file's mode alone, and several cases rewrite one.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("setting the mode of %s: %v — nothing was measured", path, err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v — nothing was measured", path, err)
	}
	return string(body)
}
