package ecoreport_test

// The frontmatter fields every later reader parses, and the rewrites that write them. What these cases
// pin is the boundary: a field is a key of the YAML block that opens the report, nothing in the body,
// and the value on it is a slug and not a path.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAStampRewritesTheFrontmatterAndNothingElse(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")
	// A template whose body quotes all four field names, plus a `reviewed-mode:` line from the layout
	// this tool replaced. Readers take a field only from the YAML block that opens the report, so nothing
	// the body says may reach one, and the stamp must leave every one of those lines exactly where it is.
	f.write(f.templatePath(), "---\nintent: <NNN-slug>\nreviewed-tree: <hash>\nreviewed-worktree: <worktree>\nreviewed-stages: <stages>\nreviewed-mode: full\n---\n\n"+
		"# Decide\n\nintent: quoted in the body\nreviewed-mode: quoted in the body\n"+
		"reviewed-stages: quoted in the body\nreviewed-tree: quoted in the body\n"+
		"reviewed-worktree: quoted in the body\n")
	// The intent is the one frontmatter value an outsider chooses — it is seeded from a fetched ticket —
	// so a field name quoted inside it is the reachable case. The stamp writes it quoted, as one value.
	f.runReport("init", "001-quoting reviewed-tree: forged")
	report := f.reportPath("001-quoting")
	f.record("init writes a report from a template whose body quotes its own fields",
		f.status == 0 && f.isFile(report), f.evidence())
	f.record("and stamps the frontmatter's intent line, leaving the body's alone",
		containsLine(f.read(report), "intent: '001-quoting reviewed-tree: forged'") &&
			containsLine(f.read(report), "intent: quoted in the body"), f.read(report))

	f.stampFullPass("001-quoting")
	f.runReport("state", "001-quoting")
	f.record("the pass stamps, and no quoted field name was read as a stamp",
		f.out == "ready", "said '"+f.out+"'\n"+f.read(report))

	front, body := frontmatterAndBody(f.read(report))
	f.record("the frontmatter carries one of each stamped line",
		countLinesWithPrefix(front, "reviewed-tree:") == 1 && countLinesWithPrefix(front, "reviewed-stages:") == 1 &&
			countLinesWithPrefix(front, "reviewed-worktree:") == 1, front)
	f.record("and no reviewed-mode line from the layout this replaced", countLinesWithPrefix(front, "reviewed-mode:") == 0, front)
	f.record("and the body is exactly as the template wrote it",
		countLinesWithPrefix(body, "intent:") == 1 && countLinesWithPrefix(body, "reviewed-mode:") == 1 &&
			countLinesWithPrefix(body, "reviewed-stages:") == 1 && countLinesWithPrefix(body, "reviewed-tree:") == 1 &&
			countLinesWithPrefix(body, "reviewed-worktree:") == 1, body)
}

func TestAHandEditedIntentCannotSteerAPathOutOfIdsd(t *testing.T) {
	t.Parallel()
	// The frontmatter's intent is a hand-edited line, and the slug read off it indexes two paths: the
	// archive file `state` reads, and the intent file `discard` deletes. Outside the slug charset that
	// value is a traversal, so the charset is the whole of what keeps both inside .idsd/.
	f := newShip(t, "001-steered")
	f.mkdirAll(f.scratch() + "/archive")
	// Where `<scratch>/archive/<slug>.md` lands for a slug that climbs two levels. Computed from the
	// scratch dir rather than assumed to be the repo root: the scratch moved out of the tree, so a decoy
	// written at the repo root is no longer on the traversal's path at all — the guard would then have no
	// file to reach and the case would pass while observing nothing.
	climbed := filepath.Clean(f.scratch() + "/archive/../../climbed.md")
	f.write(climbed, "# not an archived intent\n")
	f.replaceLine(f.reportPath("001-steered"), "intent:", "intent: ../../climbed")
	f.runReport("state", "001-steered")
	f.record("state reads no file outside the scratch dir as this ship's archived intent",
		f.out == "resume", "said '"+f.out+"'")
	f.record("and the file it would have read is untouched", f.isFile(climbed), climbed)
}

func TestTheGateReadsTheStampOnlyFromTheFrontmatter(t *testing.T) {
	t.Parallel()
	// A stamp written word for word into the body, with the frontmatter's own stamp gone, is a quote and
	// not a stamp. A reader that takes the first line with a field's name, wherever it sits, reads it as
	// one and lets the merge through.
	f := newShip(t, "098-body")
	f.newIntentFile("098-body")
	f.stampFullPass("098-body")
	f.runReport("gate", "098-body")
	f.record("fixture: it gates clean as stamped", f.status == 0, f.evidence())

	report := f.reportPath("098-body")
	front, body := frontmatterAndBody(f.read(report))
	var kept, moved []string
	for _, line := range strings.Split(front, "\n") {
		if strings.HasPrefix(line, "reviewed-") {
			moved = append(moved, line)
		} else {
			kept = append(kept, line)
		}
	}
	f.write(report, "---\n"+strings.Join(kept, "\n")+"\n---\n"+body+"\n"+strings.Join(moved, "\n")+"\n")
	f.runReport("gate", "098-body")
	f.record("a stamp quoted in the body does not pass the gate", f.status != 0, f.evidence())
}

