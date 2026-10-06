package voicecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

// The words the corpus below treats as coined: a metaphor for a mechanism, and a word a reader would
// have to have been in the room for. A repository keeps no list of its own: its hyphenated names are read off its tree.
var fixtureCoined = []string{"climb", "drift", "slip", "hedge", "sprocket", "wobble"}

const (
	houseCorpus = "testdata/voice/house"
	plainCorpus = "testdata/voice/plain"
)

// The two corpora are the same notes written twice: once in the register this check exists to find,
// once in the register the rule asks for. Each note sits under a heading naming what it describes. They
// hold the shape and not any subject, and they are written for this suite so it depends on no tree
// outside this repository.
func readCorpus(t *testing.T, dir string) (string, []string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no corpus under %s (%v), so this run says nothing", dir, err)
	}
	body, err := os.ReadFile(names[0])
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Base(names[0]), shell.SplitLines(string(body))
}

func voiceScanner() scanner {
	return scanner{profile: ProfileProse, coined: fixtureCoined}
}

// paragraph is a run of lines between blank lines, by the 1-based lines it spans.
type paragraph struct{ start, end int }

// paragraphsOf lists a corpus's paragraphs. A heading names the note under it and is no paragraph.
func paragraphsOf(lines []string) []paragraph {
	var found []paragraph
	open := false
	for i, line := range lines {
		at := i + 1
		switch {
		case strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#"):
			open = false
		case open:
			found[len(found)-1].end = at
		default:
			found = append(found, paragraph{at, at})
			open = true
		}
	}
	return found
}

// Every paragraph written in the house register has to produce a finding. A paragraph that slips
// through is text the lane would pass unread, which is the whole of what this check exists to stop.
func TestEveryParagraphInTheHouseRegisterProducesAFinding(t *testing.T) {
	name, lines := readCorpus(t, houseCorpus)
	found := voiceScanner().scanProse(name, lines)
	paragraphs := paragraphsOf(lines)
	if len(paragraphs) < 7 {
		t.Fatalf("the house corpus holds %d paragraphs; it has to exercise every check", len(paragraphs))
	}
	for _, p := range paragraphs {
		reported := false
		for _, f := range found {
			if p.start <= f.Line && f.Line <= p.end {
				reported = true
			}
		}
		if !reported {
			t.Errorf("no finding in the house-register paragraph at lines %d-%d:\n%s",
				p.start, p.end, strings.Join(lines[p.start-1:p.end], "\n"))
		}
	}
}

// Every check has to fire somewhere in the corpus. A check that fires nowhere is one this suite
// cannot tell from a check that was deleted.
func TestTheHouseCorpusExercisesEveryCheck(t *testing.T) {
	name, lines := readCorpus(t, houseCorpus)
	fired := map[string]int{}
	for _, f := range voiceScanner().scanProse(name, lines) {
		fired[f.Check]++
	}
	for _, check := range AllChecks {
		if slices.Contains(kindChecks, check) {
			continue
		}
		if fired[check] == 0 {
			t.Errorf("%s fires nowhere in the house corpus, so this suite cannot tell it from a deleted check", check)
		}
	}
}

// The same notes in the register the rule asks for. Zero, because a rule whose own examples trip its
// check is a rule nobody can satisfy.
func TestTheRegisterTheRuleAsksForReportsNothing(t *testing.T) {
	name, lines := readCorpus(t, plainCorpus)
	found := voiceScanner().scanProse(name, lines)
	if len(found) != 0 {
		t.Fatalf("%d finding(s) over prose written the way the rule asks:\n%s", len(found), render(found))
	}
}

