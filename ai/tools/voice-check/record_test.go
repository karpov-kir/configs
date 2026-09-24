package voicecheck

import (
	"fmt"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

// checksOf is the check names a run reported, in report order. A case then says which checks fired.
// That says more than a count.
func checksOf(found []Finding) []string {
	var out []string
	for _, f := range found {
		out = append(out, f.Check)
	}
	return out
}

func TestRecordFindings(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want []string
	}{
		{
			name: "a fact and a tie clears every check",
			text: `fact: The book took a posting format long before the probe reported the same format.
bears_on: getFormatClaim
does: returns LedgerClaim.Accepted where either answers it
---
// The book took a posting format long before the probe reported it.
// ` + "`getFormatClaim`" + ` returns ` + "`Accepted`" + ` where either answers ` + "`Accepted`" + `.
export function getFormatClaim(formatName: string): LedgerClaim {
  return LedgerClaim.Accepted;
}`,
		},
		{
			// The shape a reviewer read on 2026-09-22 and answered "And what?". Its prose is sound, so the
			// register checks pass it, and this check reports it.
			name: "a fact that stops leaves the block naming nothing",
			text: `fact: The book took a posting format long before the probe reported the same format.
bears_on: getFormatClaim
does: returns LedgerClaim.Accepted where either answers it
---
// The book took a posting format long before the probe reported the same format.
export function getFormatClaim(formatName: string): LedgerClaim {
  return LedgerClaim.Accepted;
}`,
			want: []string{checkRecordUnnamed},
		},
		{
			// A fact whose bearing is what a caller does with the result belongs at the caller. The writer
			// cannot reach that file, so the identifier-set test is the guard here.
			name: "bears_on naming a caller fails at this site",
			text: `fact: A ledger ignoring the posting scheme stalls on its first posting under that scheme.
bears_on: warmUpPosting
does: gates on this claim instead of the combination
---
// A ledger ignoring the scheme stalls on its first posting. warmUpPosting, the caller, gates on this claim.
export async function claimsSchemeThroughStandardApi(scheme: PostingScheme): Promise<boolean> {
  return probe.requestAccess(scheme).then(() => true, () => false);
}`,
			want: []string{checkRecordElsewhere, checkRecordUntied},
		},
		{
			name: "a one-line declaration takes does: none",
			text: `fact: A ledger build here is missing LedgerBook.SETTLED, so the value is spelled out.
bears_on: SETTLED
does: none
---
// ` + "`LedgerBook.SETTLED`" + ` is missing on some ledger builds here, so ` + "`SETTLED`" + ` is spelled out.
const SETTLED = 2;`,
		},
		{
			name: "does: none on a declaration with a body is refused",
			text: `fact: The book took a posting format long before the probe reported the same format.
bears_on: getFormatClaim
does: none
---
// The book took a format long before the probe did, and ` + "`getFormatClaim`" + ` reads both.
export function getFormatClaim(formatName: string): LedgerClaim {
  return LedgerClaim.Accepted;
}`,
			want: []string{checkRecordSlot},
		},
		{
			name: "a tie the body spells nothing of",
			text: `fact: The probe hedges where the ledger will not commit.
bears_on: getProbeFormatClaim
does: keeps the reader honest
---
// The probe hedges where the ledger will not commit. ` + "`getProbeFormatClaim`" + ` keeps the reader honest.
export function getProbeFormatClaim(formatName: string): LedgerClaim {
  return LedgerClaim.Unknown;
}`,
			want: []string{checkRecordUntied},
		},
		{
			name: "a missing slot is named",
			text: `fact: The probe hedges where the ledger will not commit.
---
// The probe hedges where the ledger will not commit.
const HEDGED = 'maybe';`,
			want: []string{checkRecordSlot, checkRecordSlot},
		},
		{
			// Run 10's shape is a block on an object member, and the writer piped the member's enclosing
			// `return {` first. The check read an empty block and refused the name the block spelled.
			name: "code piped above the block is context, and the block still says bears_on",
			text: `fact: how a ledger treats a book left with no posting is unknown
bears_on: keepsBook
does: returns false for a posting book where no posting declares the currency
---
  return {
    // How a ledger treats a book left with no posting is unknown.
    // ` + "`keepsBook`" + ` returns false for a posting book where no posting declares ` + "`currency`" + `.
    keepsBook: book =>
      !isPostingBook(book) || hasPostingDeclaring(book, currency),`,
		},
		{
			name: "a block piped without a record says so",
			text: `// The probe hedges where the ledger will not commit.
const HEDGED = 'maybe';`,
			want: []string{checkRecordSlot},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checksOf(RecordFindings("-", shell.SplitLines(tc.text)))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("checks %v, want %v", got, tc.want)
			}
		})
	}
}

