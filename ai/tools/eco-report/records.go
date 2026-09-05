package ecoreport

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kk-flavor/tools/shell"
)

// The records every agent in the clone shares. What a record is, and what the count and the date on
// each entry mean, is `~/.kk-flavor/standards/records.md`; what goes in each one is the skill that
// owns it, named in that kind's header below.
//
// One file per clone, under a scratch root every worktree shares. Two sessions doing a hand-run
// read-modify-write at once leave whichever wrote second and drop every entry the other added, and
// the loss shows in no diff: the file simply holds one agent's version. So every mutation comes
// through here, under flock(2).
//
// The lock goes on the record's own descriptor, never on a lockfile beside it. The kernel drops a
// flock when the process exits, so a killed run leaves nothing behind; a lockfile left by a killed
// run blocks every later agent until a human breaks it.
//
// The rewrite is in place, not a staged copy renamed over the file. rename(2) replaces the inode, so
// a sibling holding the lock would be holding it on a file no longer at that path, and the next
// writer would take the lock on the new one and see nothing in the way.

type recordKind struct {
	name  string
	file  string
	bound int
	// Whether the header below says a human owns the wording. `records.md` → **Reaching the cap** lets
	// an agent take every move at the cap unasked and exempts exactly these, so the note that hands over
	// the commands has to know which it is writing to.
	isHumanOwned bool
	header       string
}

// Every record is capped, because an uncapped one grows until nobody reads it. The cap is held here
// rather than advised: `append` refuses on a full record, and the only way past a full one is `admit`,
// which puts one entry out for every one it lets in. So no sequence of these subcommands can grow a
// record past its number.
//
// What the tool never does is choose the loser. Which entry has stopped earning its place — and
// whether the answer is to evict it, fold it into a neighbour or promote it — is a judgment
// `records.md` gives the agent, not a count this tool may act on alone.
//
// The numbers differ because what one entry costs differs. A decision log, a playbook and a
// vocabulary are each read by one agent starting one piece of work, and they sit together. Every
// constraint is inherited by every intent, so that record is the one paid for most often and is held
// at half the rest.
const (
	decisionsBound   = 100
	playbookBound    = 100
	languageBound    = 100
	constraintsBound = 50
)

// The most one entry may hold, in bytes. The longest entry any of these records carries today runs to
// about 800, so this clears real practice by more than double and never argues with an entry someone
// meant. What it stops is the other order of magnitude: a whole ticket, or a file, pasted in as one
// "decision".
const entryBound = 2000

// The judge the cap's contest is settled by, and the one an over-cap prune uses — `records.md` → **The
// cap evicts by what the record can afford to lose**. A constant, because two notes hand it over and
// they must not offer different judges.
const judgeCommand = "~/.kk-flavor/scripts/bloat-judge.sh record-entry  # the new entry and every incumbent on stdin"

// The moves at the cap that free a slot at no loss, in `records.md`'s order. A constant for the same
// reason judgeCommand is — the same two notes quote both.
const capLadderRungs = "delete what is no longer true, promote what must not be lost, fold two entries carrying one idea together"

// The rule line every header carries, built from the bound rather than written out under each: four
// hand-written copies are four records that can disagree about one cap.
func fullAtLine(bound int) string {
	return "Appended per `~/.kk-flavor/standards/records.md`. Full at " + strconv.Itoa(bound) + " entries — past that a new one displaces one already here.\n"
}

