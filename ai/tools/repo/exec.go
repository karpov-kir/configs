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

// Exec answers by running git. It is what every command in this module wires in; only the suites
// substitute anything else.
//
// Env, when set, replaces the child's environment entirely. Two callers need it and for opposite
// reasons: eco-report points HOME at the invocation's own, because git reads its global config from
// there and a run given another HOME must not answer from the caller's; repo-key strips GIT_DIR and
// GIT_WORK_TREE, because git reads the repository's location from the environment before it reads the
// directory it was handed, so a hook in a linked worktree would key its own clone.
type Exec struct {
	Env []string
}

// Git is the interface Exec satisfies. Stated as an assignment so the compiler reports a drift here
// rather than at the first caller that wires one in.
var _ Git = Exec{}

// Every call goes through here, so the flags that must never be left off are left off nowhere.
//
//   - `-C dir` rather than cmd.Dir, so the path appears in the error git prints.
//   - `core.quotePath=false` with `-z`: without either, a path holding a non-ASCII byte comes back
//     C-quoted or newline-split, and the caller reads a name no file has.
//   - `--no-ext-diff` and `--no-textconv` on anything that diffs: both are things the reader's own git
//     config can turn on, and either makes the answer a property of the machine.
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

// The one-line answers. Trailing newline off, since every caller of these wants the value.
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

// git exits non-zero when the key is unset, which is the whole of what separates "unset" from "set to
// the empty string" — and those two mean opposite things to the caller.
func (e Exec) ConfigValue(dir, key string) (string, bool) {
	value, err := e.line(dir, "config", "--get", key)
	if err != nil {
		return "", false
	}
	return value, true
}

// `--quiet`, so a revision naming nothing is the empty answer git gives rather than an error. An
// unborn HEAD reaches here on every fresh repository, and the callers all read it as "no commit yet".
// `--end-of-options` so a revision beginning with a dash is a revision and not a flag.
func (e Exec) Resolve(dir, rev string) (string, error) {
	out, err := e.run(dir, "rev-parse", "--verify", "--quiet", "--end-of-options", rev)
	if err != nil {
		// Exit 1 with nothing on stdout is `--quiet`'s way of saying the revision names nothing.
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
// The record is `:<srcmode> <dstmode> <srcsha> <dstsha> <status>`, so the destination blob and the
// letter come out of one call and a caller needs no second listing to read the new content.
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
		// git spells "this side does not exist" as an all-zero mode and an all-zero object id, which
		// reads as a value rather than as an absence. Emptied here so a caller cannot mistake one for a
		// real mode or ask for an object that is not there.
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

// `--text`, and it is the load-bearing flag: one NUL byte in a file, or a `* -diff` attribute written
// by whoever wrote the branch, collapses the body to "Binary files … differ" and a scan reading this
// exits 0 over a real hit. `--src-prefix`/`--dst-prefix` against `diff.noprefix`, `--no-color` against
// `color.diff=always`, `--no-ext-diff` against an external driver, and the `core.quotePath=false`
// `run` already supplies against a non-ASCII path arriving C-quoted — each is a parser's anchor that
// the reader's own config would otherwise move.
//
// No default revision here. "What HEAD means when the caller named nothing" is the caller's policy,
// and a port that decided it would be answering a question nobody asked.
func (e Exec) Patch(dir string, revisions, pathspec []string) ([]byte, error) {
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-relative",
		"--text", "--src-prefix=a/", "--dst-prefix=b/"}
	args = append(args, revisions...)
	return e.run(dir, append(args, pathspecArgs(pathspec)...)...)
}

// An all-zero mode or object id is git's way of saying the side is absent, not a value to use.
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

func (e Exec) Show(dir, rev, path string) ([]byte, error) {
	return e.run(dir, "show", "--textconv", rev+":"+path)
}

// One process for the whole list. `cat-file --batch` reads object names on stdin and answers in the
// order it was asked, so the caller's own slice is the index into what comes back.
//
// `-z` on the input, because a path holding a newline would otherwise arrive as two object names and
// neither names a file. It moves the input side only; `-Z`, which moves both, wants a newer git than
// macOS ships, and the output side needs no help because every terminator this reads is one git wrote.
//
// No `--batch-check` pass ahead of it. That would learn every size from a second process to save
// piping the occasional oversized blob, and a spawn on this machine costs more than the piping does —
// the header `--batch` prints before each object already carries the size, which is what lets one pass
// step over a large one instead of handing it over.
func (e Exec) ContentsAt(dir, rev string, paths []string, maxBytes int64, visit func(string, []byte)) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"cat-file", "--batch", "-z"}
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	if e.Env != nil {
		cmd.Env = e.Env
	}
	var asked bytes.Buffer
	for _, path := range paths {
		asked.WriteString(rev + ":" + path + "\x00")
	}
	// A buffer rather than a pipe written here: os/exec copies a non-file stdin from a goroutine of its
	// own, and writing the list inline would deadlock against a git already blocked on a full stdout.
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
		// Drained before waiting: git blocks on a stdout nobody is reading, and Wait would then never
		// return.
		_, _ = io.Copy(io.Discard, out)
	}
	if err := cmd.Wait(); err != nil {
		return gitError(args, said.String(), err)
	}
	return readErr
}