// The two corpora say the same things under the same headings, so a difference in what the check
// reports is a difference in register and not in subject.
func TestTheTwoCorporaDescribeTheSameSymbols(t *testing.T) {
	headings := func(dir string) []string {
		_, lines := readCorpus(t, dir)
		var names []string
		for _, line := range lines {
			if strings.HasPrefix(line, "## ") {
				names = append(names, line)
			}
		}
		sort.Strings(names)
		return names
	}
	house, plain := headings(houseCorpus), headings(plainCorpus)
	if len(house) == 0 {
		t.Fatal("the house corpus names nothing")
	}
	if strings.Join(house, "\n") != strings.Join(plain, "\n") {
		t.Fatalf("the corpora describe different symbols, so the difference between them is not register:\n--- house ---\n%s\n--- plain ---\n%s",
			strings.Join(house, "\n"), strings.Join(plain, "\n"))
	}
}

func render(found []Finding) string {
	var out []string
	for _, f := range found {
		out = append(out, "  "+f.String())
	}
	return strings.Join(out, "\n")
}

func TestEachCheckFiresOnItsOwnShapeAndNotOnPlainProse(t *testing.T) {
	cases := []struct {
		check string
		fires string
		plain string
	}{
		{checkBold, "The **one** rule here.", "The one rule is stated once."},
		{checkContrast, "Read from the book rather than the entry.", "Reads the book's attribute."},
		{checkContrast, "Instead of totalling, the ledger is left whole.", "Accepts an Entry instead of a string."},
		{checkCounterfactal, "Otherwise the reader picks the older entry.", "The reader picks the older entry when nothing narrows."},
		{checkNoSubject, "Counted across the whole ledger.", "Counts every entry across the ledger."},
		{checkNoSubject, "Reading entries out of a ledger.", "Reads entries out of a ledger."},
		{checkIntensifier, "Nothing ties the entry to its book.", "The entry has no compile-time tie to its book."},
		{checkPositional, "Distinct from the token above.", "Distinct from `Untotalled`."},
		{checkCoined, "The reader climbs to the newest entry.", "The reader selects the newest entry."},
	}
	s := voiceScanner()
	for _, c := range cases {
		t.Run(c.check+"/"+c.fires, func(t *testing.T) {
			if !hasCheck(s.scanProse("f.md", []string{c.fires}), c.check) {
				t.Errorf("%q produced no %s finding, so the check cannot fire", c.fires, c.check)
			}
			if hasCheck(s.scanProse("f.md", []string{c.plain}), c.check) {
				t.Errorf("%q produced a %s finding, and it is plain prose", c.plain, c.check)
			}
		})
	}
}

func hasCheck(found []Finding, check string) bool {
	for _, f := range found {
		if f.Check == check {
			return true
		}
	}
	return false
}

func TestAnImperativeEndingInIngIsNotADroppedSubject(t *testing.T) {
	imperatives := []string{"Bring the choice to the human.", "String the calls together.",
		"Ring the bell once.", "Swing the branch back."}
	gerunds := []string{"Reading the ledger out of the response.", "Counting across the ledger.",
		"Narrowing the ledger to one entry."}
	for _, sentence := range imperatives {
		if opensWithGerund(sentence) {
			t.Errorf("%q was read as a dropped subject, and it is an imperative", sentence)
		}
	}
	for _, sentence := range gerunds {
		if !opensWithGerund(sentence) && !reParticipleOpen.MatchString(sentence) {
			t.Errorf("%q dropped its subject and the check said nothing", sentence)
		}
	}
}

// The instruction profile reads a rule file's prose and nothing it uses as structure. A heading is a
// label; a fenced block is the example the rule is stating; the frontmatter is machine-read.
func TestTheInstructionProfileSkipsHeadingsFencesAndFrontmatter(t *testing.T) {
	file := []string{
		"---",
		"name: nothing-here",
		"---",
		"# Nothing above this line",
		"",
		"```ts",
		"Counted across the whole ledger rather than per book.",
		"```",
		"",
		"State the fact rather than the alternative.",
	}
	s := scanner{profile: ProfileInstruction}
	found := s.scanProse("x.md", file)
	if len(found) != 1 {
		t.Fatalf("want one finding, the prose line's; got %d:\n%s", len(found), render(found))
	}
	if found[0].Line != 10 || found[0].Check != checkContrast {
		t.Fatalf("want the contrast on line 10; got %s", found[0])
	}
}

