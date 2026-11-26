package installer

import (
	"errors"
	"os"
	"syscall"

	"configs/ai/tools/shell"
)

// tree is the filesystem a run writes through, and where a write is checked against the root it is
// allowed to touch. Every mutating call goes through it. A write made with os directly is a write
// the bound never saw.
type tree struct {
	// writeRoot is RunOptions.WriteRoot resolved physically. Empty is unbounded.
	writeRoot string
	onBreach  func(message string)

	// The answer this cannot give, an unreadable count, is a state no filesystem here produces, and
	// the write that would follow it shares somebody's private file. That is why it is a field.

	// linkCount is how many names a file has.
	linkCount func(path string) (uint64, error)
}

func newTree(writeRoot string, onBreach func(message string)) *tree {
	if writeRoot != "" {
		// The root is resolved once, because a temp directory is handed back as /var/folders/… on macOS
		// and /var is itself a symlink to /private/var. An unresolved root compared against resolved
		// paths makes contained refuse everything, and a guard that always fires gets deleted.
		writeRoot = realDir(writeRoot)
	}
	return &tree{writeRoot: writeRoot, onBreach: onBreach, linkCount: statLinkCount}
}

// The parent is resolved physically, and any symlink in it is followed. A run leaves
// $home/.config/nvim pointing into this checkout, and a later write at $home/.config/nvim/init.lua
// then lands in a real config file in the working tree.

// A textual comparison of the path leaves that door open, and the incident in the package comment
// went through it.

// Answers whether this path may be written. The question comes before the write.
func (t *tree) contained(path string) error {
	if t.writeRoot == "" {
		return nil
	}
	parent := shell.NearestExistingParent(path)
	if parent == "" {
		return t.breach(path, "no directory above it resolves")
	}
	if !shell.IsWithin(parent, t.writeRoot) {
		return t.breach(path, "its nearest existing parent resolves to "+parent)
	}
	return nil
}

func (t *tree) breach(path, reason string) error {
	message := "refusing to write " + path + " — " + reason + "; only " + t.writeRoot +
		" may be written, and this is the containment guard, not a failing case"
	t.onBreach(message)
	return errors.New(message)
}

// `ln -sfn`, minus the `-f` that would also unlink a regular file. A link may be written over a
// symlink alone, which carries no data of its own, and the caller has already asked that question. A
// clearance of anything else here would put the deletion this package refuses to make behind a name
// that reads as harmless.

// `-n` is the whole reason the old link is removed instead of written through. `ln -s X Y` with Y
// already a symlink to a directory creates the link INSIDE Y, and that is how a stray link ends up
// in the checkout.

// Links source at target, over a symlink already there.
func (t *tree) symlink(source, target string) error {
	if err := t.contained(target); err != nil {
		return err
	}
	if shell.IsSymlink(target) {
		if err := os.Remove(target); err != nil {
			return err
		}
	}
	return os.Symlink(source, target)
}

func (t *tree) mkdirAll(dir string) error {
	if err := t.contained(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// Remove one name. A path already gone is success, which is what `rm -f` answers and what an
// uninstall run twice needs.
func (t *tree) remove(path string) error {
	if err := t.contained(path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// A hardlinked file passes the symlink check and the existence check alike, so no other guard
// catches it. Its contents are shared with a file the caller never named.

// An answer that cannot be read comes back as an error, and never as a count. An unestablished link
// count is the case where writing might share someone's file. This is the wrong place to assume the
// safe answer.

// The shell reached that state routinely. `stat -c` is GNU's format flag and `-f` is BSD's. On Linux
// `stat -f` is the filesystem flag, and it prints a block of facts a numeric comparison silently
// reads as one link.

// Go asks the kernel. The way left is a filesystem with a stat of its own, and that is why linkCount,
// a field of tree, is a seam.

// Returns how many names this file has.
func statLinkCount(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("no link count on this filesystem")
	}
	return uint64(status.Nlink), nil
}

// Where a directory really is, or empty — physical, for the reason RunOptions.Repo states.
func realDir(dir string) string {
	return shell.CanonicalDir(dir)
}

func readLink(path string) (string, error) {
	return os.Readlink(path)
}

// Whether this process may write to the file, which is access(2)'s answer and `[ -w ]`'s. The kernel
// is asked, because root ignores the mode bits and a capability grants write without them.
func isWritable(path string) bool {
	return syscall.Access(path, 0x2) == nil
}
