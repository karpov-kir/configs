package ecoreport

import (
	"strconv"
	"strings"

	"configs/ai/tools/shell"
)

type recordKind struct {
	name  string
	file  string
	bound int
	// Where the file sits. A local record lives inside one ship's folder and needs that ship named; a
	// project one sits in the project agent directory. Nothing infers this — every caller says which it writes,
	// because the two are merged later and a write to the wrong one is invisible until then.
	isLocal bool
	header  string
}

// Every record is capped, because an uncapped one grows until nobody reads it. The cap is held here
// rather than advised: `append` refuses on a full record, and the only way past a full one is `admit`,
// which puts one entry out for every one it lets in. So no sequence of these subcommands can grow a
// record past its number.
//
// What the tool never does is choose the loser. Which entry has stopped earning its place — and
// whether the answer is to evict it, fold it into a neighbour or promote it — is a judgment
// `records.md` gives the agent, not a count this tool may act on alone.
const (
	decisionsBound = 100
	playbookBound  = 100
	languageBound  = 100
)

// A local record covers one ship. Past a couple of dozen entries the thing to fix is the ship's scope,
// not the record's size, so the cap is set where it surfaces that rather than where it would hold a
// project's worth. The three are equal because nothing distinguishes what one entry costs at this
// scale — they are read together, once, by the agent finishing that ship.
const localBound = 25

// The most one entry may hold, in bytes. The longest entry any of these records carries today runs to
// about 800, so this clears real practice by more than double and never argues with an entry someone
// meant. What it stops is the other order of magnitude: a whole ticket, or a file, pasted in as one
// "decision".
const entryBound = 2000

// The standard behind every rule here, cited by the path it opens at rather than by its filename. An
// agent told to work a file it cannot find works the message alone, and the message is the shorter of
// the two. The notes name the judge by a full path for the same reason.
const recordsStandard = "~/.kk-flavor/standards/records.md"

// JudgeCommand is the judge command the cap's last step and an over-cap prune run — `records.md` says
// what the cap evicts. A constant, because two notes quote it and must agree on the judge. Exported so
// the repository-level suite can compare the standard's copy against it. That case reads a file outside
// the module, and `go test` keys no cache on it, so it reports `ok (cached)` over a changed standard.
const JudgeCommand = `JUDGE_PROVIDER="${JUDGE_PROVIDER:-codex}" ~/.kk-flavor/scripts/reader-judge.sh record-entry  # the new entry and every incumbent on stdin`

// The moves at the cap that free a slot at no loss, in `records.md`'s order. A constant for the same
// reason JudgeCommand is — the same two notes quote both.
const capLadderRungs = "delete what is no longer true, promote what must not be lost, fold two entries carrying one idea together"

// The judge rung both notes hand over: the command, and how to read each answer it can come back
// with. Beside the two constants above for the reason they are constants — a rung quoted twice is a
// rung two agents can read two ways.
//
// Exit 2 earns a line of its own because from where the agent stands it looks exactly like exit 0:
// `reader-judge.sh` stays silent and exits 2 when the run failed — an unknown kind, a missed
// deadline, an answer that was not numbers. An agent who reads that silence as exit 0 takes it for a
// clean artifact under a cap that holds, which is a verdict no run produced.
func judgeRung() []string {
	return []string{
		"    " + JudgeCommand,
		"  It deletes what it names, and naming nothing (exit 0, output unchanged) is one of its answers.",
		"  Exit 2 is not an answer: the judge did NOT run — an unknown kind, a missed deadline, an answer",
		"  that was not numbers — so this rung is unmet. Nothing may go on it, and the append stays refused",
		"  until a run of the judge answers.",
	}
}

// The rule line every header carries, built from the bound rather than written out under each: four
// hand-written copies are four records that can disagree about one cap. What it says past the cap is
// what the refusal says, because the two are one file talking about one number — `records.md` →
// **Reaching the cap** leaves an entry that frees no slot unrecorded, so no header may promise the
// newcomer an incumbent's place.
func fullAtLine(bound int) string {
	return "Appended per `" + recordsStandard + "`. Full at " + strconv.Itoa(bound) +
		" entries — past that an append refuses, and a new entry lands only if a move at the cap frees a slot.\n"
}

// The header a record is created with, so a first append lands a file that already states its own
// bound. `records.md` → **The cap evicts by what the record can afford to lose** requires the file to
// carry it, and a record created without one is a record nobody prunes. The number is read from the
// constant rather than written out again: a header saying 40 over an append refusing at 30 is a drift
// no reader of either could see.
//
// Each header also names who the file is written for and who owns its wording. These records sit side by
// side under one directory and read alike, and a human who opens `decisions.md` expecting the charter's
// register finds an agent talking to the next agent. What the line says is the audience, never a
// prohibition: nothing here is secret, and a human is free to read any of it.
//
// The pruning point each header states. `records.md` → the record's own definition wants exactly one
// per record, and six records under two roots cannot each be found by reading a skill — so each file
// says its own, by the moment rather than by the skill that arrives at it.
const (
	prunedAtFinalize = "Pruned only when a ship is finalized, where this record and the finishing ship's are in hand together.\n"
	prunedInShip     = "Pruned by the ship that owns it, when that ship reads it. Merged upward at finalize, and archived with the ship whether or not that merge lands.\n"
)

const decisionHeadings = "\n## Promotion candidates\n\n## Decisions\n\n"

var recordKinds = []recordKind{
	{
		name:  "project-decisions",
		file:  "decisions.md",
		bound: decisionsBound,
		header: "# Decisions\n\n" +
			"Written for the next agent — never presented to a human, and no human maintains it.\n" +
			fullAtLine(decisionsBound) + prunedAtFinalize + decisionHeadings,
	},
	{
		name:  "project-playbook",
		file:  "playbook.md",
		bound: playbookBound,
		header: "# Playbook\n\n" +
			"How this repo is operated, written for the next agent — never presented to a human,\n" +
			"and no human maintains it.\n" +
			fullAtLine(playbookBound) + prunedAtFinalize + "\n",
	},
	{
		name:  "project-language",
		file:  "language.md",
		bound: languageBound,
		header: "# Language\n\n" +
			"The project's ubiquitous language, written for the next agent — an agent keeps it current.\n" +
			fullAtLine(languageBound) + prunedAtFinalize +
			"A term no artifact uses is deleted, whatever its count.\n\n",
	},
	{
		name:    "local-decisions",
		file:    "decisions.md",
		bound:   localBound,
		isLocal: true,
		header: "# Decisions — this ship's\n\n" +
			"What this ship settled, written for the next agent. Merged into the project's own at finalize,\n" +
			"where an entry restating one already there becomes a bump rather than a second line.\n" +
			fullAtLine(localBound) + prunedInShip + decisionHeadings,
	},
	{
		name:    "local-playbook",
		file:    "playbook.md",
		bound:   localBound,
		isLocal: true,
		header: "# Playbook — this ship's\n\n" +
			"How this repo is operated, found while building this ship. Merged into the project's own at\n" +
			"finalize, where an entry restating one already there becomes a bump rather than a second line.\n" +
			fullAtLine(localBound) + prunedInShip + "\n",
	},
	{
		name:    "local-language",
		file:    "language.md",
		bound:   localBound,
		isLocal: true,
		header: "# Language — this ship's\n\n" +
			"Terms this ship coined or narrowed. Merged into the project's own at finalize: the same term in\n" +
			"the same sense bumps, while the same term in another sense is a contradiction nothing merges.\n" +
			fullAtLine(localBound) + prunedInShip + "\n",
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
