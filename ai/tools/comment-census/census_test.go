package commentcensus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func linesOf(text string) []string { return strings.Split(text, "\n") }

func TestASummaryIsTheFirstSentenceOverADeclaration(t *testing.T) {
	blocks := Blocks(linesOf("// Returns the rows. One row is a posting.\nexport function rows() {}\n"))
	if len(blocks) != 1 {
		t.Fatalf("%d block(s), want 1", len(blocks))
	}
	summary, ok := blocks[0].Summary()
	if !ok || summary != "Returns the rows" {
		t.Errorf("summary %q, ok %v", summary, ok)
	}
	if got := blocks[0].Notes(); len(got) != 1 || got[0] != "One row is a posting" {
		t.Errorf("notes %q", got)
	}
}

// Every sentence of a block standing over plain code is a note. A rule aimed at notes would
// otherwise skip a file header's first sentence and reach the rest of it.
func TestABlockOverNoDeclarationIsAllNotes(t *testing.T) {
	blocks := Blocks(linesOf("// A header. A second sentence.\n  return 1;\n"))
	if len(blocks) != 1 {
		t.Fatalf("%d block(s), want 1", len(blocks))
	}
	if _, ok := blocks[0].Summary(); ok {
		t.Errorf("a block standing over a return statement was given a summary")
	}
	if got := blocks[0].Notes(); len(got) != 2 {
		t.Errorf("notes %q, want both sentences", got)
	}
}

func TestEachShapeReachesItsOwnSentenceAndLeavesTheOthers(t *testing.T) {
	cases := []struct {
		shape    string
		reaches  string
		declines string
	}{
		{"note-connective", "The ledger refuses the entry, so the balance is Unknown",
			"A closing period carries no postings"},
		{"so-clause", "The list is sorted, so the search is binary", "The list is sorted"},
		{"counterfactual-consequence", "The name is short, so a longer one would ask about the prior period",
			"The name is short, so parseName reads two fields"},
		{"anthropomorphism", "An account can say no to the posting", "An account refuses the posting"},
		{"elided-verb", "A row goes as soon as the ledger does", "A row goes when the ledger removes it"},
		{"negated-case", "Returns the row, or undefined unless exactly one is priced",
			"Returns the row, or undefined when none is priced"},
	}
	byName := map[string]Shape{}
	for _, s := range Shapes() {
		byName[s.Name] = s
	}
	for _, c := range cases {
		s, ok := byName[c.shape]
		if !ok {
			t.Fatalf("no shape named %q", c.shape)
		}
		if s.Hits(c.reaches) == "" {
			t.Errorf("%s did not reach %q", c.shape, c.reaches)
		}
		if hit := s.Hits(c.declines); hit != "" {
			t.Errorf("%s reached %q on %q, and that sentence is the control", c.shape, hit, c.declines)
		}
	}
}

func TestAMetaphorVerbIsFoundInEveryInflection(t *testing.T) {
	for _, sentence := range []string{"A true answer settles the period", "The claim settled the period",
		"Settling the period", "The field hides the answer"} {
		if MetaphorHit(sentence) == "" {
			t.Errorf("no metaphor verb found in %q", sentence)
		}
	}
	if hit := MetaphorHit("The parser reads two fields"); hit != "" {
		t.Errorf("the control sentence matched %q", hit)
	}
}

// A compound the code spells is the domain's word. Only one the code lacks is the comment's own.
func TestACompoundTheCodeSpellsIsNotCoined(t *testing.T) {
	identifiers := map[string]bool{"byterange": true}
	if got := CoinedCompounds("The byte-range request is partial", identifiers); len(got) != 0 {
		t.Errorf("byte-range read as coined: %q", got)
	}
	if got := CoinedCompounds("A period-blind yes still settles it", identifiers); len(got) != 1 || got[0] != "period-blind" {
		t.Errorf("period-blind not found: %q", got)
	}
}

// plainSetEnv names the directory of reviewed source the census counts. It is an environment
// variable and never a committed path: the set is somebody else's code, and this repository is
// public. Only counts and the sentences a shape reached leave the run.
const plainSetEnv = "JUDGE_EVAL_PLAIN"

var plainSetExtensions = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".go": true}

