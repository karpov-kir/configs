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

// AbsentOrOutOfReach answers which of the two things a failed existence test means. shell.PathExists
// returns false for a file nobody wrote and for one sitting behind a directory this process cannot
// open, and those are different findings: absent is the reviewed tree's own defect and the
// always-loaded tier was still measured whole, while out of reach leaves that figure short by
// whatever the file holds. Calling the second one "does not exist" sends a reader hunting for a file
// sitting exactly where the router says it is.
//
// isAbsent is true only for a path nobody wrote. Otherwise reason names what stopped the answer and
// is never empty, so a caller can print it without testing for a blank.
//
// One implementation rather than one per tool, and that is the point of it living here. ecostats and
// ecocheck describe one tree, so they may not disagree about what a permission failure means — two
// detectors answering one question differently is the hazard, not the wording. A copy in each, under
// a comment in each asserting that the two agree, is that invariant enforced by absence: nothing
// reports the moment it stops holding.
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
