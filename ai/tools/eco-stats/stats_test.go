package ecostats_test

// The cases of `~/.claude/skills/kk-reduce/scripts/stats-test.sh`, one for one and under the same
// names, driven in-process. That suite is still the cross-check: a case here and the case of the same
// name there have to be the same case, so a change to one is a change to both.
//
// The agreement cases at the top hold the invariant that matters: for one tree, both tools report the
// same router figure. They run ecocheck rather than shelling out to check.sh, for the same reason
// everything else here is in-process.

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestAgreementWithCheck(t *testing.T) {
	t.Run("agree on a plain budget", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two three four five\n")
		f.assertScriptsAgree()
	})

	t.Run("agree when a budget file has no final newline", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two three")
		f.assertScriptsAgree()
	})

	t.Run("agree with a Read-always target in the budget", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n")
		f.write(f.root+"/kk-flavor/standards/core.md", "alpha beta gamma\n")
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.assertScriptsAgree()
	})

	t.Run("agree when an import resolves into the budget", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
		f.newHome()
		f.write(f.home+"/.claude/FOO.md", "one two three\n")
		f.assertScriptsAgree()
	})
}

func TestAShortFigureNeverReachesTheLedger(t *testing.T) {
	t.Run("exits 2 and says why, on an unreadable budget file", func(t *testing.T) {
		f := newUnreadableBudgetFile(t)
		stdout, stderr, status := f.run(f.root)
		if status != 2 || !strings.Contains(stdout+stderr, "budget file refused") {
			t.Errorf("status: %d\n%s", status, indent(stdout+stderr))
		}
	})

	// This can't be an agreement case: nothing prints a figure at all on an exit 2, so the comparison
	// would put ecocheck's number against an empty string and go red whatever ecocheck did — saying
	// nothing about the refusal asserted here.
	t.Run("and check.sh refuses it too, rather than counting a file it cannot read", func(t *testing.T) {
		f := newUnreadableBudgetFile(t)
		output := f.checkOutput()
		if !strings.Contains(output, "budget file refused") {
			t.Errorf("%s", indent(output))
		}
	})
}

func TestTheLedgerIsMeasuredApartFromTheInstructions(t *testing.T) {
	t.Run("a ledger leaves prose unchanged and reports its own words", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two three\n")
		without := f.figure("prose")
		f.newLedger("alpha beta gamma delta\n")
		with := f.figure("prose")
		reported := f.figure("ledger")
		if without == "" || with != without || reported != "4" {
			t.Errorf("prose without it: %q\nprose with it: %q\nledger reported: %q (want 4)",
				without, with, reported)
		}
	})

	t.Run("a symlink at the ledger path takes nothing out of prose", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two three\n")
		without := f.figure("prose")
		f.installStats()
		f.write(f.base+"/outside.md", "outside words\n")
		f.symlink(f.base+"/outside.md", f.root+"/skills/kk-reduce/stats.md")
		with := f.figure("prose")
		reported := f.figure("ledger")
		if without == "" || with != without || reported != "0" {
			t.Errorf("prose without it: %q\nprose with it: %q\nledger reported: %q (want 0)",
				without, with, reported)
		}
	})
}

func TestAProbeShapedImportIsReportedNotHidden(t *testing.T) {
	// The file sits where the traversal lands, so a resolver with the guard gone would find it. This
	// case asserts on the reported refusal alone, so it would pass with the file absent too;
	// ecocheck's twin of this fixture is the one that also asserts the name stayed uncounted.
	t.Run("reports a path-shaped import name instead of leaving it to read as drift", func(t *testing.T) {
		f := newRoot(t)
		f.newHome()
		f.write(f.root+"/CLAUDE.md", "# Root\n\n@../../escape.md\n")
		f.write(f.root+"/escape.md", "secret words here\n")
		stdout, stderr, _ := f.run(f.root)
		if !strings.Contains(stdout+stderr, "import refused") {
			t.Errorf("%s", indent(stdout+stderr))
		}
	})
}

func TestASkillMountedFromOutsideTheTreeIsReportedApart(t *testing.T) {
	// The fixture skill lives beside the root, not under it: a mount resolving inside the root is
	// excluded.
	t.Run("counts a mounted-outside description apart from the tree's own", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.newHome()
		f.mkdirAll(f.base + "/outside-skill")
		f.mkdirAll(f.home + "/.claude/skills")
		f.write(f.base+"/outside-skill/SKILL.md", skillWithDescription("outside-skill"))
		f.symlink(f.base+"/outside-skill", f.home+"/.claude/skills/outside-skill")
		// A second mount, this one resolving *inside* the root: the tree's own skill must not count.
		// The suite stays green if you delete it, which is exactly why it looks removable — take it
		// out and nothing here tests the exclusion.
		f.mkdirAll(f.root + "/skills/inside-skill")
		f.write(f.root+"/skills/inside-skill/SKILL.md", skillWithDescription("inside-skill"))
		f.symlink(f.root+"/skills/inside-skill", f.home+"/.claude/skills/inside-skill")
		if reported := f.figure("mounted outside"); reported != "4" {
			t.Errorf("reported: %q (want 4)", reported)
		}
	})

	t.Run("says nothing about mounted skills when this tree is not the installed one", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.newHomeWithoutFlavorMount()
		f.mkdirAll(f.base + "/outside-skill-2")
		f.mkdirAll(f.home + "/.claude/skills")
		f.write(f.base+"/outside-skill-2/SKILL.md", skillWithDescription("outside-skill-2"))
		f.symlink(f.base+"/outside-skill-2", f.home+"/.claude/skills/outside-skill-2")
		if reported := f.figure("mounted outside"); reported != "" {
			t.Errorf("reported: %q (want no line at all)", reported)
		}
	})
}

