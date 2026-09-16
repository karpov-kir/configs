package ecoreport

import (
	"bytes"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"kk-flavor/tools/shell"
)

// Finalizing one ship: the deterministic tail of a merge, taken under a slot no two ships share.
//
// The judgment half is not here. Which of a ship's records survive into the project's is
// `~/.kk-flavor/standards/records.md`'s logic and the finalizing skill's to apply, and it happens
// before this runs — so everything below is mechanical and nothing in it can ask a human. That
// ordering is the whole reason the slot is safe to hold: a question inside it would stall every other
// ship behind a thread nobody is watching.

// Where the slot lives, and why there. The COMMON git dir, so every worktree of one clone contends
// for the one slot — the records they merge into are per-clone, and a per-worktree lock would let two
// siblings write them at once while each believed it held the only slot.
const mergeSlotFile = "idsd-merge-slot"

// Its own exit code. A caller that cannot tell "wait your turn" from "your tree is bad" does the wrong
// thing either way: it re-runs gates that were fine, or it sits on a red that is real. 1 is a gate's
// block and 2 is "this did not run", so neither could carry it.
//
// Not a `state` token, which is the reach every reader has for this and the wrong one twice over.
// `state` already prints `finalize`, meaning this ship is ready to be finalized — a `finalizing` beside
// it would sit two letters from an unrelated meaning. And the slot is clone-wide while `state` answers
// for one ship: a ship waiting behind another's slot is still `ready`, because what is blocked is the
// act and not the ship.
const exitMergeSlotHeld = 4

// The holder, as the slot records it. The worktree rather than a pid: the slot spans several
// invocations of this tool, so no process alive at the moment it was taken is alive when the next
// caller reads it. A worktree is what a session owns for its whole life, which makes it the name a
// caller can check against its own list of live sessions rather than merely trust.
type mergeSlot struct {
	intent   string
	worktree string
	taken    time.Time
}

func (r *run) mergeSlotPath() string { return r.gitCommonPath(mergeSlotFile) }

// Nothing, or the holder. A slot whose file cannot be parsed is treated as held by an unknown holder
// rather than as absent: absent is the answer that lets a second ship in, and a corrupt slot is
// exactly when that must not happen.
func (r *run) readMergeSlot() *mergeSlot {
	content, err := os.ReadFile(r.mergeSlotPath())
	if err != nil {
		return nil
	}
	lines := shell.SplitLines(string(content))
	held := &mergeSlot{intent: "<unreadable>", worktree: "<unreadable>"}
	if len(lines) > 0 && lines[0] != "" {
		held.intent = lines[0]
	}
	if len(lines) > 1 && lines[1] != "" {
		held.worktree = lines[1]
	}
	if len(lines) > 2 {
		if unix, convErr := strconv.ParseInt(strings.TrimSpace(lines[2]), 10, 64); convErr == nil {
			held.taken = time.Unix(unix, 0)
		}
	}
	return held
}