// Bold is markdown, so a rule file's own bold is structure and the prose profile's concern alone. A
// rule file's bold is answered for by whoever rewrites it, never by a check that would report every
// defined term in the tree.
func TestBoldIsAFindingInABodyAndNotInARuleFile(t *testing.T) {
	line := "The **one** rule."
	if !hasCheck(scanner{profile: ProfileProse}.scanProse("b.md", []string{line}), checkBold) {
		t.Error("the prose profile did not report a bold span")
	}
	if hasCheck(scanner{profile: ProfileInstruction}.scanProse("x.md", []string{line}), checkBold) {
		t.Error("the instruction profile reported a bold span, which is a rule file's own structure")
	}
}

func TestAnInlineCodeSpanIsNotReadAsProse(t *testing.T) {
	quoted := "Close with one line: `Next: <the one immediate action>`."
	bare := "Close with one line: Next: the one immediate action."
	s := scanner{profile: ProfileInstruction}
	if hasCheck(s.scanProse("x.md", []string{quoted}), checkIntensifier) {
		t.Error("a template inside backticks was read as prose")
	}
	if !hasCheck(s.scanProse("x.md", []string{bare}), checkIntensifier) {
		t.Error("the same words outside backticks produced no finding, so the span is not what was skipped")
	}
}

func TestAFindingEchoesTheLineAsItWasTyped(t *testing.T) {
	line := "Guarded with `hasOwnProperty` rather than indexed directly."
	found := voiceScanner().scanProse("f.md", []string{line})
	if !hasCheck(found, checkNoSubject) {
		t.Fatal("the participial opener was not reported, so this case measures nothing")
	}
	for _, f := range found {
		if strings.Contains(f.Text, "  ") {
			t.Errorf("%s echoes the blanked line rather than the typed one", f)
		}
		if f.Check == checkNoSubject && !strings.Contains(f.Text, "`hasOwnProperty`") {
			t.Errorf("the echoed text lost the identifier the writer typed: %s", f)
		}
	}
}

func TestASentenceThatWrapsAcrossTwoLinesIsReadWhole(t *testing.T) {
	wrapped := []string{
		"Read the book's own attribute",
		"alone and a book shaped the other way wins.",
	}
	found := voiceScanner().scanProse("f.md", wrapped)
	if !hasCheck(found, checkCounterfactal) {
		t.Fatalf("the wrapped counterfactual was not read:\n%s", render(found))
	}
	for _, f := range found {
		if f.Check != checkCounterfactal {
			continue
		}
		if f.Line != 1 {
			t.Errorf("reported on line %d, want line 1 where the sentence starts", f.Line)
		}
		if !strings.Contains(f.Text, "alone and") {
			t.Errorf("the echoed text stops at the line break: %q", f.Text)
		}
	}
}

// A paragraph in a rule file wraps in a repository file and does not in a field you type into. Both
// shapes reach the prose and instruction profiles whole.
func TestAWrappedParagraphInAProseFileIsReadWhole(t *testing.T) {
	wrapped := []string{"State the fact rather", "than the alternative.", "", "A second paragraph."}
	found := scanner{profile: ProfileInstruction}.scanProse("x.md", wrapped)
	if !hasCheck(found, checkContrast) {
		t.Fatalf("the contrast spanning the line break was not read:\n%s", render(found))
	}
	if found[0].Line != 1 {
		t.Errorf("reported on line %d, want line 1 where the sentence starts", found[0].Line)
	}
}

