package ecostats_test

// The measurement cases, driven in-process. They were ported one for one from a shell suite that no
// longer exists; git history is where the pairing can still be read.
//
// The agreement cases at the top hold the invariant that matters: for one tree, both tools report the
// same router figure. They call into ecocheck directly rather than running its binary, for the same
// reason everything else here is in-process.

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
	for _, fixture := range refusedBudgetFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Run("exits 2 and says why, on a refused budget file", func(t *testing.T) {
				f := fixture.build(t)
				stdout, stderr, status := f.run(f.root)
				if status != 2 || !strings.Contains(stdout+stderr, "budget file refused") {
					t.Errorf("status: %d\n%s", status, indent(stdout+stderr))
				}
			})

			// Both tools print their figure on this path, so the agreement is meaningful here: each is
			// short by the same refused file, and a disagreement is one of them counting a file it
			// could not read.
			t.Run("and check.sh refuses it too, reporting the same router figure rather than counting it", func(t *testing.T) {
				f := fixture.build(t)
				output := f.checkOutput()
				if !strings.Contains(output, "budget file refused") {
					t.Errorf("%s", indent(output))
				}
				f.assertScriptsAgree()
			})
		})
	}
}

func TestAShortFigureIsStillReported(t *testing.T) {
	// Withholding the row is not withholding the reading. Prose, scripts, the ledger and skills all
	// measured, and a caller handed an empty report cannot tell a refused budget file from a tree
	// that vanished.
	for _, fixture := range refusedBudgetFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Run("a refused budget file still prints every figure, and marks the short one apart from `+`", func(t *testing.T) {
				f := fixture.build(t)
				stdout, _, status := f.run(f.root)
				// `uncounted import` is the `+` lower-bound note. A refusal supports no lower bound, so
				// borrowing that wording here would teach a reader to read a floor off a figure that has none.
				if status != 2 || !strings.Contains(stdout, "prose:") ||
					!strings.Contains(stdout, "SHORT:") || strings.Contains(stdout, "uncounted import") {
					t.Errorf("status: %d (want 2)\n%s", status, indent(stdout))
				}
			})
		})
	}
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

func TestAMissingReadAlwaysTargetCannotReachTheTerminalRaw(t *testing.T) {
	// The name comes out of inject.md, which a reviewed branch writes. `\x1b[2K` erases the line it
	// lands on, so an unsanitised name deletes whatever the run printed beside it. Both tools report
	// this same missing target, so both are held to it.
	t.Run("an ESC byte in a Read-always target is stripped by both tools", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [core](standards/be\x1b[2Kfore.md)\n")
		f.write(f.root+"/CLAUDE.md", "one two\n")
		_, fromStats, _ := f.run(f.root)
		fromCheck := f.checkOutput()

		// The message must still have fired: a run that reported nothing carries no ESC byte either,
		// and would pass the byte count alone while saying nothing about sanitising.
		for _, said := range []struct{ tool, output string }{{"stats", fromStats}, {"check", fromCheck}} {
			if !strings.Contains(said.output, "under Read always") || strings.Contains(said.output, "\x1b") {
				t.Errorf("%s said:\n%s", said.tool,
					indent(strings.ReplaceAll(said.output, "\x1b", "<ESC>")))
			}
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

func TestTheNoteCannotCarryAControlByteIntoTheLedger(t *testing.T) {
	// The note is written by whoever ran the skill, committed, and read back by a later pass. `\x1b[2K`
	// erases the line it lands on, so an ESC left in it edits the terminal of everyone who later reads
	// the ledger — the byte TestAMissingReadAlwaysTargetCannotReachTheTerminalRaw bars from a message,
	// barred here from the record.
	t.Run("an ESC in the note reaches neither the ledger nor the terminal", func(t *testing.T) {
		f := newRoot(t)
		ledger := f.newLedger("| date | prose | scripts | always-loaded | skills | what ran |\n|---|---|---|---|---|---|\n")
		f.write(f.root+"/CLAUDE.md", "one two\n")
		before := rowsIn(t, ledger)
		stdout, stderr, status := f.run("--append", "ran a pass\x1b[2K and stopped", f.root)
		written := readFile(t, ledger)
		appended := rowsIn(t, ledger) - before

		// The row has to have landed: a run that appended nothing carries no ESC either, and would
		// pass a byte check while saying nothing about sanitising.
		if appended != 1 || strings.Contains(written+stdout+stderr, "\x1b") {
			t.Errorf("status: %d\nrows appended: %d (want 1)\n%s", status, appended,
				indent(strings.ReplaceAll(written+stdout+stderr, "\x1b", "<ESC>")))
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

// The two ways a budget file gets refused, run through the same cases. `containedInRoot` refuses a
// symlink, a non-regular file and an unreadable one with one message, so either builder reaches the
// refusal — but only the second can be built by every process. Both are here rather than only the
// portable one: the mode-000 file is the sole cover for the `isReadable` limb, and dropping it would
// leave that limb passing on its neighbours.
var refusedBudgetFixtures = []struct {
	name  string
	build func(*testing.T) *fixture
}{
	{"unreadable by mode", newUnreadableBudgetFile},
	{"a directory where the budget file belongs", newRefusedBudgetFile},
}

// True when a mode of 000 actually stops this process reading. Probed rather than compared against
// uid 0: root is the common case, but CAP_DAC_OVERRIDE without root and a filesystem that does not
// carry the bit behave the same way, and all three make the fixture below a file the tool reads
// happily. Answering by observation means this needs no list of the environments that lie.
func modeDeniesRead(t *testing.T) bool {
	t.Helper()
	probe := t.TempDir() + "/probe"
	if err := os.WriteFile(probe, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	if err := os.Chmod(probe, 0o000); err != nil {
		t.Fatalf("chmod probe: %v", err)
	}
	file, err := os.Open(probe)
	if err != nil {
		return true
	}
	file.Close()
	return false
}

// The `isReadable` limb of the refusal: a regular file, in the root, that the process cannot open.
// Root reads a mode-000 file regardless of the mode, so on a root runner this condition does not
// exist to be built — and no construction substitutes, because every other way of making a read fail
// (a directory, a dangling symlink, a missing path) is refused by an earlier limb and never reaches
// this one. Hence a skip rather than a rewrite: the refusal itself stays covered on those runners by
// newRefusedBudgetFile, and only the limb that is genuinely unreachable goes unasserted.
func newUnreadableBudgetFile(t *testing.T) *fixture {
	t.Helper()
	if !modeDeniesRead(t) {
		t.Skip("this process reads a mode-000 file regardless of the mode (root, or CAP_DAC_OVERRIDE), so a budget file it cannot read cannot be built here — the refusal stays covered by the directory fixture beside this one")
	}
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n")
	f.write(f.root+"/kk-flavor/standards/core.md", "alpha beta gamma\n")
	f.chmod(f.root+"/kk-flavor/standards/core.md", 0o000)
	return f
}

// The same refusal reached with no mode bit involved, so it holds for every user root included: a
// directory standing where the Read-always target should be a file. It exists, so it is not the
// missing-target branch, and it is not regular, so containment refuses it.
func newRefusedBudgetFile(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n")
	f.mkdirAll(f.root + "/kk-flavor/standards/core.md")
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
