package repo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Exec answers by running git. Every command in this module wires it in, and only the suites
// substitute anything else.
type Exec struct {
	// Env replaces the child's whole environment when it is set. eco-report points HOME at the
	// invocation's own, because git reads its global config from there and a run given another HOME
	// would answer from the caller's. repo-key strips GIT_DIR and GIT_COMMON_DIR, because git reads the
	// repository's location out of the environment before it reads the directory it was handed.
	Env []string
}

// Git is the interface Exec satisfies. The assignment makes the compiler report a mismatch here,
// ahead of the first caller that wires one in.
var _ Git = Exec{}

// Every git call goes through here, so the two flags every call needs are set in one place. `-C dir`
// stands in for cmd.Dir, and git then prints the path in its own error text. The config key
// core.quotePath, set to false, joins the `-z` each listing passes. Drop either one, and a path holding
// a non-ASCII byte comes back C-quoted or newline-split, and the caller reads a name no file has.
func (e Exec) run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	if e.Env != nil {
		cmd.Env = e.Env
	}
	var out, said bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &said
	if err := cmd.Run(); err != nil {
		return out.Bytes(), gitError(args, said.String(), err)
	}
	return out.Bytes(), nil
}

// git's own words reach the caller. A summary of them sends a reader looking for a cause this process
// already had in hand.
func gitError(args []string, said string, err error) error {
	if reason := strings.TrimSpace(said); reason != "" {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), reason)
	}
	return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// Returns a one-line answer with the trailing newline taken off, because every caller of these wants
// the value.
func (e Exec) line(dir string, args ...string) (string, error) {
	out, err := e.run(dir, args...)
	return strings.TrimRight(string(out), "\n"), err
}

func (e Exec) TopLevel(dir string) (string, error) {
	return e.line(dir, "rev-parse", "--show-toplevel")
}

// Absolute, always. `--git-common-dir` answers a bare `.git` in an ordinary repository, which would
// otherwise resolve against whatever directory the next caller happened to stand in.
func (e Exec) CommonDir(dir string) (string, error) {
	return e.absoluteGitPath(dir, "rev-parse", "--git-common-dir")
}

func (e Exec) GitDir(dir string) (string, error) {
	return e.absoluteGitPath(dir, "rev-parse", "--git-dir")
}

func (e Exec) GitPath(dir, name string) (string, error) {
	return e.absoluteGitPath(dir, "rev-parse", "--git-path", name)
}

func (e Exec) absoluteGitPath(dir string, args ...string) (string, error) {
	answer, err := e.line(dir, args...)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(answer) {
		answer = filepath.Join(dir, answer)
	}
	return filepath.Clean(answer), nil
}

func (e Exec) Prefix(dir string) (string, error) {
	return e.line(dir, "rev-parse", "--show-prefix")
}

// git exits non-zero when the key is unset, and that exit code is all that separates unset from set to
// the empty string. The two mean opposite things to the caller.
func (e Exec) ConfigValue(dir, key string) (string, bool) {
	value, err := e.line(dir, "config", "--get", key)
	if err != nil {
		return "", false
	}
	return value, true
}

