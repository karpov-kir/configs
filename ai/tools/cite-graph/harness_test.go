package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// One read of a tree, with the tool's own stderr captured rather than the process's: a kind and a
// genuine ambiguity both resolve to nothing, and only the notice tells them apart.
func graph(t *testing.T, root string) (map[string]map[string]bool, []edge, string) {
	t.Helper()
	var stderr bytes.Buffer
	defined, edges, _ := read(root, &stderr)
	return defined, edges, stderr.String()
}

// How many paths one read of the tree did not reach, which is what the exit code is taken from.
func skippedUnder(t *testing.T, root string) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	_, _, skipped := read(root, &stderr)
	return skipped, stderr.String()
}

// The whole command over a tree: exit code, stdout, stderr. A case reading only stdout cannot tell a
// report from a refusal, which is the confusion the exit code exists to settle.
func runOver(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}
