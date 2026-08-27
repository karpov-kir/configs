package ecoreport

import (
	"os"

	"kk-flavor/tools/shell"
)

// The scratch directory's whole life: excluded, promoted to durable, or torn down. Two of the three
// are destructive — `discard` removes files the throwaway mode keeps no copy of anywhere, and
// `promote` writes to the human's index — so every refusal here stands between this tool and work
// nothing else can recover.

func (r *run) cmdCheckIgnore() {
	// Runs before anything else (init included, so it never requires the report to exist), and before
	// any fingerprinting `git add -A`, so nothing scratch is ever staged.
	r.assertRepoModeReadable()
	if r.repoMode() == "committed" {
		// A path already tracked also answers "not ignored" here — the case most worth the warning.
		// Asked through the same predicate `init` enforces, or this prints ok where init then refuses
		// and sends the human back to this command.
		unignored := ""
		for _, entry := range r.ignoreSurface() {
			if _, travels := r.ignoredSourceTravels(r.root + "/" + entry); !travels {
				unignored += " '" + entry + "'"
			}
		}
		if unignored == "" {
			r.line("ok: qualify-reports/ is gitignored (committed idsd repo)")
			r.exit(0)
		}
		r.errLines("WARN: NOT gitignored:" + unignored + " — add each to .gitignore (shared idsd setup)")
		r.exit(1)
	}
	if err := r.addLocalExclusion(); err != nil {
		r.refuse("error: could not add '.idsd/' to " + r.gitPath("info/exclude") + " — the scratch dir is NOT excluded")
	}
	r.line("ok: throwaway run — .idsd/ excluded locally via .git/info/exclude (.gitignore untouched)")
}

func (r *run) cmdPromote() {
	// Promotion is about the whole .idsd/, so it names no single report — it only needs one to exist,
	// as the evidence that a ship happened here.
	if len(r.reportNames()) == 0 {
		r.refuse("error: no qualify report under " + r.reportsDir + " — nothing to promote")
	}
	r.assertRepoModeReadable()
	if r.repoMode() == "committed" {
		r.line("already committed — .idsd/ is tracked; nothing to promote")
		r.exit(0)
	}
	gitignore := r.root + "/.gitignore"
	// Refuse a symlinked .gitignore before anything is written: afterwards the append has already
	// landed in whatever the link points at, and no refusal below undoes that write.
	if shell.IsSymlink(gitignore) {
		r.refuse("error: "+gitignore+" is a symlink -> "+readLink(gitignore)+" — not promoted, and nothing was written.",
			"  git ignores nothing it cannot read, so entries added through the link would take effect nowhere",
			"  while this reported success and staged the report. Replace it with a regular file, then re-run.")
	}
	if err := r.dropLocalExclusion(); err != nil {
		r.exit(2)
	}
	// Staging waits until every entry is written, or a promotion that reports success with one
	// missing lets that file reach a commit.
	unwritten := ""
	for _, entry := range r.ignoreSurface() {
		if err := appendLine(gitignore, entry); err != nil {
			unwritten += " '" + entry + "'"
		}
	}
	if unwritten != "" {
		r.refuseUnpromoted("error: could not add"+unwritten+" to "+gitignore+" — not promoted.",
			"  Nothing was staged, so the report is not on its way into a commit.")
	}
	// Writing an entry is not the same as it taking effect — ask git, and with -v, because
	// `core.excludesFile` and `.git/info/exclude` answer the plain question too and are this machine's
	// alone. Only root `.gitignore` is the shared answer this subcommand claims to have written.
	unignored := ""
	for _, entry := range r.ignoreSurface() {
		if r.ignoreSourceOf(r.root+"/"+entry) != ".gitignore" {
			unignored += " '" + entry + "'"
		}
	}
	if unignored != "" {
		r.refuseUnpromoted("error: the entries are in "+gitignore+", but git still does not ignore:"+unignored+" — not promoted.",
			"  git ignores nothing it cannot read; a symlinked or unreadable .gitignore does exactly this.",
			"  Nothing was staged, so the report is not on its way into a commit.")
	}
	if r.passThrough("git", "-C", r.root, "add", ".idsd", ".gitignore") != 0 {
		r.refuseUnpromoted("error: could not stage .idsd/ and .gitignore — not promoted")
	}
	// `git add` on a directory whose every file is ignored stages nothing and still exits 0, and
	// qualify-reports/ is ignored by the entry just written — so with nothing else under .idsd/, the add
	// is a no-op. Success is read from the mode for that reason, never from the add's exit: unpromoted,
	// the next check-ignore re-excludes .idsd/ and the whole promotion silently un-happens.
	if r.repoMode() != "committed" {
		r.refuseUnpromoted("error: nothing under .idsd/ could be staged, so it is still a throwaway — not promoted.",
			"  Every file there is ignored. A durable .idsd/ needs something that is not: an intent, a charter, a constitution.",
			"  The .gitignore entry stays (it is wanted in both modes); the local exclusion is back.")
	}
	r.line("promoted: .idsd/ staged, qualify-reports/ ignored via .gitignore — commit when ready (not committed here)")
}

