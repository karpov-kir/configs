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

// The two records every agent in the clone shares. What a record is, and what the count and the date
// on each entry mean, is `~/.kk-flavor/standards/records.md`; what goes in these two in particular is
// `~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log** and `~/.claude/skills/idsd-build/SKILL.md`
// → **Phase 3**.
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
	name   string
	file   string
	header string
}

// Roughly 40 lines, as both skills state. Nothing is deleted on reaching it: the bound is reported and
// the candidate named. Which entry has stopped earning its place is a judgment about reach that
// `records.md` gives the agent, not a count this tool may act on alone.
const recordBound = 40

// The most one entry may hold, in bytes. The longest entry either record carries today runs to about
// 800, so this clears real practice by more than double and never argues with an entry someone meant.
// What it stops is the other order of magnitude: a whole ticket, or a file, pasted in as one
// "decision".
const entryBound = 2000

// The header a record is created with, so a first append lands a file that already states its own
// bound. `records.md` → **The cap evicts by reach, not by age** requires the file to carry it, and a
// record created without one is a record nobody prunes.
var recordKinds = []recordKind{
	{
		name: "decisions",
		file: "decisions.md",
		header: "# Decisions\n\n" +
			"For the next agent, not for presentation. Appended per `~/.kk-flavor/standards/records.md`.\n" +
			"Bound: roughly 40 lines — evict lowest count first, oldest date breaking a tie.\n\n",
	},
	{
		name: "playbook",
		file: "playbook.md",
		header: "# Playbook\n\n" +
			"How this repo is operated. Appended per `~/.kk-flavor/standards/records.md`. Bound: roughly 40 lines.\n\n",
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

// `record <append|bump|evict> <decisions|playbook> "<text>"`.
func (r *run) cmdRecord(args []string) {
	if len(args) != 3 {
		r.refuse("usage: report.sh record {append|bump|evict} {"+recordNames()+"} \"<text>\"",
			"  append adds `1x | <today> | <text>`; bump raises one entry's count and dates it today; evict removes one.",
			"  bump and evict take text that appears in exactly one entry, not a line number.")
	}
	op, name, text := args[0], args[1], args[2]
	kind := recordKindFor(name)
	if kind == nil {
		r.refuse("error: '" + name + "' is not a shared record — the ones this tool writes are " + recordNames() + ".")
	}
	// An entry is one line: `records.md`'s count and date sit on the same line as what they count. A
	// newline here lands a second line the parser reads as a non-entry — invisible to the count, to
	// eviction and to every later bump, while looking present in the file.
	if strings.ContainsAny(text, "\n\r") {
		r.refuse("error: an entry is one line and this one spans several — "+kind.file+" is unchanged.",
			"  Say it in one line, or append several entries.")
	}
	// Every other control byte, for the reason `init.go` collapses the intent value: this text is stored
	// verbatim and echoed back, by the append, by an ambiguous refusal quoting candidates, and by the
	// bound note naming what to evict. An ESC there erases the lines printed above it, so an entry
	// seeded from a fetched ticket or from the tree under review can show the human a decision the file
	// does not hold. Collapsed rather than refused, unlike the two guards around it: those break the
	// record's own format, this only drives the terminal, and an invisible byte is not something the
	// agent can act on. It applies to bump and evict too, whose search text still has to match what this
	// has already collapsed.
	text = shell.Oneline(text)
	if strings.TrimSpace(text) == "" {
		r.refuse("error: the entry is empty — " + kind.file + " is unchanged.")
	}
	// The record's stated bound counts lines, so one line of any length sits inside it: 200 KB appended
	// as a single entry leaves `noteBound` reporting a file well within its cap. Every agent in every
	// worktree of the clone reads that file, and `playbook.md` survives `discard`, so an unbounded entry
	// is a permanent charge on every later agent's context.
	if len(text) > entryBound {
		r.refuse("error: the entry is "+strconv.Itoa(len(text))+" bytes, past the "+strconv.Itoa(entryBound)+" an entry may hold — "+kind.file+" is unchanged.",
			"  A record entry is one line the next agent reads, not a document. Say it shorter, or put the long form where it belongs and record the pointer.")
	}
	// Before recordPath, which resolves and guards a path, and before the append that creates the
	// scratch directory: an operation nobody spelled right should leave nothing behind at all, and a
	// refusal that first made a directory is a command that did nothing and still wrote.
	if op != "append" && op != "bump" && op != "evict" {
		r.refuse("error: '" + op + "' is not a record operation — they are append, bump and evict.")
	}
	path := r.recordPath(kind)
	switch op {
	case "append":
		r.recordAppend(kind, path, text)
	case "bump":
		r.recordBump(kind, path, text)
	case "evict":
		r.recordEvict(kind, path, text)
	}
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

// 0700, matching `init`'s own creation of this tree. Called by the append alone: a bump or evict that
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
		// Only an append creates a record. A bump or evict that created one would leave an empty file
		// behind after refusing, and `playbook.md` is on survivingContent's durable list (paths.go):
		// a nought-byte one left by a typo keeps a throwaway `.idsd/` standing for good, the mode's
		// zero-traces contract broken by a command that refused.
		if !creating && errors.Is(err, fs.ErrNotExist) {
			r.refuse("error: there is no "+path+" yet — nothing was written.",
				"  Only `record append` creates a record; bump and evict act on entries already in one.")
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
	for _, entry := range recordEntriesIn(lines) {
		if entry.text == text {
			r.refuse("error: "+path+" already holds that entry — nothing was appended.",
				"  "+entry.String(),
				"  It is a restatement, so bump it: report.sh record bump "+kind.name+" \"<text identifying it>\"")
		}
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
	r.line("appended to %s: %s", kind.file, entry)
	r.noteBound(kind, path, append(lines, entry.String()))
}

// The two ops that act on an entry already there. Each rewrites the whole file, so each holds the
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
	r.line("bumped in %s: %s", kind.file, bumped)
	r.noteBound(kind, path, lines)
}

func (r *run) recordEvict(kind *recordKind, path, text string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()

	found := r.oneMatchingEntry(path, lines, text, "evicted")
	lines = append(lines[:found.line], lines[found.line+1:]...)
	r.overwriteRecord(path, handle, lines)
	r.line("evicted from %s: %s", kind.file, found)
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

// The one entry the text names. Never a guess: bumping or evicting the wrong entry rewrites reasoning
// the next agent reads as settled, and nothing downstream would catch it. `verb` is what did NOT
// happen, which every refusal here ends on.
func (r *run) oneMatchingEntry(path string, lines []string, text, verb string) recordEntry {
	var matched []recordEntry
	for _, entry := range recordEntriesIn(lines) {
		if strings.Contains(entry.text, text) {
			matched = append(matched, entry)
		}
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
		quoted = append(quoted, "  "+entry.String())
	}
	r.refuse(append([]string{"error: that text appears in " + strconv.Itoa(len(matched)) + " entries of " + path + " — nothing was " + verb + "."},
		append(quoted, "  Name text that appears in exactly one of them.")...)...)
	panic("unreachable")
}

// Crossing the bound is said out loud and never acted on. Whether the candidate should be promoted
// rather than dropped is `records.md` → **Promotion is the exit upward**.
func (r *run) noteBound(kind *recordKind, path string, lines []string) {
	entries := recordEntriesIn(lines)
	if len(entries) <= recordBound {
		return
	}
	lowest := entries[0]
	for _, entry := range entries[1:] {
		if entry.count < lowest.count || (entry.count == lowest.count && entry.date < lowest.date) {
			lowest = entry
		}
	}
	r.errLines("note: "+path+" now holds "+strconv.Itoa(len(entries))+" entries, past its bound of roughly "+strconv.Itoa(recordBound)+".",
		"  Lowest count, oldest date first, the entry to evict is:",
		"  "+lowest.String(),
		"  Promote it if it has become a rule, then: report.sh record evict "+kind.name+" \"<text identifying it>\"")
}

// The date an entry carries. Local, because the record is read by whoever is at this machine.
func today() string {
	return time.Now().Format("2006-01-02")
}
