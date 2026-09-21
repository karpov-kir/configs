// A `go build ./` inside a main package's own directory drops the binary beside the source, named
// after that directory. .gitignore covers bin/ and dist/ and neither of those, and gitignore cannot
// express "a file named after its directory", so the paths are written out one per line. That the
// names are distinct in the first place is shipped_test.go's. This case is the list's guard: the list
// cannot notice a tool added after it, and it fails in the direction that hurts, by leaving a ~3MB
// binary untracked for the next `git add -A` to commit.
package tools_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestEveryBuiltBinaryIsIgnored(t *testing.T) {
	dirs := mainPackageDirs(t)
	if len(dirs) == 0 {
		t.Fatal("found no main packages, so this case would pass against any .gitignore at all")
	}

	var artifacts []string
	for _, dir := range dirs {
		artifacts = append(artifacts, filepath.Join(dir, filepath.Base(dir)))
	}
	ignored := ignoredByGit(t, artifacts)
	for _, artifact := range artifacts {
		if !ignored[artifact] {
			t.Errorf("`go build ./` in %s drops %s, which .gitignore does not cover — add a line for it, "+
				"or the next `git add -A` in any session commits the binary", filepath.Dir(artifact), artifact)
		}
	}
}

// Reports which of these paths git ignores. The answer is git's own, and a parser of ours could
// agree with the file and still disagree with git about precedence, anchoring or negation.
//
// One invocation covers the whole list, so a module holding two dozen tools spends one process here
// instead of two dozen.
func ignoredByGit(t *testing.T, paths []string) map[string]bool {
	t.Helper()
	// git takes the paths on stdin and writes back the ones it ignores. `-z` on both streams, because
	// git C-quotes a path holding a quote or a non-ASCII byte.
	ask := exec.Command("git", "check-ignore", "-z", "--stdin")
	ask.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	answered, err := ask.Output()
	var exit *exec.ExitError
	// Exit 1 is git saying it ignores none of them, which is an answer. Anything else — git missing, no
	// repository, a bad invocation — leaves the question unanswered. A case that never ran fails here,
	// so a green run always means git answered.
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		t.Fatalf("git check-ignore could not answer for %v: %v", paths, err)
	}
	ignored := map[string]bool{}
	for _, path := range strings.Split(strings.TrimSuffix(string(answered), "\x00"), "\x00") {
		if path != "" {
			ignored[path] = true
		}
	}
	return ignored
}

// Every directory holding a `package main`, verbatim — the directory the build actually runs in,
// which is what names the dropped binary. shipped_test.go reads the same list as tool names.
func mainPackageDirs(t *testing.T) []string {
	t.Helper()
	found := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// bin/ holds build output, and a stray checkout under it would be read as source.
			if entry.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isPackageMain(string(body)) {
			found[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	var dirs []string
	for dir := range found {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// The clause only, so a `package main` inside a comment or a string does not count as one.
func isPackageMain(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "package main" {
			return true
		}
	}
	return false
}
