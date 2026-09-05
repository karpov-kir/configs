// The two arms have to agree about a carriage return. The diff arm reads through bufio.Scanner, whose
// line split strips a trailing \r; the untracked arm splits on "\n" itself and keeps it. Unstripped,
// the control-byte guard reads that \r as binary and drops every line of a CRLF file — silently, and
// after Reached has already counted the file, so the summary reports a denominator it did not cover
// and the same content answers differently depending on whether git happens to track it.
package diffscan

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const crlfBody = "// one\r\n// two\r\nx := 1\r\n"
const lfBody = "// one\n// two\nx := 1\n"

func countedLines(t *testing.T, walk func(*Result, func(AddedLine)) error) (map[string]int, Result) {
	t.Helper()
	var result Result
	seen := map[string]int{}
	if err := walk(&result, func(added AddedLine) {
		if added.Text != "" {
			seen[added.File]++
		}
	}); err != nil {
		t.Fatalf("the walk failed, so this case measured nothing: %v", err)
	}
	return seen, result
}

func TestTheUntrackedArmReadsCRLFLikeTheDiffArm(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	writeFile(t, filepath.Join(dir, "crlf.go"), crlfBody)
	writeFile(t, filepath.Join(dir, "lf.go"), lfBody)

	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkUntracked(dir, Options{MaxFileBytes: 1 << 20}, visit)
	})

	if seen["lf.go"] != 3 {
		t.Fatalf("the LF control yielded %d countable lines, wanted 3 — the fixture is wrong, not the code", seen["lf.go"])
	}
	if seen["crlf.go"] != 3 {
		t.Errorf("the CRLF file yielded %d countable lines against the control's 3, and %d line(s) were counted binary — "+
			"a \\r is a line ending, not a control byte", seen["crlf.go"], result.BinaryLines)
	}
	if result.BinaryLines != 0 {
		t.Errorf("%d line(s) were dropped as binary over text files, so the run covered less than Reached claims", result.BinaryLines)
	}
}

// The other half of "agree": the same three lines through the diff arm. Without it the case above
// could be satisfied by changing both arms in the wrong direction.
func TestTheDiffArmReadsCRLF(t *testing.T) {
	diff := "diff --git a/crlf.go b/crlf.go\n--- a/crlf.go\n+++ b/crlf.go\n@@ -0,0 +1,3 @@\n" +
		"+// one\r\n+// two\r\n+x := 1\r\n"
	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkDiff([]byte(diff), visit)
	})
	if seen["crlf.go"] != 3 {
		t.Errorf("the diff arm yielded %d countable lines, wanted 3 (%d counted binary)", seen["crlf.go"], result.BinaryLines)
	}
}

func TestARealControlByteIsStillBinary(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	writeFile(t, filepath.Join(dir, "esc.go"), "// fine\n// bad\x1bhere\n")

	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkUntracked(dir, Options{MaxFileBytes: 1 << 20}, visit)
	})
	if seen["esc.go"] != 1 {
		t.Errorf("%d line(s) reached the visitor, wanted only the clean one", seen["esc.go"])
	}
	if result.BinaryLines != 1 {
		t.Errorf("%d line(s) counted binary, wanted the one holding an ESC", result.BinaryLines)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed, so nothing was measured: %v %s", err, out)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("could not write the fixture %s: %v", path, err)
	}
}

// `--` separates revisions from pathspecs, and passing the two halves to git as one list is how this
// scanner came to report a clean tree over real staged changes: `--` counted as a revision, so HEAD
// was never appended and git diffed against the INDEX. Exit 0, nothing found — the failure this
// package exists to prevent, reached through the form its own error text recommends.
func TestRevisionsNamedSeparatesThemFromPathspecs(t *testing.T) {
	for _, row := range []struct {
		name         string
		args         []string
		named, paths []string
	}{
		{"nothing at all", nil, nil, nil},
		{"revisions only", []string{"HEAD", "origin/main"}, []string{"HEAD", "origin/main"}, nil},
		{"pathspecs only, which names no revision", []string{"--", "src/"}, []string{}, []string{"--", "src/"}},
		{"both halves", []string{"HEAD", "--", "a.go", "b.go"}, []string{"HEAD"}, []string{"--", "a.go", "b.go"}},
		{"a bare separator", []string{"--"}, []string{}, []string{"--"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			named, paths := RevisionsNamed(row.args)
			if !slices.Equal(named, row.named) {
				t.Errorf("revisions = %v, wanted %v", named, row.named)
			}
			if !slices.Equal(paths, row.paths) {
				t.Errorf("pathspecs = %v, wanted %v", paths, row.paths)
			}
		})
	}
}

// The whole point of the split, driven against real git rather than asserted on the argument list: a
// staged change must be visible through `-- <path>`, because that is the invocation the tool tells
// people to use.
func TestAPathspecScanStillDefaultsToHead(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", ".")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	run("add", "a.go")
	run("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n// staged\n"), 0o644); err != nil {
		t.Fatalf("staging: %v", err)
	}
	run("add", "a.go")

	bare, err := Diff(dir, nil)
	if err != nil {
		t.Fatalf("the bare form failed: %v", err)
	}
	if !strings.Contains(string(bare), "+// staged") {
		t.Fatalf("the bare form did not see the staged change, so this case cannot tell the split from "+
			"a broken fixture:\n%s", bare)
	}

	// A second changed file, so the pathspec has something to exclude. Without one, a run that drops
	// the pathspecs entirely still shows a.go and the case cannot tell the two apart.
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package a\n// elsewhere\n"), 0o644); err != nil {
		t.Fatalf("seeding the second file: %v", err)
	}
	run("add", "b.go")

	scoped, err := Diff(dir, []string{"--", "a.go"})
	if err != nil {
		t.Fatalf("the pathspec form failed: %v", err)
	}
	if !strings.Contains(string(scoped), "+// staged") {
		t.Errorf("`-- a.go` saw no staged change, so it diffed against the index rather than HEAD. "+
			"That reports a clean tree over real work and exits 0:\n%s", scoped)
	}
	if strings.Contains(string(scoped), "+// elsewhere") {
		t.Errorf("`-- a.go` returned b.go as well, so the pathspec never reached git and the scope the "+
			"caller asked for was silently ignored:\n%s", scoped)
	}
}
