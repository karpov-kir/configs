package ecoreport

import (
	"os"
	"path/filepath"
	"strings"

	"configs/ai/tools/shell"
)

// Every repository question the tool asks, plus the single ignore mechanism it writes through:
// `.gitignore`, via appendLine, for `promote`. Every `.git/info/exclude` mention in this file is a
// READ that classifies an ignore source. The code that removes a stale entry lives in migrate.go.

// Questions about ignoring, tracking or worktrees go to git itself. The answer has to match what git
// will tell the human's next command, and a model of git kept inside this tool can disagree with it.
// `ai/tools/repo` names those questions, and r.git answers them.

type gitAnswer struct {
	value string
	err   error
}

// The repository questions that cannot change while one invocation runs, asked once each.
//
// ONE of them can move under us after all, and that is why this is a memo rather than a package
// cache: `git add` changes the index, so `stagedIndex` clears the entry instead of leaving a stale
// "external" behind a tree that is now committed. `discard` reads that answer before deleting, so a
// stale one is not a slow report — it is the wrong one, over a tracked .idsd/.
func (r *run) askOnce(question string, ask func() (string, error)) (string, error) {
	if hit, ok := r.gitMemo[question]; ok {
		return hit.value, hit.err
	}
	value, err := ask()
	if r.gitMemo == nil {
		r.gitMemo = map[string]gitAnswer{}
	}
	r.gitMemo[question] = gitAnswer{value, err}
	return value, err
}

func (r *run) forgetIndexAnswers() {
	r.gitMemo = nil
}

// Absolute path to <name> in this worktree's git dir. An empty answer would build a bare "$root/" in
// the tree, so gitPath refuses it.
func (r *run) gitPath(name string) string {
	if gitDir, ok := layoutGitDir(r.root); ok {
		return filepath.Join(gitDir, name)
	}
	path, err := r.askOnce("git-path "+name, func() (string, error) { return r.git.GitPath(r.root, name) })
	if err != nil || path == "" {
		r.sayWhatGitSaid(err)
		r.refuse("error: could not resolve '" + name + "' inside the git dir (git rev-parse --git-path)")
	}
	return path
}

// Whether the index holds anything under .idsd/, memoized: it is what decides the repo mode, and
// several callers read it in one run.
func (r *run) trackedIdsd() (string, error) {
	return r.askOnce("tracked .idsd", func() (string, error) {
		tracked, err := r.git.Tracked(r.root, ".idsd")
		return strings.Join(tracked, "\n"), err
	})
}

func (r *run) repoMode() string {
	tracked, _ := r.trackedIdsd()
	if tracked != "" {
		return "committed"
	}
	return "external"
}

// The two answers are not equally safe: `discard`'s guard is `committed`, so a failed read falling
// through to `external` deletes a tracked .idsd/. That is why this is a separate assertion rather
// than a refusal inside repoMode, which every call site reads as a value.
func (r *run) assertRepoModeReadable() {
	if _, err := r.trackedIdsd(); err != nil {
		r.refuse("error: could not read the index (git ls-files .idsd) — the repo mode is unknown, and it decides whether .idsd/ is tracked in the repo or kept outside it")
	}
}

// git's own account of a failure, beside the refusal that follows it. The child used to inherit
// stderr, so what git said reached the human. The port hands it back as an error instead, and these
// are the call sites that used to let it through. One line, because a path git quotes back carries the
// tree's own bytes and a newline in one would forge a second — and what reads these is another agent.
func (r *run) sayWhatGitSaid(err error) {
	if err != nil {
		r.errLines("  git said: " + shell.Oneline(err.Error()))
	}
}

