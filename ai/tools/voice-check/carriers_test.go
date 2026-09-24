package voicecheck

import (
	"slices"
	"strings"
	"testing"
)

// addedFrom builds the added lines of one file, numbered from 1.
func addedFrom(file string, lines ...string) *addedLines {
	a := newAddedLines()
	for i, line := range lines {
		a.take(file, i+1, line)
	}
	return a
}

// treeOf answers a grep over a tree holding these lines.
func treeOf(lines ...string) treeReader {
	return func(names []string) (map[string][]string, error) {
		held := map[string][]string{}
		for _, line := range lines {
			for _, name := range names {
				if strings.Contains(line, name) {
					held[name] = append(held[name], line)
				}
			}
		}
		return held, nil
	}
}

func carrierChecks(t *testing.T, blocks []string, added *addedLines, tree treeReader) []string {
	t.Helper()
	found, err := CarrierFindings(blocks, added, tree)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range found {
		out = append(out, f.Check+" "+f.Text)
	}
	return out
}

// Run 10's shape: the archived block moved into a string constant named as a sentence, on a
// catalogue field no code reads.
func TestAStringCarrierOnAFieldNobodyReadsIsThreeFindings(t *testing.T) {
	block := "// The accrual export has no base ledger to fall back on, so a book without the closing\n" +
		"// profile cannot post it."
	catalogue := []string{
		"  postingReason?: string;",
		"const ACCRUAL_EXPORT_HAS_NO_BASE_LEDGER_TO_FALL_BACK_ON =",
		"  'The accrual export has no base ledger to fall back on.';",
		"    postingReason: ACCRUAL_EXPORT_HAS_NO_BASE_LEDGER_TO_FALL_BACK_ON,",
	}
	got := carrierChecks(t, []string{block}, addedFrom("tests/ledger/Catalogue.ts", catalogue...), treeOf(catalogue...))
	for _, want := range []string{
		checkCarrierUnread + " postingReason",
		checkCarrierSentence + " ACCRUAL_EXPORT_HAS_NO_BASE_LEDGER_TO_FALL_BACK_ON",
		checkCarrierQuotes + " the accrual export has no base",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no %q among %v", want, got)
		}
	}
}

// The carriers the contract names pass: a field some code reads, a short constant name, a lint
// message a human reads when the rule fires, and a test named for the fact.
func TestARealCarrierPasses(t *testing.T) {
	block := "// A posting is retried three times, and the poster gives up on it after the third."
	added := addedFrom("src/postings/Retry.ts",
		"export const POSTING_RETRIES = 3;",
		"  closingProfile?: string;",
		"      message: 'A posting is retried three times, and the poster gives up on it after the third.',",
	)
	added.take("src/postings/Retry.test.ts", 1,
		"it('gives up on a posting after it is retried three times, and the poster gives up', () => {")
	tree := treeOf(
		"export const POSTING_RETRIES = 3;",
		"  for (let attempt = 0; attempt < POSTING_RETRIES; attempt++) {",
		"  closingProfile?: string;",
		"  if (book.closingProfile === undefined) {",
	)
	if got := carrierChecks(t, []string{block}, added, tree); len(got) != 0 {
		t.Fatalf("findings %v over carriers the contract names", got)
	}
}

// A constant whose only line is its own declaration has no reader.
func TestAConstantNobodyReadsIsUnread(t *testing.T) {
	added := addedFrom("src/Ledger.ts", "export const CLOSING_NOTE = 'closing';")
	got := carrierChecks(t, []string{"// unrelated block of six or more words here"}, added,
		treeOf("export const CLOSING_NOTE = 'closing';"))
	if !slices.Equal(got, []string{checkCarrierUnread + " CLOSING_NOTE"}) {
		t.Fatalf("got %v", got)
	}
}

// An object literal's entry passes a value to whatever reads it, often a library the tree never
// spells, so it is no field of the tree's own.
func TestAnObjectEntryIsNoUnreadField(t *testing.T) {
	added := addedFrom("jest.config.js", "  testTimeout: 5000,")
	if got := carrierChecks(t, []string{"// a block"}, added, treeOf("  testTimeout: 5000,")); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestANameCountsItsWords(t *testing.T) {
	for name, want := range map[string]int{
		"ACCRUAL_EXPORT_HAS_NO_BASE_LEDGER": 6,
		"UNIQUE_POSTING_WORKER_URL_KEY":     5,
		"postingsRunOnlyInNode":             5,
		"LEVEL_2_1_IS_LOW":                  3,
		"keepsBook":                         2,
	} {
		if got := nameWords(name); got != want {
			t.Errorf("%s: %d words, want %d", name, got, want)
		}
	}
}
