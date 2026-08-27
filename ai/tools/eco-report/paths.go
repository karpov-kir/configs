package ecoreport

import (
	"os"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// Which report an invocation acts on, and what else on disk belongs to the ship it names. Every path
// built here is built from a slug, so the charset check in reportNameFor is the whole of what keeps a
// write inside qualify-reports/.

// The report's filename stem for an intent frontmatter value. Empty means unusable, and the caller
// refuses: the value reaches a path here, so the slug charset is what stops a `../` escaping
// qualify-reports/. A standalone review has no slug and shares the one `review` stem.
func reportNameFor(value string) string {
	// Leading whitespace goes BEFORE the truncation. Without that, a value starting with a space
	// truncates to nothing and takes the `review` arm, while intentSlug strips the same whitespace and
	// recovers the real slug — so the filename and the frontmatter name different ships.
	slug := firstField(trimLeadingSpace(value))
	switch {
	case slug == "" || strings.HasPrefix(slug, "review:"):
		return "review"
	// A leading dot is refused outright, not merely made path-safe: the glob in reportNames cannot
	// match one, so `..-qualify-report.md` would sit in the directory addressable by its own name and
	// invisible to every discovery path — a ship whose report stands open while `state` answers
	// `no-report` and `idsd-ship continue` starts a fresh one over it.
	case strings.HasPrefix(slug, "."), !isSlugCharset(slug):
		return ""
	}
	return slug
}

func (r *run) setReportPaths(name string) {
	r.report = r.reportsDir + "/" + name + reportSuffix
	// Per-pass bookkeeping, in the git dir: no commit and no `git add -A` reaches it, and it is
	// per-worktree so ships cannot collide. Under .idsd/, committed mode would commit them. Keyed by
	// stem, or `invalidate` on one intent would clear another's stage markers and free its stamp.
	r.stageReturnsDir = r.gitPath("idsd-stage-returns/" + name)
}

func stemOfReportPath(path string) string {
	return strings.TrimSuffix(shell.BaseName(path), reportSuffix)
}

// Every report present, one filename stem per line. A dotfile is invisible here, as it was to the
// shell's glob — which is why reportNameFor refuses a leading dot rather than sanitising one.
func (r *run) reportNames() []string {
	entries, err := os.ReadDir(r.reportsDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, reportSuffix) {
			continue
		}
		if !shell.IsRegularFile(r.reportsDir + "/" + name) {
			continue
		}
		names = append(names, strings.TrimSuffix(name, reportSuffix))
	}
	return names
}

// Resolve which report this invocation acts on: the named one, or the only one present. 0 = resolved,
// 1 = none present, 3 = several and none named. Never guesses; the package header says what a wrong
// guess costs.
func (r *run) resolveReport(name string) int {
	if name != "" {
		stem := reportNameFor(name)
		if stem == "" {
			r.refuse("error: '" + name + "' names no report — a report file is named after the intent, so it must be a slug ([0-9A-Za-z._-]) or a \"review: <description>\"")
		}
		r.setReportPaths(stem)
		return 0
	}
	names := r.reportNames()
	if len(names) == 0 {
		return 1
	}
	if len(names) != 1 {
		r.ambiguousNames = strings.Join(names, "\n")
		return 3
	}
	r.setReportPaths(names[0])
	return 0
}

// resolveReport's 3, refused with the names listed. The argument is what the caller must do instead:
// `state` sends the human to `list`, the rest only want the name.
func (r *run) refuseAmbiguous(instead string) {
	r.refuse("error: several qualify reports are open — "+instead+":", indentLines(r.ambiguousNames, "  "))
}

// Every path a report has ever lived at, and none of them is this one. These are literal history, so
// they never move with a rename: a repo whose ship was in flight across either change has its report
// here, where nothing looks. The harm is silence — `state` answers no-report and a fresh ship starts
// over live work — so every path that reports finding none says these exist, on stderr, leaving
// `state` printing exactly one token. `promote` is the exception: it refuses for want of anything
// durable, not of a report.
func (r *run) legacyNote() {
	found := ""
	for _, path := range []string{r.root + "/.idsd/ship-report.md", r.root + "/.idsd/ship-reports"} {
		if shell.PathExists(path) {
			found += " " + path
		}
	}
	if found == "" {
		return
	}
	r.errLines(
		"note: nothing reads these any more, and a report left at one is a ship nothing will resume:"+found,
		"  Move what is still live to "+r.reportsDir+"/<intent>-qualify-report.md, or delete it.")
}

// Every frontmatter reader answers empty for a file it could not open, and empty is in isUnstamped's
// set, so an unreadable report answers `resume` with an unstamped tree and a clean stage record, all
// from a file nothing ever opened. Takes the consequence as an argument, like assertWritePathsAreReal:
// each caller's is different, and a refusal naming the wrong one sends the human to the wrong place.
func (r *run) assertReportIsReadable(consequence string) {
	if !isReadable(r.report) {
		r.refuse("error: " + r.report + " cannot be read — " + consequence)
	}
}

