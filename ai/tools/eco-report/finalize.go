package ecoreport

import (
	"os"
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
func (r *run) takeMergeSlot(intent string, isForced bool) {
	if held := r.readMergeSlot(); held != nil {
		// Already ours, for this ship. The caller brackets the whole merge — the judging half writes the
		// project's records, and two ships judging one cap at once is what the slot exists to stop, so
		// it has to be takeable before finalize rather than only by it.
		if held.intent == intent && held.worktree == r.root {
			return
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
}

func (r *run) releaseMergeSlot() { _ = rmFile(r.mergeSlotPath()) }

// The ship's own scratch, deleted before the folder moves. Moved first, the archived folder would
// carry three local records nothing prunes and a report that outlived the pass it recorded — and the
// archive is the durable record, so what lands there is what every later reader believes was kept.
var shipScratchFiles = []string{"decisions.md", "playbook.md", "language.md", reportName}

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

	r.takeMergeSlot(stem, isForced)
	defer r.releaseMergeSlot()

	for _, scratch := range shipScratchFiles {
		_ = rmFile(r.shipDir(stem) + "/" + scratch)
	}
	// The stage markers live in the git dir, which the folder move never reaches — left behind, the
	// next ship for this intent inherits a completed stage record and stamps for free.
	_ = os.RemoveAll(r.stageReturnsDir)

	if err := os.MkdirAll(shell.DirName(target), 0o700); err != nil {
		r.refuse("error: could not create " + shell.DirName(target) + " (" + err.Error() + ") — this ship's scratch is already gone, so re-run once the directory can be made.")
	}
	// os.Rename rather than moveFile: that helper falls back to a file copy across devices, which
	// cannot move a directory. Both paths are under one scratch root, so a rename is always what
	// happens here — and a cross-device error is worth surfacing rather than half-answering.
	if err := os.Rename(r.shipDir(stem), target); err != nil {
		r.refuse("error: could not move " + r.shipDir(stem) + " to " + target + " (" + err.Error() + ") — this ship's scratch is already gone, and its intent is still under intents/.")
	}
	rmdirIfEmpty(r.intentsDir)
	r.line("finalized %s — its scratch is gone and it is archived at %s", stem, target)
}
