package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The fixtures every case here builds its tree with.

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
	defined, edges := read(root, &stderr)
	return defined, edges, stderr.String()
}
