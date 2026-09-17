package installer

import (
	"errors"
	"os"
	"syscall"

	"kk-flavor/tools/shell"
)

// tree is the filesystem a run writes through, and the one place a write is checked against the root
// it is allowed to touch. Every mutating call goes through it; a write made with os directly is a
// write the bound never saw.
type tree struct {
	// writeRoot is RunOptions.WriteRoot resolved physically. Empty is unbounded.
	writeRoot string
	onBreach  func(message string)
	// linkCount is how many names a file has. A field because the answer it cannot give — a count
	// nobody could read — is a state no filesystem here produces, and the write that would follow it
	// shares somebody's private file.
	linkCount func(path string) (uint64, error)
}

func newTree(writeRoot string, onBreach func(message string)) *tree {
	if writeRoot != "" {
		// Resolved once, because a temp directory is handed back as /var/folders/… on macOS while /var
		// is itself a symlink to /private/var. Comparing an unresolved root against resolved paths
		// makes the check below refuse everything, and a guard that always fires gets deleted.
		writeRoot = realDir(writeRoot)
	}
	return &tree{writeRoot: writeRoot, onBreach: onBreach, linkCount: statLinkCount}
}

// Whether this path may be written, asked before the write rather than noticed after it.
//
// The parent is resolved physically, following any symlink in it. A run leaves $home/.config/nvim
// pointing into this checkout, and a later write at $home/.config/nvim/init.lua then lands in a real
// config file in the working tree — which is the door a textual comparison of the path leaves open
// and the one the incident in the package comment went through.
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

// Link source at target, over a symlink already there.
//
// `ln -sfn`, minus the `-f` that would also unlink a regular file: the one target a link may be
// written over is a symlink, which carries no data of its own, and the caller has already asked that
// question. Clearing anything else here would put the deletion this package refuses to make behind a
// name that reads as harmless.
//
// `-n` is the whole reason the old link is removed rather than written through: `ln -s X Y` where Y is
// already a symlink to a directory creates the link INSIDE Y, which is how a stray link ends up in the
// checkout.
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

func (t *tree) appendLine(file, line string) error {
	if err := t.contained(file); err != nil {
		return err
	}
	handle, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := handle.WriteString(line + "\n"); err != nil {
		handle.Close()
		return err
	}
	return handle.Close()
}

// The three ways replaceFile fails, held apart because each sends the reader somewhere different:
// nowhere to put the new bytes, the new bytes could not be written, or the swap itself failed. Only
// the first leaves nothing to clean up, and the second and third both leave the original as it was.
var (
	errNoTemporary = errors.New("could not create a temporary file")
	errNotWritten  = errors.New("could not write the replacement")
	errNotSwapped  = errors.New("could not swap the replacement in")
)

// Replace a file's contents through a temp file in its own directory, then rename. Same directory so
// the rename is on one filesystem and therefore atomic: a killed run leaves the original untouched
// rather than a half-written CLAUDE.md.
//
// The original's mode is carried over, so a file the human made executable or group-writable does not
// silently come back as whatever the umask says.
func (t *tree) replaceFile(file string, content []byte) error {
	if err := t.contained(file); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(shell.DirName(file), ".kk-region.")
	if err != nil {
		return errNoTemporary
	}
	name := temporary.Name()
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		os.Remove(name)
		return errNotWritten
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return errNotWritten
	}
	if info, err := os.Stat(file); err == nil {
		os.Chmod(name, info.Mode().Perm())
	}
	if err := os.Rename(name, file); err != nil {
		os.Remove(name)
		return errNotSwapped
	}
	return nil
}

// How many names this file has. A hardlinked file is neither a symlink nor a missing one, so nothing
// else catches it — and the append path copies the file's existing contents into the replacement,
// which for a link to somebody's private file copies that file into the project. The rename breaks
// the link so the original is never modified, but the read has already happened, and this package has
// no business reading a file its caller only named one name for.
//
// An answer that cannot be read is an error rather than one. A link count nobody established is the
// case where writing might share someone's file, so it is the wrong place to assume the safe answer.
// The shell reached that state routinely: `stat -c` is GNU's format flag and `-f` is BSD's, and on
// Linux `stat -f` is --file-system, which prints a block of filesystem facts that a numeric
// comparison silently reads as one link. Go asks the kernel, so the only way left is a filesystem
// whose stat is not the platform's — which is why the field above is a seam.
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

// Whether this process may write to the file, which is access(2)'s answer and `[ -w ]`'s. Asked of
// the kernel rather than read off the mode bits, because root ignores them and a capability grants
// them without one.
func isWritable(path string) bool {
	return syscall.Access(path, 0x2) == nil
}
