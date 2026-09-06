package ecoreport

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// Which report an invocation acts on, and what else on disk belongs to the ship it names. Every path
// built here is built from a slug, so the charset check in reportNameFor is the whole of what keeps a
// write inside intents/.

// The report's filename stem for an intent frontmatter value. Empty means unusable, and the caller
// refuses: the value reaches a path here, so the slug charset is what stops a `../` escaping
// intents/. A standalone review has no slug and shares the one `review` stem, so its folder holds a
// report and the three intent-local records but no intent.md.
func reportNameFor(value string) string {
	// Leading whitespace goes BEFORE the truncation. Without that, a value starting with a space
	// truncates to nothing and takes the `review` arm, while intentSlug strips the same whitespace and
	// recovers the real slug — so the filename and the frontmatter name different ships.
	slug := firstField(trimLeadingSpace(value))
	switch {
	case slug == "" || strings.HasPrefix(slug, "review:"):
		return "review"
	// A leading dot is refused outright, not merely made path-safe, and it guards two things of very
	// different sizes. The small one: reportNames skips a dot-named entry, so such a ship would sit in
	// the directory addressable by its own name and invisible to every discovery path — a report
	// standing open while `state` answers `no-report` and `idsd-ship continue` starts a fresh one over it.
	//
	// The large one. The stem is a DIRECTORY COMPONENT of both paths it builds, so `..` in it climbs out
	// of each. `shipDir("..")` is `intents/..`, which IS the scratch root — and `discard` os.RemoveAlls
	// the ship folder, so that one deletion takes every other ship with it. setReportPaths hands the
	// same stem to `gitPath("idsd-stage-returns/" + name)`, where `idsd-stage-returns/..` IS the git dir
	// — which `invalidate`, `close` and `discard` each os.RemoveAll, taking the index, the objects and
	// the refs. Go permits both: RemoveAll refuses a trailing `.` and not a trailing `..`, so it
	// recurses in.
	case strings.HasPrefix(slug, "."), !isSlugCharset(slug):
		return ""
	}
	return slug
}

// The two paths a stem builds, and they are not equally forgiving — which is why every caller filters
// the name BEFORE this rather than here. No assertion of its own on purpose: reportNameFor's refusal
// and reportNames' skip are the guards, and a second copy of the same predicate here would shadow
// them, leaving nothing able to observe whether either still works.
func (r *run) setReportPaths(name string) {
	r.report = r.shipDir(name) + "/" + reportName
	// Per-pass bookkeeping, in the git dir: no commit and no `git add -A` reaches it, and it is
	// per-worktree so ships cannot collide. Under .idsd/, committed mode would commit them. Keyed by
	// stem, or `invalidate` on one intent would clear another's stage markers and free its stamp.
	r.stageReturnsDir = r.gitPath("idsd-stage-returns/" + name)
}

// The stem is the ship folder's own name now, so this reads the parent rather than trimming a suffix.
func stemOfReportPath(path string) string {
	return shell.BaseName(shell.DirName(path))
}

// One ship's folder.
func (r *run) shipDir(name string) string {
	return r.intentsDir + "/" + name
}

// An archived ship keeps its folder, so the record a build leaves travels as one directory.
func (r *run) archiveDir(name string) string {
	return r.idsdDir + "/archive/" + name
}

