package projectmcp

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// The mode a config file is created with when the project has none yet.
const newConfigMode = 0o644

// The previous contents of a project's config file, or empty where there is none.
//
// A symlink or a hard link is refused rather than followed. The write below replaces the file by
// rename, so following a link would either write through it — landing outside the project, at
// whatever the link names — or break a hard link the project deliberately made, and the human would
// see neither.
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

// Written beside the file and renamed over it, so a reader of the config never sees half of one, and
// a failure part-way leaves the previous version intact.
//
// The temporary carries this process's id and is created exclusively: two runs at once then fail to
// create rather than writing over each other's half-written file.
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
