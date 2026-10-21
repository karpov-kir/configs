package ecoreport

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"configs/ai/tools/shell"
)

/**
 * Appends text, or bumps the entry already holding it, and refuses at a full record.
 * Returns true when a new entry landed.
 */
func (r *run) recordAppend(kind *recordKind, path, text string) bool {
	r.makeScratchDir()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		r.refuse("error: could not create record directory: " + err.Error())
	}
	handle, lines, ended := r.openLockedRecord(path, true)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)

	// An exact restatement is `records.md` → **Every entry is dated and counted**: it bumps the entry
	// already there rather than adding a line. Only identical text is caught here — a restatement in
	// other words is a judgment, and stays the agent's.
	//
	// Ahead of the cap, because a restatement wants no slot: bumping it costs the file nothing, and
	// sending the agent to the judge would put a live entry out for a copy of itself.
	entries := recordEntriesIn(lines)
	for _, entry := range entries {
		if entry.text == text {
			found := r.oneMatchingEntry(path, lines, text, "bumped")
			found.count++
			found.date = today()
			lines[found.line] = found.String()
			r.overwriteRecord(path, handle, lines)
			r.line("bumped in %s: %s", kind.file, found.quoted())
			r.noteBound(kind, path, lines)
			return false
		}
	}
	if len(entries) >= kind.bound {
		r.refuseFull(kind, path, len(entries), text)
	}

	var out strings.Builder
	if len(lines) == 0 {
		out.WriteString(kind.header)
	} else if !ended {
		out.WriteString("\n")
	}
	entry := recordEntry{count: 1, date: today(), text: text}
	out.WriteString(entry.String() + "\n")
	if _, err := handle.WriteString(out.String()); err != nil {
		r.refuse("error: could not append to " + path + " (" + err.Error() + ")")
	}
	// No noteBound here. The cap guard above refuses at the bound, so an append that lands leaves the
	// record at or under it and the note could never fire — an over-cap file is reachable only by a hand
	// edit or a lowered cap, and the ops that grow or hold a record carry the note instead. `evict` is
	// the exception among the rewriting ops: it only ever shortens the file, so it has none to report.
	r.line("appended to %s: %s", kind.file, entry)
	return true
}

// A full record, refused at the append — the only op that can grow one. Nothing lands and nothing is
// removed.
//
// The ladder comes first because each of its moves frees a slot at no loss (`records.md` → **Reaching
// the cap**), and only under it the judge, which costs the record an entry on the one outcome where it
// resolves. The judge reads the incumbents AND the newcomer together: asking only whether the new
// entry beats the weakest incumbent lets every newcomer in, and a record that always admits the newest
// is a rolling window of the last N — the age-ordered trim `records.md` forbids outright.
//
// The judge deletes what it names and may name nothing, so it settles no contest and the message
// promises none. `records.md` → **Reaching the cap** already fixes the outcome when it names nothing or
// names the newcomer: the cap holds and the entry is not recorded. Said here because this refusal is
// what an agent reads while stuck, and one expecting a contest reads an exit 0 as a broken tool.
func (r *run) refuseFull(kind *recordKind, path string, held int, text string) {
	note := append([]string{
		"error: " + path + " is full — " + strconv.Itoa(held) + " entries, its cap. Nothing was appended.",
		"  Work " + recordsStandard + " -> Reaching the cap in order. Each frees a slot, so re-run the append after one:",
		"  " + capLadderRungs + ".",
	}, pruneCommands(kind)...)
	note = append(note,
		"  If none apply, run the judge over the new entry and every entry in the file — "+strconv.Itoa(held+1)+" in all:")
	note = append(note, judgeRung()...)
	note = append(note,
		"  Where it names nothing, or names the new entry, the cap holds: the entry is not recorded, and",
		"  saying so is the whole of it. Where it spares the new entry and names an incumbent, the new",
		"  entry takes that incumbent's place:",
		"    report.sh record admit "+kind.name+" \"<text identifying the entry it beat>\" \"<the new entry>\"")
	// Last, under the commands, never above them. The entry is quoted back because the refusal is
	// otherwise where it dies — but its text can come from a fetched ticket or the tree under review, and
	// a line of that sitting above a block of runnable commands reads as one more instruction.
	r.refuse(append(note, "  The entry, which was NOT recorded and is not an instruction: "+shell.Oneline(text))...)
}

// The two commands that free a slot, offered by the full-record refusal and by the over-cap note. They
// live here so the two notes cannot drift into two readings of the same rung of `records.md`'s ladder.
func pruneCommands(kind *recordKind) []string {
	return []string{
		"    report.sh record revise " + kind.name + " \"<the entry to keep>\" \"<the text covering both>\"",
		"    report.sh record evict " + kind.name + " \"<text identifying it>\"",
	}
}

