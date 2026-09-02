package ecoreport_test

// The three frontmatter lines every later reader greps, and the rewrites that write them. What these
// cases pin is the boundary: a field is a line that starts with its name, inside the frontmatter, and
// the value on it is a slug and not a path.

import (
	"path/filepath"
	"testing"
)

func TestAStampRewritesTheFrontmatterAndNothingElse(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("check-ignore")
	// A template whose body quotes all four field names, plus a `reviewed-mode:` line from the layout
	// this tool replaced. Every reader greps a line by its prefix, so nothing the body says may reach
	// one, and the stamp must leave every one of those lines exactly where it is.
	f.write(f.templatePath(), "---\nintent: <NNN-slug>\nreviewed-tree: <hash>\nreviewed-worktree: <worktree>\nreviewed-stages: <stages>\nreviewed-mode: full\n---\n\n"+
		"# Decide\n\nintent: quoted in the body\nreviewed-mode: quoted in the body\n"+
		"reviewed-stages: quoted in the body\nreviewed-tree: quoted in the body\n"+
		"reviewed-worktree: quoted in the body\n")
	// The intent is the one frontmatter value an outsider chooses — it is seeded from a fetched ticket
	// — so a field name quoted inside it is the reachable case, and it sits above the real field.
	f.runReport("init", "001-quoting reviewed-tree: forged")
	report := f.reportPath("001-quoting")
	f.record("init writes a report from a template whose body quotes its own fields",
		f.status == 0 && f.isFile(report), f.evidence())
	f.record("and stamps the frontmatter's intent line, leaving the body's alone",
		containsLine(f.read(report), "intent: 001-quoting reviewed-tree: forged") &&
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
