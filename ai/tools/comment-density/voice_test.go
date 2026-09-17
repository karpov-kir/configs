package density

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"kk-flavor/tools/diffscan"
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
		{"unknown check", "allow loudness rather than # because", "a check this scan does not run"},
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
	got, origin, ok := voiceConfPath(dir)
	if !ok || got != repoConf || origin != confRepository {
		t.Fatalf("resolved %q as %q (found %v), want the repository's own conf at %q", got, origin, ok, repoConf)
	}
}

func TestAConfThatDoesNotParseRefusesTheRunRatherThanScanningWithHalfOfIt(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, voiceConfName)
	if err := os.WriteFile(conf, []byte("coined climb\nallow contrast rather than\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMENT_VOICE_CONF", conf)
	if _, _, _, err := voiceConfig(dir); err == nil {
		t.Fatal("a conf with an entry carrying no reason was accepted")
	}
}

func TestAMissingConfIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMMENT_VOICE_CONF", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "absent"))
	coined, allowed, _, err := voiceConfig(dir)
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

// Over a diff the scan holds only the added lines, and the rest of the file is a gap. A gap read as a
// blank line makes every block look like a file header, because nothing but blanks stands above it —
// and a header is allowed twice a block's length, so five-to-eight-line blocks pass unreported in the
// one mode the lane actually runs.
func TestABlockInADiffDoesNotInheritTheFileHeadersAllowance(t *testing.T) {
	// A five-prose-line block at lines 40-46 and nothing else added. Every line above it is a gap, so
	// the array holds blanks there and the header test has only `held` to tell it this is not the top
	// of a file.
	lines := make([]string, 46)
	body := []string{"/**", " * One.", " * Two.", " * Three.", " * Four.", " * Five.", " */"}
	for i, text := range body {
		lines[39+i] = text
	}
	within := map[int]bool{}
	for at := 40; at <= 46; at++ {
		within[at] = true
	}
	found := voiceScanner().scanSource("f.ts", lines, within)
	if !hasCheck(found, checkLongBlock) {
		t.Fatalf("a five-line block deep in a file was not reported long:\n%s", render(found))
	}
}

// The same block at the top of a new file is a header and keeps the header's allowance, so the fix
// above does not simply delete the allowance.
func TestARealFileHeaderInADiffKeepsItsAllowance(t *testing.T) {
	lines := []string{"/**", " * One.", " * Two.", " * Three.", " * Four.", " * Five.", " */", "const a = 1;"}
	within := map[int]bool{}
	for at := 1; at <= len(lines); at++ {
		within[at] = true
	}
	if found := voiceScanner().scanSource("f.ts", lines, within); hasCheck(found, checkLongBlock) {
		t.Fatalf("a six-line header at the top of a new file was reported long:\n%s", render(found))
	}
}

// A `/*` run ends at a gap. Without that, a diff rewording one `/**` whose `*/` it never touched runs
// that block to the end of the file, swallowing every later comment into it — which reports one long
// block that does not exist and lets a register check match across the seam between two of them.
func TestAStarRunDoesNotSwallowTheCommentsBelowAGap(t *testing.T) {
	lines := make([]string, 20)
	lines[9] = "/** Reworded opening."
	for _, at := range []int{13, 15, 17, 19} {
		lines[at-1] = "// A separate one-line note."
	}
	within := map[int]bool{10: true, 13: true, 15: true, 17: true, 19: true}
	found := voiceScanner().scanSource("f.ts", lines, within)
	if hasCheck(found, checkLongBlock) {
		t.Fatalf("four one-line notes below an unclosed opening were read as one long block:\n%s", render(found))
	}
	blocks := commentBlocksIn(lines, onlyAdded(within))
	if len(blocks) != 5 {
		t.Fatalf("want five blocks, one per added line; got %d: %v", len(blocks), blocks)
	}
}

