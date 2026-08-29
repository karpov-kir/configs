package ecocheck_test

import (
	"testing"
)

// A rule cited by its number resolves in no file; the heading that number opens does.
func TestBareRuleIDCitations(t *testing.T) {
	t.Run("fires on a rule cited by its number", func(t *testing.T) {
		newNumberedCitationOverItsHeading(t).reports(bareRule)
	})

	// The half a reader acts on: the finding has to carry the citation they should have written.
	t.Run("and names the heading that number opens", func(t *testing.T) {
		newNumberedCitationOverItsHeading(t).reports(principlesName + " → **" + principlesHeading + "**")
	})

	// The same citation in a tree holding no heading of that number. It is still dangling, so the
	// finding names the form rather than going quiet.
	t.Run("names the form when no heading of that number resolves", func(t *testing.T) {
		newNumberedCitationWithNoHeading(t).reports("<the numbered heading>")
	})

	// The form the finding above recommends, in a tree where it resolves — the scan must not report
	// the thing it asks for.
	t.Run("stays quiet on the citation form that resolves", func(t *testing.T) {
		f := newRoot(t)
		f.writeNumberedPrinciples()
		f.write(f.root+"/kk-flavor/standards/citer.md",
			"the rule is ["+principlesName+"]("+principlesName+") → **"+principlesHeading+"**\n")
		f.doesNotReport(bareRule)
	})
}

// The fixture the cases above share. Each piece is a constant and the resolving form is built from the
// same two the fixture is, for the reason laneScriptRef is a constant: the shell suite states the same
// fixture, and a case here and the case of the same name there have to be the same case.
const (
	rulePhrase        = "Core Principle"
	principlesName    = "core-principles.md"
	principlesHeading = "3. Surgical changes"
)

// The citation on its own, in a tree holding no heading of that number: still dangling, so the
// finding names the form rather than going quiet.
func newNumberedCitationWithNoHeading(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/standards/citer.md", "follow "+rulePhrase+" 3 when you touch a file\n")
	return f
}

// The same citation over the heading that number opens, which is what lets the finding quote back the
// form to write instead.
func newNumberedCitationOverItsHeading(t *testing.T) *fixture {
	t.Helper()
	f := newNumberedCitationWithNoHeading(t)
	f.writeNumberedPrinciples()
	return f
}

func (f *fixture) writeNumberedPrinciples() {
	f.t.Helper()
	f.write(f.root+"/kk-flavor/standards/"+principlesName, "## "+principlesHeading+"\n")
}
