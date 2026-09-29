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
// The book took a posting format long before the probe reported it,
// so this function returns ` + "`Accepted`" + ` for either answer.
export function getFormatClaim(formatName: string): LedgerClaim {
  return LedgerClaim.Accepted;
}`,
		},
		{
			// The act's subject is the domain thing. The check that a block said this function put the
			// phrase in 50 of 72 blocks by 2026-09-29, and it went.
			name: "a tie whose subject is the domain thing needs no this function",
			text: `fact: The book took a posting format long before the probe reported the same format.
bears_on: getFormatClaim
does: returns LedgerClaim.Accepted where either answers it
---
// The book took a posting format long before the probe reported it,
// so either answer is ` + "`Accepted`" + `.
export function getFormatClaim(formatName: string): LedgerClaim {
  return LedgerClaim.Accepted;
}`,
		},
		{
			name: "a value declaration opens on its verb",
			text: `fact: Settlement profile 5 posts in the accrual scheme.
bears_on: AccrualProfile5
does: none
---
  // Names settlement profile 5, whose postings are accrual.
  AccrualProfile5 = 'accrual-5',`,
		},
		{
			name: "a value declaration opening on this member is refused",
			text: `fact: Settlement profile 5 posts in the accrual scheme.
bears_on: AccrualProfile5
does: none
---
  // This member names settlement profile 5, whose postings are accrual.
  AccrualProfile5 = 'accrual-5',`,
			want: []string{checkValueThisOpens},
		},
		{
			name: "a value is never the actor of keeps",
			text: `fact: The oldest ledgers refuse a posting type that carries a currency.
bears_on: POSTING_TYPE
does: none
---
// The oldest ledgers refuse a posting type that carries a currency. So this constant keeps the type and drops the currency.
const POSTING_TYPE = 'entry';`,
			want: []string{checkValueActor},
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
// ` + "`LedgerBook.SETTLED`" + ` is missing on some ledger builds here, so this constant spells the value out.
const SETTLED = 2;`,
		},
		{
			name: "does: none on a declaration with a body is refused",
			text: `fact: The book took a posting format long before the probe reported the same format.
bears_on: getFormatClaim
does: none
---
// The book took a format long before the probe did, so this function reads both.
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
// The probe hedges on an uncommitted ledger, so this function keeps the reader honest.
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
			name: "code piped above the block is context, and the block still names its declaration",
			text: `fact: how a ledger treats a book left with no posting is unknown
bears_on: keepsBook
does: returns false for a posting book where no posting declares the currency
---
  return {
    // How a ledger treats a book left with no posting is unknown.
    // So this member returns false for a posting book where no posting declares ` + "`currency`" + `.
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

func TestDoesSharesOneSegment(t *testing.T) {
	body := recordSegments("if (probe.WebKitPostingKeys && requirement.scheme === PostingScheme.Deferred) {")
	if !namesSomethingIn("takes the Deferred path", body) {
		t.Fatal("a tie naming the branch by its own word reads as untied")
	}
	if namesSomethingIn("keeps the reader honest", body) {
		t.Fatal("a tie naming nothing in the branch reads as tied")
	}
}

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

func TestADataDeclarationOfAnyLengthTakesDoesNone(t *testing.T) {
	record := "fact: The vendor names these in its own export.\nbears_on: %s\ndoes: none\n---\n// The vendor names these in its own export, and the spelling here is the vendor's.%.0s\n"
	for _, tc := range []struct{ kind, name, code string }{
		{"enum", "SettlementScheme", "export enum SettlementScheme {\n  Accrual = 'accrual',\n}"},
		{"interface", "PostingRow", "export interface PostingRow {\n  format: string;\n}"},
		{"type", "PostingShape", "export type PostingShape = {\n  format: string;\n};"},
		{"object constant", "schemeIdentifiers", "export const schemeIdentifiers: Record<Scheme, string[]> = {\n  accrual: ['a'],\n};"},
		{"array constant", "LEDGER_SOURCES", "export const LEDGER_SOURCES: LedgerSource[] = [\n  { id: 'accrual-eu' },\n];"},
		{"computed key", "EscrowDeferred", "  [Clearing.EscrowDeferred]: undefined,"},
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

func TestABlockNamingItsOwnDeclarationIsRefused(t *testing.T) {
	record := "fact: The book took a posting format long before the probe reported the same format.\n" +
		"bears_on: getFormatClaim\ndoes: returns LedgerClaim.Accepted where either answers it\n---\n"
	code := "\nexport function getFormatClaim(formatName: string): LedgerClaim {\n  return LedgerClaim.Accepted;\n}"
	named := record + "// The book took a format long before the probe did, so `getFormatClaim` returns `Accepted`." + code
	if got := checksOf(RecordFindings("-", shell.SplitLines(named))); len(got) != 1 || got[0] != checkRecordSelfNamed {
		t.Fatalf("a block naming its own declaration reports %v", got)
	}
	// A branch declares no name, so its record names the identifier the branch reads, and the block does too.
	branch := "fact: a ledger answers 0 for a request that never reached it\nbears_on: status\ndoes: keeps a status from 100 up\n---\n" +
		"// A ledger answers 0 for a request that never reached it, and this branch keeps a `status` from 100 up.\n" +
		"if (status >= 100) {\n  return status;\n}"
	if got := checksOf(RecordFindings("-", shell.SplitLines(branch))); len(got) != 0 {
		t.Fatalf("a branch naming the value it reads reports %v", got)
	}
}

// A file stem inside a backticked path is the audit's path class. A run's writers could cite no file
// of another repository, because the check read a humped stem there as a bare name.
func TestAStemInsideABacktickedPathIsNoBareName(t *testing.T) {
	s := voiceScanner()
	for _, note := range []string{
		"// The pool in `packages/server/src/connections/connectionPool.ts` closes idle links, and this row must match it.",
		"// The pool in `acme/ledger/packages/server/src/connections/connectionPool.ts` closes idle links, and this row must match it.",
		"// The pool in `connectionPool.ts` closes idle links, and this row must match it.",
	} {
		if hasCheck(s.scanSource("f.ts", []string{note, "export const LINKS = 3;"}, nil, nil), checkBareIdent) {
			t.Errorf("%q reports its path's stem as a bare name", note)
		}
	}
	if !hasCheck(s.scanSource("f.ts", []string{"// The pool in connectionPool closes idle links, and this row must match it.", "export const LINKS = 3;"}, nil, nil), checkBareIdent) {
		t.Error("a bare stem outside a path passes")
	}
	// A call holds a dot and names no file, and neither does a member access. Prose after a path is read.
	for _, note := range []string{
		"// The pool in `apiResponse.json()` closes idle links, and this row must match it.",
		"// The pool in `pool.ts` holds connectionPool open, and this row must match it.",
	} {
		if !hasCheck(s.scanSource("f.ts", []string{note, "export const LINKS = 3;"}, nil, nil), checkBareIdent) {
			t.Errorf("%q reports no bare name", note)
		}
	}
	// A member access holds a dot and names a field, and its humped name is still a bare name.
	if !hasCheck(s.scanSource("f.ts", []string{"// The pool in `retryPolicy.value` closes idle links, and this row must match it.", "export const LINKS = 3;"}, nil, nil), checkBareIdent) {
		t.Error("a humped name inside a member access passes as a path")
	}
}
