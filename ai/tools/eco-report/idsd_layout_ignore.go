package ecoreport

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

type layoutIgnoreRewrite struct {
	path string
	body []byte
	mode os.FileMode
}

func (r *run) planLayoutIgnoreMigration() ([]layoutIgnoreRewrite, error) {
	var rewrites []layoutIgnoreRewrite
	for _, path := range []string{filepath.Join(r.root, ".gitignore"), r.gitCommonPath("info/exclude")} {
		parent := filepath.Dir(path)
		if info, err := os.Lstat(parent); err == nil && !info.IsDir() {
			return nil, fmt.Errorf("ignore parent is not a real directory: %s", parent)
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		var body []byte
		mode := os.FileMode(0o644)
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("ignore target is not a regular file (symlink?): %s", path)
			}
			if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Nlink > 1 {
				return nil, fmt.Errorf("ignore target is a hardlink: %s", path)
			}
			body, err = os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			mode = info.Mode().Perm()
		}
		updated := replaceOwnedIgnorePatterns(string(body))
		if path == filepath.Join(r.root, ".gitignore") && r.repoMode() == "committed" {
			updated = ownedIgnorePatternsLast(updated)
		}
		if !bytes.Equal(body, []byte(updated)) {
			rewrites = append(rewrites, layoutIgnoreRewrite{path: path, body: []byte(updated), mode: mode})
		}
	}
	return rewrites, nil
}

func replaceOwnedIgnorePatterns(body string) string {
	lines := strings.SplitAfter(body, "\n")
	for i, line := range lines {
		ending := ""
		if strings.HasSuffix(line, "\n") {
			line = strings.TrimSuffix(line, "\n")
			ending = "\n"
			if strings.HasSuffix(line, "\r") {
				line = strings.TrimSuffix(line, "\r")
				ending = "\r\n"
			}
		}
		for _, pattern := range ignoreSurface() {
			legacy := strings.Replace(pattern, "/for-agents/", "/", 1)
			if line == legacy {
				line = pattern
				break
			}
		}
		lines[i] = line + ending
	}
	return strings.Join(lines, "")
}

// A later negation can cancel an earlier owned rule. Keep the owned rules last,
// while preserving every unrelated line and its original order.
func ownedIgnorePatternsLast(body string) string {
	patterns := ignoreSurface()
	var updated strings.Builder
	for _, line := range strings.SplitAfter(body, "\n") {
		pattern := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if !slices.Contains(patterns, pattern) {
			updated.WriteString(line)
		}
	}
	if updated.Len() > 0 && !strings.HasSuffix(updated.String(), "\n") {
		updated.WriteByte('\n')
	}
	for _, pattern := range patterns {
		updated.WriteString(pattern + "\n")
	}
	return updated.String()
}

func writeLayoutIgnore(rewrite layoutIgnoreRewrite) error {
	file, err := os.CreateTemp(filepath.Dir(rewrite.path), ".idsd-ignore-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(rewrite.mode); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(rewrite.body); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), rewrite.path)
}