// Every report present, one ship-folder name per line. A dot-named folder is skipped below, which is
// why reportNameFor refuses a leading dot rather than sanitising one.
//
// Held to the same slug charset a named intent is. A name from this listing goes on to be printed as a
// `list` row, to name a report path, and — through resolveReport with no argument — to decide what
// `discard` deletes; nothing between here and there checks it. A directory name holds no `/`, so it
// could not traverse, but it can hold a newline: a folder called
// `ev<LF>fakeship<TAB>ready<LF>il` puts a whole forged `fakeship  ready` row into the listing
// `idsd-ship continue` routes on.
//
// A bad name is skipped, not repaired: a name rewritten to something addressable would name a folder
// that is not there. It is counted instead, because a listing quietly short of a ship is the one
// failure this tool must not have.
func (r *run) reportNames() []string {
	entries, err := os.ReadDir(r.intentsDir)
	if err != nil {
		// Absent is "no reports open", which every caller reads correctly — before `init` there is no
		// such directory. Present but unreadable is a different fact wearing the same shape, and it
		// reaches the destructive branch: `survivingContent` reads the empty list as "no other ship is in
		// flight" and `discard` goes on to remove the whole .idsd/, which in throwaway mode is the only
		// copy of a parallel ship's report. The same rule assertRepoModeReadable states.
		if !errors.Is(err, fs.ErrNotExist) {
			r.refuse("error: could not read "+r.intentsDir+" ("+err.Error()+") — which reports are open is unknown.",
				"  That decides what discard deletes and what list shows, so nothing was read as 'no reports'.")
		}
		return nil
	}
	var names []string
	r.unnameableReports = 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.IsDir() {
			continue
		}
		// A ship folder with no report in it is a ship whose report was closed, or one authored before any
		// pass ran. Neither is an open report, and neither is a name to list.
		if !shell.IsRegularFile(r.shipDir(name) + "/" + reportName) {
			continue
		}
		stem := name
		if !isSlugCharset(stem) {
			r.unnameableReports++
			continue
		}
		names = append(names, stem)
	}
	return names
}

// Said out loud by every caller that reports on the set: a report skipped in silence is a ship nothing
// resumes. The name is bounded and sanitised, since it is the one thing here this tool did not write.
func (r *run) noteUnnameableReports() {
	if r.unnameableReports == 0 {
		return
	}
	r.errLines("note: " + strconv.Itoa(r.unnameableReports) + " folder(s) under " + r.intentsDir +
		" are named outside the slug charset ([0-9A-Za-z._-]) and were NOT listed — a report is named after its intent. Rename each, or delete it.")
}

// What resolving an intent name to a report answered. The numbers are inherited exit codes, which is
// why there is no 2; the names are what a call site tests against.
type reportLookup int

const (
	reportResolved  reportLookup = 0
	reportNoneOpen  reportLookup = 1
	reportAmbiguous reportLookup = 3
)

// Resolve which report this invocation acts on: the named one, or the only one present. Never
// guesses; the package header says what a wrong guess costs.
func (r *run) resolveReport(name string) reportLookup {
	if name != "" {
		stem := reportNameFor(name)
		if stem == "" {
			r.refuse("error: '" + name + "' names no report — a report lives in a folder named after the intent, so it must be a slug ([0-9A-Za-z._-]) or a \"review: <description>\"")
		}
		r.setReportPaths(stem)
		return reportResolved
	}
	names := r.reportNames()
	if len(names) == 0 {
		return reportNoneOpen
	}
	if len(names) != 1 {
		r.ambiguousNames = strings.Join(names, "\n")
		return reportAmbiguous
	}
	r.setReportPaths(names[0])
	return reportResolved
}

// The ambiguous outcome, refused with the names listed. The argument is what the caller must do instead:
// `state` sends the human to `list`, the rest only want the name.
func (r *run) refuseAmbiguous(instead string) {
	r.refuse("error: several qualify reports are open — "+instead+":", indentLines(r.ambiguousNames, "  "))
}

// Every frontmatter reader answers empty for a file it could not open, and empty is in isUnstamped's
// set. So an unreadable report answers `resume` with an unstamped tree and a clean stage record, all
// from a file nothing ever opened. Takes the consequence as an argument for the reason
// assertWritePathsAreReal does.
func (r *run) assertReportIsReadable(consequence string) {
	if !isReadable(r.report) {
		r.refuse("error: " + r.report + " cannot be read — " + consequence)
	}
}

