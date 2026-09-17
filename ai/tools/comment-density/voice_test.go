package density

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

// The words the corpus below treats as coined: a metaphor for a mechanism, and a word a reader would
// have to have been in the room for. A repository names its own in `comment-voice.conf`.
var fixtureCoined = []string{"climb", "drift", "slip", "hedge", "sprocket", "wobble"}

const (
	houseCorpus = "testdata/voice/house"
	plainCorpus = "testdata/voice/plain"
)

// The two corpora are the same declarations written twice: once in the register this check exists to
// find, once in the register the rule asks for. They hold the shape and not any subject, and they are
// written for this suite so it depends on no tree outside this repository.
//
// A labelled corpus that does live outside it is read by the env-gated cases at the end of this file.
func readCorpus(t *testing.T, dir string) (string, []string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.ts"))
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
	return scanner{profile: ProfileComment, coined: fixtureCoined}
}

// Every block written in the house register has to produce a finding. A block that slips through is a
// comment the lane would pass unread, which is the whole of what this check exists to stop.
func TestEveryBlockInTheHouseRegisterProducesAFinding(t *testing.T) {
	name, lines := readCorpus(t, houseCorpus)
	found := voiceScanner().scanSource(name, lines, nil)
	blocks := commentBlocks(lines)
	if len(blocks) < 7 {
		t.Fatalf("the house corpus holds %d blocks; it has to exercise every check", len(blocks))
	}
	for _, b := range blocks {
		reported := false
		for _, f := range found {
			if b.start <= f.Line && f.Line <= b.end {
				reported = true
			}
		}
		if !reported {
			t.Errorf("no finding in the house-register block at lines %d-%d:\n%s",
				b.start, b.end, strings.Join(lines[b.start-1:b.end], "\n"))
		}
	}
}

// Every check has to fire somewhere in the corpus. A check that fires nowhere is one this suite
// cannot tell from a check that was deleted.
func TestTheHouseCorpusExercisesEveryCheck(t *testing.T) {
	name, lines := readCorpus(t, houseCorpus)
	fired := map[string]int{}
	for _, f := range voiceScanner().scanSource(name, lines, nil) {
		fired[f.Check]++
	}
	for _, check := range AllChecks {
		if fired[check] == 0 {
			t.Errorf("%s fires nowhere in the house corpus, so this suite cannot tell it from a deleted check", check)
		}
	}
}

// The same declarations in the register the rule asks for. Zero, because a rule whose own examples
// trip its check is a rule nobody can satisfy.
func TestTheRegisterTheRuleAsksForReportsNothing(t *testing.T) {
	name, lines := readCorpus(t, plainCorpus)
	found := voiceScanner().scanSource(name, lines, nil)
	if len(found) != 0 {
		t.Fatalf("%d finding(s) over prose written the way the rule asks:\n%s", len(found), render(found))
	}
}