// `--quiet` turns a revision git cannot find into the empty answer, in place of an error. An unborn
// HEAD reaches here on every fresh repository, and the callers all read the empty answer as "no commit
// yet". `--end-of-options` keeps a revision that begins with a dash from being read as a flag.
func (e Exec) Resolve(dir, rev string) (string, error) {
	out, err := e.run(dir, "rev-parse", "--verify", "--quiet", "--end-of-options", rev)
	if err != nil {
		// Exit 1 with an empty stdout is how `--quiet` says the revision resolves to no object.
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 && strings.TrimSpace(string(out)) == "" {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (e Exec) MergeBase(dir, left, right string) (string, error) {
	return e.line(dir, "merge-base", left, right)
}

func (e Exec) Tracked(dir string, pathspec ...string) ([]string, error) {
	return e.listing(dir, append([]string{"ls-files", "-z", "--full-name"}, pathspecArgs(pathspec)...)...)
}

func (e Exec) Untracked(dir string, pathspec ...string) ([]string, error) {
	args := []string{"ls-files", "--others", "--exclude-standard", "-z", "--full-name"}
	return e.listing(dir, append(args, pathspecArgs(pathspec)...)...)
}

// `--full-tree`, because `ls-tree` on its own answers the subtree of the directory git ran in and names
// it from there. This question is about what a COMMIT holds, and a caller asking it from a
// subdirectory would otherwise get that directory's files under names the repository root does not
// know.
func (e Exec) NamesAt(dir, rev string) ([]string, error) {
	return e.listing(dir, "ls-tree", "-r", "-z", "--full-tree", "--name-only", rev)
}

// `--no-relative`, because `diff.relative=true` in the reader's config names `a.go` for `pkg/a.go` and
// drops every changed file outside the directory git ran in. `--diff-filter=d` drops deletions: every
// caller here reads the file afterwards.
func (e Exec) Changed(dir string, revisions, pathspec []string) ([]string, error) {
	args := []string{"diff", "--name-only", "-z", "--no-ext-diff", "--no-textconv", "--no-color",
		"--no-relative", "--diff-filter=d"}
	args = append(args, revisions...)
	return e.listing(dir, append(args, pathspecArgs(pathspec)...)...)
}

// `--no-ext-diff` and `--no-textconv` go on every diff here. The reader's own git config can turn
// either on, and the answer then becomes a property of their machine.
func (e Exec) ChangedWithStatus(dir string, revisions, pathspec []string) ([]Change, error) {
	args := []string{"diff", "--raw", "-z", "--no-renames", "--no-ext-diff", "--no-textconv",
		"--no-color", "--no-relative"}
	args = append(args, revisions...)
	out, err := e.run(dir, append(args, pathspecArgs(pathspec)...)...)
	if err != nil {
		return nil, err
	}
	return parseRawDiff(string(out))
}

// `--raw -z` alternates a colon-led metadata record and the path it belongs to, both NUL-terminated.
// The record is `:<srcmode> <dstmode> <srcsha> <dstsha> <status>`, so one call gives the destination
// blob and the status letter. A caller reads the new content with no second listing.
func parseRawDiff(out string) ([]Change, error) {
	fields := strings.Split(out, "\x00")
	var changes []Change
	for i := 0; i < len(fields); i++ {
		record := fields[i]
		if record == "" {
			continue
		}
		if !strings.HasPrefix(record, ":") {
			return nil, fmt.Errorf("git printed a raw diff record the port cannot read: %q", record)
		}
		i++
		if i >= len(fields) {
			return nil, fmt.Errorf("git printed the raw diff record %q with no path after it", record)
		}
		parts := strings.Fields(strings.TrimPrefix(record, ":"))
		if len(parts) < 5 {
			return nil, fmt.Errorf("git printed a raw diff record of %d field(s), wanted 5: %q", len(parts), record)
		}
		// git spells an absent side as an all-zero mode and an all-zero object id, and both read as
		// values. The port empties both, so a caller cannot mistake one for a real mode or ask for an
		// object git does not hold.
		changes = append(changes, Change{
			Status:  parts[4],
			Path:    fields[i],
			OldMode: presentOrEmpty(parts[0]),
			NewMode: presentOrEmpty(parts[1]),
			OldBlob: presentOrEmpty(parts[2]),
			Blob:    presentOrEmpty(parts[3]),
		})
	}
	return changes, nil
}

// Patch is the change set as unified diff text, with no default revision: what HEAD means for a caller
// that named none is the caller's policy. `--find-renames` reports a moved file as the rename it is. A
// rename otherwise arrives as a delete and an add, with every line of the moved file counted as new
// work.
func (e Exec) Patch(dir string, revisions, pathspec []string) ([]byte, error) {
	// `--text` keeps the body readable. One NUL byte in a file collapses it to "Binary files … differ",
	// and a `* -diff` attribute in the branch's own `.gitattributes` collapses it as well. A scan over
	// that body finds no added lines and exits 0 over a real hit. The prefix, colour and ext-diff flags
	// hold the parser's other anchors against `diff.noprefix`, `color.diff=always` and a diff driver.
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-relative",
		"--text", "--src-prefix=a/", "--dst-prefix=b/", "--find-renames"}
	args = append(args, revisions...)
	return e.run(dir, append(args, pathspecArgs(pathspec)...)...)
}

// An all-zero mode or object id is how git spells an absent side, and this returns "" for it.
func presentOrEmpty(field string) string {
	if strings.Trim(field, "0") == "" {
		return ""
	}
	return field
}

func (e Exec) Status(dir string) ([]string, error) {
	out, err := e.run(dir, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, err
	}
	var entries []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			entries = append(entries, line)
		}
	}
	return entries, nil
}