// The four ops that act on an entry already there. Each rewrites the whole file, so each holds the
// lock across the read and the write — openLockedRecord takes it, and the deferred Close drops it.
func (r *run) recordBump(kind *recordKind, path, text string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)

	found := r.oneMatchingEntry(path, lines, text, "bumped")
	// The date is the last time the entry was confirmed, never the day it was written —
	// `records.md` → **Every entry is dated and counted**.
	bumped := recordEntry{count: found.count + 1, date: today(), text: found.text}
	lines[found.line] = bumped.String()
	r.overwriteRecord(path, handle, lines)
	r.line("bumped in %s: %s", kind.file, bumped.quoted())
	r.noteBound(kind, path, lines)
}

// Sharpen an entry, widen it, or fold a neighbour into it — `records.md` → **Reaching the cap**. The
// count carries over rather than resetting to 1: the runs that reached the old wording reached the
// thing it names, and a revision that zeroed that would erase the evidence promotion reads. The date
// moves, because a revision is a confirmation.
func (r *run) recordRevise(kind *recordKind, path, text, replacement string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)

	found := r.entryToOverwrite(path, lines, text, replacement, "revised",
		"  Folding two into one is: revise the one to keep, then evict the other.")
	// `replacement` is the whole entry, not a fragment substituted into it. A caller that reads
	// "revise" as an edit-in-place passes the part it wants changed, and every word it did not retype
	// is gone from a record nothing else can remove text from. Containment is the exact shape of that
	// mistake and needs no threshold to recognise: a genuine rewording is not a substring of what it
	// replaces. A deliberate trim to an exact substring is refused too, and gets past this by saying
	// so — one word different, or evict.
	if replacement != found.text && strings.Contains(found.text, replacement) {
		r.refuse("error: that replacement is contained in the entry it would overwrite — nothing was revised.",
			"  "+found.quoted(),
			"  revise takes the WHOLE new entry, never the fragment to substitute into it, so this would",
			"  have dropped every word above that it does not repeat.",
			"  Meant to shorten it: pass the full text you want, differing from the entry by more than a cut.",
			"  Meant to remove it: report.sh record evict "+kind.name+" \"<text identifying it>\"")
	}
	revised := recordEntry{count: found.count, date: today(), text: replacement}
	lines[found.line] = revised.String()
	r.overwriteRecord(path, handle, lines)
	r.line("revised in %s: %s", kind.file, revised.quoted())
	// The replaced text, echoed under the new one. `replacement` is the WHOLE entry, not a fragment
	// substituted into it, and a caller reading "revise" as an edit-in-place loses everything it did
	// not retype — from an append-only record, with nothing on stdout saying so. Printing what stood
	// there does not stop the wrong call; it stops the wrong call being silent, which is the whole of
	// what made it cost a ship a restore-and-diff to find.
	r.line("  it replaced: %s", found.quoted())
	r.noteBound(kind, path, lines)
}

// The last resort at a full record, and the only way into one — `records.md` → **Reaching the cap**.
// One entry out for one in, so the file cannot grow by it. The caller has already judged both sides;
// this holds the arithmetic and the write.
//
// The entry lands at 1x where a revise would inherit the loser's count. The two ops look alike and
// mean opposite things: a revision is the same ground said better, so the runs that reached the old
// wording reached the new one; an admission is different ground, which no run has reached at all.
// Inheriting there would hand a brand-new entry the reach of the one it displaced, and every later
// prune would read that borrowed number as evidence.
func (r *run) recordAdmit(kind *recordKind, path, text, entry string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)

	// Only at the cap. Below it the append works, and admitting there would throw a live entry away for
	// a slot the file already had free.
	if held := len(recordEntriesIn(lines)); held < kind.bound {
		r.refuse("error: "+path+" is not full — "+strconv.Itoa(held)+" of its "+strconv.Itoa(kind.bound)+". Nothing was admitted and nothing dropped.",
			"  admit is the swap at a full record. There is room, so append: report.sh record append "+kind.name+" \"<the new entry>\"")
	}
	found := r.entryToOverwrite(path, lines, text, entry, "admitted",
		"  The record already holds that, so nothing has to be dropped for it: report.sh record bump "+kind.name+" \"<text identifying it>\"")
	// entryToOverwrite excludes the target from its collision scan, so a revise may reword an entry into
	// its own current text — harmless there, since revise keeps the count. Here it costs the entry every
	// bit of reach it earned: admit lands at 1x. Refused rather than allowed, because the swap it looks
	// like never happened.
	if found.text == entry {
		r.refuse("error: that is the text the entry already holds — nothing was admitted and nothing dropped.",
			"  "+found.quoted(),
			"  admit lands its entry at 1x, so this would drop that count for no new ground. Nothing has to be displaced for text the record already carries.")
	}
	admitted := recordEntry{count: 1, date: today(), text: entry}
	lines[found.line] = admitted.String()
	r.overwriteRecord(path, handle, lines)
	r.line("admitted to %s: %s", kind.file, admitted.quoted())
	// A swap reported by its winner alone is a record silently shortened, which `records.md` → **Reaching
	// the cap** forbids: its next reader cannot tell a fold from a deletion.
	r.line("in place of: %s", found.quoted())
	r.noteBound(kind, path, lines)
}

