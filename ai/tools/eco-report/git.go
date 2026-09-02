package ecoreport

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	"kk-flavor/tools/shell"
)

// Every git call the tool makes, and the one ignore mechanism it writes through — `.gitignore`, via
// appendLine, for `promote`. The `.git/info/exclude` mentions below are all READS that classify an
// ignore source; the code that wrote that file went with the in-tree layout, and what removes a stale
// entry now lives in migrate.go.
//
// git is asked rather than reimplemented everywhere it answers a question about ignoring, tracking or
// worktrees: the answer has to be the one git will give the human's next command, not this tool's model
// of it.

// One child process, with the invocation's directory and HOME. HOME matters to more than the
// fingerprint path: git reads its global config out of it, so a run pointed at another HOME has to
// point its children there too or they answer from a config the caller replaced.
func (r *run) command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	if r.home != os.Getenv("HOME") {
		cmd.Env = append(os.Environ(), "HOME="+r.home)
	}
	return cmd
}

// `$(cmd)`: stdout captured with its trailing newlines stripped, plus the exit status. A nil stderr
// is `2>/dev/null`; r.errOut is the inherited stderr, where the child's own account of a failure is
// part of what the caller reports.
func (r *run) capture(stderr io.Writer, name string, args ...string) (string, int) {
	cmd := r.command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = stderr
	status := exitStatus(cmd.Run())
	return strings.TrimRight(out.String(), "\n"), status
}

// `git -C "$root" …`, which is every git call but the first.
func (r *run) captureGit(stderr io.Writer, args ...string) (string, int) {
	return r.capture(stderr, "git", append([]string{"-C", r.root}, args...)...)
}

// A child whose output is the caller's: `git add` reports what it could not stage, and that account
// is the whole of what the human gets when staging fails.
func (r *run) passThrough(name string, args ...string) int {
	cmd := r.command(name, args...)
	cmd.Stdout = r.out
	cmd.Stderr = r.errOut
	return exitStatus(cmd.Run())
}

// 127 for anything that never ran, which is the status a shell reports for a command it could not
// execute — and, like every non-zero here, one no caller reads as a result.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return 127
}

