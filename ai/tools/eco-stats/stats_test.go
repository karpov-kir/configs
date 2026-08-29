package ecostats_test

// The measurement cases, driven in-process. They were ported one for one from a shell suite that no
// longer exists; git history is where the pairing can still be read.
//
// The agreement cases at the top hold the invariant that matters: for one tree, both tools report the
// same router figure. They call into ecocheck directly rather than running its binary, for the same
// reason everything else here is in-process.

import (
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

	// A router may name one file twice — two triggers reaching the same standard is the ordinary way
	// that happens. It is one file in context either way, so counting it twice overstates the tier and
	// the two tools stop agreeing. Every other case here lists each target once, which is why the
	// disagreement survived: the invariant was only ever asserted on trees that could not show it.
	t.Run("agree when the router lists one target twice", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n- [core again](standards/core.md)\n")
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

// A refusal says "not read, not counted", and a figure carrying the refused file's words says the
// opposite in the same output. The word count is the half that had no bound: wordsAcross streams, so
// it counted a file no reader here may read, and the run then called an exact figure SHORT. Only the
// bound decides membership, and ecocheck applies the same one to the same tier.
func TestARefusedBudgetFilesWordsAreNotInTheFigure(t *testing.T) {
	// inject.md is the only budget file left holding words, and it holds seven of them. Counted, the
	// oversize target would add four more — `alpha`, `beta`, `gamma`, and the run of NUL bytes behind
	// them, which carries no space so it counts as one word — so eleven is what this figure read
	// before the bound reached the count.
	t.Run("the router figure holds none of the oversize file's words", func(t *testing.T) {
		f := newOversizeBudgetFile(t)
		if got := f.figure("always-loaded"); got != "7" {
			t.Fatalf("always-loaded = %q, want \"7\" — 11 is the refused file's words counted anyway", got)
		}
	})

	// The count in the exit line is how many files were refused, not how many readers looked at one.
	// The budget count and the import scan both reach this file.
	t.Run("a file every reader reaches is refused once", func(t *testing.T) {
		f := newOversizeBudgetFile(t)
		stdout, stderr, status := f.run(f.root)
		if got := strings.Count(stdout+stderr, "budget file refused"); got != 1 {
			t.Errorf("%d refusal lines, want 1\n%s", got, indent(stdout+stderr))
		}
		if !strings.Contains(stdout+stderr, "1 budget file(s) refused") || status != 2 {
			t.Errorf("status: %d\n%s", status, indent(stdout+stderr))
		}
	})

	// The row is still withheld and every sound figure still printed, exactly as the other two
	// refusals do it — asserted here rather than in refusedBudgetFixtures, because the subtests over
	// that table quote ecocheck's own refusal wording and ecocheck words this one "file too large".
	t.Run("the row is withheld and the other figures still print", func(t *testing.T) {
		f := newOversizeBudgetFile(t)
		stdout, _, status := f.run(f.root)
		if status != 2 || !strings.Contains(stdout, "prose:") ||
			!strings.Contains(stdout, "SHORT:") || strings.Contains(stdout, "uncounted import") {
			t.Errorf("status: %d (want 2)\n%s", status, indent(stdout))
		}
	})

	// Both tools apply the same bound to the same tier, so both are short by the same file and their
	// router figures still match. A disagreement here is one of them counting a file it did not read —
	// which is the whole of what this bound decides.
	t.Run("ecocheck refuses the same file and reports the same router figure", func(t *testing.T) {
		f := newOversizeBudgetFile(t)
		if output := f.checkOutput(); !strings.Contains(output, "over the 8388608-byte bound") {
			t.Errorf("ecocheck did not refuse it, so agreeing with it proves nothing\n%s", indent(output))
		}
		f.assertScriptsAgree()
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

// The tool's own maxFileBytes, which this package cannot see from outside it. Stated rather than
// imported, so a case that drifts from the bound shows up as a fixture that stops being refused.
const overTheReadBound = 8<<20 + 1

// The third way a budget file gets refused, and the only one whose words the tool could still have
// counted: a Read-always target over the read bound. wordsAcross streams and never slurps, so nothing
// in the word count stopped where the line read stops — the figure went out holding this file's words
// while the same run printed "not read, not counted" and marked the figure SHORT.
//
// Sparse, because the size is the whole of what the bound tests and truncating to it costs no disk.
// The leading words are real so the case can discriminate: counted, they move the router figure and
// check.sh's figure never holds them.
func newOversizeBudgetFile(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n")
	target := f.root + "/kk-flavor/standards/core.md"
	f.write(target, "alpha beta gamma\n")
	if err := os.Truncate(target, overTheReadBound); err != nil {
		t.Fatalf("truncate the oversize budget file: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Size() <= overTheReadBound-1 {
		t.Fatalf("the fixture is %v bytes, inside the bound; the case proves nothing", info)
	}
	return f
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

// A whole-file read of a file the measured tree wrote. awk streamed; this does not, so every line
// becomes a slice header: a committed 64 MiB of newlines packs to about 65 KB and took 2.46 GB of
// resident memory here, and half a gigabyte of it is an OOM kill rather than a measurement. The read
// was left unbounded on the grounds that a bound would leave the always-loaded figure short with
// nothing saying so — which is what refuseBudgetFile already exists to prevent.
func TestAnOversizeBudgetFileIsRefusedRatherThanRead(t *testing.T) {
	build := func(t *testing.T) *fixture {
		t.Helper()
		f := newRoot(t)
		// Just past the bound, and made of newlines because that is the shape that costs the most for
		// the fewest committed bytes.
		body := make([]byte, overTheReadBound)
		for i := range body {
			body[i] = '\n'
		}
		f.write(f.root+"/kk-flavor/inject.md", string(body))
		// Prose the run can measure, or it exits 2 for a scan that read nothing and never reaches the
		// refusal count this case is about.
		f.write(f.root+"/kk-flavor/standards/core.md", "alpha beta gamma\n")
		return f
	}

	t.Run("exits 2 and refuses it rather than reading it", func(t *testing.T) {
		f := build(t)
		stdout, stderr, status := f.run(f.root)
		if status != 2 || !strings.Contains(stdout+stderr, "budget file refused") {
			t.Errorf("status: %d (want 2)\n%s", status, indent(stdout+stderr))
		}
	})

	t.Run("and names the bound it was over", func(t *testing.T) {
		f := build(t)
		stdout, stderr, _ := f.run(f.root)
		if !strings.Contains(stdout+stderr, "-byte bound") || !strings.Contains(stdout+stderr, "NOT read") {
			t.Errorf("%s", indent(stdout+stderr))
		}
	})

	// One refusal, not one per reader: a budget file is read here and again by the import scan, and
	// the count in the exit line is meant to be how many files were refused.
	t.Run("and counts it once however many readers reach it", func(t *testing.T) {
		f := build(t)
		stdout, stderr, _ := f.run(f.root)
		if !strings.Contains(stdout+stderr, "1 budget file(s) refused") {
			t.Errorf("%s", indent(stdout+stderr))
		}
	})

	// The census reads a SKILL.md through the same bound, and it is the only reader that reaches the
	// bound with nothing else applying one to the same file — the budget path now counts words under
	// a bound of its own, so an oversize budget file is refused whether or not the line read stops.
	// Without this, removing the line read's bound changes no output any case here reads.
	t.Run("and refuses an oversize SKILL.md the census would otherwise read", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two three\n")
		skill := f.root + "/skills/big/SKILL.md"
		f.mkdirAll(f.root + "/skills/big")
		f.write(skill, skillWithDescription("big"))
		if err := os.Truncate(skill, overTheReadBound); err != nil {
			t.Fatalf("truncate the oversize skill: %v", err)
		}
		stdout, stderr, status := f.run(f.root)
		if status != 2 || !strings.Contains(stdout+stderr, "budget file refused") {
			t.Errorf("status: %d (want 2)\n%s", status, indent(stdout+stderr))
		}
	})

	// The control: a file just under the bound is read as it always was, so the refusal above is the
	// bound and not a fixture the tool could never measure.
	t.Run("while a file under the bound is measured as before", func(t *testing.T) {
		f := newRoot(t)
		body := make([]byte, 1<<20)
		for i := range body {
			body[i] = '\n'
		}
		f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n"+string(body))
		stdout, stderr, status := f.run(f.root)
		if status != 0 || strings.Contains(stdout+stderr, "budget file refused") {
			t.Errorf("status: %d (want 0)\n%s", status, indent(stdout+stderr))
		}
	})
}