// A blank line ends a paragraph, so two paragraphs are never read as one sentence. The two halves here
// match nothing apart and match a contrast spine together, so a paragraph that ran past its blank line
// would report a phrase nobody wrote.
func TestABlankLineEndsAParagraph(t *testing.T) {
	apart := []string{"Read the book,", "", "never the entry."}
	if found := (scanner{profile: ProfileInstruction}).scanProse("x.md", apart); len(found) != 0 {
		t.Fatalf("two paragraphs were read as one:\n%s", render(found))
	}
	together := []string{"Read the book,", "never the entry."}
	if found := (scanner{profile: ProfileInstruction}).scanProse("x.md", together); !hasCheck(found, checkContrast) {
		t.Fatalf("one paragraph over two lines was not read whole:\n%s", render(found))
	}
}

func TestACoinedWordOpeningOnAMultiByteRuneCompiles(t *testing.T) {
	for _, word := range []string{"échelon", "über", "日本語", "rung"} {
		pattern := coinedInIdentifier(word)
		if pattern == nil {
			t.Errorf("%q produced no pattern", word)
		}
	}
	s := scanner{profile: ProfileProse, coined: []string{"échelon"}}
	if found := s.scanProse("f.md", []string{"Reads the échelon from the entry."}); !hasCheck(found, checkCoined) {
		t.Error("a coined word opening on a multi-byte rune was not reported")
	}
}

func TestTheInstructionProfileDoesNotRunTheCoinedCheck(t *testing.T) {
	line := "Cut the hedge frames and the drift in tense."
	s := scanner{coined: []string{"hedge", "drift"}}

	s.profile = ProfileInstruction
	if found := s.scanProse("x.md", []string{line}); hasCheck(found, checkCoined) {
		t.Errorf("a rule file was reported for using English:\n%s", render(found))
	}
	s.profile = ProfileProse
	if found := s.scanProse("b.md", []string{line}); !hasCheck(found, checkCoined) {
		t.Error("the prose profile did not run the coined check, and a body about the work should")
	}
}

// A backticked identifier in a body is a quotation, so a coined word inside one is no finding. The
// same identifier written bare is the coined word in the writer's own prose.
func TestACoinedWordInsideBackticksIsAQuotation(t *testing.T) {
	s := scanner{profile: ProfileProse, coined: []string{"sprocket"}}
	if found := s.scanProse("b.md", []string{"The rename is readSprocket, landing next."}); !hasCheck(found, checkCoined) {
		t.Error("a body did not report a coined word inside a bare identifier")
	}
	if found := s.scanProse("b.md", []string{"The rename is `readSprocket`, landing next."}); hasCheck(found, checkCoined) {
		t.Error("a body reported a coined word inside a quoted identifier")
	}
}

// A request names its scheme literally, and run 25's writer bent "the request names no scheme" into a
// passive to pass. The idiom for an absent thing still draws the finding.
func TestNamesNoOfAThingACallNamesIsPlain(t *testing.T) {
	s := voiceScanner()
	plain := "A ledger that ignores the mode stalls, which is why the request names no scheme."
	if found := s.scanProse("f.md", []string{plain}); hasCheck(found, checkCoined) {
		t.Errorf("%q reports a coined phrase", plain)
	}
	for _, idiom := range []string{"The entry names no owner, so the walk skips it.", "The entry names nothing the book holds."} {
		if found := s.scanProse("f.md", []string{idiom}); !hasCheck(found, checkCoined) {
			t.Errorf("%q reports no coined phrase", idiom)
		}
	}
}

func TestAStreamOverTheCapIsRefusedRatherThanTruncated(t *testing.T) {
	under := strings.Repeat("x", 16)
	if _, err := readAllCapped(strings.NewReader(under), 32); err != nil {
		t.Fatalf("a stream inside the cap was refused: %v", err)
	}
	at := strings.Repeat("x", 32)
	if _, err := readAllCapped(strings.NewReader(at), 32); err != nil {
		t.Fatalf("a stream exactly at the cap was refused: %v", err)
	}
	over := strings.Repeat("x", 33)
	body, err := readAllCapped(strings.NewReader(over), 32)
	if err == nil {
		t.Fatalf("a stream over the cap was truncated to %d bytes and read as complete", len(body))
	}
	if !strings.Contains(err.Error(), "report clean over the rest") {
		t.Fatalf("the refusal does not say what truncation would have cost: %v", err)
	}
}