func loadPlainSet(t *testing.T) [][]string {
	t.Helper()
	dir := os.Getenv(plainSetEnv)
	if dir == "" {
		t.Skipf("%s names no directory, so no rule proposed here has been counted", plainSetEnv)
	}
	var paths []string
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && plainSetExtensions[strings.ToLower(filepath.Ext(path))] {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("%s names a directory the census could not read: %v", plainSetEnv, err)
	}
	if len(paths) == 0 {
		t.Fatalf("%s names a directory holding no source file the census reads", plainSetEnv)
	}
	sort.Strings(paths)
	var files [][]string
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("a file in the set could not be read: %v", err)
		}
		files = append(files, strings.Split(string(body), "\n"))
	}
	return files
}

// The census itself, which asserts no bar. A shape's count is the evidence a rule is cut against. A
// bar invented here would teach the next rule to clear it.
func TestCensusOverThePlainSet(t *testing.T) {
	files := loadPlainSet(t)
	rep := Measure(files)
	var out strings.Builder
	fmt.Fprintf(&out, "\nfiles %d, comment blocks %d, summaries over a declaration %d, note sentences %d\n\n",
		rep.Files, rep.Blocks, rep.Summaries, rep.Notes)
	fmt.Fprintf(&out, "%-30s %6s  %s\n", "shape", "count", "of")
	for _, s := range rep.Shapes {
		fmt.Fprintf(&out, "%-30s %6d  %d\n", s.Name, s.Count, denominatorOf(s.Name, rep))
	}
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.SoNamed.Name, rep.SoNamed.Count, rep.SoNamed.Count+rep.SoPronoun.Count+rep.SoUnnamed.Count)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.SoPronoun.Name, rep.SoPronoun.Count, rep.SoNamed.Count+rep.SoPronoun.Count+rep.SoUnnamed.Count)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.SoUnnamed.Name, rep.SoUnnamed.Count, rep.SoNamed.Count+rep.SoPronoun.Count+rep.SoUnnamed.Count)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.SoBoth.Name, rep.SoBoth.Count, rep.SoUnnamed.Count)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.Restating.Name, rep.Restating.Count, rep.Summaries)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.ShownBy.Name, rep.ShownBy.Count, rep.Notes)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.AboutCode.Name, rep.AboutCode.Count, rep.Notes)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.BareFact.Name, rep.BareFact.Count, rep.Blocks)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.Unanchored.Name, rep.Unanchored.Count, rep.Blocks)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.BareIdent.Name, rep.BareIdent.Count, rep.Notes)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", rep.Spelled.Name, rep.Spelled.Count,
		rep.Spelled.Count+rep.Coined.Count)
	fmt.Fprintf(&out, "\n%-30s %6d  %d\n", "coined-compound", rep.Coined.Count, rep.Blocks)
	fmt.Fprintf(&out, "%-30s %6d  %d\n", "note over 2 sentences", rep.Long.Count, rep.Blocks)
	fmt.Fprintf(&out, "\n%-30s %6s\n", "metaphor verb", "count")
	for _, v := range rep.Verbs {
		fmt.Fprintf(&out, "%-30s %6d\n", v.Name, v.Count)
	}
	for _, s := range rep.Shapes {
		if len(s.Samples) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", s.Name, len(s.Samples), s.Count)
		for _, sample := range s.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	for _, v := range rep.Verbs {
		if len(v.Samples) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", v.Name, len(v.Samples), v.Count)
		for _, sample := range v.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	for _, tally := range []Tally{rep.BareFact, rep.Unanchored, rep.BareIdent} {
		if len(tally.Samples) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", tally.Name, len(tally.Samples), tally.Count)
		for _, sample := range tally.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	if len(rep.AboutCode.Samples) > 0 {
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", rep.AboutCode.Name,
			len(rep.AboutCode.Samples), rep.AboutCode.Count)
		for _, sample := range rep.AboutCode.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	if len(rep.ShownBy.Samples) > 0 {
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", rep.ShownBy.Name,
			len(rep.ShownBy.Samples), rep.ShownBy.Count)
		for _, sample := range rep.ShownBy.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	if len(rep.Spelled.Samples) > 0 {
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", rep.Spelled.Name,
			len(rep.Spelled.Samples), rep.Spelled.Count)
		for _, sample := range rep.Spelled.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	if len(rep.Restating.Samples) > 0 {
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", rep.Restating.Name,
			len(rep.Restating.Samples), rep.Restating.Count)
		for _, sample := range rep.Restating.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	for _, t := range []Tally{rep.SoUnnamed, rep.SoPronoun, rep.SoNamed} {
		if len(t.Samples) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n%s — %d sample(s) of %d:\n", t.Name, len(t.Samples), t.Count)
		for _, sample := range t.Samples {
			fmt.Fprintf(&out, "  so %s\n", sample)
		}
	}
	if len(rep.Coined.Samples) > 0 {
		fmt.Fprintf(&out, "\ncoined-compound — %d sample(s) of %d:\n", len(rep.Coined.Samples), rep.Coined.Count)
		for _, sample := range rep.Coined.Samples {
			fmt.Fprintf(&out, "  %s\n", sample)
		}
	}
	t.Log(out.String())
}

// A shape is counted against the sentences it was offered. One offered both summaries and notes has
// both as its denominator, and reporting it against the notes alone overstates it.
func denominatorOf(name string, rep Report) int {
	for _, shape := range Shapes() {
		if shape.Name != name {
			continue
		}
		switch shape.Over {
		case "summary":
			return rep.Summaries
		case "note":
			return rep.Notes
		}
	}
	return rep.Summaries + rep.Notes
}

// The consequence clause the pattern keeps names this code's own element. A clause whose subject the
// file never spells is what Kirill read twice as a confusing second half.
func TestASoClauseIsSortedByWhetherItNamesTheCode(t *testing.T) {
	identifiers := map[string]bool{"parsename": true, "toelements": true}
	named, unnamed := "The name is short, so parseName reads two fields",
		"The name is short, so a longer one would ask about the prior period"
	if _, namesCode, found := SoClauseSubject(named, identifiers); !found || !namesCode {
		t.Errorf("the clause naming parseName was not read as naming the code")
	}
	if _, namesCode, found := SoClauseSubject(unnamed, identifiers); !found || namesCode {
		t.Errorf("the clause about a hypothetical name was read as naming the code")
	}
	if _, _, found := SoClauseSubject("The name is short", identifiers); found {
		t.Errorf("a sentence carrying no so clause was read as carrying one")
	}
}

// A clause naming its element inside backticks names it. The plain set writes most of its element
// names that way.
func TestASoClauseNamesTheCodeInsideBackticks(t *testing.T) {
	identifiers := map[string]bool{"toelements": true}
	_, namesCode, found := SoClauseSubject("The collection is live, so `toElements` copies it", identifiers)
	if !found || !namesCode {
		t.Errorf("a backticked element name was not read as naming the code")
	}
}

// The strike behind restates-code: the declaration already spells every content word of the summary,
// so the strike takes all of them.
func TestASummaryRestatingTheDeclarationLeavesNoWord(t *testing.T) {
	lines := linesOf("// x\nexport function listPostingCells(): PostingCell[] {\n  return cells;\n}\n")
	blocks := Blocks(lines)
	words := DeclarationWords(lines, blocks[0])
	survived, hadContent := Restates("Lists every posting cell", words)
	if !hadContent || len(survived) != 0 {
		t.Errorf("survived %q, and every word of that summary is in the declaration", survived)
	}
	survived, _ = Restates("Cells are ordered by settlement to match the catalog", words)
	if len(survived) == 0 {
		t.Errorf("a summary carrying an outside fact was struck whole")
	}
}

// The count restates-code reads moves with bodyWindow, which is a number this file chose. A check
// whose finding count depends on such a number reports, and it never deletes.
func TestTheStrikeReadsTheDeclarationsOwnWords(t *testing.T) {
	lines := linesOf("// x\nexport function isPriced(book: Element): boolean {\n" +
		"  return hasCurrency(book.getAttribute('currency'));\n}\n")
	words := DeclarationWords(lines, Blocks(lines)[0])
	for _, word := range []string{"pric", "book", "currency", "element"} {
		if !words[word] {
			t.Errorf("the declaration spells %q and the strike does not hold it", word)
		}
	}
}

// A catalogue constant's description says what the data is. The strike reads it as a restatement,
// because a constant's name carries the same words, and deleting it would take the description of
// an asset with it. This is why the check reports.
func TestACatalogueDescriptionReadsAsARestatement(t *testing.T) {
	lines := linesOf("// x\nexport const LEDGER_WITH_ACCRUAL_ENTRIES = 'ledger-accrual';\n")
	words := DeclarationWords(lines, Blocks(lines)[0])
	survived, hadContent := Restates("Ledger with accrual entries", words)
	if !hadContent || len(survived) != 0 {
		t.Skipf("the strike left %q, so this spelling is not the false-positive shape", survived)
	}
}
