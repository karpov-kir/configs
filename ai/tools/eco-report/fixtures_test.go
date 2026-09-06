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
	"sort"
	"strings"
	"testing"
)

// Every git spawn here goes to whatever PATH names first. Most of them are the tool's own, and it
// resolves `git` through PATH the same way. On macOS that first name is `/usr/bin/git`, an xcrun
// shim. Going through the shim costs 2.06x the real git behind it: 37.9ms a spawn against 18.4ms.
//
// This package makes about 4300 spawns, and the shim had pushed it past `go test`'s 600s default:
// a FAIL at 603.4s on a loaded machine.
//
// Resolved once, because `xcrun` is itself a spawn. No-op wherever xcrun is absent, which is every
// Linux runner.
func TestMain(m *testing.M) {
	if out, err := exec.Command("xcrun", "-f", "git").Output(); err == nil {
		resolved := strings.TrimSpace(string(out))
		if info, statErr := os.Stat(resolved); statErr == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			dir := filepath.Dir(resolved)
			path := os.Getenv("PATH")
			if !strings.HasPrefix(path, dir+string(os.PathListSeparator)) {
				os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
			}
		}
	}
	os.Exit(m.Run())
}

// Fixture I/O. The builders fail the case rather than returning an error: a fixture that did not get
// built leaves its assertions passing against a tree they were never given. The queries answer
// instead. A case asks `exists`, `isFile` or `read` a question, and a false or empty answer is its
// result, not a broken fixture.
func (f *fixture) mkdirAll(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// Builds the parents too. A ship is a directory now, so most fixture writes land one level inside one
// that does not exist yet, and a case that had to mkdir before every write would say more about the
// layout than about what it is testing.
func (f *fixture) write(path, content string) {
	f.t.Helper()
	f.mkdirAll(filepath.Dir(path))
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
	f.mkdirAll(filepath.Dir(link))
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
	return f.gitIn(f.repo, args...)
}

// The same question asked from another directory, for the linked-worktree cases: a worktree is its
// own root, and what git answers there — its git dir above all — is not what it answers in the repo
// that created it.
func (f *fixture) gitIn(dir string, args ...string) (string, int) {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
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

func countLines(text string, counts func(string) bool) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if counts(line) {
			count++
		}
	}
	return count
}

func countLinesEqual(text, line string) int {
	return countLines(text, func(candidate string) bool { return candidate == line })
}

func countLinesWithPrefix(text, prefix string) int {
	return countLines(text, func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func countNonEmptyLines(text string) int {
	return countLines(text, func(line string) bool { return line != "" })
}

func countLinesEndingWith(text, suffix string) int {
	return countLines(text, func(line string) bool { return strings.HasSuffix(line, suffix) })
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

// The report split at its frontmatter delimiters, so a case can say which half a line is in. Line 1
// opens the frontmatter and the next `---` closes it, which is the rule the rewrites apply.
func frontmatterAndBody(report string) (frontmatter, body string) {
	lines := strings.Split(report, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", report
	}
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return strings.Join(lines[1:i+1], "\n"), strings.Join(lines[i+2:], "\n")
		}
	}
	return strings.Join(lines[1:], "\n"), ""
}

// `cksum <file>` reduced to the two fields a stage marker holds. False means cksum(1) is not on this
// machine.
func posixCksum(path string) (string, bool) {
	out, err := exec.Command("cksum", path).Output()
	if err != nil {
		return "", false
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", false
	}
	return fields[0] + " " + fields[1], true
}

// A marker's value beside cksum(1)'s own answer for the same file. False means cksum(1) is not on
// this machine, and the comparison is skipped rather than failed.
func (f *fixture) markerAgainstCksum(marker, report string) (held, want string, ok bool) {
	f.t.Helper()
	want, ok = posixCksum(report)
	if !ok {
		f.t.Logf("skip  cksum(1) is not on this machine — the digest comparison cannot run")
		return "", "", false
	}
	return strings.TrimRight(f.read(marker), "\n"), want, true
}

func sortedWords(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	sort.Strings(words)
	return strings.Join(words, " ") + " "
}

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

// One frontmatter field's value out of a report's text — the assertion counterpart to the tool's own
// fieldValue, which reads a file rather than a string a case already holds.
func fieldFrom(text, field string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, field+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, field+":"))
		}
	}
	return ""
}