// Absolute path to <name> in this worktree's git dir. --git-path answers relative for an ordinary
// repo, absolute in a linked worktree; an empty answer would build a bare "$root/" in the tree.
func (r *run) gitPath(name string) string {
	path, status := r.captureGit(r.errOut, "rev-parse", "--git-path", name)
	if status != 0 || path == "" {
		r.refuse("error: could not resolve '" + name + "' inside the git dir (git rev-parse --git-path)")
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return r.root + "/" + path
}

func (r *run) repoMode() string {
	tracked, _ := r.captureGit(nil, "ls-files", ".idsd")
	if tracked != "" {
		return "committed"
	}
	return "throwaway"
}

// The two answers are not equally safe: `discard`'s guard is `committed`, so a failed read falling
// through to `throwaway` deletes a tracked .idsd/. That is why this is a separate assertion rather
// than a refusal inside repoMode, which every call site reads as a value.
func (r *run) assertRepoModeReadable() {
	if _, status := r.captureGit(nil, "ls-files", ".idsd"); status != 0 {
		r.refuse("error: could not read the index (git ls-files .idsd) — the repo mode is unknown, and it decides whether .idsd/ is the durable record or scratch to delete")
	}
}

// Which file git read to ignore a path, or empty when nothing ignores it. `-v` because the answer is
// the whole question here: `core.excludesFile` and `.git/info/exclude` satisfy the plain `-q` form
// too, and each caller below accepts a different set of sources.
func (r *run) ignoreSourceOf(path string) string {
	answer, _ := r.captureGit(nil, "check-ignore", "-v", path)
	first, _, _ := strings.Cut(answer, "\n")
	source, _, _ := strings.Cut(first, ":")
	return source
}

// Ignored has to mean ignored by something that travels with the repository. `core.excludesFile` and
// `.git/info/exclude` are one machine's, so they answer the plain `-q` question while ignoring nothing
// on anybody else's clone, and the next `git add -A` there stages the report. Returns the source it
// read, so a caller can name it.
//
// `.git/info/exclude` is rejected here, which it once was not: throwaway mode wrote the scratch
// exclusion into that file, and this predicate had to accept its own work. Nothing writes it any more,
// and the only caller left is committed mode — where a rule no clone can see is exactly what must not
// pass.
//
// Absolute paths are rejected before `*/.gitignore` is matched, or `core.excludesFile=~/.gitignore` —
// the common global setup — passes as a repo-relative rule.
func (r *run) ignoredSourceTravels(path string) (string, bool) {
	source := r.ignoreSourceOf(path)
	switch {
	case strings.HasPrefix(source, "/"):
		return source, false
	case source == ".git/info/exclude" || strings.HasSuffix(source, "/.git/info/exclude"):
		return source, false
	case source == ".gitignore" || strings.HasSuffix(source, "/.gitignore"):
		return source, true
	}
	return source, false
}

// `check-ignore` is the documented first step, and this is the assertion that it happened. A report
// written where git does not ignore it sits inside the tree it fingerprints, so `state` answers
// `re-qualify` straight after a complete four-stage stamp and `gate` blocks on freshness with nothing
// that can clear it. One predicate for every caller, or `check-ignore` asks a weaker question than
// `init` enforces and the remedy `init` names cannot satisfy it.
func (r *run) assertReportsDirIsIgnored() {
	// Outside the tree, git ignores nothing because git contains nothing: the requirement is met by
	// the location itself rather than by an ignore rule, and asking check-ignore about a path the repo
	// does not hold would refuse every throwaway init. The location is asserted instead, which is the
	// stronger of the two — an ignore entry can be edited away, a path outside the tree cannot.
	if r.idsdDir != r.treeIdsdDir() {
		r.assertScratchIsUnreachableByGit()
		return
	}
	source, travels := r.ignoredSourceTravels(r.reportsDir + "/")
	if travels {
		return
	}
	readNote := ""
	if source != "" {
		readNote = "  A global core.excludesFile does not count — it belongs to this machine alone, so a clone would commit the report. Source read: " + source
	}
	r.refuse("error: nothing in this repository ignores "+r.reportsDir+" — the report was NOT initialized.",
		"  Run report.sh check-ignore first; it is what excludes the scratch, by the mechanism that fits the repo mode.",
		"  Written here, the report would sit inside its own fingerprint, so every stamp would be stale on arrival.",
		readNote)
}

// What must never be committed or fingerprinted, one path per line relative to the root. `promote`
// writes a .gitignore entry per line and `check-ignore` verifies one per line, so the two cannot
// disagree. The durable record is deliberately absent — committed mode keeps it tracked.
//
// The whole directory, never a path per report: the next intent's report does not exist when `promote`
// runs, so an entry per file would leave it tracked.
//
// Built from the IN-TREE layout, not from r.reportsDir, which in throwaway mode is outside the tree
// and would make the trim a no-op — putting an absolute path into .gitignore, where it matches
// nothing while both writer and verifier agree it is fine. These entries describe where the reports
// land once the directory IS in the tree, which is the only state either caller is about.
func (r *run) ignoreSurface() []string {
	return []string{".idsd/qualify-reports/"}
}

// The trailing-newline check is the point: appending to a file whose last line has none fuses the
// two, and then neither the human's own rule nor the entry just added matches anything.
func appendLine(file, entry string) error {
	if content, err := os.ReadFile(file); err == nil {
		for _, line := range shell.SplitLines(string(content)) {
			if line == entry {
				return nil
			}
		}
	}
	handle, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	defer handle.Close()
	if isNonEmptyFile(file) && !endsWithNewline(file) {
		if _, err := handle.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = handle.WriteString(entry + "\n")
	return err
}

// `[ -n "$(tail -c 1 "$file")" ]` inverted: a last byte that is a newline, or a file too short to
// have one.
func endsWithNewline(file string) bool {
	handle, err := os.Open(file)
	if err != nil {
		return false
	}
	defer handle.Close()
	info, err := handle.Stat()
	if err != nil || info.Size() == 0 {
		return true
	}
	last := make([]byte, 1)
	if _, err := handle.ReadAt(last, info.Size()-1); err != nil {
		return false
	}
	return last[0] == '\n'
}
