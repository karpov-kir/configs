package ecoroot

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

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

// Opened rather than access(2)'d, because opening is the question containment asks: access(2) says yes
// where the open still fails. ecoreport answers it the other way, for the reason on its own isReadable.
func isReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// AbsentOrOutOfReach answers which of the two things a failed existence test means. shell.PathExists
// returns false for a file nobody wrote and for one sitting behind a directory this process cannot
// open, and those are different findings: absent is the reviewed tree's own defect and the
// always-loaded tier was still measured whole, while out of reach leaves that figure short by
// whatever the file holds. Calling the second one "does not exist" sends a reader hunting for a file
// sitting exactly where the router says it is.
//
// isAbsent is true only for a path nobody wrote. Otherwise reason names what stopped the answer and
// is never empty, so a caller can print it without testing for a blank.
func AbsentOrOutOfReach(path string) (isAbsent bool, reason string) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, ""
	}
	if err != nil {
		return false, err.Error()
	}
	// Lstat answered, so only the Stat behind PathExists failed — a dangling symlink is the shape that
	// reaches here. Neither call produced a reason to print, so the caller gets this one.
	return false, "neither Stat nor Lstat could answer for it"
}
