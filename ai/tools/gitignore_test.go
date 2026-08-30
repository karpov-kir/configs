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

	for _, dir := range dirs {
		artifact := filepath.Join(dir, filepath.Base(dir))
		if !ignoredByGit(t, artifact) {
			t.Errorf("`go build ./` in %s drops %s, which .gitignore does not cover — add a line for it, "+
				"or the next `git add -A` in any session commits the binary", dir, artifact)
		}
	}
}

// git's own answer rather than a parser of ours, which could agree with the file and still disagree
// with git about precedence, anchoring or negation.
func ignoredByGit(t *testing.T, path string) bool {
	t.Helper()
	err := exec.Command("git", "check-ignore", "-q", "--", path).Run()
	if err == nil {
		return true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false
	}
	// Anything else — git missing, no repository, a bad invocation — means the question went
	// unanswered, and a case that did not run must not read as one that passed.
	t.Fatalf("git check-ignore could not answer for %s: %v", path, err)
	return false
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