// Resolve, or refuse naming what the caller must pass. Every subcommand that reads an existing report
// opens with this, and the optional stem is always its last argument.
func (r *run) requireReport(name string) {
	switch r.resolveReport(name) {
	case reportNoneOpen:
		r.refuse(append([]string{"error: no qualify report under " + r.intentsDir + " — run report.sh init \"<intent>\" first"}, r.noReportNote...)...)
	case reportAmbiguous:
		r.refuseAmbiguous("name which as the last argument")
	}
	if !shell.IsRegularFile(r.report) {
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
	// refusing would leave an empty scratch directory standing in the mode whose contract is zero
	// traces. Safe to let through because it is a fixed literal: unlike a slug, it cannot be a
	// typo of another ship.
	if slug == "review" {
		return
	}
	if shell.IsRegularFile(r.shipDir(slug)+"/"+intentName) || shell.IsRegularFile(r.archiveDir(slug)+"/"+intentName) {
		return
	}
	r.refuse("error: nothing here belongs to '"+slug+"' — nothing was discarded.",
		"  Looked for "+r.report+",",
		"  .idsd/intents/"+slug+"/"+intentName+" and .idsd/archive/"+slug+"/"+intentName+", and found none of them.",
		"  Check the name against report.sh list; a standalone review is discarded before its report is closed, not after.")
}

// What is left under .idsd/ that is not this ship's scratch, as a printable list — empty means
// `discard` may take the whole directory. What counts as remaining is named, never "the .idsd/ root
// is non-empty", so a stray dotfile cannot keep the dir alive. `decisions.md` is deliberately NOT on
// the list — `~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log** makes it throwaway
// scratch by design. `roadmap.md` is off it for its own reason: `~/.claude/skills/idsd-intent/SKILL.md`
// → **Phase 3 — Emit** generates it from the intents' own frontmatter, so whenever it holds anything,
// the intents arm below is already keeping .idsd/ standing for the intents it came from.
//
// Read after this ship's own files are gone, so every count it takes is of what survives.
func (r *run) survivingContent() string {
	kept := ""
	for _, durable := range []string{"charter.md", "constraints.md", "language.md", "playbook.md"} {
		if shell.PathExists(r.idsdDir + "/" + durable) {
			kept += " " + durable
		}
	}
	// A parallel ship's report is another human's work in flight, so it keeps .idsd/ standing.
	// Counted by re-reading intents/ once this ship's folder is gone — the caller's rmdir only
	// tidies the directory when it empties, and its status is discarded.
	if left := len(r.reportNames()); left != 0 {
		kept += " " + strconv.Itoa(left) + " other qualify report(s)"
	}
	// Anything at all under intents/ or archive/ keeps .idsd/ alive, but the label counts what is
	// actually there — "other intents" for a stray `.DS_Store` tells the human something untrue.
	intents, archive := r.intentsDir, r.idsdDir+"/archive"
	if shell.PathExists(intents) || shell.PathExists(archive) {
		if left := countShipFolders(intents, archive); left > 0 {
			kept += " " + strconv.Itoa(left) + " other intent(s)"
		} else {
			kept += " unrecognised content under intents/ or archive/"
		}
	}
	return kept
}

// Ship folders holding an intent, across the directories named. The folder itself must be a real
// directory — the entry type is read without following a link — and the intent inside it must stat as
// a regular file, so a folder holding only this ship's leftover scratch is not counted.
func countShipFolders(dirs ...string) int {
	count := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !shell.IsRegularFile(dir+"/"+entry.Name()+"/"+intentName) {
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
	r.assertScratchDirsAreReal(outcome)
	// The report is never legitimately a symlink, and `--force` does not override this. The write is a
	// staged copy then rename, so it replaces a link instead of following it. What this catches is
	// `--force` destroying whatever link the human left there — including a dangling one, which an
	// existence test cannot even see.
	if shell.IsSymlink(r.report) {
		r.refuse("error: "+r.report+" is a symlink -> "+shell.Oneline(readLink(r.report))+" — "+outcome+".",
			"  the report is always a regular file. Remove the link, then re-run.")
	}
}

// The two directories every write goes through, apart from the report itself — because `promote`
// needs exactly this half and has no report resolved when it runs: it moves the whole scratch
// directory and names no single ship. A rename does not follow a link at its source, so a link here
// is MOVED into the tree and staged as a symlink blob rather than followed, and `git ls-files` then
// answers "committed" so the promotion reports success.
func (r *run) assertScratchDirsAreReal(outcome string) {
	for _, writeDir := range []string{r.idsdDir, r.intentsDir} {
		if shell.IsSymlink(writeDir) {
			r.refuse("error: "+writeDir+" is a symlink -> "+shell.Oneline(readLink(writeDir))+" — "+outcome+".",
				"  the scratch directory and its intents/ are always real directories. Remove the link, then re-run.")
		}
	}
}

// A link's target is arbitrary text that never has to resolve, so it is the one string in a refusal
// here that nobody in this account wrote. Every caller collapses it through shell.Oneline before
// printing, the way root.go always has: an ESC in a target erases the lines printed above it, and the
// agent reading that output acts on what it shows.
func readLink(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}
