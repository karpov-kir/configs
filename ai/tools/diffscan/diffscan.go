// The half two scanners share: turning a caller's arguments into the ADDED lines of a change set, and
// counting what was and was not read while doing it.
//
// `comment-density` and `dup-literals` ask different questions of those lines — one classifies them,
// the other compares them — but reach them the same way, and every subtlety in getting there is one a
// second copy would drift on: which arguments are refused and why, the git flags that pin the diff's
// shape, what counts as binary, and the anchor that stops a file's own content forging a header. Both
// were shell, both stated this separately, and the two copies had already diverged on `core.quotePath`
// and on the source prefixes.
//
// The denominator is part of the contract, not decoration. An empty report at exit 0 means "nothing
// matched" only when files actually reached the scan, and "nothing was read" when they did not — so
// every caller gets Reached, Countable and SkippedUnread back and must print them.
package diffscan

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Binary is a NUL in the first 8KB, the probe the untracked arm uses.
const binaryProbeBytes = 8192

// The longest diff line a scan will read. A var rather than a const so a suite can drive the refusal
// it causes without a 16MB fixture; nothing in production assigns it.
var MaxDiffLineBytes = 16 * 1024 * 1024

// Options are the parts the two scanners differ on.
type Options struct {
	// MaxFileBytes skips an untracked file larger than this, unread.
	MaxFileBytes int64
	// SkipSecretNamed refuses to read an untracked file whose NAME marks it as secret-bearing. Set by a
	// scanner that echoes file CONTENT back into its report: two .env files sharing one API token is
	// the ordinary case, and the token would print. A scanner that reports only paths leaves it off.
	SkipSecretNamed bool
	// Announce receives a line for each skip that has to be visible rather than merely counted.
	Announce func(string)
}

// Result is what a scan read, and what it did not.
type Result struct {
	// Reached is files the scan opened — the denominator. Zero means this run says nothing about the
	// change set, which is a different statement from "nothing matched".
	Reached int
	// SkippedUnread is files the scan declined without reading: over the byte cap, binary, or named as
	// secret-bearing. They have to reach the tally or the summary claims a denominator it never covered.
	SkippedUnread int
	// BinaryLines is added lines dropped for holding a control byte.
	BinaryLines int
}

// An added line, with the file it belongs to. File is empty for a diff whose header the scan could not
// attribute, which a caller skips.
type AddedLine struct {
	File string
	Text string
}

// RefuseNonRevisions rejects an argument that is a path or an option before any scan runs.
//
// `git diff <path>` is legal and diffs against the INDEX, so a path quietly accepted scans the wrong
// change set and exits 0 — indistinguishable from a clean tree. `--output=` alone drains the diff into
// a file, so the scan sees nothing and exits 0 over a real hit. Both are refused rather than skipped,
// and refused before the scan rather than during it, so the tool fails closed.
func RefuseNonRevisions(self string, args []string, cwd string) error {
	for _, arg := range args {
		if arg == "--" {
			return nil
		}
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("'%s' is an option, not a git-diff revision — the scan did NOT run.\n"+
				"  this script takes revisions only (HEAD, origin/main, a..b); paths go after '--'.", arg)
		}
		if _, err := os.Stat(filepath.Join(cwd, arg)); err != nil {
			continue
		}
		if resolvesAsRevision(cwd, arg) {
			continue
		}
		return fmt.Errorf("'%s' is a path, not a git-diff revision — the scan did NOT run.\n"+
			"  pass a revision (HEAD, origin/main, a..b); paths, if you must, go after '--'.", arg)
	}
	return nil
}

func resolvesAsRevision(cwd, arg string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", arg+"^{}")
	cmd.Dir = cwd
	return cmd.Run() == nil
}

// Diff is the change set's raw diff.
//
// The flags pin the shape the parser keys off — `+++ b/<path>`, a leading `+` — which `diff.noprefix`,
// `color.diff=always` or an external diff driver would break. `core.quotePath=false`, or a non-ASCII
// path arrives C-quoted and fails the `b/` test. `--text`, or one NUL byte, or a `* -diff` written by
// whoever wrote the branch, collapses the body to "Binary files … differ" and the scan exits 0 over a
// real hit.
func Diff(cwd string, revisions []string) ([]byte, error) {
	args := []string{
		"-c", "core.quotePath=false", "diff", "--no-ext-diff", "--no-textconv", "--no-color",
		"--text", "--src-prefix=a/", "--dst-prefix=b/",
	}
	if len(revisions) == 0 {
		args = append(args, "HEAD")
	} else {
		args = append(args, revisions...)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		return nil, errors.New("git rejected these arguments — exit 2, the scan did NOT run. Not a clean result.")
	}
	return out.Bytes(), nil
}