// stripMarker and isComment have to agree about a lone `*`. They disagreed: isComment refuses a `*`
// with no space after it, stripMarker stripped it anyway, so `**Bold**` opening a starless line inside
// a `/* */` block became `*Bold**` — the check could not fire and the echo was corrupt.
func TestABoldSpanOpeningAStarlessLineSurvivesTheMarkerStrip(t *testing.T) {
	if got := stripMarker("**Bold** and the rest."); got != "**Bold** and the rest." {
		t.Fatalf("stripMarker returned %q, want the line untouched", got)
	}
	if got := stripMarker(" * A continuation."); got != "A continuation." {
		t.Fatalf("stripMarker returned %q, want the continuation marker taken off", got)
	}
	lines := []string{"/*", "**Bold** opens this line.", " */", "const a = 1;"}
	found := scanner{profile: ProfileComment}.scanSource("f.ts", lines, nil)
	if !hasCheck(found, checkBold) {
		t.Fatalf("the bold span was not reported:\n%s", render(found))
	}
	for _, f := range found {
		if f.Check == checkBold && f.Text != "**Bold**" {
			t.Errorf("the echoed span is corrupt: %q", f.Text)
		}
	}
}

// A coined word opening on a multi-byte rune must not be title-cased by the byte. Sliced, it leaves an
// orphaned continuation byte in the pattern and regexp refuses it, so the tool dies with a Go stack
// trace where its own refusal belongs.
func TestACoinedWordOpeningOnAMultiByteRuneCompiles(t *testing.T) {
	for _, word := range []string{"échelon", "über", "日本語", "rung"} {
		pattern := coinedInIdentifier(word)
		if pattern == nil {
			t.Errorf("%q produced no pattern", word)
		}
	}
	s := scanner{profile: ProfileComment, coined: []string{"échelon"}}
	if found := s.scanSource("f.ts", []string{"// Reads the échelon from the entry."}, nil); !hasCheck(found, checkCoined) {
		t.Error("a coined word opening on a multi-byte rune was not reported")
	}
}

// This scan echoes file CONTENT, so it takes the guard dup-literals takes: a file whose NAME marks it
// as secret-bearing is declined unread, and the decline is announced so the report's denominator is
// honest. Without it a sentence out of a `.env` reaches the orchestrator's transcript and any body
// drafted from it.
func TestASecretNamedFileIsDeclinedUnreadAndSaidSo(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	r.write("deploy.env", "# Otherwise the fallback key is used and nobody notices.\nKEY=redacted\n")
	r.run("--voice")
	r.expectStdoutLacks("nobody notices")
	r.expectStdoutLacks("deploy.env:")
	r.expectStderrHas("secret")
}

// The same sentence in a file with an ordinary name is reported, so the case above measures the guard
// and not the check going quiet.
func TestTheSameSentenceInAnOrdinaryFileIsStillReported(t *testing.T) {
	r := newRepo(t)
	r.write("keep.go", "package fixture\n")
	r.commit("base")
	r.write("deploy.go", "// Otherwise the fallback key is used and nobody notices.\npackage fixture\n")
	r.run("--voice")
	r.expectCode(1)
	r.expectStdoutHas("deploy.go")
}