func TestTwoCoinedWordsPartedByOneByteAreTwoFindings(t *testing.T) {
	s := scanner{profile: ProfileProse, coined: []string{"rung"}}
	for _, line := range []string{"rung rung", "The rung. Rung again."} {
		found := s.scanProse("f.md", []string{line})
		coined := 0
		for _, f := range found {
			if f.Check == checkCoined {
				coined++
			}
		}
		if coined != 2 {
			t.Errorf("%q reported %d coined finding(s); want 2", line, coined)
		}
	}
}

// The contrast spine with the comma dropped and a conjunction in its place, beside the two shapes it
// is not. The `both` fixtures hold those two. One forbids a pair of things. The other defines one
// thing against another, which is outside what the reader asked.
//
// What tells them apart is whether the words before the conjunction already carry a negation.
func TestTwoProhibitionsInOneSentenceAreNotTheSpine(t *testing.T) {
	s := scanner{profile: ProfileProse}
	spine := []string{
		"It is logged and not believed.",
		"The string is thrown and not the object.",
		"It is a survey and no verdict.",
	}
	both := []string{
		"Use no nesting and no preamble above the items.",
		"Write no speculative abstraction and no flexibility the task did not ask for.",
		"Use no headings, and no bold lead-in restating its own line.",
		// Neither half is a negation, so the conjunction joins two things the sentence names.
		"The string is thrown and the object is kept.",
	}
	for _, line := range spine {
		if !hasCheck(s.scanProse("f.md", []string{line}), checkContrast) {
			t.Errorf("%q is the spine and produced no finding", line)
		}
	}
	for _, line := range both {
		if hasCheck(s.scanProse("f.md", []string{line}), checkContrast) {
			t.Errorf("%q forbids two things and was read as the spine", line)
		}
	}
}

// A threshold off by one here would report every note the rule asks for.
func TestAConformingNoteIsNotAClauseDepthFinding(t *testing.T) {
	s := scanner{profile: ProfileProse}
	conforming := "Some platforms reject a detached call, so the method is called on its object."
	if hasCheck(s.scanProse("f.md", []string{conforming}), checkClauseDepth) {
		t.Errorf("%q is the note pattern the rule asks for and it produced a clause-depth finding", conforming)
	}
	deep := "The call is kept because the platform rejects it, which the older fleet does while it upgrades."
	if !hasCheck(s.scanProse("f.md", []string{deep}), checkClauseDepth) {
		t.Errorf("%q holds four connectives and produced no clause-depth finding", deep)
	}
}

// A term the rules ask to be said in the words of the code's condition takes "in which". That phrase
// defines the term, and the act-first note it sits in passes.
func TestATermDefinedWithInWhichIsNotAClauseDepthFinding(t *testing.T) {
	s := scanner{profile: ProfileProse}
	note := "Drops a posting book in which no posting declares the currency, because narrowing would empty it."
	if hasCheck(s.scanProse("f.md", []string{note}), checkClauseDepth) {
		t.Errorf("%q is the wording the Terms rule asks for and it produced a clause-depth finding", note)
	}
}

func TestOneNegationIsNotADoubleNegativeFinding(t *testing.T) {
	s := scanner{profile: ProfileProse}
	single := "The field is not set on an older export."
	if hasCheck(s.scanProse("f.md", []string{single}), checkDoubleNeg) {
		t.Errorf("%q carries one negation and produced a double-negative finding", single)
	}
	double := "The code is not absent and it is not unknown."
	if !hasCheck(s.scanProse("f.md", []string{double}), checkDoubleNeg) {
		t.Errorf("%q carries two negations and produced no finding", double)
	}
}