// Take it, or refuse naming who has it. `--force` breaks a slot whose holder is gone — the tool cannot
// see that for itself, since the session it would be asking about is not a process it started, so the
// judgment is the caller's and this only carries it out and says so.
// Reports whether THIS call took the slot. A caller that brackets the whole merge took it before
// finalize ran, and finalize releasing it on the way out hands the rest of that bracket to whoever is
// waiting — the judging half writes the project's records outside this process, which is the window
// the slot exists to close.
func (r *run) takeMergeSlot(intent string, isForced bool) bool {
	if held := r.readMergeSlot(); held != nil {
		// Already ours, for this ship. The caller brackets the whole merge — the judging half writes the
		// project's records, and two ships judging one cap at once is what the slot exists to stop, so
		// it has to be takeable before finalize rather than only by it.
		if held.intent == intent && held.worktree == r.root {
			return false
		}
		if !isForced {
			age := ""
			if !held.taken.IsZero() {
				age = ", taken " + strconv.Itoa(int(time.Since(held.taken).Round(time.Minute)/time.Minute)) + " minute(s) ago"
			}
			// Both halves collapsed, for the reason gate.go collapses the values it quotes: neither is text
			// this tool chose. The intent is the holder's own argv, and the worktree is what git handed back
			// for their checkout, control bytes intact. An ESC in either rewrites the lines printed above it
			// — and what reads a slot refusal is another agent, waiting its turn.
			r.errLines("error: another ship holds the merge slot — '"+shell.Oneline(held.intent)+"' in "+shell.Oneline(held.worktree)+age+". Nothing was finalized.",
				"  Finalizing is serial: it moves the archive, regenerates the roadmap and writes the project's records, which every ship shares.",
				"  Wait for it, or re-run with --force once you have established that holder is gone.",
				"  Establishing that is yours: this tool started no process it could ask about. Look for a session working in that worktree — none, and the slot outlived its holder.")
			r.exit(exitMergeSlotHeld)
		}
		r.line("reclaimed the merge slot from '%s' in %s", shell.Oneline(held.intent), shell.Oneline(held.worktree))
	}
	// Written before the first destructive step below, so a crash leaves a slot a later --force clears
	// rather than a half-archived ship nothing was holding.
	slot := intent + "\n" + r.root + "\n" + strconv.FormatInt(time.Now().Unix(), 10) + "\n"
	if err := os.WriteFile(r.mergeSlotPath(), []byte(slot), 0o600); err != nil {
		r.refuse("error: could not take the merge slot at " + r.mergeSlotPath() + " (" + err.Error() + ") — nothing was finalized.")
	}
	return true
}

func (r *run) releaseMergeSlot() { _ = rmFile(r.mergeSlotPath()) }

// The slot asked on the write it was built for. Until this, only `finalize` and an explicit
// `merge-slot take` consulted it, so a lane that appended to a project record without taking it passed
// no check at all and the ship holding the slot learned nothing.
//
// Observed on 2026-09-16 on a clone where eleven changes landed at once, then reproduced in a probe
// clone of two worktrees. With the slot held, a second worktree's `git merge` and its
// `record append project-decisions` both went through at exit 0, and the holder's slot file came back
// byte-identical. The merge half is not fixable here and never was: `git merge` is the session's own
// command and this tool is not in its path. So what the slot promises is narrowed to the records, and
// on those it is now kept.
//
// Only the project records. A local one belongs to one ship and no other lane can reach it, so
// refusing there would stall a build for a slot it has no business waiting on.
//
// Ownership is the worktree, not the intent. A project record belongs to no single ship, so
// `record project-*` carries no slug to compare against the slot's — and a worktree is what a session
// owns for its whole life (mergeSlot above), which makes it the one name both sides can state. The
// holder's own judging half runs from the worktree it took the slot in, so it writes freely; any other
// worktree is the collision.
//
// No slot, no refusal: a clone where nobody brackets a merge behaves exactly as it did. An unreadable
// slot refuses, for the reason readMergeSlot treats one as held — absent is the answer that lets a
// second writer in, and a corrupt slot is when that must not happen.
func (r *run) refuseSharedRecordUnderAForeignSlot(kind *recordKind) {
	if kind.isLocal {
		return
	}
	held := r.readMergeSlot()
	if held == nil || held.worktree == r.root {
		return
	}
	age := ""
	if !held.taken.IsZero() {
		age = ", taken " + strconv.Itoa(int(time.Since(held.taken).Round(time.Minute)/time.Minute)) + " minute(s) ago"
	}
	// Both quoted halves collapsed, as takeMergeSlot collapses them: neither is text this tool chose,
	// and what reads this refusal is another agent waiting its turn.
	r.errLines("error: another ship holds the merge slot — '"+shell.Oneline(held.intent)+"' in "+shell.Oneline(held.worktree)+age+". "+kind.file+" is unchanged.",
		"  "+kind.name+" is the project's own record, and merging entries into it is what the slot makes serial.",
		"  Wait for that ship, or take the slot yourself once you have established its holder is gone.")
	r.exit(exitMergeSlotHeld)
}