// A tie names the branch by one of its identifiers and writes ordinary English around it. Every word
// shared would refuse that, so the test is one segment.
func TestDoesSharesOneSegment(t *testing.T) {
	body := recordSegments("if (probe.WebKitPostingKeys && requirement.scheme === PostingScheme.Deferred) {")
	if !namesSomethingIn("takes the Deferred path", body) {
		t.Fatal("a tie naming the branch by its own word reads as untied")
	}
	if namesSomethingIn("keeps the reader honest", body) {
		t.Fatal("a tie naming nothing in the branch reads as tied")
	}
}

// `MediaSource` meets mediaSourceClaim, an identifier holding it, as a segment. It never meets a word
// holding it as letters.
func TestSegmentsAreNotSubstrings(t *testing.T) {
	set := recordSegments("const mediaSourceClaim = 1; const sourced = 2;")
	if !set["source"] || !set["media"] {
		t.Fatal("the humps of an identifier are not in its segment set")
	}
	if !spellsTheName("mediaSourceClaim", "const mediaSourceClaim = 1;") {
		t.Fatal("an identifier does not meet itself")
	}
	if spellsTheName("getFormatClaim", "// the format this row carries") {
		t.Fatal("a shared hump reads as the whole name")
	}
}

// A data declaration takes `does: none` whatever its length, because the value beneath is the tie.
// Run 9's writers filled `does` on an enum, an interface and an object constant to get past a check.
// The check read the opening brace as a body, and the writers invented the tie the rule removes.
func TestADataDeclarationOfAnyLengthTakesDoesNone(t *testing.T) {
	record := "fact: The vendor names these in its own export.\nbears_on: %s\ndoes: none\n---\n// The vendor names these in its own export, and `%s` keeps its spelling.\n"
	for _, tc := range []struct{ kind, name, code string }{
		{"enum", "SettlementScheme", "export enum SettlementScheme {\n  Accrual = 'accrual',\n}"},
		{"interface", "PostingRow", "export interface PostingRow {\n  format: string;\n}"},
		{"type", "PostingShape", "export type PostingShape = {\n  format: string;\n};"},
		{"object constant", "schemeIdentifiers", "export const schemeIdentifiers: Record<Scheme, string[]> = {\n  accrual: ['a'],\n};"},
		{"array constant", "LEDGER_SOURCES", "export const LEDGER_SOURCES: LedgerSource[] = [\n  { id: 'accrual-eu' },\n];"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			text := fmt.Sprintf(record, tc.name, tc.name) + tc.code
			if got := checksOf(RecordFindings("-", shell.SplitLines(text))); len(got) != 0 {
				t.Fatalf("a %s with does: none reports %v", tc.kind, got)
			}
		})
	}
	// A function still owes its tie.
	fn := fmt.Sprintf(record, "readRate", "readRate") + "export function readRate(book: LedgerBook): number {\n  return book.rate;\n}"
	if got := checksOf(RecordFindings("-", shell.SplitLines(fn))); len(got) != 1 || got[0] != checkRecordSlot {
		t.Fatalf("a function with does: none reports %v, want record-slot-missing", got)
	}
}
