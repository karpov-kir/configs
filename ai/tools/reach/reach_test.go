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
	"configs/ai/tools/runtest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// Clipped, so an append copies instead of writing into a slot this slice still owns. Two of the
// appends on it run on parallel cases.
//
// Twelve commands is safe on its own, since a slice at capacity reallocates. Seventeen is not: Go
// hands the append spare room, and two callers then write the same slot.
var resolveCommands = slices.Clip(append([]string{"cat", "mkdir", "mv", "rm", "rmdir", "sleep"}, stampCommands...))

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

// The first line reportingBinary, the fake tool const, prints. Cases hold the WHOLE of stdout against
// this and the arguments. Under `--run` that stream is the tool's, and a path or a warning on it is a
// line every caller of every stub would have to learn to drop.
const reportingMark = "the tool ran\n"

// The status reportingBinary, the fake tool const, leaves. Neither 0 nor 2, so "the binary's own status
// reached the caller" cannot be satisfied by resolve.sh succeeding or refusing.
const reportingExit = 7

// A command ready to launch, with an environment of its own. HOME sits under this case's own temp
// directory, because these scripts reach tools that read and write beneath it. PATH is named by the
// caller, because half the cases here are about a machine missing `go` or a way to hash a file.

// The working directory is an empty one, unknown to every part of the fixture. Every script here finds
// its own directory from `BASH_SOURCE`, and reaching one from cwd instead is the defect the stubs exist
// to stop. No case is given a cwd that could hide it.
// The script is bash's argument, and bash is what this execs. Linux refuses to exec a file any process
// holds open for writing, with ETXTBSY. These cases write their fixture scripts and run them, and one
// case's open descriptor reaches another case's fork. That failed the go job on the first push to
// main, and macOS has no such rule, so it passed here. bash opens the script to read.
func newLaunch(t *testing.T, script, path string, arguments ...string) *exec.Cmd {
	t.Helper()
	home := t.TempDir()
	command := exec.Command(runtest.Bash(t), append([]string{runtest.Runnable(t, script)}, arguments...)...)
	command.Dir = t.TempDir()
	command.Env = []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + path,
	}
	return command
}

// A binary served: exit 0, and stdout carrying that path alone. The whole of stdout, because a caller
// execs what it reads, and a build log or a warning on that stream is a string no shell can run.
func expectServed(t *testing.T, got runtest.Run, binary string) {
	t.Helper()
	if got.Code != 0 || got.Stdout != binary+"\n" {
		t.Errorf("wanted exit 0 and %q alone on stdout, which is what the caller execs\n%v", binary, got)
	}
}

// A directory every fixture is built under, resolved physically. macOS reaches a temp directory through
// a symlinked /var, and these scripts write executables.

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
	root := runtest.Sandboxed(t, sandbox, filepath.Join(sandbox, name))
	runtest.WriteFile(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.24\n", 0o644)
	dir := toolsIn(root)
	runtest.WriteFile(t, filepath.Join(dir, tool, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	copyScripts(t, dir)
	return dir
}

// A checkout that ships binaries and no Go source, which is the shape a skill mounted from a
// source-less checkout has. No go.mod above it either, which is what tells that shape apart from a
// checkout holding an orphan binary. The resolver is still here, or there would be no tool to run.
func newSourcelessDir(t *testing.T, sandbox, name string) string {
	t.Helper()
	dir := toolsIn(runtest.Sandboxed(t, sandbox, filepath.Join(sandbox, name)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("building the source-less fixture: %v — nothing was measured", err)
	}
	copyScripts(t, dir)
	return dir
}

func copyScripts(t *testing.T, dir string) {
	t.Helper()
	for _, script := range []string{resolveScript, stampScript} {
		runtest.WriteFile(t, filepath.Join(dir, filepath.Base(script)), runtest.ReadFile(t, runtest.Runnable(t, script)), 0o755)
	}
}

// A binary in the fixture's bin/, written straight to disk. resolve.sh serves any runnable regular file
// it finds there, so the cases about serving one need no toolchain at all.
func placeBinary(t *testing.T, tools, name, body string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(tools, "bin", name)
	runtest.WriteFile(t, path, body, mode)
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
	runtest.Sandboxed(t, sandbox, dir)
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
	runtest.WriteFile(t, filepath.Join(dir, "go"), toolchain, 0o755)
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