// The ship's own scratch, deleted before the folder moves: the report, which outlived the pass it
// recorded and which any later pass reproduces.
//
// The three local records used to be on this list, on the reasoning that the judging half had already
// merged them upward and the archive should not carry a second copy nothing prunes. That reasoning
// holds for the path where the merge succeeds, and the merge is exactly what is allowed not to. The
// finalizing skill leaves an entry unmerged on a contradiction — "stays unmerged until they settle
// it" — and on a full project cap, so a ship can reach here still holding entries no project record
// received. Deleting them then destroys the only copy, and `language.md` is the one a later pass
// cannot reconstruct: a term's meaning is not recoverable from the code that used it.
//
// So they ride into the archive. A duplicate under a ship's own archived folder costs a reader one
// question about which copy is authoritative; the alternative costs them the record. Tidiness is
// recoverable and the entries are not.
var shipScratchFiles = []string{reportName}

// The slot as its own act, so a caller can hold it across the judging half of a merge — which writes
// the project's records and therefore must not run beside another ship's. `finalize` still takes one
// for itself when none is held, so the common single-ship path needs neither call.
func (r *run) cmdMergeSlot(args []string) {
	switch argAt(args, 0) {
	case "take":
		name, isForced := nameAndForceFlag(args[1:])
		if name == "" {
			r.refuse("usage: report.sh merge-slot take <NNN-slug> [--force]")
		}
		r.takeMergeSlot(name, isForced)
		r.line("holding the merge slot for %s", name)
	case "release":
		r.releaseMergeSlot()
		r.line("released the merge slot")
	default:
		r.refuse("usage: report.sh merge-slot {take <NNN-slug> [--force]|release}")
	}
}

func (r *run) cmdFinalize(args []string) {
	name, isForced := nameAndForceFlag(args)
	r.requireReport(name)
	stem := stemOfReportPath(r.report)
	r.assertWritePathsAreReal("nothing was finalized")

	// Before the slot, because it can refuse: a refusal inside the slot is one every other ship waits
	// behind for nothing.
	target := r.archiveDir(stem)
	if shell.PathExists(target) {
		r.refuse("error: "+target+" already exists — nothing was finalized.",
			"  A ship archives once. Move or remove what is there, then re-run.")
	}

	// Released only if this call is what took it. A slot the caller was already holding is the caller's
	// to release, and step 4 of the finalizing skill has not run when this returns.
	if tookSlot := r.takeMergeSlot(stem, isForced); tookSlot {
		defer r.releaseMergeSlot()
	}

	for _, scratch := range shipScratchFiles {
		_ = rmFile(r.shipAgentsDir(stem) + "/" + scratch)
	}
	// The stage markers live in the git dir, which the folder move never reaches — left behind, the
	// next ship for this intent inherits a completed stage record and stamps for free.
	_ = os.RemoveAll(r.stageReturnsDir)
	r.clearResultManifest()

	if err := os.MkdirAll(shell.DirName(target), 0o700); err != nil {
		r.refuse("error: could not create " + shell.DirName(target) + " (" + err.Error() + ") — this ship's report is already gone, so re-run once the directory can be made.")
	}
	// os.Rename rather than moveFile: that helper falls back to a file copy across devices, which
	// cannot move a directory. Both paths are under one scratch root, so a rename is always what
	// happens here — and a cross-device error is worth surfacing rather than half-answering.
	if err := os.Rename(r.shipDir(stem), target); err != nil {
		r.refuse("error: could not move " + r.shipDir(stem) + " to " + target + " (" + err.Error() + ") — this ship's report is already gone, and its intent is still under intents/.")
	}
	rmdirIfEmpty(r.intentsDir)
	staged := ""
	if r.repoMode() == "committed" {
		staged = r.stageArchivedShip(stem)
	}
	r.line("finalized %s — its report is gone and it is archived%s at %s", stem, staged, target)
}

