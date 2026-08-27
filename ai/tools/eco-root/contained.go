package ecoroot

import (
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// --- shared:contained-in-root ---
// True when a path's directory sits at or under rootCanon, which is shell.CanonicalDir of the root.
//
// A symlink is refused rather than resolved: CanonicalDir canonicalises a *directory*, so it never
// sees the final component, and a link at a budget path would walk through a check that only tested
// its parent. A regular file, or nothing: existence alone admits a FIFO or a device, which a read
// then blocks on forever, and a dangling symlink fails existence entirely — so callers enter on
// exists-or-is-a-symlink and both become a refusal, not a silent drop. Readable too, not just
// regular: a mode-000 file passes every type test, and the read behind the figure then fails,
// leaving a file counted whose words are not.
func containedInRoot(rootCanon, path string) bool {
	if shell.IsSymlink(path) || !shell.IsRegularFile(path) || !isReadable(path) {
		return false
	}
	dir := shell.CanonicalDir(shell.DirName(path))
	if rootCanon == "" || dir == "" {
		return false
	}
	return dir == rootCanon || strings.HasPrefix(dir, rootCanon+"/")
}

// --- end shared:contained-in-root ---

// `[ -r ]` answered by opening the file, which is the question containment asks: a file admitted here
// is one whose words the figure behind it can actually read, and access(2) says yes where the open
// still fails. ecoreport asks access(2) instead, deliberately and for its own reason — see the note
// on isReadable in its shell.go. Neither is the shared one, so `shell` holds no readability test.
func isReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}
