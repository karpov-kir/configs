package ecoreport

import (
	"os"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// The scratch directory's whole life: excluded, promoted to durable, or torn down. Two of the three
// are destructive — `discard` removes files the throwaway mode keeps no copy of anywhere, and
// `promote` writes to the human's index — so every refusal here stands between this tool and work
// nothing else can recover.

// The ignore-surface entries `attempt` reports as failing, rendered as the quoted run all three
// refusals below echo: ` '.idsd/qualify-reports/'`, and empty when none failed. `attempt` may act
// rather than merely test — promote's use of it is the appendLine that writes the entry.
func (r *run) ignoreEntriesFailing(attempt func(entry string) bool) string {
	failing := ""
	for _, entry := range r.ignoreSurface() {
		if attempt(entry) {
			failing += " '" + entry + "'"
		}
	}
	return failing
}

func (r *run) cmdCheckIgnore() {
	// Runs before anything else (init included, so it never requires the report to exist), and before
	// any fingerprinting `git add -A`, so nothing scratch is ever staged.
	r.assertRepoModeReadable()
	if r.repoMode() == "committed" {
		// A repo promoted out of throwaway still carries the old rule, and in this mode it is worse than
		// stale: it makes git ignore untracked files under .idsd/, so a new intent never gets tracked.
		// Unconditional here because committed mode has no reconcile step to wait for.
		r.removeStaleExclusion()
		// A path already tracked also answers "not ignored" here — the case most worth the warning.
		// Asked through the same predicate `init` enforces, or this prints ok where init then refuses
		// and sends the human back to this command.
		unignored := r.ignoreEntriesFailing(func(entry string) bool {
			_, travels := r.ignoredSourceTravels(r.root + "/" + ignoreProbe(entry))
			return !travels
		})
		if unignored == "" {
			r.line("ok: qualify-reports/ is gitignored (committed idsd repo)")
			r.exit(0)
		}
		r.errLines("WARN: NOT gitignored:" + unignored + " — add each to .gitignore (shared idsd setup)")
		r.exit(1)
	}
	// Nothing is written in the tree any more, so there is nothing to exclude and no exclusion to keep
	// in step across worktrees. What has to hold instead is that the resolved location really is out of
	// git's reach, and that no directory from the older in-tree layout is left holding the only copy of
	// someone's work.
	r.assertScratchIsUnreachableByGit()
	r.reconcileTreeIdsdDir()
	// After the reconcile, so the entry can only be excluding a directory that is already gone.
	r.removeStaleExclusion()
	r.line("ok: throwaway run — idsd scratch is %s, where `git add -A` cannot reach it (nothing to exclude)", r.idsdDir)
}

func (r *run) cmdPromote() {
	// Promotion is about the whole scratch directory, so it names no single report — it only needs one
	// to exist, as the evidence that a ship happened here.
	if len(r.reportNames()) == 0 {
		r.refuse("error: no qualify report under " + r.intentsDir + " — nothing to promote")
	}
	r.assertRepoModeReadable()
	if r.repoMode() == "committed" {
		r.line("already committed — .idsd/ is tracked; nothing to promote")
		r.exit(0)
	}
	// Before anything is written or verified. A repo that ran a throwaway ship under the old layout still
	// carries a `.idsd/` rule in .git/info/exclude, and every step below reads the wrong answer through
	// it: `git add` stages nothing and exits 0, the mode check refuses, and the message blames the human
	// for having nothing to promote — while the .gitignore verification reads info/exclude as the source.
	r.removeStaleExclusion()

	gitignore := r.root + "/.gitignore"
	// Refuse a symlinked .gitignore before anything is written: afterwards the append has already
	// landed in whatever the link points at, and no refusal below undoes that write.
	if shell.IsSymlink(gitignore) {
		r.refuse("error: "+gitignore+" is a symlink -> "+shell.Oneline(readLink(gitignore))+" — not promoted, and nothing was written.",
			"  git ignores nothing it cannot read, so entries added through the link would take effect nowhere",
			"  while this reported success and staged the report. Replace it with a regular file, then re-run.")
	}
	// The SOURCE, checked with the same eye the destination gets below. `init` and `discard` both refuse a
	// symlinked scratch dir; this path did not, and it is the one that stages — so a link here reaches the
	// history every clone pulls. assertScratchDirsAreReal has the mechanism.
	r.assertScratchDirsAreReal("not promoted, and nothing was moved")
	// The destination is checked before the ignore entries are written, so a promotion that cannot
	// possibly finish writes nothing at all.
	target := r.treeIdsdDir()
	r.assertPromotionTargetIsClear(target)

	// .gitignore FIRST, and verified, because it is what keeps qualify-reports/ out of the commit. Do
	// the move first and a failure here leaves the reports sitting in the tree, tracked by the next
	// `git add -A`. Written this way round, a failure below leaves only a spare entry in .gitignore —
	// which is wanted in both modes anyway.
	unwritten := r.ignoreEntriesFailing(func(entry string) bool {
		return appendLine(gitignore, entry) != nil
	})
	if unwritten != "" {
		r.refuse("error: could not add"+unwritten+" to "+gitignore+" — not promoted.",
			"  Nothing was moved and nothing was staged.")
	}
	// Writing an entry is not the same as it taking effect — ask git, and with -v, because
	// `core.excludesFile` and `.git/info/exclude` answer the plain question too and are this machine's
	// alone. Only root `.gitignore` is the shared answer this subcommand claims to have written.
	unignored := r.ignoreEntriesFailing(func(entry string) bool {
		return r.ignoreSourceOf(r.root+"/"+ignoreProbe(entry)) != ".gitignore"
	})
	if unignored != "" {
		r.refuse("error: the entries are in "+gitignore+", but git still does not ignore:"+unignored+" — not promoted.",
			"  git ignores nothing it cannot read; a symlinked or unreadable .gitignore does exactly this.",
			"  Nothing was moved and nothing was staged.")
	}

	moved := r.idsdDir
	r.movePromotedScratch(target)

	// The index moves here, so the memoized `ls-files .idsd` answer goes with it: it is what decides
	// committed from throwaway, and `discard` reads that before deleting.
	r.forgetIndexAnswers()
	if r.passThrough("git", "-C", r.root, "add", ".idsd", ".gitignore") != 0 {
		r.refuseUnmoved(moved, target, "error: could not stage .idsd/ and .gitignore — not promoted.")
	}
	// `git add` on a directory whose every file is ignored stages nothing and still exits 0, and
	// qualify-reports/ is ignored by the entry just written — so with nothing else under .idsd/, the add
	// is a no-op. Success is read from the mode for that reason, never from the add's exit.
	if r.repoMode() != "committed" {
		r.refuseUnmoved(moved, target,
			"error: nothing under "+target+" could be staged, so this is still a throwaway — not promoted.",
			"  Every file there is ignored. A durable .idsd/ needs something that is not: an intent, a charter, a playbook.")
	}
	r.line("promoted: moved the scratch to %s and staged it, qualify-reports/ ignored via .gitignore — commit when ready (not committed here)", target)
}

// A refusal after the move puts the scratch back where it came from. Left in the tree it is the worst
// of both states: the intents sit untracked while the human has been told the promotion did not
// happen, and the next `git add -A` picks up whatever .gitignore does not cover. The undo is the
// inverse rename, so if it fails, say where the files actually are — the one thing the human needs.
func (r *run) refuseUnmoved(from, to string, lines ...string) {
	if err := os.Rename(to, from); err != nil {
		lines = append(lines, "  WARNING: the scratch was moved to "+to+" and could not be put back ("+err.Error()+").",
			"  Your intents are there, in the working tree. Move them to "+from+" yourself, or promote again.")
		r.refuse(lines...)
	}
	lines = append(lines, "  The scratch is back at "+from+"; the .gitignore entry stays (it is wanted in both modes).")
	r.refuse(lines...)
}

// Nothing is promoted onto existing content. An in-tree .idsd/ here was made by hand or left by the
// older layout, and merging two of them is a decision this tool does not get to make silently — one
// side's charter would win and the other would vanish with nothing said.
func (r *run) assertPromotionTargetIsClear(target string) {
	if shell.IsSymlink(target) {
		r.refuse("error: "+target+" is a symlink -> "+shell.Oneline(readLink(target))+" — not promoted, and nothing was written.",
			"  .idsd/ in the tree is always a real directory. Remove the link, then re-run.")
	}
	if !shell.PathExists(target) {
		return
	}
	// Files, not directory entries — the same distinction reconcileTreeIdsdDir needs, and for the same
	// reason. Here it decides whether the empty skeleton a finished migration or an aborted `mkdir -p`
	// leaves behind refuses the promotion, sending the human to reconcile two empty directories.
	count, sample, err := filesUnder(target)
	if err != nil {
		r.refuse("error: could not read " + target + " (" + err.Error() + ") — whether it holds anything is unknown, so nothing was promoted.")
	}
	if count == 0 {
		return
	}
	r.refuse("error: "+target+" already holds "+strconv.Itoa(count)+" file(s) — not promoted, and nothing was written.",
		"  Still there: "+strings.Join(sample, " ")+sampleTail(count, len(sample)),
		"  Promotion moves "+r.idsdDir+" here, and merging the two is not something this decides for you:",
		"  one side's charter or playbook would silently win. Reconcile them by hand, then re-run.")
}

// The move itself. A rename, never a recursive copy: a copy has a half-done state, and in throwaway
// mode the thing being copied is the only version of the human's intents anywhere. A rename cannot
// cross filesystems, and an override root legitimately can be on another volume — that case refuses
// and hands the move to the human rather than growing a copier with partial failures of its own.
func (r *run) movePromotedScratch(target string) {
	if err := os.MkdirAll(shell.DirName(target), 0o777); err != nil {
		r.refuse("error: could not create " + shell.DirName(target) + " — not promoted, and nothing was moved.")
	}
	// RemoveAll, not Remove: an empty target directory is in the rename's way, and what stands there may
	// be the empty directory skeleton a migration leaves, which Remove cannot take. Nothing here is lost —
	// assertPromotionTargetIsClear has already established it holds no file.
	if shell.PathExists(target) {
		if err := os.RemoveAll(target); err != nil {
			r.refuse("error: could not clear the empty " + target + " (" + err.Error() + ") — not promoted, and nothing was moved.")
		}
	}
	if err := os.Rename(r.idsdDir, target); err != nil {
		r.refuse("error: could not move "+r.idsdDir+" to "+target+" ("+err.Error()+") — not promoted.",
			"  Nothing was moved. If the two are on different filesystems, a rename cannot span them:",
			"  copy the directory there yourself, remove the original, then re-run. The .gitignore entry is already in place.")
	}
}

// Named, this runs with no report at all, so `done` can `close` first and still `discard` after. That
// is the one order that composes, and the one `idsd-ship` → `done` uses; reversed, `close` has no
// report left to read and refuses.
func (r *run) cmdDiscard() {
	switch r.resolveReport(r.arg(1)) {
	case reportNoneOpen:
		r.refuse("error: nothing to discard — no qualify report under "+r.intentsDir+", and no intent named",
			"  Name the intent to discard a ship whose report is already closed.")
	case reportAmbiguous:
		r.refuseAmbiguous("name which as the last argument")
	}
	r.assertRepoModeReadable()
	if r.repoMode() == "committed" {
		r.refuse("committed idsd repo — .idsd/ is the durable record; nothing to discard")
	}
	// Without this, a symlinked `.idsd` lets every deletion below reach through to a target outside the
	// repo. `init` carries the same guard, for the same reason: a link there can steer a write out.
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
	// The whole folder rather than its files one by one: a ship's intent, its three intent-local records
	// and its report all live in it, so nothing here has to enumerate them. The stem reached this through
	// reportNameFor, which refuses a leading dot and holds the slug charset — that guard is the whole of
	// what keeps this RemoveAll inside intents/, and it matters more here than it did when this removed
	// named files.
	_ = os.RemoveAll(r.shipDir(stem))
	if slug != "" {
		_ = os.RemoveAll(r.archiveDir(slug))
	}
	// The stage markers sit in the git dir, which the .idsd/ removal below never reaches.
	_ = os.RemoveAll(r.stageReturnsDir)
	rmdirIfEmpty(r.intentsDir, r.idsdDir+"/archive")
	if kept := r.survivingContent(); kept != "" {
		// `close` may already have taken the report, and a ship can have no intent file, so
		// assertShipExists guarantees only that one of the two was there.
		r.line("discarded: removed what remained of this ship; kept .idsd/ (still holds:%s)", kept)
		return
	}
	_ = os.RemoveAll(r.idsdDir)
	// The scratch never lived in the tree, so removing it leaves nothing behind for a sibling worktree to
	// trip over and no exclusion to drop. "Removed" holds only because assertShipExists ran: nothing
	// reaches here without a report or an intent file to remove, so a second run or a wrong slug cannot
	// claim this having deleted nothing.
	r.line("discarded: removed the idsd scratch at %s (throwaway, zero traces)", r.idsdDir)
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
	rmdirIfEmpty(r.intentsDir)
	r.line("closed %s — its stage markers are gone; decisions.md is untouched", shell.BaseName(r.report))
}