func TestAnOverLongNoteIsRefusedRatherThanAppended(t *testing.T) {
	t.Run("a 60-word note appends nothing, a short one appends one row", func(t *testing.T) {
		f := newRoot(t)
		ledger := f.newLedger("| date |\n")
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.run("--append", wordsCount(60), f.root)
		afterLong := rowsIn(t, ledger)
		f.run("--append", "short enough to keep", f.root)
		afterShort := rowsIn(t, ledger)
		if afterLong != 1 || afterShort != 2 {
			t.Errorf("rows after the long note: %d (want 1)\nrows after the short one: %d (want 2)",
				afterLong, afterShort)
		}
	})
}

func TestTheNoteCannotForgeALedgerRow(t *testing.T) {
	t.Run("a note carrying a newline and a pipe still writes exactly one 6-column row", func(t *testing.T) {
		f := newRoot(t)
		ledger := f.newLedger("| date | prose | scripts | always-loaded | skills | what ran |\n|---|---|---|---|---|---|\n")
		f.write(f.root+"/CLAUDE.md", "one two\n")
		before := rowsIn(t, ledger)
		f.run("--append", "first line\nsecond line | with a pipe", f.root)
		appended := rowsIn(t, ledger) - before
		lines := strings.Split(strings.TrimSuffix(readFile(t, ledger), "\n"), "\n")
		row := lines[len(lines)-1]
		// Escaped pipes come out before the count: `\|` separates no column, so counting raw `|` would
		// read the guard working as the guard failing.
		columns := strings.Count(strings.ReplaceAll(row, `\|`, ""), "|")
		if appended != 1 || columns != 7 {
			t.Errorf("rows appended: %d (want 1)\nunescaped pipes: %d (want 7)\n%s", appended, columns, row)
		}
	})
}

func TestAMissingLedgerIsOpenedWithAHeaderAReaderCanUse(t *testing.T) {
	// The only case that leaves stats.md out of the fixture, so it is the only one that reaches the
	// header written when the ledger does not exist. Every other --append case creates the file first
	// and never sees that block. The `+` legend is asserted because it is the one thing in the header
	// a reader cannot reconstruct from the columns, and the header is where a fresh ledger states it.
	t.Run("creates the ledger with the column header, the + legend, and one row under them", func(t *testing.T) {
		f := newRoot(t)
		f.installStats()
		f.write(f.root+"/CLAUDE.md", "one two\n")
		fresh := f.root + "/skills/kk-reduce/stats.md"
		f.run("--append", "opening row", f.root)
		content := readFile(t, fresh)
		rows := rowsIn(t, fresh)
		if rows != 3 ||
			!strings.Contains(content, "| date | prose | scripts | always-loaded | skills | what ran |") ||
			!strings.Contains(content, "lower bound") {
			t.Errorf("lines starting '|': %d (want 3 — header, rule, row)\n%s", rows, indent(content))
		}
	})
}

func TestTheLedgerIsNotWrittenThroughASymlink(t *testing.T) {
	t.Run("refuses to append through a symlinked ledger", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.installStats()
		f.write(f.base+"/decoy-target.md", "untouched\n")
		f.symlink(f.base+"/decoy-target.md", f.root+"/skills/kk-reduce/stats.md")
		f.run("--append", "should not land", f.root)
		if decoy := readFile(t, f.base+"/decoy-target.md"); decoy != "untouched\n" {
			t.Errorf("the decoy was written: %s", decoy)
		}
	})

	// The seed and the live ledger are a .md/source pair, which the shared-region scan cannot cover —
	// it reads `*.sh` only. They had already drifted: the seed carried none of the three rules the
	// real file owns, so a fresh install started with no protection for the column. Nothing but this
	// case notices, because the seed path runs only when there is no ledger — never on the tree that
	// would show it.
	t.Run("the seeded ledger says what the live one says", func(t *testing.T) {
		f := newRoot(t)
		f.installStats()
		f.run("--append", "seed, start", f.root)
		seeded := ledgerProse(readFile(t, f.root+"/skills/kk-reduce/stats.md"))
		live, err := os.ReadFile(liveLedger)
		if err != nil {
			t.Fatalf("the live ledger is what this case compares against: %v", err)
		}
		if seeded == "" || seeded != ledgerProse(string(live)) {
			t.Errorf("seeded:\n%slive:\n%s", indent(seeded), indent(ledgerProse(string(live))))
		}
	})
}

// The tree's own ledger, from the package directory `go test` runs in.
const liveLedger = "../../skills/kk-reduce/stats.md"

func newUnreadableBudgetFile(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n")
	f.write(f.root+"/kk-flavor/standards/core.md", "alpha beta gamma\n")
	f.chmod(f.root+"/kk-flavor/standards/core.md", 0o000)
	return f
}

func skillWithDescription(name string) string {
	return "---\nname: " + name + "\ndescription: four words exactly here\n---\n"
}

// `awk 'BEGIN { for (i = 1; i <= n; i++) printf "word%d ", i }'`.
func wordsCount(n int) string {
	var note strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&note, "word%d ", i)
	}
	return note.String()
}