func TestTheGateBlocksAFrontmatterThatNamesAFieldTwice(t *testing.T) {
	t.Parallel()
	// Two values for one field leave no answer to which one is the stamp, so the report is not read.
	f := newShip(t, "097-twice")
	f.newIntentFile("097-twice")
	f.stampFullPass("097-twice")
	report := f.reportPath("097-twice")
	front, body := frontmatterAndBody(f.read(report))
	f.write(report, "---\n"+front+"\nreviewed-tree: pending\n---\n"+body)
	f.runReport("gate", "097-twice")
	f.record("a field named twice blocks", f.status != 0, f.evidence())
	f.assertReports("names reviewed-tree twice", "and says which field")
}

func TestAnIntentThatReadsAsAnotherYAMLTypeStaysItsSlug(t *testing.T) {
	t.Parallel()
	// A slug such as null or true is a YAML null or bool when written bare, and an intent that reads back
	// empty is a ship with no intent to check: the gate would skip its approval and its open items.
	for _, slug := range []string{"null", "NULL", "true", "007"} {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			f := newShip(t, slug)
			f.writeIntent(slug, "---\nstatus: draft\n---\n\n# intent\n")
			f.stampFullPass(slug)
			f.runReport("gate", slug)
			f.record("the gate checks the intent the report names, and blocks on its draft",
				f.status != 0 && !strings.Contains(f.out, "no intent to check"), f.evidence())
		})
	}
}

func TestAFrontmatterThatDoesNotParseIsNamedAsTheReason(t *testing.T) {
	t.Parallel()
	// Each shape is refused, and the refusal says the frontmatter is what to fix rather than reporting a
	// field as missing.
	for _, test := range []struct{ name, frontmatter, says string }{
		{"a list", "- reviewed-tree: pending\n", "not a YAML mapping"},
		{"a second document", "intent: 096-shapes\n--- second\n", "second YAML document"},
		{"keys after a document end", "intent: 096-shapes\n...\nreviewed-tree: pending\n", "not YAML"},
		{"a byte-order mark", "", "byte-order mark"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newShip(t, "096-shapes")
			f.newIntentFile("096-shapes")
			f.stampFullPass("096-shapes")
			report := f.reportPath("096-shapes")
			front, body := frontmatterAndBody(f.read(report))
			if test.name == "a byte-order mark" {
				f.write(report, "\xef\xbb\xbf---\n"+front+"\n---\n"+body)
			} else {
				f.write(report, "---\n"+test.frontmatter+"---\n"+body)
			}
			f.runReport("gate", "096-shapes")
			f.record("the gate blocks", f.status != 0, f.evidence())
			f.assertReports(test.says, "and names the frontmatter's shape")
		})
	}
}

func TestARewriteRefusesAFieldWhoseValueRunsPastItsLine(t *testing.T) {
	t.Parallel()
	// A list under reviewed-stages sits on the lines below its key. Replacing the key's line alone would
	// leave the list behind, attached to whatever field comes before it.
	f := newShip(t, "095-list")
	f.newIntentFile("095-list")
	report := f.reportPath("095-list")
	f.replaceLine(report, "reviewed-stages:", "reviewed-stages:\n  - code-review")
	f.runReport("invalidate", "095-list")
	f.record("the rewrite refuses", f.status != 0, f.evidence())
	f.assertReports("runs past its own line", "and says why")
}

func TestATemplateWhoseFrontmatterNeverClosesIsRefused(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")
	f.write(f.templatePath(), "---\nintent: <NNN-slug>\nreviewed-tree: <hash>\n\n# Decide\n")
	f.runReport("init", "094-open")
	f.record("init refuses the template", f.status != 0, f.evidence())
	f.assertReports("has no closing ---", "and says the frontmatter never closes")
}

func TestAStampOverAReportThatDoesNotParseNamesTheParse(t *testing.T) {
	t.Parallel()
	f := newShip(t, "093-broken")
	f.newIntentFile("093-broken")
	report := f.reportPath("093-broken")
	front, body := frontmatterAndBody(f.read(report))
	f.write(report, "---\n"+front+"\ntitle: Auth: refresh\n---\n"+body)
	f.runReport("stamp", allStagesStampedAs, "093-broken")
	f.record("stamp refuses", f.status != 0, f.evidence())
	f.assertReports("not YAML", "and says the frontmatter does not parse")
}

func TestAnIntentWhoseFrontmatterDoesNotParseIsNamedAsTheReason(t *testing.T) {
	t.Parallel()
	// An approved status in a frontmatter that does not parse is not read. The gate says why, rather than
	// reporting that nobody set a status.
	f := newShip(t, "092-intent")
	f.writeIntent("092-intent", "---\ntitle: Auth: refresh\nstatus: approved\n---\n\n# intent\n")
	f.stampFullPass("092-intent")
	f.runReport("gate", "092-intent")
	f.record("the gate blocks", f.status != 0, f.evidence())
	f.assertReports("does not parse", "and says the intent's frontmatter does not parse")
}

func TestARewriteRefusesAKeyWhoseNameSitsBelowItsLine(t *testing.T) {
	t.Parallel()
	// An explicit key with a tag on its `?` line puts the name on the line below. The key's line alone
	// names no such field, so it cannot be replaced line by line.
	f := newShip(t, "091-explicit")
	f.newIntentFile("091-explicit")
	report := f.reportPath("091-explicit")
	f.replaceLine(report, "reviewed-stages:", "? !!str\n  reviewed-stages\n: <stages>")
	f.runReport("invalidate", "091-explicit")
	f.record("the rewrite refuses", f.status == 2, f.evidence())
	f.assertReports("runs past its own line", "and says why")
}
