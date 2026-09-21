package projectmcp

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// The mode a config file is created with when the project has none yet.
const newConfigMode = 0o644

// writeConfig, the function that writes a project's config file, replaces it by rename. A followed
// symlink writes outside the project, at whatever the link names, and a followed hard link breaks a
// link the project deliberately made. The human sees neither outcome, so a link here is refused.

// The previous contents of a project's config file, or empty where there is none.
func readConfig(file string) (string, error) {
	info, err := os.Lstat(file)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || linkCount(info) != 1 {
		return "", fmt.Errorf("%s must be a regular unlinked file", file)
	}
	text, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(text), nil
}

func linkCount(info os.FileInfo) uint64 {
	status, isUnix := info.Sys().(*syscall.Stat_t)
	if !isUnix {
		return 1
	}
	return uint64(status.Nlink)
}

// The temporary carries this process's id and is created exclusively, so two runs at once both fail
// to create it and neither writes over the other's half-written file.

// writeConfig writes beside the file and renames over it, so a reader of the config never sees half a
// file, and a failure midway leaves the previous version intact.
func writeConfig(file, text, previous string) error {
	if text == previous {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(newConfigMode)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	temporary := fmt.Sprintf("%s.kk-mcp-%d", file, os.Getpid())
	handle, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err = handle.WriteString(text); err != nil {
		handle.Close()
		os.Remove(temporary)
		return err
	}
	if err = handle.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	if err = os.Rename(temporary, file); err != nil {
		os.Remove(temporary)
		return err
	}
	return nil
}
