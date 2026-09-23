package buildgate_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	buildgate "configs/ai/tools/build-gate"
	"configs/ai/tools/repo"
)

// A git hook runs with GIT_DIR set. This case runs real git, because the fingerprint does.
func TestAStrayGitDirMovesNeitherTheRecordNorTheFingerprint(t *testing.T) {
	for _, variable := range []string{"GIT_DIR", "GIT_COMMON_DIR"} {
		t.Run(variable, func(t *testing.T) {
			here, other := newGitRepo(t), newGitRepo(t)
			// The other repository ignores f. A fingerprint read through it misses the new file.
			write(t, filepath.Join(other, ".git", "info", "exclude"), "f\n")
			t.Chdir(here)
			t.Setenv(variable, filepath.Join(other, ".git"))
			for _, args := range [][]string{{"open", "FE-267"}, {"stamp"}} {
				if code := buildgate.Command(args, io.Discard, io.Discard); code != 0 {
					t.Fatalf("%v exited %d", args, code)
				}
			}
			if _, err := os.Stat(filepath.Join(here, ".git", "kk-build-gate")); err != nil {
				t.Fatalf("the record is not in the worktree's own git dir: %v", err)
			}
			if _, err := os.Stat(filepath.Join(other, ".git", "kk-build-gate")); err == nil {
				t.Fatalf("the record landed in the repository %s named", variable)
			}
			write(t, filepath.Join(here, "f"), "edit\n")
			if code := buildgate.Command([]string{"check"}, io.Discard, io.Discard); code != 1 {
				t.Fatalf("check exited %d after an edit, want 1: the fingerprint read the repository %s named", code, variable)
			}
		})
	}
}

func newGitRepo(t *testing.T) string {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", dir)
	cmd.Env = repo.WithoutGitLocation(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}

func write(t *testing.T, path, body string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