// A paragraph wraps, and the phrase then spans two lines. The check reads the joined paragraph, so a
// wrapped phrase is the same phrase.
func TestACoinedPhraseIsMatchedAcrossAWrappedLine(t *testing.T) {
	s := scanner{profile: ProfileProse}
	wrapped := []string{"A pairing that cannot occur has no", "name in the catalogue."}
	if !hasCheck(s.scanProse("f.md", wrapped), checkCoined) {
		t.Errorf("a built-in coined phrase broken across two lines produced no finding")
	}
	plain := []string{"A pairing that cannot occur is not listed in the catalogue."}
	if hasCheck(s.scanProse("f.md", plain), checkCoined) {
		t.Errorf("the plain form produced a coined finding")
	}
}

func TestAStemEndingInEdIsNoParticiple(t *testing.T) {
	s := scanner{profile: ProfileProse}
	for _, stem := range []string{
		"The gate holds, proceed to the next stage.",
		"The count rose, need for a second pass.",
		"One row was red, against a green tree.",
	} {
		if hasCheck(s.scanProse("f.md", []string{stem}), checkNoSubject) {
			t.Errorf("%q was read as a dropped subject, and its `ed` is part of the stem", stem)
		}
	}
	for _, participle := range []string{
		"The tree measures clean, fed by a scan nobody ran.",
		"The row stays, read against the baseline it carries.",
	} {
		if !hasCheck(s.scanProse("f.md", []string{participle}), checkNoSubject) {
			t.Errorf("%q dropped its subject and went unreported", participle)
		}
	}
}

// The contrast spine defines a thing against what it is not. A comma before `and` opens a new clause,
// so the same words carry a second fact. The instruction tree held ten of those.
func TestACommaBeforeAndOpensAClauseAndNotTheSpine(t *testing.T) {
	s := scanner{profile: ProfileProse}
	for _, clause := range []string{
		"The gates are green, and no requirement is left undelivered.",
		"The rule names its own model, and no worker is lowered to match.",
	} {
		if hasCheck(s.scanProse("f.md", []string{clause}), checkContrast) {
			t.Errorf("%q states a second fact and was read as a contrast", clause)
		}
	}
	for _, spine := range []string{
		"It is a survey and no verdict.",
		"It is a trailer and not a subject prefix.",
	} {
		if !hasCheck(s.scanProse("f.md", []string{spine}), checkContrast) {
			t.Errorf("%q is the spine and went unreported", spine)
		}
	}
}

func TestATableRowIsDataInBothTextProfiles(t *testing.T) {
	table := []string{
		"| Verdict | Action |",
		"|---|---|",
		"| `keep` | leave it, since the block states a fact the code cannot show anywhere |",
		"| `obvious` | delete it, because every sentence restates the name or the lines beneath |",
	}
	for _, profile := range textProfiles {
		s := scanner{profile: profile}
		for _, one := range s.scanProse("f.md", table) {
			if one.Check == checkLongSentence || one.Check == checkClauseDepth {
				t.Errorf("the %s profile read the rows as one sentence: %v", profile, one)
			}
		}
	}
}

func TestUnpunctuatedListItemsAreReadOneAtATime(t *testing.T) {
	list := []string{
		"# Scope",
		"",
		"- `shared`: `Config`, the loader and its defaults move into the shared package with no change",
		"- `player`: the player reads its settings from the shared loader instead of its own copy",
		"* `ui`: the settings page renders from the same loader and drops its private fallback values",
		"1. `tests`: every suite that built a config by hand now calls the shared builder",
		"2) `docs`: the README section on configuration points at the shared package",
	}
	for _, profile := range textProfiles {
		for _, one := range (scanner{profile: profile}).scanProse("f.md", list) {
			if one.Check == checkLongSentence {
				t.Errorf("the %s profile read the items as one sentence: %v", profile, one)
			}
		}
	}
}