// WalkDiff calls visit for every added line, attributing each to its file.
//
// `diff --git` is the anchor, never `+++` alone: every line in a diff BODY carries a `+`, `-` or space
// prefix, so no file content can forge one. Without it an added line reading `++ b/other` arrives as
// `+++ b/other`, reassigns the file, and every added line after it is counted against a file that is
// not in the change.
//
// A line longer than MaxDiffLineBytes is a refusal, not a truncation: read short, the scan would carry
// on over a diff it had not seen all of and report a clean tree.
func (r *Result) WalkDiff(diff []byte, visit func(AddedLine)) error {
	file := ""
	pending := false
	scanner := bufio.NewScanner(bytes.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), MaxDiffLineBytes)
	for scanner.Scan() {
		raw := scanner.Text()
		switch {
		case strings.HasPrefix(raw, "diff --git "):
			file, pending = "", true
			r.Reached++
		case pending && strings.HasPrefix(raw, "+++ "):
			pending = false
			file = headerPath(strings.TrimPrefix(raw, "+++ "))
		case strings.HasPrefix(raw, "+"):
			text := raw[1:]
			// C0 controls except tab, and DEL. Not `[^[:print:]]`, which would also drop every line
			// holding a UTF-8 character — a non-ASCII identifier, an em dash in a comment.
			if hasControl(text) {
				r.BinaryLines++
				continue
			}
			if file == "" {
				continue
			}
			visit(AddedLine{File: file, Text: text})
		}
	}
	return scanner.Err()
}

// The path out of a `+++ ` field. Unquoted first: git C-quotes a path holding a control character
// whatever `core.quotePath` says, so the parser has to read that form either way. A field that is
// neither a `b/` path nor a quoted one names no file, and its lines are skipped rather than counted
// against whatever came before.
func headerPath(field string) string {
	if strings.HasPrefix(field, `"`) {
		unquoted, err := strconv.Unquote(field)
		if err != nil {
			return ""
		}
		field = unquoted
	}
	if !strings.HasPrefix(field, "b/") {
		return ""
	}
	return field[len("b/"):]
}

// WalkUntracked calls visit for every line of every untracked, un-ignored file, as though each were
// added. Only reached when the caller named no revisions: with revisions the caller asked about two
// commits, and a file in neither is not what they asked about.
func (r *Result) WalkUntracked(cwd string, opts Options, visit func(AddedLine)) error {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	names := strings.Split(string(out), "\x00")
	sort.Strings(names)
	for _, name := range names {
		if name == "" {
			continue
		}
		if opts.SkipSecretNamed && secretNamed(name) {
			if opts.Announce != nil {
				opts.Announce(fmt.Sprintf("skipping untracked '%s' — its name marks it as secret-bearing; it was NOT scanned.", name))
			}
			r.SkippedUnread++
			continue
		}
		full := filepath.Join(cwd, name)
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() || info.Size() > opts.MaxFileBytes {
			r.SkippedUnread++
			continue
		}
		body, err := os.ReadFile(full)
		if err != nil {
			r.SkippedUnread++
			continue
		}
		if isBinary(body) {
			r.SkippedUnread++
			continue
		}
		r.Reached++
		for _, line := range strings.Split(string(body), "\n") {
			if hasControl(line) {
				r.BinaryLines++
				continue
			}
			visit(AddedLine{File: name, Text: line})
		}
	}
	return nil
}

// A scanner that echoes file CONTENT is a route from an untracked secret into the transcript, the
// qualify report, and any PR comment drafted from either.
//
// A skip list rather than a digest of the finding: digesting would protect the secret and destroy the
// tool, because a hash is not something a reader can go and look for. The name-shaped patterns match
// the basename, so a nested `config/.env.local` is caught; `credential` and `secret` match anywhere,
// so a directory named for its contents is caught too.
func secretNamed(name string) bool {
	base := path.Base(name)
	for _, glob := range []string{".env*", "*.pem", "*.key", "id_rsa*", "id_dsa*"} {
		if ok, _ := path.Match(glob, base); ok {
			return true
		}
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "credential") || strings.Contains(lower, "secret")
}

func isBinary(body []byte) bool {
	probe := body
	if len(probe) > binaryProbeBytes {
		probe = probe[:binaryProbeBytes]
	}
	return bytes.IndexByte(probe, 0) >= 0
}

func hasControl(text string) bool {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\t' {
			continue
		}
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}