// Which file git read to ignore a path, or empty when nothing ignores it. `-v` because the answer is
// the whole question here: `core.excludesFile` and `.git/info/exclude` satisfy the plain `-q` form
// too, and each caller below accepts a different set of sources.
//
// A matched pattern is not an ignored path, and `-v` does not distinguish them: it reports the LAST
// pattern that matched, negations included, and exits 0 for either. So a `!` rule sitting after the
// entry answers with the entry's own file as its source while git stages the path anyway, and every
// caller here reads that as "ignored, and by something that travels". The pattern is read for that
// reason, not merely the source.
func (r *run) ignoreSourceOf(path string) string {
	answer, _ := r.git.IgnoreSource(r.root, path)
	first, _, _ := strings.Cut(answer, "\n")
	// `<source>:<line>:<pattern>\t<pathname>`.
	source, afterSource, _ := strings.Cut(first, ":")
	_, pattern, _ := strings.Cut(afterSource, ":")
	if matched, _, _ := strings.Cut(pattern, "\t"); strings.HasPrefix(matched, "!") {
		return ""
	}
	return source
}

// Ignored has to mean ignored by something that travels with the repository. `core.excludesFile` and
// `.git/info/exclude` are one machine's: they satisfy the plain `-q` question while ignoring nothing on
// anybody else's clone, where the next `git add -A` stages the report. Absolute paths are rejected
// before `*/.gitignore` is matched, or `core.excludesFile=~/.gitignore` passes as a repo-relative rule.
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
// `re-qualify` straight after a complete four-stage stamp and `gate` blocks on freshness with
// nothing to clear it. One predicate for every caller, or `init`'s own remedy cannot satisfy it.
func (r *run) assertReportIsIgnored() {
	// Outside the tree, git ignores nothing because git contains nothing: the requirement is met by
	// the location itself rather than by an ignore rule, and asking check-ignore about a path the repo
	// does not hold would refuse every external init. The location is asserted instead, which is the
	// stronger of the two — an ignore entry can be edited away, a path outside the tree cannot.
	if r.idsdDir != r.treeIdsdDir() {
		r.assertScratchIsUnreachableByGit()
		return
	}
	source, travels := r.ignoredSourceTravels(r.report)
	if travels {
		return
	}
	readNote := ""
	if source != "" {
		readNote = "  A global core.excludesFile does not count — it belongs to this machine alone, so a clone would commit the report. Source read: " + source
	}
	r.refuse("error: nothing in this repository ignores "+r.report+" — the report was NOT initialized.",
		"  Run report.sh check-ignore first; it is what keeps the record out of the tree, by the mechanism that fits the repo mode.",
		"  Written here, the report would sit inside its own fingerprint, so every stamp would be stale on arrival.",
		readNote)
}

// What must never be committed or fingerprinted, one path per line relative to the root. `promote`
// writes a .gitignore entry per line and `check-ignore` verifies one per line, so the two cannot
// disagree. The durable record is deliberately absent — committed mode keeps it tracked.
//
// A pattern per ignorable file, never the directory: intents/<slug>/ also holds intent.md, which is
// the durable record and must stay tracked. The `*` is what covers the ship folders that do not exist
// yet when `promote` runs.
//
// Built from the IN-TREE layout, never from a resolved path, which in external mode is outside the
// tree — that would put an absolute path into .gitignore, where it matches nothing while both writer
// and verifier agree it is fine. These entries describe where the files land once the directory IS in
// the tree, which is the only state either caller is about.
func ignoreSurface() []string {
	return []string{
		".idsd/intents/*/for-agents/decisions.md",
		".idsd/intents/*/for-agents/language.md",
		".idsd/intents/*/for-agents/playbook.md",
		".idsd/intents/*/for-agents/" + reportName,
	}
}

// A path the entry would match, for the callers that ask git whether an entry took effect.
// `git check-ignore` reads its argument as a literal pathname rather than as a glob, so an entry
// holding `*` can only be verified through a path it covers — asked about the pattern itself, git
// answers about a directory named `*`, which nothing creates and every rule fails to match. The
// segment is deliberately not a slug any ship could take.
const ignoreProbeSegment = "__probe__"

func ignoreProbe(entry string) string {
	return strings.ReplaceAll(entry, "*", ignoreProbeSegment)
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
	if !endsWithNewline(file) {
		if _, err := handle.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = handle.WriteString(entry + "\n")
	return err
}

// An empty file counts as ending in one: it is the file appendLine has just created, and a separator
// written into one would put the first entry under a blank line.
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
