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
	"testing"
)

const crlfBody = "// one\r\n// two\r\nx := 1\r\n"
const lfBody = "// one\n// two\nx := 1\n"

// countedLines drives one arm and returns how many non-empty lines reached the visitor per file.
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

	// The LF file is the control: identical content, one byte per line different. If it also came back
	// empty the case would be measuring a broken fixture rather than the \r.
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

// A real control byte still marks the line binary. Without this the fix above could have been "stop
// checking for control bytes", which would let a NUL-bearing line into a report.
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
