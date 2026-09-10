package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeVersionAndAvailabilityChangeTheCacheKey(t *testing.T) {
	root := newLibFixture(t)
	bin := t.TempDir()
	for _, name := range []string{"go", "git"} {
		binary, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(binary, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	node := filepath.Join(bin, "node")
	writeNode := func(body string) {
		t.Helper()
		if err := os.WriteFile(node, []byte("#!/bin/sh\n[ \"$*\" = --version ] || exit 99\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	readKey := func() string {
		t.Helper()
		said := &strings.Builder{}
		g := &gate{env: Env{Root: root, SelfDigest: "same-gate"}, errOut: said}
		if code := g.resolveMachine(); code != 0 {
			t.Fatalf("machine resolution exited %d: %s", code, said.String())
		}
		g.manifest = []manifestLine{{hash: "same-source", path: "input.txt"}}
		key, _ := g.keyMaterial(unit{id: "node-suite", inputs: []string{"input.txt"}, cmd: "run"})
		return key
	}
	writeNode("echo v22.0.0\n")
	first := readKey()
	if again := readKey(); again != first {
		t.Fatal("unchanged Node invalidated the cache key")
	}
	writeNode("echo v24.0.0\n")
	replaced := readKey()
	if replaced == first {
		t.Error("replacing Node left the cache key unchanged")
	}
	writeNode("echo v24.0.0\nexit 1\n")
	broken := readKey()
	if broken == replaced {
		t.Error("a failed Node version check left the successful cache key unchanged")
	}
	if err := os.Remove(node); err != nil {
		t.Fatal(err)
	}
	missing := readKey()
	if missing == replaced {
		t.Error("removing Node left the cache key unchanged")
	}
	writeNode("echo v24.0.0\n")
	if restored := readKey(); restored != replaced {
		t.Error("restoring the same Node version did not restore the cache key")
	}
}