// One answer per path asked, read in that order. An object git is about to write arrives as its header,
// its bytes and one terminator; a path the revision does not hold arrives as the header alone.
func readBatch(from *bufio.Reader, paths []string, maxBytes int64, visit func(string, []byte)) error {
	for _, path := range paths {
		kind, size, err := batchHeader(from)
		if err != nil {
			return fmt.Errorf("git cat-file --batch, reading the answer for %q: %w", path, err)
		}
		if kind == "" {
			continue
		}
		// Both branches consume the terminator git writes after the bytes, so the next header begins
		// where the next path's answer does. An object stepped over is still written in full.
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
// echo carries the path VERBATIM, newline and all, so a reader taking one line as one header loses its
// place for every object after an absent path with an odd name — hence lines until one of the two
// shapes ends. An empty type is that absence.
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

// Long enough to be a hash and hexadecimal: the one thing in a header that a fragment of an echoed
// path will not look like.
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

// Every path in one call. `check-ignore` exits 1 when it matched nothing, which is an answer and not a
// failure, so only exit 2 and above reaches the caller as one.
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

// Empty and no error where the path is not ignored, which is `check-ignore`'s exit 1.
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
			// git writes `prunable <reason>`, so the prefix and not the whole word.
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

// `--` goes on whether or not a pathspec follows it. Without it, a file called HEAD in the working
// tree makes `git diff HEAD` ambiguous, and the branch under review can commit that file and switch
// the tool off for everyone reviewing it.
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

// WithoutGitLocation is the caller's environment with the two variables that relocate git's idea of
// the repository removed. git reads them before it reads the directory it was handed, so a process run
// from a git hook — which is given them — would otherwise answer about the hook's repository whatever
// directory it was asked about, and a consumer keying a directory off that answer would create, write
// and later remove directories under the wrong name.
//
// Exactly two. GIT_WORK_TREE moves `--show-toplevel` but not the store, and a tool asked about a
// directory inside a work tree its caller declared is being asked the question its caller meant.
// GIT_OBJECT_DIRECTORY and GIT_DISCOVERY_ACROSS_FILESYSTEM name no other repository either: discovery
// across a mount boundary still has to land on an ancestor that genuinely holds the path. Stripping
// any of them would read as a guard while guarding nothing.
//
// GIT_CEILING_DIRECTORIES stays on purpose. Set on the repository's own root it stops discovery rather
// than redirecting it, so honouring it costs a refusal and never a wrong answer.
func WithoutGitLocation(environ []string) []string {
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