// The header a record is created with, so a first append lands a file that already states its own
// bound. `records.md` → **The cap evicts by what the record can afford to lose** requires the file to
// carry it, and a record created without one is a record nobody prunes. The number is read from the
// constant rather than written out again: a header saying 40 over an append refusing at 30 is a drift
// no reader of either could see.
//
// Each header also names who the file is written for and who owns its wording. These four sit side by
// side under one directory and read alike, and a human who opens `decisions.md` expecting the charter's
// register finds an agent talking to the next agent. What the line says is the audience, never a
// prohibition: nothing here is secret, and a human is free to read any of it.
var recordKinds = []recordKind{
	{
		name:  "decisions",
		file:  "decisions.md",
		bound: decisionsBound,
		header: "# Decisions\n\n" +
			"Written for the next agent — never presented to a human, and no human maintains it.\n" +
			fullAtLine(decisionsBound) + "\n",
	},
	{
		name:  "playbook",
		file:  "playbook.md",
		bound: playbookBound,
		header: "# Playbook\n\n" +
			"How this repo is operated, written for the next agent — never presented to a human,\n" +
			"and no human maintains it.\n" +
			fullAtLine(playbookBound) + "\n",
	},
	{
		name:  "language",
		file:  "language.md",
		bound: languageBound,
		header: "# Language\n\n" +
			"The project's ubiquitous language, written for both — an agent keeps it current,\n" +
			"and a human may correct any entry.\n" +
			fullAtLine(languageBound) +
			"A term no artifact uses is deleted, whatever its count.\n\n",
	},
	{
		name:         "constraints",
		file:         "constraints.md",
		bound:        constraintsBound,
		isHumanOwned: true,
		header: "# Constraints\n\n" +
			"Thresholds every intent inherits. The human owns every line — an agent proposes one,\n" +
			"and never edits one without confirmation.\n" +
			fullAtLine(constraintsBound) +
			"One that rules out nothing another does not is deleted.\n\n",
	},
}

func recordKindFor(name string) *recordKind {
	for i := range recordKinds {
		if recordKinds[i].name == name {
			return &recordKinds[i]
		}
	}
	return nil
}

func recordNames() string {
	names := make([]string, 0, len(recordKinds))
	for _, kind := range recordKinds {
		names = append(names, kind.name)
	}
	return strings.Join(names, "|")
}

// One parsed line of a record, `<count>x | <date> | <the entry>`. A line that does not parse is not an
// entry: the header, a blank line, and the multi-line block one record already carries are preserved
// untouched and never counted, evicted or matched.
type recordEntry struct {
	line  int
	count int
	date  string
	text  string
}

func (e recordEntry) String() string {
	return strconv.Itoa(e.count) + "x | " + e.date + " | " + e.text
}

// The form an entry takes in a message, never the form it takes in the file. `entryText` collapses
// control bytes on the way in, but a record is a plain file a skill's template seeds and a human edits
// by hand, so a line no `record` op ever saw can hold an ESC — and every refusal and report here reads
// one back to a terminal, where it erases the lines printed above it. Read text is collapsed at the
// point it is printed, the same as `recordPath` does with a link target, and the stored bytes are left
// alone: collapsing those would rewrite an entry nobody asked to change.
func (e recordEntry) quoted() string {
	return shell.Oneline(e.String())
}

func parseRecordEntry(index int, line string) (recordEntry, bool) {
	parts := strings.SplitN(line, " | ", 3)
	if len(parts) != 3 {
		return recordEntry{}, false
	}
	count, err := strconv.Atoi(strings.TrimSuffix(parts[0], "x"))
	if err != nil || !strings.HasSuffix(parts[0], "x") || count < 1 || parts[1] == "" {
		return recordEntry{}, false
	}
	return recordEntry{line: index, count: count, date: parts[1], text: parts[2]}, true
}