// Named, this runs with no report at all, so `done` can `close` first and still `discard` after. That
// is the one order that composes, and the one `idsd-ship` → `done` uses; reversed, `close` has no
// report left to read and refuses.
func (r *run) cmdDiscard() {
	switch r.resolveReport(r.arg(1)) {
	case 1:
		r.legacyNote()
		r.refuse("error: nothing to discard — no qualify report under "+r.reportsDir+", and no intent named",
			"  Name the intent to discard a ship whose report is already closed.")
	case 3:
		r.refuseAmbiguous("name which as the last argument")
	}
	r.assertRepoModeReadable()
	if r.repoMode() == "committed" {
		r.refuse("committed idsd repo — .idsd/ is the durable record; nothing to discard")
	}
	// Without this, a symlinked `.idsd` lets every deletion below reach through to a target outside the
	// repo. `init` has carried the same guard since a link there could steer a write out.
	r.assertWritePathsAreReal("nothing was discarded")
	// The filename is the ship's name here — it came from the caller, or from being the only report
	// open. The frontmatter is read only to cross-check it, and only when there is a report left to
	// read: a closed ship has none, and nothing about the deletion below needed it.
	stem := stemOfReportPath(r.report)
	r.assertShipExists(stem)
	slug := stem
	if shell.IsRegularFile(r.report) {
		// A report that is present must be readable. Falling back to the filename for one we cannot open
		// would skip the cross-check below on the single path that deletes another ship's intent file.
		r.assertReportIsReadable("nothing was discarded, because its intent cannot be cross-checked (permissions?)")
		slug = r.intentSlug()
	}
	// The two must name the same ship before anything is deleted. Out of step, what gets deleted is
	// another ship's in-flight intent file, which throwaway mode keeps no copy of anywhere.
	if slug != "" && slug != stem {
		r.refuse("error: "+r.report+" is named for '"+stem+"' but records 'intent: "+slug+"' — nothing was discarded.",
			"  Those are different ships, and discard deletes the intent file the frontmatter names.",
			"  Nothing writes that line after init, so a hand-edit or a bug is what put them out of step.",
			"  Reconcile the two by hand, then re-run.")
	}
	if slug != "" {
		_ = rmFile(r.root + "/.idsd/intents/" + slug + ".md")
		_ = rmFile(r.root + "/.idsd/archive/" + slug + ".md")
	}
	_ = rmFile(r.report)
	// The stage markers sit in the git dir, which the .idsd/ removal below never reaches.
	_ = os.RemoveAll(r.stageReturnsDir)
	rmdirIfEmpty(r.reportsDir, r.root+"/.idsd/intents", r.root+"/.idsd/archive")
	if kept := r.survivingContent(); kept != "" {
		// `close` may already have taken the report, and a ship can have no intent file, so
		// assertShipExists guarantees only that one of the two was there.
		r.line("discarded: removed what remained of this ship; kept .idsd/ (still holds:%s)", kept)
		return
	}
	_ = os.RemoveAll(r.root + "/.idsd")
	// .git/info/exclude is shared across worktrees, and a parallel throwaway ship's .idsd/ must stay
	// excluded. Drop it only from the last worktree.
	// "Removed" holds only because assertShipExists ran: nothing reaches here without a report or an
	// intent file to remove. Without it, a second run or any wrong slug claims this having deleted nothing.
	if r.worktreeCount() > 1 {
		r.line("discarded: removed .idsd/ scratch; kept the shared exclusion (other worktrees exist)")
		return
	}
	// Read the return, don't just call it: dropLocalExclusion fails when it cannot read or replace the
	// exclude file, and "zero traces" over a surviving entry is the one claim here a human acts on
	// without checking.
	if err := r.dropLocalExclusion(); err != nil {
		r.errLines("discarded: removed .idsd/ scratch, but the '.idsd/' entry in " + r.gitPath("info/exclude") + " could not be removed — it is still excluded")
		r.exit(2)
	}
	r.line("discarded: removed .idsd/ scratch and its local exclusion (throwaway, zero traces)")
}

// Retire one ship's scratch once it has landed. `done` calls this after the commit succeeds; the open
// `- [ ]` refusal is what stops a hand-run from dropping a decision nobody routed.
func (r *run) cmdClose(args []string) {
	name, isForced := nameAndForceFlag(args)
	r.requireReport(name)
	if !isForced {
		r.readOpenTodos("nothing was closed.")
		if r.openTodos != "" {
			r.refuse("error: "+r.report+" still holds open '- [ ]' — nothing was closed.",
				indentLines(r.openTodos, "  "),
				"  Resolve or route each, or re-run with --force to discard them.")
		}
	}
	_ = rmFile(r.report)
	// The stage markers are in the git dir, so removing the report leaves them behind, and the next
	// ship for this intent would inherit a completed stage record and stamp for free.
	_ = os.RemoveAll(r.stageReturnsDir)
	rmdirIfEmpty(r.reportsDir)
	r.line("closed %s — its stage markers are gone; decisions.md is untouched", shell.BaseName(r.report))
}
