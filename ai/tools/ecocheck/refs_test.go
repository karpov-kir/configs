package ecocheck_test

// Citations name their section in the delimited form.

import (
	"testing"
)

func TestDelimitedSectionCitations(t *testing.T) {
	t.Run("fires on a citation whose section is not delimited", func(t *testing.T) {
		newCitation(t, "One home").reports(undelimited)
	})

	t.Run("accepts the bolded form", func(t *testing.T) {
		newCitation(t, "**One home**").doesNotReport(undelimited)
	})

	t.Run("accepts the backticked form, which the parser also reads exactly", func(t *testing.T) {
		newCitation(t, "`One home`").doesNotReport(undelimited)
	})

	// A shell comment cites the same way a document does, and the scan reads both.
	t.Run("fires on an undelimited citation inside a shell comment", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/target.md", "# Target\n\n## One home\n")
		f.newScript("citer.sh", "#!/usr/bin/env bash\n# untested: fixture\n# the rule is target.md → One home\ntrue")
		f.reports(undelimited)
	})
}

// A standard citing `target.md → <section>`, where the section arrives in the form under test.
func newCitation(t *testing.T, section string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/standards/target.md", "# Target\n\n## One home\n")
	f.write(f.root+"/kk-flavor/standards/citer.md",
		"see [target.md](target.md) → "+section+" for the rule\n")
	return f
}
