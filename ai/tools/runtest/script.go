package runtest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run is what one launch of a script came back with. The two streams are kept apart, because a
// refusal has to be audible on stderr and a caller reads its answer off stdout.
type Run struct {
	Stdout string
	Stderr string
	Code   int
}

// Said asks whether either stream holds the wording. A refusal is asserted on what it says. Several
// refusals share exit 2, so a case reading the code passes on whatever the fixture broke first.
func (r Run) Said(wording string) bool {
	return strings.Contains(r.Stdout, wording) || strings.Contains(r.Stderr, wording)
}

func (r Run) String() string {
	return fmt.Sprintf("exit %d\nstdout: %s\nstderr: %s", r.Code, r.Stdout, r.Stderr)
}

// Launch runs a prepared command and collects both streams. A non-zero exit is an answer the case
// reads. A command that failed to start stops the case, which has then measured no script.
func Launch(t *testing.T, command *exec.Cmd) Run {
	t.Helper()
	var out, err strings.Builder
	command.Stdout, command.Stderr = &out, &err
	result := Run{}
	var exit *exec.ExitError
	switch runErr := command.Run(); {
	case runErr == nil:
	case errors.As(runErr, &exit):
		result.Code = exit.ExitCode()
	default:
		t.Fatalf("could not run %s: %v — nothing was measured", command.Path, runErr)
	}
	result.Stdout, result.Stderr = out.String(), err.String()
	return result
}

// Bash is this machine's bash, found on PATH. The interpreter takes the script as a path, and that is
// also what keeps Linux from answering ETXTBSY for a file some process still holds open.
func Bash(t *testing.T) string {
	t.Helper()
	found, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("no bash on this machine (%v) — every script here is one, so nothing was measured", err)
	}
	return found
}

// Runnable resolves a script to an absolute path and checks it is an executable file. A case that
// reached for a path like that and found none would fail for the fixture's reason and not its own.
func Runnable(t *testing.T, script string) string {
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

// Sandbox is this case's own directory with its symlinks resolved. macOS hands out a /var temp
// directory that resolves into /private/var, and Sandboxed compares resolved paths.
func Sandbox(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving this case's temp directory: %v — nothing was measured", err)
	}
	return dir
}

// Sandboxed returns the path once it is inside the sandbox. What runs after a fixture write is an
// executable, so a path escaping the sandbox stops the case before anything runs.
func Sandboxed(t *testing.T, sandbox, path string) string {
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

// WriteFile puts a fixture file on disk with its parents.
func WriteFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("building %s: %v — nothing was measured", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("writing %s: %v — nothing was measured", path, err)
	}
	// os.WriteFile leaves an existing file's mode alone, and several cases rewrite one.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("setting the mode of %s: %v — nothing was measured", path, err)
	}
}

// ReadFile reads a fixture file back. A read that fails stops the case, since the assertion after it
// would be made against an empty string.
func ReadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v — nothing was measured", path, err)
	}
	return string(body)
}

// ExpectRefusal holds a run to exit 2 and the wording that names its cause.

// The missing-command check sits in the middle for a reason. A PATH fixture short of something the
// script calls produces a refusal of its own, and the case would then be measuring the fixture.
func ExpectRefusal(t *testing.T, got Run, wording string) {
	t.Helper()
	if got.Code != 2 {
		t.Errorf("wanted exit 2 and the refusal %q\n%v", wording, got)
		return
	}
	if got.Said("command not found") || got.Said(": not found") {
		t.Errorf("a missing command produced this refusal, not %q — the PATH is short of something the "+
			"script calls, so this case measured that instead\n%v", wording, got)
		return
	}
	if !got.Said(wording) {
		t.Errorf("the refusal does not say %q, so a caller cannot tell this cause from the others that also "+
			"exit 2\n%v", wording, got)
	}
}
