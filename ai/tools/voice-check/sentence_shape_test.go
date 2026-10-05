package voicecheck

import (
	"strings"
	"testing"
)

// A note on an enum or a literal union that names two of its own members in code spans is a finding. A
// note naming one member, notes on each member, a note stating the members' order, a function's note and
// an alias of no literals are none.
func TestANoteTellingMembersApartBelongsOnEachMember(t *testing.T) {
	enum := "export enum Claim {\n  Refused = 'refused',\n  Unknown = 'unknown',\n}\n"
	for _, tc := range []struct {
		name, source string
		fires        bool
	}{
		{"enum note naming both", "// `Refused` is a refusal, and `Unknown` is no answer.\n" + enum, true},
		{"one-line enum", "// `Refused` is a refusal, and `Unknown` is no answer.\nexport enum Claim { Refused = 'r', Unknown = 'u' }\n", true},
		{"union note naming both", "// `accrual` books at once, and `deferred` books at period end.\nexport type Format = 'accrual' | 'deferred';\n", true},
		{"generic union", "// `accrual` books at once, and `deferred` at period end.\nexport type Format<T> = 'accrual' | 'deferred';\n", true},
		{"continued union", "// `accrual` books at once, and `deferred` at period end.\nexport type Format =\n  | 'accrual'\n  | 'deferred';\n", true},
		{"enum note naming one", "// `Unknown` sends the posting to a settlement run.\n" + enum, false},
		{"members named without code spans", "// None of the postings is kept, and All of them are read.\nexport enum Pick {\n  None,\n  All,\n}\n", false},
		{"an ordering", "// Names the claim, ordered from `Refused` to `Unknown`, which the audit sorts by.\n" + enum, false},
		{"per-member notes", "export enum Claim {\n  // A clearing house refused.\n  Refused = 'refused',\n  // No house answered.\n  Unknown = 'unknown',\n}\n", false},
		{"function naming member words", "// `Refused` and `Unknown` postings are both kept.\nexport function keep(p: Posting): boolean {\n  return true;\n}\n", false},
		{"object alias with literals", "// `x` and `y` name the two axes.\nexport type Shape = { a: 'x'; b: 'y' };\n", false},
	} {
		lines := strings.Split(strings.TrimSuffix(tc.source, "\n"), "\n")
		found := membersInOneNote("x.ts", commentBlocksIn(lines, onlyAdded(nil))[0], lines, lines)
		if (len(found) > 0) != tc.fires {
			t.Errorf("%s: fired %v, want %v", tc.name, len(found) > 0, tc.fires)
		}
	}
}

// The shape checks fire on the reviewed shapes and leave the rule's own phrasings, nouns ending in -ing
// and ordinary possessives alone.
func TestSentenceShapeChecksFireOnTheReviewedShapesOnly(t *testing.T) {
	scan := func(sentence string) map[string]bool {
		found := map[string]bool{}
		s := scanner{profile: ProfileComment}
		s.sentenceShape(sentence, 0, func(check string, _, _ int) { found[check] = true })
		return found
	}
	for _, tc := range []struct {
		sentence, check string
		fires           bool
	}{
		{"Only an API's refusal is Unsupported.", checkQuantifierOpen, true},
		{"Anything no API established is Unknown.", checkQuantifierOpen, true},
		{"Every caller removes postings from the ledger while it walks the result.", checkQuantifierOpen, false},
		{"None when the house never answered.", checkQuantifierOpen, false},
		{"Only an API's refusal is Unsupported.", checkNominalisation, true},
		{"The house's approval comes first.", checkNominalisation, true},
		{"The ledger's settlement closes the day.", checkNominalisation, false},
		{"The house's reference names the account.", checkNominalisation, false},
		{"This is the one call that's validation enough.", checkNominalisation, false},
		{"The tests record that claim as the result without playing.", checkDanglingVerb, true},
		{"When loading, the house drops the cache.", checkDanglingVerb, true},
		{"The house drops the cache without warning.", checkDanglingVerb, false},
		{"This check rests on nothing.", checkDanglingVerb, false},
		{"Tells whether a posting settles, by expecting a refusal.", checkDanglingVerb, false},
	} {
		if got := scan(tc.sentence)[tc.check]; got != tc.fires {
			t.Errorf("%s on %q: fired %v, want %v", tc.check, tc.sentence, got, tc.fires)
		}
	}
}