// The untracked arm gets its guard from diffscan's Options. The diff arm has its own, and a diff is
// where a branch somebody else wrote arrives — so it needs its own case or only half the guard is held.
func TestASecretNamedFileInADiffIsDeclinedUnread(t *testing.T) {
	diff := "diff --git a/deploy.env b/deploy.env\n--- a/deploy.env\n+++ b/deploy.env\n" +
		"@@ -0,0 +1 @@\n+# Otherwise the fallback key is used and nobody notices.\n"
	found, err := voiceScanner().scanDiff([]byte(diff))
	if err != nil {
		t.Fatalf("the scan refused the diff: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("a secret-named file in a diff was read and echoed: %s", render(found))
	}
	ordinary := strings.ReplaceAll(diff, "deploy.env", "deploy.go")
	found, err = voiceScanner().scanDiff([]byte(ordinary))
	if err != nil || len(found) == 0 {
		t.Fatalf("the same sentence in an ordinary file was not reported (%v): %s", err, render(found))
	}
}

// A hunk header's line number is caller-controlled on the `-` arm, and scanChange sizes a slice by it.
// Unbounded, one added line at `@@ +10000000` measured 166 MB resident.
func TestAnAbsurdLineNumberInAHunkHeaderIsDropped(t *testing.T) {
	diff := "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n" +
		"@@ -1,0 +2147483000,1 @@\n+// Otherwise the caller pays for it.\n"
	s := voiceScanner()
	found, err := s.scanDiff([]byte(diff))
	if err != nil {
		t.Fatalf("the scan refused the diff: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("a line past the cap was scanned: %s", render(found))
	}
	within := "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n" +
		"@@ -1,0 +2,1 @@\n+// Otherwise the caller pays for it.\n"
	found, err = s.scanDiff([]byte(within))
	if err != nil || len(found) == 0 {
		t.Fatalf("a line inside the cap was not scanned (%v): %s", err, render(found))
	}
}

// A conf that took effect says so. Silent, a conf a repository ships can allow every check and the run
// still reports `0 finding(s)` and "clean, which says the register was read".
func TestARunThatReadAConfNamesIt(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, voiceConfName)
	if err := os.WriteFile(conf, []byte("coined climb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMENT_VOICE_CONF", conf)
	var out, errs strings.Builder
	Run("comment-density.sh", []string{"--voice", "--profile=prose", conf}, dir,
		Config{MaxRatio: 0.3, MinLines: 5, MaxFileBytes: 1 << 18}, &out, &errs)
	if !strings.Contains(errs.String(), voiceConfName) || !strings.Contains(errs.String(), "1 coined word") {
		t.Fatalf("the run did not name the conf it read: %q", errs.String())
	}
}

// The conf path is one a repository can ship, so it is one a repository can ship as a symlink. A
// non-regular file is declined, and the refusal names the file rather than what it found inside it.
func TestANonRegularConfIsDeclinedWithoutEchoingIt(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(secret, []byte("NPM_TOKEN=abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, voiceConfName)
	if err := os.Symlink(secret, conf); err != nil {
		t.Skipf("this filesystem does not take symlinks: %v", err)
	}
	t.Setenv("COMMENT_VOICE_CONF", conf)
	_, _, _, err := voiceConfig(dir)
	if err == nil {
		t.Fatal("a symlinked conf was read")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("got %v, want the refusal that declines a non-regular file — a parse error here means it was read", err)
	}
	if strings.Contains(err.Error(), "NPM_TOKEN") || strings.Contains(err.Error(), "abcdef") {
		t.Fatalf("the refusal echoed the file it was aimed at: %v", err)
	}
}

// A conf that does not parse is refused by line, and the refusal carries no text off the line. A
// symlinked or mistaken conf otherwise prints its first token into the transcript.
func TestAParseRefusalNamesTheLineAndNotItsContents(t *testing.T) {
	_, _, err := parseVoiceConf("NPM_TOKEN=abcdef\n")
	if err == nil {
		t.Fatal("a line that is neither directive was accepted")
	}
	if strings.Contains(err.Error(), "NPM_TOKEN") || strings.Contains(err.Error(), "abcdef") {
		t.Fatalf("the refusal echoed the line: %v", err)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("the refusal does not name the line: %v", err)
	}
}

// A coined word is a codebase's invented vocabulary. A rule file is prose about writing and uses the
// ordinary English word a codebase may have coined, so the instruction profile does not run the check
// — otherwise a machine-level conf naming one project's terms reports every repository's rule files
// for using English, and the baseline stops being reproducible from one machine to another.
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
	s.profile = ProfileComment
	if found := s.scanSource("f.ts", []string{"// " + line}, nil); !hasCheck(found, checkCoined) {
		t.Error("the comment profile did not run the coined check, which is its whole subject")
	}
}

// In a comment the check reads past the backtick blanking, because a coined word inside backticks is
// an identifier built on the term. In a body it does not: there the backticks quote a literal.
func TestOnlyACommentReadsACoinedWordInsideBackticks(t *testing.T) {
	s := scanner{coined: []string{"sprocket"}}
	s.profile = ProfileComment
	if found := s.scanSource("f.ts", []string{"// Reads `readSprocket` from the entry."}, nil); !hasCheck(found, checkCoined) {
		t.Error("a comment did not report a coined word inside an identifier")
	}
	s.profile = ProfileProse
	if found := s.scanProse("b.md", []string{"The rename is `readSprocket`, landing next."}); hasCheck(found, checkCoined) {
		t.Error("a body reported a coined word inside a quoted identifier")
	}
}

// A bound that truncates is worse than no bound: the scan reports clean over the half it never saw,
// and git orders a diff by path, so padding an early file pushes a hostile one past the cut. The
// documented entry point pipes `gh pr diff` in, where a diff over any per-file cap is ordinary.
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

// A hunk header's line number is walked by a counter that can overflow, so it arrives negative. A
// negative number passes a ceiling test and then indexes a zero-length slice.
func TestALineNumberBelowOneIsRefusedLikeOneAboveTheCap(t *testing.T) {
	for _, at := range []int{-1, 0, maxDiffLine + 1} {
		added := newAddedLines()
		var said []string
		s := voiceScanner()
		s.notice = func(line string) { said = append(said, line) }
		var result diffscan.Result
		if !s.skip(added, diffscan.AddedLine{File: "f.go", Line: at, Text: "// Otherwise."}, &result) {
			t.Errorf("line %d was taken", at)
		}
		if at != 0 && len(said) == 0 {
			t.Errorf("line %d was dropped with nothing said, so the run closes on `clean` over what it discarded", at)
		}
	}
	added := newAddedLines()
	s := voiceScanner()
	var result diffscan.Result
	if s.skip(added, diffscan.AddedLine{File: "f.go", Line: 1, Text: "// Otherwise."}, &result) {
		t.Error("line 1 was refused, so the floor cuts real lines")
	}
}

// A file the scan declines says so once, however many of its lines the diff carried. A notice per line
// buries the report it belongs to.
func TestADeclinedFileIsAnnouncedOnceNotPerLine(t *testing.T) {
	added := newAddedLines()
	said := 0
	s := voiceScanner()
	s.notice = func(string) { said++ }
	var result diffscan.Result
	for at := 1; at <= 5; at++ {
		s.skip(added, diffscan.AddedLine{File: "deploy.env", Line: at, Text: "// Otherwise."}, &result)
	}
	if said != 1 {
		t.Fatalf("a five-line decline said %d thing(s); want one", said)
	}
}

// A coined word twice over, parted by one byte, is two findings. Matched rather than asserted
// boundaries consume the separator, so a scan resuming past the match swallows the second.
func TestTwoCoinedWordsPartedByOneByteAreTwoFindings(t *testing.T) {
	s := scanner{profile: ProfileComment, coined: []string{"rung"}}
	for _, line := range []string{"// rung rung", "// The rung. Rung again."} {
		found := s.scanSource("f.ts", []string{line}, nil)
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

// The contrast spine with the comma dropped and a conjunction in its place.
func TestTheContrastSpineIsCaughtWithAConjunction(t *testing.T) {
	s := scanner{profile: ProfileComment}
	for _, line := range []string{
		"// It is logged and not believed.",
		"// The string is thrown and not the object.",
		"// It is a survey and no verdict.",
	} {
		if !hasCheck(s.scanSource("f.ts", []string{line}, nil), checkContrast) {
			t.Errorf("%q produced no contrast finding", line)
		}
	}
	plain := "// The string is thrown and the object is kept."
	if hasCheck(s.scanSource("f.ts", []string{plain}, nil), checkContrast) {
		t.Errorf("%q produced a contrast finding, and it names two real things", plain)
	}
}

// A conf line that is not valid UTF-8 reaches a regular expression, and regexp refuses invalid UTF-8.
// Through MustCompile that is a panic printing the conf's own bytes and a stack trace of host paths —
// which undoes the refusal-without-echoing the rest of the conf handling was written for.
func TestAConfLineThatIsNotUTF8IsRefusedWithoutEchoingIt(t *testing.T) {
	_, _, err := parseVoiceConf("coined API\xffKEY\n")
	if err == nil {
		t.Fatal("a conf line holding invalid UTF-8 was accepted")
	}
	if strings.Contains(err.Error(), "API") || strings.Contains(err.Error(), "KEY") {
		t.Fatalf("the refusal echoed the line: %v", err)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("the refusal does not name the line: %v", err)
	}
}

// A conf present but unusable refuses rather than falling back. A dangling symlink at either searched
// path would otherwise leave the scan running with no coined words and no allowlist, reporting clean —
// and a default quietly restored cannot be told from the override working.
func TestAConfPresentButUnusableRefusesRatherThanFallingBack(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".kk-flavor"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, ".kk-flavor", voiceConfName)
	if err := os.Symlink(filepath.Join(dir, "absent"), conf); err != nil {
		t.Skipf("this filesystem does not take symlinks: %v", err)
	}
	t.Setenv("COMMENT_VOICE_CONF", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "machine"))
	if _, _, _, err := voiceConfig(dir); err == nil {
		t.Fatal("a dangling conf symlink was treated as no conf at all")
	}
}

// A finding the allowlist answers is still a finding the text carried. Uncounted, a conf a repository
// ships silences every check and the run still reports `0 finding(s)` and `clean`.
func TestASuppressedFindingIsCounted(t *testing.T) {
	_, allowed, err := parseVoiceConf("allow contrast rather than # the fixture's own phrase, quoted\n")
	if err != nil {
		t.Fatal(err)
	}
	suppressed := 0
	s := scanner{profile: ProfileComment, allowed: allowed, suppressed: &suppressed}
	if found := s.scanSource("f.ts", []string{"// Read the book rather than the entry."}, nil); len(found) != 0 {
		t.Fatalf("want the finding suppressed; got %s", render(found))
	}
	if suppressed != 1 {
		t.Fatalf("suppressed %d finding(s); want 1", suppressed)
	}
}

// Two prohibitions in one sentence are not the contrast spine. "Use no nesting and no preamble"
// forbids two things; "It is logged and not believed" defines one thing against another the reader
// did not ask about. Told apart by whether what stands before the conjunction is already a negation.
func TestTwoProhibitionsInOneSentenceAreNotTheSpine(t *testing.T) {
	s := scanner{profile: ProfileComment}
	spine := []string{
		"// It is logged and not believed.",
		"// The string is thrown and not the object.",
		"// It is a survey and no verdict.",
	}
	both := []string{
		"// Use no nesting and no preamble above the items.",
		"// Write no speculative abstraction and no flexibility the task did not ask for.",
		"// Use no headings, and no bold lead-in restating its own line.",
	}
	for _, line := range spine {
		if !hasCheck(s.scanSource("f.ts", []string{line}, nil), checkContrast) {
			t.Errorf("%q is the spine and produced no finding", line)
		}
	}
	for _, line := range both {
		if hasCheck(s.scanSource("f.ts", []string{line}, nil), checkContrast) {
			t.Errorf("%q forbids two things and was read as the spine", line)
		}
	}
}

// A shell script opens on an interpreter directive, which `#` makes look like a comment. Counted as
// one it joins the file header below it and spends a line of that header's allowance, so a script
// whose header sits exactly at the limit reports long for saying which interpreter runs it.
func TestAShebangIsNotPartOfTheFileHeader(t *testing.T) {
	header := []string{"#!/usr/bin/env bash"}
	for i := 1; i <= 8; i++ {
		header = append(header, "# Line "+strconv.Itoa(i)+".")
	}
	header = append(header, "set -euo pipefail")
	s := scanner{profile: ProfileComment}
	if hasCheck(s.scanSource("stub.sh", header, nil), checkLongBlock) {
		t.Error("an eight-line header under a shebang was reported long")
	}
	nine := append(append([]string{}, header[:9]...), "# Line 9.", "set -euo pipefail")
	if !hasCheck(s.scanSource("stub.sh", nine, nil), checkLongBlock) {
		t.Error("a nine-line header was not reported long, so the shebang rule cut too much")
	}
	// Only on line one. A `#!` further down is an ordinary comment.
	if !isShebang(1, "#!/bin/sh") || isShebang(2, "#!/bin/sh") {
		t.Error("the shebang rule does not hold to the first line")
	}
}

// The bar counts whole files, so it has to agree: a shebang is not a comment line there either.
func TestTheBarDoesNotCountAShebangAsAComment(t *testing.T) {
	withBang := statsOf("#!/usr/bin/env bash\n# One.\ncode\n")
	plain := statsOf("# One.\ncode\n")
	if withBang.comments != plain.comments {
		t.Fatalf("the shebang added %d comment line(s) to the count", withBang.comments-plain.comments)
	}
}

// The note pattern spends one connective on `so`, so a conforming note must not be a finding and the
// check has to start at two. A threshold off by one here would report every note the rule asks for.
func TestAConformingNoteIsNotAClauseDepthFinding(t *testing.T) {
	s := scanner{profile: ProfileComment}
	conforming := "// Some platforms reject a detached call, so the method is called on its object."
	if hasCheck(s.scanSource("f.ts", []string{conforming}, nil), checkClauseDepth) {
		t.Errorf("%q is the note pattern the rule asks for and it produced a clause-depth finding", conforming)
	}
	deep := "// The call is kept because the platform rejects it, which the older fleet does while it upgrades."
	if !hasCheck(s.scanSource("f.ts", []string{deep}, nil), checkClauseDepth) {
		t.Errorf("%q holds four connectives and produced no clause-depth finding", deep)
	}
}

// One negation is how a fact is stated. Two is what the reader has to resolve against each other.
func TestOneNegationIsNotADoubleNegativeFinding(t *testing.T) {
	s := scanner{profile: ProfileComment}
	single := "// The field is not set on an older export."
	if hasCheck(s.scanSource("f.ts", []string{single}, nil), checkDoubleNeg) {
		t.Errorf("%q carries one negation and produced a double-negative finding", single)
	}
	double := "// The code is not absent and it is not unknown."
	if !hasCheck(s.scanSource("f.ts", []string{double}, nil), checkDoubleNeg) {
		t.Errorf("%q carries two negations and produced no finding", double)
	}
}

// `a; b` is a list. A semicolon with a clause after it is two sentences written as one, and the tail
// length is what tells them apart.
func TestASemicolonListIsNotASemicolonFinding(t *testing.T) {
	s := scanner{profile: ProfileComment}
	list := "// Reads three fields: owner; entry; book."
	if hasCheck(s.scanSource("f.ts", []string{list}, nil), checkSemicolon) {
		t.Errorf("%q is a list and produced a semicolon finding", list)
	}
	join := "// The platform rejects the call; the method is called on its object instead of the reference."
	if !hasCheck(s.scanSource("f.ts", []string{join}, nil), checkSemicolon) {
		t.Errorf("%q joins two clauses and produced no semicolon finding", join)
	}
}

// A comment wraps, and the phrase then spans two lines. The check reads the joined block, so a wrapped
// phrase is the same phrase.
func TestACoinedPhraseIsMatchedAcrossAWrappedLine(t *testing.T) {
	s := scanner{profile: ProfileComment}
	wrapped := []string{"// A pairing that cannot occur has no", "// name in the catalogue."}
	if !hasCheck(s.scanSource("f.ts", wrapped, nil), checkCoined) {
		t.Errorf("a built-in coined phrase broken across two lines produced no finding")
	}
	plain := []string{"// A pairing that cannot occur is not listed in the catalogue."}
	if hasCheck(s.scanSource("f.ts", plain, nil), checkCoined) {
		t.Errorf("the plain form produced a coined finding")
	}
}