func recordEntriesIn(lines []string) []recordEntry {
	var entries []recordEntry
	for i, line := range lines {
		if entry, ok := parseRecordEntry(i, line); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

// `record <append|bump|revise|evict|admit> <decisions|playbook|language|constraints> "<text>" ["<new text>"]`.
func (r *run) cmdRecord(args []string) {
	if len(args) < 3 || len(args) > 4 {
		r.refuse("usage: report.sh record {append|bump|revise|evict|admit} {"+recordNames()+"} \"<text>\" [\"<new text>\"]",
			"  append adds `1x | <today> | <text>`; bump raises one entry's count and dates it today.",
			"  revise replaces one entry's text, keeping its count; evict removes one.",
			"  admit is the swap at a full record: it drops the entry named and lands the new one at 1x.",
			"  bump, revise, evict and admit take text that appears in exactly one entry, not a line number.")
	}
	op, name, text := args[0], args[1], args[2]
	kind := recordKindFor(name)
	if kind == nil {
		r.refuse("error: '" + name + "' is not a shared record — the ones this tool writes are " + recordNames() + ".")
	}
	// Before recordPath, which resolves and guards a path, and before the append that creates the
	// scratch directory: an operation nobody spelled right should leave nothing behind at all, and a
	// refusal that first made a directory is a command that did nothing and still wrote.
	if op != "append" && op != "bump" && op != "revise" && op != "evict" && op != "admit" {
		r.refuse("error: '" + op + "' is not a record operation — they are append, bump, revise, evict and admit.")
	}
	// Only the two that write a new text take a fourth. Accepting a stray one anywhere else would
	// silently drop it, and the dropped word is as likely to be half the entry someone meant as it is a
	// typo.
	if (op == "revise" || op == "admit") != (len(args) == 4) {
		r.refuse("error: revise and admit take the entry to overwrite and the text to put there; append, bump and evict take one text only.",
			"  "+kind.file+" is unchanged.")
	}
	text = r.entryText(kind, text)
	path := r.recordPath(kind)
	switch op {
	case "append":
		r.recordAppend(kind, path, text)
	case "bump":
		r.recordBump(kind, path, text)
	case "revise":
		r.recordRevise(kind, path, text, r.entryText(kind, args[3]))
	case "evict":
		r.recordEvict(kind, path, text)
	case "admit":
		r.recordAdmit(kind, path, text, r.entryText(kind, args[3]))
	}
}

// One text argument, guarded and returned in the form the record stores. Every op runs it, over the
// search text as much as the stored one: the search has to match what a previous run already
// collapsed.
func (r *run) entryText(kind *recordKind, text string) string {
	// An entry is one line: `records.md`'s count and date sit on the same line as what they count. A
	// newline here lands a second line the parser reads as a non-entry — invisible to the count, to
	// eviction and to every later bump, while looking present in the file.
	if strings.ContainsAny(text, "\n\r") {
		r.refuse("error: an entry is one line and this one spans several — "+kind.file+" is unchanged.",
			"  Say it in one line, or append several entries.")
	}
	// Every other control byte, for the reason `init.go` collapses the intent value: this text is stored
	// verbatim and echoed back, by the append, by an ambiguous refusal quoting candidates, and by the
	// refusal that quotes the entry a revision would collide with. An ESC there erases the lines printed
	// above it, so an entry seeded from a fetched ticket or from the tree under review can show the human
	// a decision the file does not hold. Collapsed rather than refused, unlike the two guards around it:
	// those break the record's own format, this only drives the terminal, and an invisible byte is not
	// something the agent can act on.
	text = shell.Oneline(text)
	if strings.TrimSpace(text) == "" {
		r.refuse("error: the entry is empty — " + kind.file + " is unchanged.")
	}
	// The record's stated bound counts entries, so one line of any length sits inside it: 200 KB
	// appended as a single entry leaves `noteBound` reporting a file well within its cap. Every agent in
	// every worktree of the clone reads that file, and `playbook.md` survives `discard`, so an unbounded
	// entry is a permanent charge on every later agent's context.
	if len(text) > entryBound {
		r.refuse("error: the entry is "+strconv.Itoa(len(text))+" bytes, past the "+strconv.Itoa(entryBound)+" an entry may hold — "+kind.file+" is unchanged.",
			"  A record entry is one line the next agent reads, not a document. Say it shorter, or put the long form where it belongs and record the pointer.")
	}
	return text
}

// Where the record lives, which is the resolved scratch directory in either repo mode. Refuses a
// symlink for the reason assertWritePathsAreReal states: a link here steers every write, and every
// truncation, wherever it points. This refusal is what names that; the open's O_NOFOLLOW is what
// holds the answer for a link planted after it ran.
func (r *run) recordPath(kind *recordKind) string {
	r.assertScratchDirsAreReal("the record was not written")
	// The same question `check-ignore` asks, asked again here because this is the only command outside
	// `init` that creates a file under the scratch directory. Without it, a scratch root an override put
	// inside the checkout takes the write into the working tree, where `git add -A` reaches it — the
	// very layout `check-ignore` refuses.
	r.assertScratchIsUnreachableByGit()
	path := r.idsdDir + "/" + kind.file
	if shell.IsSymlink(path) {
		r.refuse("error: "+path+" is a symlink -> "+shell.Oneline(readLink(path))+" — the record was not written.",
			"  A shared record is always a regular file. Remove the link, then re-run.")
	}
	return path
}

// 0700, matching `init`'s own creation of this tree. Called by the append alone: any other op that
// created the scratch directory would leave one behind after refusing, which is the same shape as the
// nought-byte record the `creating` flag exists to prevent, one level up.
func (r *run) makeScratchDir() {
	if err := os.MkdirAll(r.idsdDir, 0o700); err != nil {
		r.refuse("error: could not create " + r.idsdDir + " (" + err.Error() + ") — the record was not written.")
	}
}

// The record, opened and locked, with everything in it. The handle is left at end-of-file, which is
// where an append writes. The third return says whether the file already ends in a newline: a record
// whose last line lacks one would otherwise take the next appended entry as that line's tail.
func (r *run) openLockedRecord(path string, creating bool) (*os.File, []string, bool) {
	// O_NOFOLLOW because recordPath's symlink refusal is an lstat and this is the open after it. Between
	// the two, anything that can write the scratch directory can drop a link at the path; one planted in
	// that window fails here instead, unwritten.
	flags := os.O_RDWR | syscall.O_NOFOLLOW
	if creating {
		flags |= os.O_CREATE
	}
	handle, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		// Only an append creates a record. A bump, revise, evict or admit that created one would leave an
		// empty file behind after refusing, and `playbook.md` is on survivingContent's durable list
		// (paths.go): a nought-byte one left by a typo keeps a throwaway `.idsd/` standing for good, the
		// mode's zero-traces contract broken by a command that refused.
		if !creating && errors.Is(err, fs.ErrNotExist) {
			r.refuse("error: there is no "+path+" yet — nothing was written.",
				"  Only `record append` creates a record; the rest act on entries already in one.")
		}
		r.refuse("error: could not open " + path + " (" + err.Error() + ") — the record was not written.")
	}
	// Blocking, deliberately: the contending writer is another agent's run of this same tool, which
	// holds the lock for one read and one write. Failing instead would hand the caller a refusal it
	// could only answer by retrying.
	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_EX); err != nil {
		handle.Close()
		r.refuse("error: could not lock " + path + " (" + err.Error() + ") — the record was not written, since without the lock a concurrent write would erase one of the two.")
	}
	content, err := io.ReadAll(handle)
	if err != nil {
		handle.Close()
		r.refuse("error: could not read " + path + " (" + err.Error() + ") — the record was not written.")
	}
	if len(content) == 0 {
		return handle, nil, true
	}
	text := string(content)
	ended := strings.HasSuffix(text, "\n")
	return handle, strings.Split(strings.TrimSuffix(text, "\n"), "\n"), ended
}

