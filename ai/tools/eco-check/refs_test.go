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

// A cited path is resolved with a test that follows symlinks, so what it names is whatever the
// reviewed tree pointed it at. `evil.md -> /dev/zero`, or a committed FIFO, made the read of it never
// return — the shell version hangs on the same fixture, both killed at 12s.
func TestCitationTargetMustBeARegularFile(t *testing.T) {
	// /dev/null stands in for /dev/zero: the same class of target, and the one that cannot hang this
	// suite on the day someone takes the guard back out.
	newDeviceTarget := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.symlink("/dev/null", f.root+"/kk-flavor/standards/evil.md")
		f.write(f.root+"/kk-flavor/standards/citer.md",
			"see [evil.md](evil.md) → **X** for the rule\n")
		return f
	}

	t.Run("fires on a cited path that resolves to a device", func(t *testing.T) {
		newDeviceTarget(t).reports(notRegular)
	})

	// The half that matters: a target nothing read must never be indistinguishable from a checked one.
	t.Run("and says it was not read rather than reading part of it", func(t *testing.T) {
		newDeviceTarget(t).reports("it was NOT read")
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