// `--no-textconv` keeps the answer the file's own bytes. A `diff` attribute with a textconv filter
// behind it comes from whoever wrote the branch, and git with that filter on returns the filter's
// rendering of the file. Every caller here reads content to measure or to compare, and the reader's
// own config must not decide what they measure.
func (e Exec) Show(dir, rev, path string) ([]byte, error) {
	return e.run(dir, "show", "--no-textconv", rev+":"+path)
}

// ContentsAt reads the whole list in one process. `cat-file --batch` takes object names on stdin and
// answers in the order it was asked, so the caller's own slice is the index into what comes back. A
// `--batch-check` pass ahead of it would learn every size from a second process, and a spawn on this
// machine costs more than piping the occasional oversized blob costs.
func (e Exec) ContentsAt(dir, rev string, paths []string, maxBytes int64, visit func(string, []byte)) error {
	if len(paths) == 0 {
		return nil
	}
	// `-z` on the input, because a path holding a newline would otherwise arrive as two object names and
	// neither of them names a file. It moves the input side alone, and `-Z` moves both but wants a newer
	// git than macOS ships. The output side needs no help, since every terminator read here is one git
	// wrote.
	args := []string{"cat-file", "--batch", "-z"}
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	if e.Env != nil {
		cmd.Env = e.Env
	}
	var asked bytes.Buffer
	for _, path := range paths {
		asked.WriteString(rev + ":" + path + "\x00")
	}
	// The list goes into a buffer, since os/exec copies a non-file stdin from a goroutine of its own. An
	// inline write would deadlock against a git already blocked on a full stdout.
	cmd.Stdin = &asked
	var said bytes.Buffer
	cmd.Stderr = &said
	out, err := cmd.StdoutPipe()
	if err != nil {
		return gitError(args, said.String(), err)
	}
	if err := cmd.Start(); err != nil {
		return gitError(args, said.String(), err)
	}
	readErr := readBatch(bufio.NewReader(out), paths, maxBytes, visit)
	if readErr != nil {
		// The pipe is drained before Wait, because git blocks on a stdout that goes unread and Wait
		// would then never return.
		_, _ = io.Copy(io.Discard, out)
	}
	if err := cmd.Wait(); err != nil {
		return gitError(args, said.String(), err)
	}
	return readErr
}

// readBatch takes one answer per path, in the order the paths were asked. An object git is about to
// write arrives as its header, its bytes and one terminator. A path the revision does not hold arrives
// as the header alone. The size in that header is what lets one pass step over a large object.
func readBatch(from *bufio.Reader, paths []string, maxBytes int64, visit func(string, []byte)) error {
	for _, path := range paths {
		kind, size, err := batchHeader(from)
		if err != nil {
			return fmt.Errorf("git cat-file --batch, reading the answer for %q: %w", path, err)
		}
		if kind == "" {
			continue
		}
		// Both branches consume the terminator git writes after the bytes, so the next header lands at
		// the start of the next path's answer. An object stepped over is still written in full.
		if kind != "blob" || size > maxBytes {
			if _, err := io.CopyN(io.Discard, from, size+1); err != nil {
				return fmt.Errorf("git cat-file --batch, stepping over the answer for %q: %w", path, err)
			}
			continue
		}
		content := make([]byte, size+1)
		if _, err := io.ReadFull(from, content); err != nil {
			return fmt.Errorf("git cat-file --batch, reading %d byte(s) for %q: %w", size, path, err)
		}
		visit(path, content[:size])
	}
	return nil
}