func (r *run) recordAppend(kind *recordKind, path, text string) {
	r.makeScratchDir()
	handle, lines, ended := r.openLockedRecord(path, true)
	defer handle.Close()

	// An exact restatement is `records.md` → **Every entry is dated and counted**: it bumps the entry
	// already there rather than adding a line. Only identical text is caught here — a restatement in
	// other words is a judgment, and stays the agent's.
	//
	// Ahead of the cap, because a restatement wants no slot: bumping it costs the file nothing, and
	// sending the agent off to hold a contest would put a live entry out for a copy of itself.
	entries := recordEntriesIn(lines)
	for _, entry := range entries {
		if entry.text == text {
			r.refuse("error: "+path+" already holds that entry — nothing was appended.",
				"  "+entry.quoted(),
				"  It is a restatement, so bump it: report.sh record bump "+kind.name+" \"<text identifying it>\"")
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
	// edit or a lowered cap, and the ops that rewrite one carry the note instead.
	r.line("appended to %s: %s", kind.file, entry)
}

// A full record, refused at the append — the only op that can grow one. Nothing lands and nothing is
// removed.
//
// The ladder comes first because each of its moves frees a slot at no loss (`records.md` → **Reaching
// the cap**), and only under it the contest, which costs the record an entry. The contest judges the
// incumbents AND the newcomer on one scale: asking only whether the new entry beats the weakest
// incumbent lets every newcomer in, and a record that always admits the newest is a rolling window of
// the last N — the age-ordered trim `records.md` forbids outright. The tie goes against the newcomer
// for the same reason: broken the other way, arrival order decides.
func (r *run) refuseFull(kind *recordKind, path string, held int, text string) {
	note := append([]string{
		"error: " + path + " is full — " + strconv.Itoa(held) + " entries, its cap. Nothing was appended.",
		"  Free a slot and re-run the append, working records.md -> Reaching the cap in order:",
		"  " + capLadderRungs + ".",
	}, pruneCommands(kind)...)
	note = append(note,
		"  If none of those apply, run the judge over the new entry and every entry in the file:",
		"    "+judgeCommand,
		"  The lowest of the "+strconv.Itoa(held+1)+" loses, oldest date breaking a tie, and a tie against the new",
		"  entry is lost by the new entry. Losing, it does not go in. Winning, it takes the loser's place:",
		"    report.sh record admit "+kind.name+" \"<text identifying the entry it beat>\" \"<the new entry>\"")
	note = append(note, humanOwnedNote(kind)...)
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

// The line that closes both notes on a record whose header says a human owns its wording: the
// commands above it are ready to run and are not the agent's to run — `records.md` → **Reaching the
// cap** makes every move there a proposal. Empty for the rest, so either note appends it unasked.
func humanOwnedNote(kind *recordKind) []string {
	if !kind.isHumanOwned {
		return nil
	}
	return []string{"  " + kind.file + " is the human's: propose each move and run none of them until they answer."}
}

// The four ops that act on an entry already there. Each rewrites the whole file, so each holds the
// lock across the read and the write — openLockedRecord takes it, and the deferred Close drops it.
func (r *run) recordBump(kind *recordKind, path, text string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()

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

	found := r.entryToOverwrite(path, lines, text, replacement, "revised",
		"  Folding two into one is: revise the one to keep, then evict the other.")
	revised := recordEntry{count: found.count, date: today(), text: replacement}
	lines[found.line] = revised.String()
	r.overwriteRecord(path, handle, lines)
	r.line("revised in %s: %s", kind.file, revised.quoted())
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
// contest would read that borrowed number as evidence.
func (r *run) recordAdmit(kind *recordKind, path, text, entry string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()

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

	found := r.oneMatchingEntry(path, lines, text, "evicted")
	lines = append(lines[:found.line], lines[found.line+1:]...)
	r.overwriteRecord(path, handle, lines)
	r.line("evicted from %s: %s", kind.file, found.quoted())
}

// The new content over the old, on the locked handle. Written over first and trimmed second, never
// truncated first: the file IS the record, there is no second copy anywhere, so a truncate that
// succeeds and a write that then fails leaves nothing at all. This order fails visibly instead — the
// old bytes are still there under a partial rewrite, and a reader arriving between the two steps sees
// a stale tail rather than an empty file.
func (r *run) overwriteRecord(path string, handle *os.File, lines []string) {
	content := joinRecords(lines)
	if _, err := handle.WriteAt(content, 0); err != nil {
		r.refuse("error: could not rewrite "+path+" ("+err.Error()+") — it may now hold part of the new content over the old.",
			"  Nothing was lost to a truncation: read it before writing to it again.")
	}
	if err := handle.Truncate(int64(len(content))); err != nil {
		r.refuse("error: rewrote "+path+" but could not trim it ("+err.Error()+") — it may now end with stale bytes of the old copy, which repeat the lines above them.",
			"  Every entry is intact, and the repeat can be a whole entry of its own: delete the repeated tail by hand.")
	}
}

// The one entry the text names. Never a guess: bumping, revising or evicting the wrong entry rewrites
// reasoning the next agent reads as settled, and nothing downstream would catch it. `verb` is what did
// NOT happen, which every refusal here ends on.
//
// An exact hit wins outright over the substring pass. Matching only by substring, an entry whose text
// contains another's whole text shadows the shorter one for good: every string identifying the short
// entry also names the long one, so the ambiguity refusal below fires forever and no op can reach the
// short entry again. Its date then rots, and the date is what `records.md` breaks an eviction tie on.
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
		"  Work records.md -> Reaching the cap, in order:",
		"  " + capLadderRungs + ", and only then evict what the judge names.",
		"    " + judgeCommand,
	}, pruneCommands(kind)...)
	note = append(note, humanOwnedNote(kind)...)
	r.errLines(note...)
}

// The date an entry carries. Local, because the record is read by whoever is at this machine.
func today() string {
	return time.Now().Format("2006-01-02")
}
