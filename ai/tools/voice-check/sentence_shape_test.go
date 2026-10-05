package voicecheck

import (
	"strings"
	"testing"
)

// A note on an enum or a literal union that names two of its own members is a finding. A note naming one
// member, notes on each member, and a note on a function naming two words that are no members are none.
func TestANoteTellingMembersApartBelongsOnEachMember(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		fires        bool
	}{
		{"enum note naming both", "// `Refused` is a refusal, and `Unknown` is no answer.\nexport enum Claim {\n  Refused = 'refused',\n  Unknown = 'unknown',\n}\n", true},
		{"union note naming both", "// 'accrual' books at once, and 'deferred' books at period end.\nexport type Format = 'accrual' | 'deferred';\n", true},
		{"enum note naming one", "// `Unknown` sends the posting to a settlement run.\nexport enum Claim {\n  Refused = 'refused',\n  Unknown = 'unknown',\n}\n", false},
		{"per-member notes", "export enum Claim {\n  // A clearing house refused.\n  Refused = 'refused',\n  // No house answered.\n  Unknown = 'unknown',\n}\n", false},
		{"function naming member words", "// Refused and Unknown postings are both kept.\nexport function keep(p: Posting): boolean {\n  return true;\n}\n", false},
	} {
		lines := strings.Split(strings.TrimSuffix(tc.source, "\n"), "\n")
		found := membersInOneNote("x.ts", commentBlocksIn(lines, onlyAdded(nil))[0], lines, lines)
		if (len(found) > 0) != tc.fires {
			t.Errorf("%s: fired %v, want %v", tc.name, len(found) > 0, tc.fires)
		}
	}
}
