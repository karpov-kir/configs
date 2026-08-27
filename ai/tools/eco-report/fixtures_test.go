package ecoreport_test

// How a fixture is built and read: the file and git primitives the builders in harness_test.go stand
// on, and the text helpers the cases compare output with. Split from the harness for size alone —
// what a case *says* is there; what puts a tree on disk is here.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Fixture I/O. Each fails the case rather than returning an error: a fixture that did not get built
// leaves its assertions passing against a tree they were never given.
func (f *fixture) mkdirAll(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func (f *fixture) write(path, content string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

func (f *fixture) appendTo(path, content string) {
	f.t.Helper()
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		f.t.Fatalf("append %s: %v", path, err)
	}
	defer handle.Close()
	if _, err := handle.WriteString(content); err != nil {
		f.t.Fatalf("append %s: %v", path, err)
	}
}

func (f *fixture) read(path string) string {
	f.t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func (f *fixture) symlink(target, link string) {
	f.t.Helper()
	if err := os.Symlink(target, link); err != nil {
		f.t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

func (f *fixture) chmod(path string, mode os.FileMode) {
	f.t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
}

func (f *fixture) remove(path string) {
	f.t.Helper()
	if err := os.RemoveAll(path); err != nil {
		f.t.Fatalf("remove %s: %v", path, err)
	}
}

func (f *fixture) copyIn(from, to string, mode os.FileMode) {
	f.t.Helper()
	content, err := os.ReadFile(from)
	if err != nil {
		f.t.Fatalf("read %s: %v", from, err)
	}
	if err := os.WriteFile(to, content, mode); err != nil {
		f.t.Fatalf("write %s: %v", to, err)
	}
}

func (f *fixture) exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (f *fixture) isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func (f *fixture) find(root string) []string {
	var found []string
	_ = filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err == nil {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func (f *fixture) entries(dir string) []string {
	listing, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range listing {
		names = append(names, entry.Name())
	}
	return names
}

func (f *fixture) git(args ...string) (string, int) {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.repo}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	status := 0
	if err := cmd.Run(); err != nil {
		status = 1
		if exit, ok := err.(*exec.ExitError); ok {
			status = exit.ExitCode()
		}
	}
	return strings.TrimRight(out.String(), "\n"), status
}

func (f *fixture) mustGit(args ...string) string {
	f.t.Helper()
	out, status := f.git(args...)
	if status != 0 {
		f.t.Fatalf("git %s failed in %s — stopping before any destructive case runs", strings.Join(args, " "), f.repo)
	}
	return out
}

// Identity is passed per-commit: the machine running this need not have one configured.
func (f *fixture) commit(message string) {
	f.t.Helper()
	f.mustGit("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", message)
}

// `grep -qx` — the whole line, never a substring, which is what several cases mean by "prints this".
func containsLine(text, line string) bool {
	for _, candidate := range strings.Split(text, "\n") {
		if candidate == line {
			return true
		}
	}
	return false
}

func countLinesEqual(text, line string) int {
	count := 0
	for _, candidate := range strings.Split(text, "\n") {
		if candidate == line {
			count++
		}
	}
	return count
}

// The state column of a `list` line — `grep '^<name>[[:space:]]' | cut -f2`.
func stateOf(listing, name string) string {
	for _, line := range strings.Split(listing, "\n") {
		if rest, ok := strings.CutPrefix(line, name); ok && strings.HasPrefix(rest, "\t") {
			return rest[1:]
		}
	}
	return ""
}

func countNonEmptyLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			count++
		}
	}
	return count
}

func sortedWords(text string) string {
	words := strings.Fields(text)
	for i := range words {
		for j := i + 1; j < len(words); j++ {
			if words[j] < words[i] {
				words[i], words[j] = words[j], words[i]
			}
		}
	}
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, " ") + " "
}

func itoa(value int) string { return strconv.Itoa(value) }

func (f *fixture) isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// `grep -rlF <needle>` — the files under a directory holding the text, which is how a case asks
// whether content from outside the repo reached .idsd/.
func (f *fixture) filesContaining(dir, needle string) []string {
	var found []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if content, err := os.ReadFile(path); err == nil && strings.Contains(string(content), needle) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func countLinesEndingWith(text, suffix string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasSuffix(line, suffix) {
			count++
		}
	}
	return count
}

func joinLines(values []string) string { return strings.Join(values, "\n") }

// `sed -i 's/^<prefix>.*/<line>/'` and `grep -v '^<prefix>'` over a fixture file: the two edits the
// template cases break their own copy with.
func (f *fixture) replaceLine(path, prefix, line string) {
	f.t.Helper()
	var kept []string
	for _, existing := range strings.Split(f.read(path), "\n") {
		if strings.HasPrefix(existing, prefix) {
			existing = line
		}
		kept = append(kept, existing)
	}
	f.write(path, strings.Join(kept, "\n"))
}

func (f *fixture) dropLines(path, prefix string) {
	f.t.Helper()
	var kept []string
	for _, existing := range strings.Split(f.read(path), "\n") {
		if !strings.HasPrefix(existing, prefix) {
			kept = append(kept, existing)
		}
	}
	f.write(path, strings.Join(kept, "\n"))
}

func indent(text string) string {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fmt.Fprintf(&out, "          %s\n", line)
	}
	return out.String()
}