// Resolve, or refuse naming what the caller must pass. Every subcommand that reads an existing report
// opens with this, and the optional stem is always its last argument.
func (r *run) requireReport(name string) {
	switch r.resolveReport(name) {
	case 1:
		r.legacyNote()
		r.refuse("error: no qualify report under " + r.reportsDir + " — run report.sh init \"<intent>\" first")
	case 3:
		r.refuseAmbiguous("name which as the last argument")
	}
	if !shell.IsRegularFile(r.report) {
		r.legacyNote()
		r.refuse("error: no qualify report for that intent (" + r.report + ")")
	}
	r.assertReportIsReadable("its state is unknown (permissions?)")
}

// Nothing of the named ship present means there is no ship to discard, whatever the argument says.
// Without this, `discard <any-legal-slug>` deletes at exit 0 and reports "zero traces" — a whole
// .idsd/ in a repo that never used idsd, or one holding only decisions.md. A slug that names a real
// ship still discards it, and must, since that is how a closed ship gets torn down.
func (r *run) assertShipExists(slug string) {
	if shell.IsRegularFile(r.report) {
		return
	}
	// `review` is the one stem with no intent file, so after `close` nothing identifies it, and
	// refusing would leave an empty .idsd/ and its exclusion standing in the mode whose contract is
	// zero traces. Safe to let through because it is a fixed literal: unlike a slug, it cannot be a
	// typo of another ship.
	if slug == "review" {
		return
	}
	if shell.IsRegularFile(r.root+"/.idsd/intents/"+slug+".md") || shell.IsRegularFile(r.root+"/.idsd/archive/"+slug+".md") {
		return
	}
	r.refuse("error: nothing here belongs to '"+slug+"' — nothing was discarded.",
		"  Looked for "+r.report+",",
		"  .idsd/intents/"+slug+".md and .idsd/archive/"+slug+".md, and found none of them.",
		"  Check the name against report.sh list; a standalone review is discarded before its report is closed, not after.")
}

// What is left under .idsd/ that is not this ship's scratch, as a printable list — empty means
// `discard` may take the whole directory. What counts as remaining is named, never "the .idsd/ root
// is non-empty", so a stray dotfile cannot keep the dir alive. `decisions.md` is deliberately NOT on
// the list — `~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log** makes it throwaway
// scratch by design. Read after this ship's own files are gone, so every count it takes is of what
// survives.
func (r *run) survivingContent() string {
	kept := ""
	for _, durable := range []string{"charter.md", "constitution.md", "language.md", "playbook.md"} {
		if shell.PathExists(r.root + "/.idsd/" + durable) {
			kept += " " + durable
		}
	}
	// A parallel ship's report is another human's work in flight, so it keeps .idsd/ standing.
	// Counted by re-reading qualify-reports/ once this ship's report is gone — the caller's rmdir only
	// tidies the directory when it empties, and its status is discarded.
	if left := len(r.reportNames()); left != 0 {
		kept += " " + strconv.Itoa(left) + " other qualify report(s)"
	}
	// Anything at all under intents/ or archive/ keeps .idsd/ alive, but the label counts what is
	// actually there — "other intents" for a stray `.DS_Store` tells the human something untrue.
	intents, archive := r.root+"/.idsd/intents", r.root+"/.idsd/archive"
	if shell.PathExists(intents) || shell.PathExists(archive) {
		if left := countMarkdownFiles(intents, archive); left > 0 {
			kept += " " + strconv.Itoa(left) + " other intent(s)"
		} else {
			kept += " unrecognised content under intents/ or archive/"
		}
	}
	return kept
}

// `find <dirs> -maxdepth 1 -name '*.md' -type f | wc -l`: lstat'd like find's own -type, so a symlink
// to an intent file is not one.
func countMarkdownFiles(dirs ...string) int {
	count := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			count++
		}
	}
	return count
}

// Takes what did not happen, because both callers reach it: `init` writes and `discard` deletes, and
// a refusal naming the wrong one tells the human to look in the wrong place.
//
// A symlink test reads only the *final* component, so every directory a write goes through needs its
// own check: a symlinked `.idsd` slips past the report's test, and every write then lands wherever it
// points.
func (r *run) assertWritePathsAreReal(outcome string) {
	for _, writeDir := range []string{r.root + "/.idsd", r.reportsDir} {
		if shell.IsSymlink(writeDir) {
			r.refuse("error: "+writeDir+" is a symlink -> "+readLink(writeDir)+" — "+outcome+".",
				"  both .idsd/ and its qualify-reports/ are always real directories inside the repo. Remove the link, then re-run.")
		}
	}
	// The report is never legitimately a symlink, and `--force` does not override this. The write is a
	// staged copy then rename, which replaces a link rather than following it — so what this catches is
	// `--force` destroying whatever link the human left there, on a dangling one an existence test
	// cannot even see.
	if shell.IsSymlink(r.report) {
		r.refuse("error: "+r.report+" is a symlink -> "+readLink(r.report)+" — "+outcome+".",
			"  the report is always a regular file. Remove the link, then re-run.")
	}
}

func readLink(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}