func TestALongSentenceInAWrappedListItemIsCaughtOnItsFirstLine(t *testing.T) {
	list := []string{
		"- a short item",
		"- this item carries one sentence that keeps on going past every reasonable length a reader",
		"  can hold in mind at once while the writer adds clause after clause without ever stopping",
		"- another short item",
	}
	found := (scanner{profile: ProfileProse}).scanProse("f.md", list)
	if !hasCheck(found, checkLongSentence) {
		t.Fatalf("the long sentence inside an item went unread:\n%s", render(found))
	}
	for _, one := range found {
		if one.Check == checkLongSentence && one.Line != 2 {
			t.Errorf("long-sentence reported on line %d, want line 2 where the item starts", one.Line)
		}
	}
}

// textProfiles lists the two profiles that read a whole text file.
var textProfiles = []Profile{ProfileProse, ProfileInstruction}

// A cell's own prose is still read. Joined, these rows make a long clausal pseudo-sentence, and each
// cell alone is plain, so a check firing per cell is the coverage a skip would have cost.
func TestACellsOwnProseIsStillRead(t *testing.T) {
	table := []string{
		"| Verdict | Action |",
		"|---|---|",
		"| `obvious` | Counted across the whole ledger, it goes. |",
	}
	s := scanner{profile: ProfileInstruction}
	found := s.scanProse("f.md", table)
	if !hasCheck(found, checkNoSubject) {
		t.Errorf("a cell dropping its subject went unread: %v", found)
	}
	for _, one := range found {
		if one.Line != 3 {
			t.Errorf("a cell's finding is reported on line %d, and its row is line 3", one.Line)
		}
	}
}

// The same words outside a table are read like any other paragraph, which leaves the row as what
// does the work in the case before this one.
func TestTheSameWordsOutsideATableAreRead(t *testing.T) {
	s := scanner{profile: ProfileProse}
	prose := []string{"Delete it because every sentence restates the name and no reader is served."}
	if found := s.scanProse("f.md", prose); len(found) == 0 {
		t.Error("a paragraph carrying the same words went unread")
	}
}

// A sentence carrying one semicolon joins two clauses, whatever the tail's length. The floor was
// written for a list, where a short tail is a field, and it let seventeen real joins through in the
// instruction tree.
func TestOneSemicolonJoinsTwoClausesWhateverItsTail(t *testing.T) {
	s := scanner{profile: ProfileProse}
	for _, join := range []string{
		"This check is important; it never fails.",
		"Extend the hand-written tests; do not clobber them.",
	} {
		if !hasCheck(s.scanProse("f.md", []string{join}), checkSemicolon) {
			t.Errorf("%q joins two clauses and went unreported", join)
		}
	}
	for _, list := range []string{
		"Reads three fields: owner; entry; book.",
		"The kinds are comment; commit; reply.",
	} {
		if hasCheck(s.scanProse("f.md", []string{list}), checkSemicolon) {
			t.Errorf("%q is a list and was read as a clause join", list)
		}
	}
}

func TestOneSemicolonInACellSeparatesFields(t *testing.T) {
	cell := []string{"| Unit | One unit of behaviour | Real in-process collaborators; no real I/O |"}
	for _, profile := range textProfiles {
		s := scanner{profile: profile}
		if hasCheck(s.scanProse("f.md", cell), checkSemicolon) {
			t.Errorf("the %s profile read a cell's two fields as a clause join", profile)
		}
	}
	prose := []string{"Real in-process collaborators; no real I/O is reached."}
	s := scanner{profile: ProfileProse}
	if !hasCheck(s.scanProse("f.md", prose), checkSemicolon) {
		t.Error("the same semicolon outside a table went unreported")
	}
}

func TestEmphasisIsTheAdverbAndNotTheDeterminer(t *testing.T) {
	s := scanner{profile: ProfileProse}
	for _, padding := range []string{
		"This is a very important check.",
		"The bound is crucially wrong.",
	} {
		if !hasCheck(s.scanProse("f.md", []string{padding}), checkIntensifier) {
			t.Errorf("%q raises a claim without adding to it and went unreported", padding)
		}
	}
	for _, naming := range []string{
		"A verify run that discovers this very case.",
		"It reads the directory from the very first entry.",
	} {
		if hasCheck(s.scanProse("f.md", []string{naming}), checkIntensifier) {
			t.Errorf("%q names a thing and was read as padding", naming)
		}
	}
}