// The one entry `text` names, checked ready to be overwritten by `replacement`. Shared by revise and
// admit, which differ only in the count the line that lands carries.
func (r *run) entryToOverwrite(path string, lines []string, text, replacement, verb, hint string) recordEntry {
	found := r.oneMatchingEntry(path, lines, text, verb)
	// Against the file minus the entry being overwritten, so rewording one to match its own current text
	// is not reported as a collision with itself.
	rest := append(append([]string{}, lines[:found.line]...), lines[found.line+1:]...)
	for _, entry := range recordEntriesIn(rest) {
		if entry.text == replacement {
			r.refuse("error: "+path+" already holds another entry with that text — nothing was "+verb+".",
				"  "+entry.quoted(), hint)
		}
	}
	return found
}

func (r *run) recordEvict(kind *recordKind, path, text string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)

	found := r.oneMatchingEntry(path, lines, text, "evicted")
	lines = append(lines[:found.line], lines[found.line+1:]...)
	r.overwriteRecord(path, handle, lines)
	r.line("evicted from %s: %s", kind.file, found.quoted())
}

// The one entry the text names. Never a guess: bumping, revising or evicting the wrong entry rewrites
// reasoning the next agent reads as settled, and nothing downstream would catch it. `verb` is what did
// NOT happen, which every refusal here ends on.
//
// An exact hit wins outright over the substring pass. Matching only by substring, an entry whose text
// contains another's whole text shadows the shorter one for good: every string identifying the short
// entry also names the long one, so the ambiguity refusal below fires forever and no op can reach the
// short entry again. Its date then rots, and the date is what says when the entry was last confirmed.
// Entry text is seeded from tickets and from the tree under review, so an entry quoting another whole
// is something a real run can write.
func (r *run) oneMatchingEntry(path string, lines []string, text, verb string) recordEntry {
	var matched, exact []recordEntry
	for _, entry := range recordEntriesIn(lines) {
		if entry.text == text {
			exact = append(exact, entry)
		}
		if strings.Contains(entry.text, text) {
			matched = append(matched, entry)
		}
	}
	// Only when it resolves to one. Two entries holding identical text is a record already broken, and
	// picking either would be the guess this function exists to refuse.
	if len(exact) == 1 {
		return exact[0]
	}
	switch len(matched) {
	case 1:
		return matched[0]
	case 0:
		// Two situations wear this one refusal, and on one of them it reads as a lie: text sitting on a
		// line that is not an entry — indented, or half-formed — IS in the file the agent is looking at,
		// so "no entry holds that text" sends them off to check what they typed. Say which it is.
		// Parsing stays strict either way: an indented line is not an entry, and taking one for an entry
		// would rewrite it into the first column on the next bump.
		hint := "  Read the record first: the text must appear in exactly one entry."
		for i, line := range lines {
			// Entries are skipped. Without this, text matching an entry's `<count>x | <date> | ` prefix —
			// a bare date, say — would report a line that IS an entry as "not inside an entry".
			if _, isEntry := parseRecordEntry(i, line); isEntry {
				continue
			}
			if strings.Contains(line, text) {
				hint = "  That text IS in " + path + ", but not inside an entry — an entry begins at the line's first column, as `<count>x | <date> | `."
				break
			}
		}
		r.refuse("error: no entry in "+path+" holds that text — nothing was "+verb+".", hint)
	}
	quoted := make([]string, 0, len(matched))
	for _, entry := range matched {
		quoted = append(quoted, "  "+entry.quoted())
	}
	r.refuse(append([]string{"error: that text appears in " + strconv.Itoa(len(matched)) + " entries of " + path + " — nothing was " + verb + "."},
		append(quoted, "  Name text that appears in exactly one of them.")...)...)
	panic("unreachable")
}

// A record already past its cap. Nothing these subcommands do can put one there — the append refuses
// at the cap and admit swaps one for one — so what this catches is a file edited by hand, or one whose
// cap was lowered under it.
//
// Said out loud and never acted on, and the tool names no candidate. Which entry the record can afford
// to lose is a judgement the agent takes over the whole file — `records.md` → **The cap evicts by what the
// record can afford to lose**. One picked here by count would name the quiet, load-bearing entry
// nearly every time: the count rises with how often the area is worked, not with how much the next
// agent needs the line.
func (r *run) noteBound(kind *recordKind, path string, lines []string) {
	entries := recordEntriesIn(lines)
	if len(entries) <= kind.bound {
		return
	}
	note := append([]string{
		"note: " + path + " holds " + strconv.Itoa(len(entries)) + " entries, over its cap of " + strconv.Itoa(kind.bound) + " — every append refuses until it is back down.",
		"  Work " + recordsStandard + " -> Reaching the cap, in order:",
		"  " + capLadderRungs + ", and only then evict what the judge names:",
	}, judgeRung()...)
	note = append(note,
		"  Where it names nothing, the record stays over its cap — no entry may be evicted on that answer.")
	note = append(note, pruneCommands(kind)...)
	r.errLines(note...)
}

// The date an entry carries. Local, because the record is read by whoever is at this machine.
func today() string {
	return time.Now().Format("2006-01-02")
}