// The archived records reach the index here: `.gitignore` covers them under intents/ and nothing covers
// the archive path, so after the rename the next commit's pathspec alone decides their fate. The vacated
// path is staged too, or that commit writes the intent at both paths. Files are named one by one: a
// directory pathspec sweeps in strays, and exits 0 having staged nothing when all its files are ignored.
func (r *run) stageArchivedShip(stem string) string {
	target := r.archiveDir(stem)
	// One read, two answers: whether the vacated path needs staging at all, and which files the removal
	// it stages will cover. `-z`, because the second answer is parsed and git quotes a path holding a
	// newline or a quote — a quoted name handed back as a pathspec matches nothing.
	tracked, status := r.captureGit(nil, "ls-files", "-z", "--", r.shipDir(stem))
	// A failed read answers the same as "nothing tracked", and here the two are not interchangeable: it
	// says which files the removal must be matched against, so guessing stages a removal with no
	// addition and the commit drops them.
	if status != 0 {
		r.refuse("error: "+stem+" is archived at "+target+", but the index could not be read (git ls-files "+r.shipDir(stem)+") — nothing was staged.",
			"  Nothing needs re-running: stage "+target+" and the vacated path yourself, then commit.")
	}
	var paths []string
	for _, path := range archivedShipFiles(target, shipRelativeTrackedFiles(tracked, r.root, r.shipDir(stem))) {
		if shell.IsRegularFile(path) {
			paths = append(paths, path)
		}
	}
	// Named only when the index still holds something under it: one unmatched pathspec fails the whole
	// add, and a ship whose intent.md was never committed matches nothing there.
	if tracked != "" {
		paths = append(paths, r.shipDir(stem))
	}
	if len(paths) == 0 {
		return ""
	}
	// Captured rather than passed through: git's account of a failure names the paths it could not stage,
	// and those are the ship folder's own bytes. git leaves a newline alone, so one of them forges a whole
	// line — and what reads this output is another agent.
	var reported bytes.Buffer
	if _, status := r.captureGit(&reported, append([]string{"add", "--"}, paths...)...); status != 0 {
		r.refuse("error: "+stem+" is archived at "+target+", but staging it failed — its records are untracked there, and a commit that stages by path will leave them behind.",
			"  git said: "+shell.Oneline(reported.String()),
			"  Nothing needs re-running: stage "+target+" yourself, then commit.")
	}
	return " and staged"
}

// Every path the archive must carry into the index: the ship's own records, which `.gitignore` kept out
// of it under intents/, plus the counterpart of each file the index DID hold there. That second half
// keeps the move symmetric — the vacated path is staged as a directory, so its removal covers every
// tracked file under it, and one missing here is a file the commit deletes with nothing added back.
// Read from the INDEX, never the moved directory: a stray an agent dropped into the ship folder is
// untracked, has no removal to match, and must not ride in.
func archivedShipFiles(target string, trackedRelative []string) []string {
	files := []string{target + "/" + intentName}
	for _, record := range agentRecordFiles {
		files = append(files, target+"/"+agentsDirName+"/"+record)
	}
	for _, path := range trackedRelative {
		if !slices.Contains(files, target+"/"+path) {
			files = append(files, target+"/"+path)
		}
	}
	return files
}

// The ship-folder-relative form of every path the index holds under it. `ls-files` answers relative to
// the root it was asked from, and the ship folder is under that root whenever this runs.
func shipRelativeTrackedFiles(lsFiles, root, shipDir string) []string {
	prefix := strings.TrimPrefix(shipDir, root+"/") + "/"
	var relative []string
	for _, path := range strings.Split(lsFiles, "\x00") {
		if rest, found := strings.CutPrefix(path, prefix); found && rest != "" {
			relative = append(relative, rest)
		}
	}
	return relative
}