// The count this mode prints for a path is the count a run over that path alone reports. The ratchet
// in voice-baseline.sh holds 72 files to a recorded number, and the two counts have to agree. A
// disagreement moves every file off its line at once.
func TestACountIsWhatARunOverThatPathAloneReports(t *testing.T) {
	r := newRepo(t)
	r.write("a.md", housey(3))
	r.write("b.md", "The register has no complaint about this line.\n")
	r.write("c.md", housey(2))
	named := []string{"a.md", "b.md", "c.md"}

	r.run(append([]string{"--profile=instruction", "--per-file"}, named...)...)
	r.expectCode(1)
	together := r.stdout.String()
	// Three files measuring zero would compare two empty reports, which a broken pairing also passes.
	if strings.Contains(together, "0 a.md") || !strings.Contains(together, "0 b.md") {
		t.Fatalf("the fixture no longer carries a file with findings beside one without: %q", together)
	}

	var apart strings.Builder
	for _, name := range named {
		r.run("--profile=instruction", name)
		findings := strings.Count(strings.TrimSpace(r.stdout.String()), "\n")
		if strings.TrimSpace(r.stdout.String()) != "" {
			findings++
		}
		fmt.Fprintf(&apart, "%d %s\n", findings, name)
	}
	if together != apart.String() {
		t.Errorf("one run reports\n%s\nand a run per path reports\n%s", together, apart.String())
	}
}

// A file with no findings gets a line too. The ratchet tells a new file measuring zero from one
// carrying findings, and a file left out of the report reads as a file that was never measured.
func TestAPathWithNoFindingsStillGetsACount(t *testing.T) {
	r := newRepo(t)
	r.write("quiet.md", "The register has no complaint about this line.\n")
	r.run("--profile=instruction", "--per-file", "quiet.md")
	r.expectCode(0)
	r.expectStdoutHas("0 quiet.md")
}

func TestCountsRunPastTheDisplayCap(t *testing.T) {
	r := newRepo(t)
	r.write("wall.md", housey(maxFindings+5))
	r.write("last.md", housey(1))

	r.run("--profile=instruction", "wall.md")
	r.expectStdoutHas("further finding(s), not shown")

	r.run("--profile=instruction", "--per-file", "wall.md", "last.md")
	printed := strings.Split(strings.TrimSpace(r.stdout.String()), "\n")
	if len(printed) != 2 || !strings.HasSuffix(printed[1], " last.md") {
		t.Fatalf("the path after the wall got no line of its own: %q", r.stdout.String())
	}
	walled := 0
	if _, err := fmt.Sscanf(printed[0], "%d", &walled); err != nil || walled <= maxFindings {
		t.Errorf("%q counts %d findings against a cap of %d, so this case crosses nothing", printed[0], walled, maxFindings)
	}
}

// A run given no path measured zero files, and exit 0 there would read as a clean tree.
func TestCountingWithNoPathRefusesTheRun(t *testing.T) {
	r := newRepo(t)
	r.run("--profile=instruction", "--per-file")
	r.expectCode(2)
	r.expectStderrHas("needs a path")
	r.expectNoStdout()
}

func TestTheBooleanCheckPassesOverAComparative(t *testing.T) {
	lines := []string{
		"A bare fact is one sentence saying no more than itself.",
	}
	for _, f := range voiceScanner().scanProse("f.md", lines) {
		if f.Check == checkAnthropo {
			t.Errorf("reported %q, and that is the ordinary word before a comparative", f.Text)
		}
	}
	saying := []string{
		"A device can say no to the entry type.",
	}
	found := false
	for _, f := range voiceScanner().scanProse("f.md", saying) {
		if f.Check == checkAnthropo {
			found = true
		}
	}
	if !found {
		t.Errorf("the guard silenced a true finding, which is the control for it")
	}
}
