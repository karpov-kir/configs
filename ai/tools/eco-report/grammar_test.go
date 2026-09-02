package ecoreport_test

// The stage record's grammar. `stamp` writes that record and the merge gate reads it, so what the
// grammar accepts is what a pass may claim to have done — which is why the vocabulary is closed
// rather than pattern-matched. Both directions are a row here: a form wrongly accepted records a
// pass that did not happen, and a legal one wrongly refused leaves a real pass unable to say what it
// did.

import (
	"strings"
	"testing"
)

// The record a pass that ran everything writes, and the base every row below varies from.
const fullRecord = "code-review,security-review,tighten,refactor"

func TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-grammar")
	f.armFullPass("001-grammar")
	report := f.reportPath("001-grammar")

	// Run bare, the usage text is the grammar's authority — the package header says so, and it is what
	// the agent reads to build a record at all. Refused as a malformed record instead, the human is
	// told their entry is wrong and never shown what a right one looks like.
	f.runReport("stamp")
	f.assertRefused("a bare stamp is refused")
	f.assertReports("usage: report.sh stamp", "and prints the grammar")
	f.record("rather than the refusal a malformed record gets",
		!strings.Contains(f.out, "invalid stage record"), f.out)

	// Each refusal leaves the pass armed, because a refused stamp writes nothing.
	for _, bad := range []struct{ record, named string }{
		{"bogus", "malformed entry: bogus"},
		// An extra entry outside the vocabulary, shaped so that nothing downstream catches it: the
		// four required stages are all present, and `stamp`'s per-stage marker check skips anything
		// marked skipped. The grammar is the only thing between this and a junk entry in the record.
		{fullRecord + ",bogus:skipped(not-applicable)", "malformed entry: bogus:skipped(not-applicable)"},
		// The retired turnaround token. It was the vocabulary before the modes were removed, so it is
		// the one wrong word a stamp is most likely to carry — and `turnaroundTrims` reads `(turnaround)`,
		// so a record still saying `fast` would gate a trimmed pass as untrimmed.
		{"code-review,security-review:skipped(fast),tighten,refactor", "malformed entry: security-review:skipped(fast)"},
		{"code-review,security-review,tighten,refactor:partial(fast)", "malformed entry: refactor:partial(fast)"},
		{"code-review,code-review,security-review,tighten,refactor", "duplicate stage: code-review"},
		{"code-review,security-review,tighten", "missing stage: refactor"},
	} {
		f.runReport("stamp", bad.record, "001-grammar")
		f.assertRefused("stamp refuses '" + bad.record + "'")
		f.assertReports("invalid stage record", "and calls it an invalid stage record, not a stage that did not run")
		f.assertReports(bad.named, "and names it: "+bad.named)
		f.record("and stamped nothing for '"+bad.record+"'",
			containsLine(f.read(report), "reviewed-tree: pending"), f.read(report))
	}

	// Whitespace goes before the record is read, so a record pasted across two lines is one record.
	// Left in, every entry after the first is malformed and a legitimate stamp is refused.
	f.runReport("stamp", "code-review, security-review,\n  tighten, refactor", "001-grammar")
	f.record("a record pasted across lines stamps as one record",
		f.status == 0, f.evidence())
	f.record("and no whitespace reaches the record the gate reads",
		containsLine(f.read(report), "reviewed-stages: "+fullRecord), f.read(report))

	// Every legal form, each one a record a real pass produces. `refactor:partial(…)` records that the
	// loop ended non-compliant, which is what ran rather than a trim; `skipped(turnaround)` is a turnaround
	// trim and `skipped(not-applicable)` an unmet condition, and only the two optional stages take
	// either.
	for _, legal := range []string{
		"code-review,security-review,tighten,refactor:partial(turnaround)",
		"code-review,security-review,tighten,refactor:partial(cap)",
		"code-review,security-review:skipped(not-applicable),tighten,refactor",
		"code-review,security-review,tighten:skipped(not-applicable),refactor",
		"code-review,security-review:skipped(turnaround),tighten,refactor",
		"code-review,security-review,tighten:skipped(turnaround),refactor",
	} {
		f.armFullPass("001-grammar")
		f.runReport("stamp", legal, "001-grammar")
		f.record("stamp accepts '"+legal+"'", f.status == 0, f.evidence())
		f.record("and records '"+legal+"' as the gate will read it",
			containsLine(f.read(report), "reviewed-stages: "+legal), f.read(report))
	}

	// A report with no reviewed-tree line at all. That line is the stamp's whole product, so a stamp
	// written into a report without one puts the record nowhere any reader looks — and the refusal has
	// to name the missing line, because `invalidate`, which the next refusal down would name, cannot
	// put it back.
	missing := newShip(t, "001-no-tree-line")
	missing.armFullPass("001-no-tree-line")
	missing.dropLines(missing.reportPath("001-no-tree-line"), "reviewed-tree:")
	missing.runReport("stamp", fullRecord, "001-no-tree-line")
	missing.assertRefused("stamp refuses a report with no reviewed-tree line")
	missing.assertReports("no 'reviewed-tree:' line", "and names the missing line")
	missing.record("and not the invalidate that cannot put it back",
		!strings.Contains(missing.out, "never invalidated"), missing.out)
}
