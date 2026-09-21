package runtest

import (
	"os"
	"testing"
)

// A case that builds an unreadable path is building a condition the running process may not have.
// Root reads a mode-000 file happily. So does a process holding CAP_DAC_OVERRIDE, and so does a
// filesystem that leaves the bit off.
//
// These probe for the condition. A comparison against uid 0 would need a list of every environment
// that behaves this way, and a probe answers for all of them at once.

// SkipUnlessModeDeniesRead leaves the case standing where a mode of 000 stops this process reading,
// and skips it elsewhere naming what went unasserted.
func SkipUnlessModeDeniesRead(t *testing.T, what string) {
	t.Helper()
	if ModeDeniesRead(t) {
		return
	}
	t.Skip("this process reads a mode-000 file regardless of the mode (root, or CAP_DAC_OVERRIDE), so " + what)
}

// SkipUnlessModeDeniesDirList is the directory twin. A walk that lists an unlistable directory takes
// its whole subtree into the figures, which is a different case from the one being written.
func SkipUnlessModeDeniesDirList(t *testing.T, what string) {
	t.Helper()
	if ModeDeniesDirList(t) {
		return
	}
	t.Skip("this process lists a mode-000 directory regardless of the mode (root, or CAP_DAC_OVERRIDE), so " + what)
}

// ModeDeniesRead and ModeDeniesDirList are the probes themselves. One case needs both answers at
// once, and it words its own skip.
func ModeDeniesRead(t *testing.T) bool {
	t.Helper()
	probe := t.TempDir() + "/probe"
	if err := os.WriteFile(probe, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	if err := os.Chmod(probe, 0o000); err != nil {
		t.Fatalf("chmod probe: %v", err)
	}
	file, err := os.Open(probe)
	if err != nil {
		return true
	}
	file.Close()
	return false
}

func ModeDeniesDirList(t *testing.T) bool {
	t.Helper()
	probe := t.TempDir() + "/probe"
	if err := os.MkdirAll(probe, 0o755); err != nil {
		t.Fatalf("mkdir probe: %v", err)
	}
	if err := os.WriteFile(probe+"/inner.md", []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	// A chmod that will not take leaves the mode guarding this directory in name only, and a listing
	// that succeeds says the same. The cleanup puts the bit back, so t.TempDir can remove the tree.
	if err := os.Chmod(probe, 0o000); err != nil {
		return false
	}
	t.Cleanup(func() { _ = os.Chmod(probe, 0o755) })
	_, err := os.ReadDir(probe)
	return err != nil
}
