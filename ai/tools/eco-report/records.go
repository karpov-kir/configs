package ecoreport

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"configs/ai/tools/shell"
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

var recordOperations = []string{"append", "bump", "revise", "evict", "admit", "classify", "merge"}

func (r *run) cmdRecord(args []string) {
	// The ship a local record belongs to, taken before the count below so the shapes stay comparable.
	// A leading flag rather than a trailing argument: the last one is already the replacement text for
	// revise and admit, and a slug there would be indistinguishable from a one-word entry.
	args = r.takeRecordIntent(args)
	if len(args) < 3 || len(args) > 4 {
		r.refuse("usage: report.sh record [--intent <NNN-slug>] {"+strings.Join(recordOperations, "|")+"} {"+recordNames()+"} \"<text>\" [\"<new text>\"]",
			"  --intent names the ship a local-* record belongs to, and is required for those and refused for the rest.",
			"  append adds `1x | <today> | <text>`; bump raises one entry's count and dates it today.",
			"  revise replaces one entry's text, keeping its count; evict removes one.",
			"  admit is the swap at a full record: it drops the entry named and lands the new one at 1x.",
			"  classify moves a decision entry: classify {project-decisions|local-decisions} <selector> {candidate|decision}.",
			"  merge appends one local-* entry to its project-* record, rebasing its relative links to that record's folder.",
			"  bump, revise, evict, admit, classify and merge take text that appears in exactly one entry, not a line number.")
	}
	op, name, text := args[0], args[1], args[2]
	kind := recordKindFor(name)
	if kind == nil {
		r.refuse("error: '"+name+"' is not a record this tool writes — they are "+recordNames()+".",
			"  The project-* and local-* pairs are two different files: a ship writes its local one, and finalize merges it upward.")
	}
	// Named deliberately in both directions. Without the first arm a local write with no ship silently
	// lands at the project root, which is the one mistake this split exists to make impossible — and it
	// would not surface until finalize found nothing to merge.
	if kind.isLocal && r.recordSlug == "" {
		r.refuse("error: " + kind.name + " belongs to one ship, so it needs --intent <NNN-slug> — nothing was written.")
	}
	if !kind.isLocal && r.recordSlug != "" {
		r.refuse("error: " + kind.name + " is the project's own record and belongs to no single ship, so --intent does not apply to it — nothing was written.")
	}
	// Before recordPath, which resolves and guards a path, and before the append that creates the
	// scratch directory: an operation nobody spelled right should leave nothing behind at all, and a
	// refusal that first made a directory is a command that did nothing and still wrote.
	if !slices.Contains(recordOperations, op) {
		last := len(recordOperations) - 1
		r.refuse("error: '" + op + "' is not a record operation — they are " +
			strings.Join(recordOperations[:last], ", ") + " and " + recordOperations[last] + ".")
	}
	if op == "merge" && !kind.isLocal {
		r.refuse("error: merge takes the local-* record it carries an entry out of, never " + kind.name + " — nothing was written.")
	}
	// This check refuses a wrong argument count instead of ignoring the extra arguments.
	// The shell splits an unquoted entry into several arguments, so dropping the rest stores a
	// part.
	if (op == "revise" || op == "admit" || op == "classify") != (len(args) == 4) {
		r.refuse("error: revise and admit take selector and replacement; classify takes selector and candidate or decision; append, bump, evict and merge take one text only.",
			"  "+kind.file+" is unchanged.")
	}
	if op == "classify" && (kind.file != "decisions.md" || (args[3] != "candidate" && args[3] != "decision")) {
		r.refuse("error: classify takes project-decisions or local-decisions and candidate or decision — nothing was written.")
	}
	text = r.entryText(kind, text)
	// After every spelling refusal above, so a misspelled command is told what it got wrong rather than
	// told to wait and then told again. Before recordPath, which resolves a path and creates the
	// directory under it, so a command that waits its turn leaves nothing behind while it waits.
	r.refuseSharedRecordUnderAForeignSlot(kind)
	path := r.recordPath(kind)
	switch op {
	case "classify":
		r.recordClassify(kind, path, text, args[3])
	case "append":
		r.recordAppend(kind, path, text)
	case "merge":
		r.recordMerge(kind, path, text)
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

// `--intent <NNN-slug>` off the front of `record`'s arguments, leaving the rest for the shape check.
// The slug is held on the run rather than threaded through, because recordPath is three calls deep and
// every one of those signatures exists to describe the record rather than the invocation.
//
// Held to reportNameFor's charset for the reason every other slug here is: it is joined into a path,
// and this one reaches a file the tool creates.
func (r *run) takeRecordIntent(args []string) []string {
	if len(args) < 2 || args[0] != "--intent" {
		return args
	}
	slug := reportNameFor(args[1])
	if slug == "" || slug == "review" {
		r.refuse("error: '" + args[1] + "' names no ship — it must be a slug ([0-9A-Za-z._-]) and cannot start with a dot. Nothing was written.")
	}
	r.recordSlug = slug
	return args[2:]
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
	path := r.projectAgentsDir() + "/" + kind.file
	if kind.isLocal {
		path = r.shipAgentsDir(r.recordSlug) + "/" + kind.file
	}
	r.assertRealPathParents(path, "the record was not written")
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
		// Only an append creates a record. A bump, revise, evict or admit that created one would leave
		// an empty file behind after refusing, and TestOnlyAnAppendCreatesARecord holds what that costs.
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