// The two corpora say the same things about the same declarations, so a difference in what the check
// reports is a difference in register and not in subject.
func TestTheTwoCorporaDeclareTheSameSymbols(t *testing.T) {
	declarations := func(dir string) []string {
		_, lines := readCorpus(t, dir)
		var names []string
		for _, line := range lines {
			if strings.HasPrefix(line, "export function ") || strings.HasPrefix(line, "export enum ") ||
				strings.HasPrefix(line, "export interface ") {
				names = append(names, line)
			}
		}
		sort.Strings(names)
		return names
	}
	house, plain := declarations(houseCorpus), declarations(plainCorpus)
	if len(house) == 0 {
		t.Fatal("the house corpus declares nothing")
	}
	if strings.Join(house, "\n") != strings.Join(plain, "\n") {
		t.Fatalf("the corpora declare different symbols, so the difference between them is not register:\n--- house ---\n%s\n--- plain ---\n%s",
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
		{checkBold, "// The **one** rule here.", "// The one rule is stated once."},
		{checkContrast, "// Read from the book rather than the entry.", "// Reads the book's attribute."},
		{checkContrast, "// Instead of totalling, the ledger is left whole.", "// Accepts an Entry instead of a string."},
		{checkCounterfactal, "// Otherwise the reader picks the older entry.", "// The reader picks the older entry when nothing narrows."},
		{checkNoSubject, "// Counted across the whole ledger.", "// Counts every entry across the ledger."},
		{checkNoSubject, "// Reading entries out of a ledger.", "// Reads entries out of a ledger."},
		{checkNoSubject, "// Narrowing a ledger to one entry.", "// Bring the choice to the human."},
		{checkIntensifier, "// Nothing ties the entry to its book.", "// The entry has no compile-time tie to its book."},
		{checkPositional, "// Distinct from the token above.", "// Distinct from `Untotalled`."},
		{checkCoined, "// The reader climbs to the newest entry.", "// The reader selects the newest entry."},
	}
	s := voiceScanner()
	for _, c := range cases {
		t.Run(c.check+"/"+c.fires, func(t *testing.T) {
			if !hasCheck(s.scanSource("f.ts", []string{c.fires}, nil), c.check) {
				t.Errorf("%q produced no %s finding, so the check cannot fire", c.fires, c.check)
			}
			if hasCheck(s.scanSource("f.ts", []string{c.plain}, nil), c.check) {
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

// An imperative can end in `ing`, and an imperative with its subject implied is the register these
// rules are written in. A check that read `Bring` as a dropped subject would report every rule file
// for obeying the rule.
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

// A block's length is its words, so the `/**` and `*/` a language demands do not spend the allowance.
// Without this a four-sentence block reads as six lines and every docstring is a finding.
func TestABlockIsMeasuredInTheLinesThatCarryWords(t *testing.T) {
	four := []string{"const a = 1;", "/**", " * One.", " * Two.", " * Three.", " * Four.", " */", "const b = 2;"}
	five := []string{"const a = 1;", "/**", " * One.", " * Two.", " * Three.", " * Four.", " * Five.", " */", "const b = 2;"}
	s := scanner{profile: ProfileComment}
	if hasCheck(s.scanSource("f.ts", four, nil), checkLongBlock) {
		t.Error("a four-sentence block was reported long; its `/**` and `*/` carry no words")
	}
	if !hasCheck(s.scanSource("f.ts", five, nil), checkLongBlock) {
		t.Error("a five-line block was not reported long")
	}
}

// A file header is allowed eight lines, because a published surface states call order, lifecycle and
// error modes there and nowhere else.
func TestAFileHeaderIsAllowedMoreThanABlockInTheBody(t *testing.T) {
	header := []string{"/**"}
	for i := 1; i <= 6; i++ {
		header = append(header, " * Line "+strconv.Itoa(i)+".")
	}
	header = append(header, " */", "const a = 1;")
	s := scanner{profile: ProfileComment}
	if hasCheck(s.scanSource("f.ts", header, nil), checkLongBlock) {
		t.Error("a six-line file header was reported long, and a header is allowed eight")
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
		"// Counted across the whole ledger rather than per book.",
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

// Bold is markdown, so a rule file's own bold is structure and the comment and prose profiles' concern
// alone. A rule file's bold is answered for by whoever rewrites it, never by a check that would report
// every defined term in the tree.
func TestBoldIsAFindingInACommentAndNotInARuleFile(t *testing.T) {
	line := "The **one** rule."
	if !hasCheck(scanner{profile: ProfileProse}.scanProse("b.md", []string{line}), checkBold) {
		t.Error("the prose profile did not report a bold span")
	}
	if hasCheck(scanner{profile: ProfileInstruction}.scanProse("x.md", []string{line}), checkBold) {
		t.Error("the instruction profile reported a bold span, which is a rule file's own structure")
	}
}

// A backticked span is the code's own text quoted into prose, so the register checks read past it.
// Without this the rule file that states `Next: <the one immediate action>` reports itself, and the
// only way to a clean tree is to stop quoting the templates the rules require.
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

// The coined check is the exception: an identifier built on a coined word is the rename the refactor
// lane owes, and backticks are where that identifier is written.
func TestACoinedWordInsideBackticksIsStillReported(t *testing.T) {
	found := voiceScanner().scanSource("f.ts", []string{"// Reads `readSprocket` from the entry."}, nil)
	if !hasCheck(found, checkCoined) {
		t.Error("a coined word inside an identifier was skipped with the rest of the span")
	}
}

// Every finding echoes the line as the writer typed it. Reading the blanked line back would hand them
// a sentence with holes in it and no way to find the words the check objected to.
func TestAFindingEchoesTheLineAsItWasTyped(t *testing.T) {
	line := "// Guarded with `hasOwnProperty` rather than indexed directly."
	found := voiceScanner().scanSource("f.ts", []string{line}, nil)
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

// A block is read as one text, so a sentence that wraps is read whole. Per line, a check sees half a
// sentence and matches nothing — and a sentence wrapping at the width of a screen is the ordinary
// case in a comment, not the exception, so this is most of what there is to find.
func TestASentenceThatWrapsAcrossTwoLinesIsReadWhole(t *testing.T) {
	wrapped := []string{
		"/**",
		" * Read the book's own attribute",
		" * alone and a book shaped the other way wins.",
		" */",
		"export function f() {}",
	}
	found := voiceScanner().scanSource("f.ts", wrapped, nil)
	if !hasCheck(found, checkCounterfactal) {
		t.Fatalf("the wrapped counterfactual was not read:\n%s", render(found))
	}
	for _, f := range found {
		if f.Check != checkCounterfactal {
			continue
		}
		if f.Line != 2 {
			t.Errorf("reported on line %d, want line 2 where the sentence starts", f.Line)
		}
		if !strings.Contains(f.Text, "alone and") {
			t.Errorf("the echoed text stops at the line break: %q", f.Text)
		}
	}
}

// The same for a paragraph in a rule file, which wraps in a repository file and does not in a field
// you type into. Both shapes reach the prose and instruction profiles.
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

// A doc tag line is the signature written out, not a sentence. Counted as prose, a function with six
// parameters is a long block for having documented them, which is the one shape the rule wants.
func TestADocTagLineIsNotProse(t *testing.T) {
	tagged := []string{
		"const a = 1;",
		"/**",
		" * Returns the book's total.",
		" * @param book the book to total",
		" * @param currency the currency to total in",
		" * @returns the total",
		" * @throws when two currencies are declared",
		" */",
		"export function totalBook() {}",
	}
	found := voiceScanner().scanSource("f.ts", tagged, nil)
	if hasCheck(found, checkLongBlock) {
		t.Errorf("a one-sentence summary over a tag list was reported long:\n%s", render(found))
	}
	prose := []string{"const a = 1;", "/**", " * One.", " * Two.", " * Three.", " * Four.", " * Five.", " */", "const b = 2;"}
	if !hasCheck(voiceScanner().scanSource("f.ts", prose, nil), checkLongBlock) {
		t.Error("five lines of prose were not reported long, so the tag rule cut too much")
	}
}

// A tag line's words do not join the segment either. Joined, `@param book the book to total` would put
// the signature in the middle of whatever sentence ran before it.
func TestADocTagLineDoesNotJoinTheSentenceAroundIt(t *testing.T) {
	lines := []string{
		"/**",
		" * Totals the book.",
		" * @param book counted across the whole ledger",
		" */",
		"export function f() {}",
	}
	if found := voiceScanner().scanSource("f.ts", lines, nil); len(found) != 0 {
		t.Fatalf("a tag line was read as prose:\n%s", render(found))
	}
}

// Two things are scoped by the diff, and a test that moved only one of them would leave the other
// free: a block the diff did not touch at all, and a line the diff did not touch inside a block it
// did. The second is what makes a reworded sentence the change set's and the line above it not.
func TestTheCommentProfileReportsOnlyWhatADiffAdded(t *testing.T) {
	lines := []string{
		"// Counted across the whole ledger.",
		"// Guarded with a flag.",
		"const a = 1;",
		"",
		"// Matched by namespace.",
		"const b = 2;",
		"",
		"/**",
		" * One.",
		" * Two.",
		" * Three.",
		" * Four.",
		" * Five.",
		" */",
		"const c = 3;",
	}
	s := voiceScanner()
	all := s.scanSource("f.ts", lines, nil)
	if len(all) != 4 {
		t.Fatalf("want three sentences and one long block when nothing scopes the scan; got %s", render(all))
	}
	// Line 2 alone: the untouched blocks go, the untouched long block with them, and so does the
	// untouched line of the block the diff did touch.
	added := s.scanSource("f.ts", lines, map[int]bool{2: true})
	if len(added) != 1 || added[0].Line != 2 {
		t.Fatalf("want only the added line of the touched block; got %s", render(added))
	}
}

func TestAnAllowlistEntryNeedsACheckItRunsAndAReason(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		wants string
	}{
		{"no reason", "allow contrast rather than", "no reason"},
		{"empty reason", "allow contrast rather than # ", "no reason"},
		{"unknown check", "allow loudness rather than # because", "not one this scan runs"},
		{"no text", "allow contrast  # because", "no matched text"},
		{"unknown keyword", "suppress contrast rather than", "neither `coined` nor `allow`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := parseVoiceConf(c.line + "\n")
			if err == nil || !strings.Contains(err.Error(), c.wants) {
				t.Fatalf("got %v, want a refusal naming %q", err, c.wants)
			}
		})
	}
}

func TestAnAllowlistDropsOnlyTheFindingItNames(t *testing.T) {
	coined, allowed, err := parseVoiceConf(
		"coined climb\n" +
			"allow contrast rather than # the host repo's own phrase in this file, quoted\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(coined) != 1 || coined[0] != "climb" {
		t.Fatalf("want one coined word; got %v", coined)
	}
	s := scanner{profile: ProfileComment, coined: coined, allowed: allowed}
	found := s.scanSource("f.ts", []string{"// Read the book rather than climb it."}, nil)
	if hasCheck(found, checkContrast) {
		t.Error("the allowed contrast finding was still reported")
	}
	if !hasCheck(found, checkCoined) {
		t.Error("the allowlist dropped a coined finding it did not name")
	}
}

// The conf is the repository's before the machine's: a coined word is a property of the codebase, and
// a machine-wide list would answer for every repository the human works in.
func TestTheRepositorysOwnConfComesBeforeTheMachines(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".kk-flavor"), 0o755); err != nil {
		t.Fatal(err)
	}
	repoConf := filepath.Join(dir, ".kk-flavor", voiceConfName)
	if err := os.WriteFile(repoConf, []byte("coined climb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir+"/machine")
	t.Setenv("COMMENT_VOICE_CONF", "")
	got, ok := voiceConfPath(dir)
	if !ok || got != repoConf {
		t.Fatalf("resolved %q (found %v), want the repository's own conf at %q", got, ok, repoConf)
	}
}

func TestAConfThatDoesNotParseRefusesTheRunRatherThanScanningWithHalfOfIt(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, voiceConfName)
	if err := os.WriteFile(conf, []byte("coined climb\nallow contrast rather than\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMENT_VOICE_CONF", conf)
	if _, _, err := voiceConfig(dir); err == nil {
		t.Fatal("a conf with an entry carrying no reason was accepted")
	}
}

func TestAMissingConfIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMMENT_VOICE_CONF", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "absent"))
	coined, allowed, err := voiceConfig(dir)
	if err != nil || len(coined) != 0 || len(allowed) != 0 {
		t.Fatalf("got %v/%v/%v, want an empty configuration and no error", coined, allowed, err)
	}
}

func TestAnUnknownProfileRefusesTheRun(t *testing.T) {
	var out, errs strings.Builder
	code := Run("comment-density.sh", []string{"--voice", "--profile=loud"}, t.TempDir(),
		Config{MaxRatio: 0.3, MinLines: 5, MaxFileBytes: 1 << 18}, &out, &errs)
	if code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if !strings.Contains(errs.String(), "no profile") {
		t.Fatalf("stderr %q names no refused profile", errs.String())
	}
}

// The check against a labelled corpus held outside this repository, read only when VOICE_CORPUS names
// a directory. A corpus a human has labelled is the only thing that says whether the check finds what
// a reader finds, and it is not always shareable, so it is named rather than committed.
//
// VOICE_CORPUS_LABELS names the blocks that were labelled, as `<path>:<line>` relative to that
// directory, separated by commas. Both are set together or neither is: a corpus with no labels would
// run and measure nothing.
func TestTheLabelledReviewStillProducesAFindingInEveryFlaggedBlock(t *testing.T) {
	corpus := os.Getenv("VOICE_CORPUS")
	if corpus == "" {
		t.Skip("VOICE_CORPUS is unset; the labelled material is private and lives outside this repository")
	}
	labels := os.Getenv("VOICE_CORPUS_LABELS")
	if labels == "" {
		t.Fatal("VOICE_CORPUS is set and VOICE_CORPUS_LABELS is not, so this run would measure nothing")
	}
	s := voiceScanner()
	for _, label := range strings.Split(labels, ",") {
		label = strings.TrimSpace(label)
		where, number, ok := strings.Cut(label, ":")
		if !ok {
			t.Fatalf("label %q is not `<path>:<line>`", label)
		}
		at, err := strconv.Atoi(number)
		if err != nil {
			t.Fatalf("label %q names no line", label)
		}
		body, err := os.ReadFile(filepath.Join(corpus, where))
		if err != nil {
			t.Fatalf("label %q names a file the corpus does not hold: %v", label, err)
		}
		lines := shell.SplitLines(string(body))
		b, found := blockAt(lines, at)
		if !found {
			t.Errorf("%s has no comment block at or above it", label)
			continue
		}
		reported := false
		for _, f := range s.scanSource(where, lines, nil) {
			if b.start <= f.Line && f.Line <= b.end {
				reported = true
			}
		}
		if !reported {
			t.Errorf("%s is labelled and the check reports nothing in its block (lines %d-%d)",
				label, b.start, b.end)
		}
	}
}

// blockAt resolves a flagged line to the block it concerns: the block holding the line, or, where the
// note hangs on a declaration, the block above it. A note on a member is a note about its docstring.
func blockAt(lines []string, at int) (block, bool) {
	var above block
	haveAbove := false
	for _, b := range commentBlocks(lines) {
		if b.start <= at && at <= b.end {
			return b, true
		}
		if b.end < at {
			above, haveAbove = b, true
		}
	}
	return above, haveAbove
}

// The false-positive side of the same measurement: what the check says about a tree nobody asked it to
// change. Gated the same way and for the same reason.
//
// VOICE_CORPUS_HOST names a directory holding that tree, VOICE_CORPUS_CEILING and
// VOICE_CORPUS_LONG_CEILING the counts a rise past fails. VOICE_CORPUS_OURS is a comma-separated list
// of path prefixes inside it whose comments were written under the register this check looks for, held
// apart because a finding there is the check working rather than a false positive. Reported, never
// asserted: the number belongs in the change's own account, and a threshold here would only teach the
// next corpus to clear it.
func TestWhatTheCheckSaysAboutAHostRepositoryThatDidNotAskForIt(t *testing.T) {
	root := os.Getenv("VOICE_CORPUS_HOST")
	if root == "" {
		t.Skip("VOICE_CORPUS_HOST is unset; the tree it reads is private and lives outside this repository")
	}
	var ours []string
	if named := os.Getenv("VOICE_CORPUS_OURS"); named != "" {
		ours = strings.Split(named, ",")
	}
	s := voiceScanner()
	counts := map[string]int{}
	files := map[string]int{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".ts", ".tsx", ".js":
		default:
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, root), "/"))
		side := "host-authored"
		for _, prefix := range ours {
			if strings.HasPrefix(rel, strings.TrimSpace(prefix)) {
				side = "ours"
			}
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[side]++
		for _, f := range s.scanSource(rel, shell.SplitLines(string(body)), nil) {
			if f.Check == checkLongBlock {
				counts[side+"/long-block"]++
				continue
			}
			counts[side]++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files["host-authored"] == 0 {
		t.Fatal("no host-authored file was read, so this run says nothing about false positives")
	}
	t.Logf("host-authored: %d register finding(s) and %d long block(s) over %d files",
		counts["host-authored"], counts["host-authored/long-block"], files["host-authored"])
	t.Logf("ours:          %d register finding(s) and %d long block(s) over %d files",
		counts["ours"], counts["ours/long-block"], files["ours"])

	// Absolute ceilings, not a comparison between the two sides. "Louder on ours than on theirs"
	// passes at twelve against eleven and says nothing about whether the check got noisier, which is
	// the direction this measurement exists to catch. The numbers live with the corpus rather than
	// here, because they are a property of that tree.
	atMost(t, "register findings over host-authored files", counts["host-authored"], "VOICE_CORPUS_CEILING")
	atMost(t, "long blocks over host-authored files", counts["host-authored/long-block"], "VOICE_CORPUS_LONG_CEILING")
}

// atMost holds a count to a ceiling the corpus carries. An unset ceiling fails rather than skips: the
// caller named a corpus, so they asked for a measurement, and a measurement with no bound is a number
// nobody can fail.
func atMost(t *testing.T, what string, got int, variable string) {
	t.Helper()
	raw := os.Getenv(variable)
	if raw == "" {
		t.Fatalf("%s is unset, so %s (%d) is measured against nothing", variable, what, got)
	}
	ceiling, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s is %q, which is no whole number", variable, raw)
	}
	if got > ceiling {
		t.Errorf("%d %s, over the ceiling of %d", got, what, ceiling)
	}
}