// A header is `<object id> SP <type> SP <size>`, or the name echoed back with " missing" after it. The
// echo carries the path VERBATIM, newline and all. A reader taking one line as one header would lose
// its place after an absent path with an odd name. This reads lines until one of the two shapes ends,
// and an empty type is that absence.
func batchHeader(from *bufio.Reader) (kind string, size int64, err error) {
	for {
		line, readErr := from.ReadString('\n')
		if readErr != nil {
			return "", 0, readErr
		}
		if strings.HasSuffix(line, " missing\n") {
			return "", 0, nil
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || !isObjectID(fields[0]) {
			continue
		}
		length, parseErr := strconv.ParseInt(fields[2], 10, 64)
		if parseErr != nil || length < 0 {
			return "", 0, fmt.Errorf("git printed the header %q, whose size is no size", strings.TrimRight(line, "\n"))
		}
		return fields[1], length, nil
	}
}

// A header's first field is long enough to be a hash and hexadecimal, which no fragment of an echoed
// path looks like.
func isObjectID(field string) bool {
	return len(field) >= 40 && strings.Trim(field, "0123456789abcdef") == ""
}

func (e Exec) Blob(dir, id string) ([]byte, int64, error) {
	said, err := e.line(dir, "cat-file", "-s", id)
	if err != nil {
		return nil, 0, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(said), 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("git cat-file -s %s answered %q, which is no size", id, said)
	}
	content, err := e.run(dir, "cat-file", "blob", id)
	return content, size, err
}

// Every path in one call. `check-ignore` exits 1 where it matched no path, and that is an answer. Only
// exit 2 and above reaches the caller as a failure.
func (e Exec) Ignored(dir string, paths []string) (map[string]bool, error) {
	ignored := map[string]bool{}
	if len(paths) == 0 {
		return ignored, nil
	}
	cmd := exec.Command("git", "-C", dir, "-c", "core.quotePath=false", "check-ignore", "-z", "--stdin")
	if e.Env != nil {
		cmd.Env = e.Env
	}
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	var out, said bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &said
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, gitError([]string{"check-ignore", "--stdin"}, said.String(), err)
		}
	}
	for _, name := range strings.Split(out.String(), "\x00") {
		if name != "" {
			ignored[name] = true
		}
	}
	return ignored, nil
}

// `check-ignore` exits 1 for a path no rule ignores. This returns empty with a nil error there.
func (e Exec) IgnoreSource(dir, path string) (string, error) {
	out, err := e.run(dir, "check-ignore", "-v", path)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (e Exec) Worktrees(dir string) ([]Worktree, error) {
	out, err := e.run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var listed []Worktree
	for _, block := range strings.Split(string(out), "\n\n") {
		var one Worktree
		found := false
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				one.Path, found = strings.TrimPrefix(line, "worktree "), true
			case strings.HasPrefix(line, "HEAD "):
				one.Head = strings.TrimPrefix(line, "HEAD ")
			case line == "bare":
				one.Bare = true
			// git writes `prunable <reason>`, so this matches on the prefix.
			case strings.HasPrefix(line, "prunable"):
				one.Prunable = true
			}
		}
		if found {
			listed = append(listed, one)
		}
	}
	return listed, nil
}

func (e Exec) Add(dir string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := e.run(dir, append([]string{"add", "--"}, paths...)...)
	return err
}

// `--` goes on whether or not a pathspec follows it. A file called HEAD in the working tree otherwise
// makes `git diff HEAD` ambiguous. The branch under review can commit that file and switch the tool
// off for everyone reviewing it.
func pathspecArgs(pathspec []string) []string {
	return append([]string{"--"}, pathspec...)
}

// The NUL-separated listings, empty entries dropped.
func (e Exec) listing(dir string, args ...string) ([]string, error) {
	out, err := e.run(dir, args...)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// WithoutGitLocation returns the caller's environment with the two variables that relocate git's idea
// of the repository taken out. git reads them before the directory it was handed, so a tool run from a
// hook answers about the hook's repository whatever directory it was asked about. A consumer keying a
// directory name off that answer creates, writes and later removes directories under the wrong name.
func WithoutGitLocation(environ []string) []string {
	// Exactly two. GIT_WORK_TREE moves `--show-toplevel` and leaves the store alone, so a caller declaring
	// a work tree gets the question it meant. GIT_OBJECT_DIRECTORY and GIT_DISCOVERY_ACROSS_FILESYSTEM
	// name no other repository, since discovery still lands on an ancestor that holds the path.
	// GIT_CEILING_DIRECTORIES only stops discovery, so honouring it costs a refusal.
	relocates := map[string]bool{"GIT_DIR": true, "GIT_COMMON_DIR": true}
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if name, _, found := strings.Cut(entry, "="); !found || !relocates[name] {
			kept = append(kept, entry)
		}
	}
	return kept
}

// Environ is os.Environ with HOME replaced, for a caller running against a HOME of its own.
func Environ(home string) []string {
	kept := []string{"HOME=" + home}
	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); name != "HOME" {
			kept = append(kept, entry)
		}
	}
	return kept
}
