package ecocheck_test

// The always-loaded budget: which @imports resolve at the mount, and which are refused and named.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

func TestImportResolvedAtTheMount(t *testing.T) {
	t.Run("counts an import the mount really holds", func(t *testing.T) {
		newMountedImport(t).doesNotReport(uncounted)
	})

	t.Run("and folds it into the budget's file count", func(t *testing.T) {
		newMountedImport(t).reports("across 3 files")
	})

	// The pattern requires a non-word character before the `@`, and that character is a rune, not a
	// byte. Cutting a fixed two carried the tail of a multi-byte one into the name: `é@alpha.md`
	// resolved as `@alpha.md`, which is nowhere, and landed on the census line instead.
	t.Run("counts an import behind a two-byte boundary rune", func(t *testing.T) {
		newMultiByteBoundaryImport(t).doesNotReport(uncounted)
	})

	t.Run("and one behind a three-byte em dash, folding both into the file count", func(t *testing.T) {
		newMultiByteBoundaryImport(t).reports("across 4 files")
	})

	t.Run("refuses to resolve when this checkout is not the installed one", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
		f.newHomeWithoutFlavorMount()
		f.write(f.home+"/.claude/FOO.md", "one two three\n")
		f.reports(uncounted)
	})

	// The file has to sit exactly where the traversal lands. Put it anywhere else and the case passes
	// on "no such file", proving nothing.
	t.Run("refuses a name that is a path rather than a bare filename", func(t *testing.T) {
		newTraversalImport(t).reports(uncounted)
	})

	t.Run("and reports that refusal instead of leaving it to read as drift", func(t *testing.T) {
		newTraversalImport(t).reports(refused)
	})

	t.Run("leaves a plain subdirectory import uncounted", func(t *testing.T) {
		newSubdirectoryImport(t).reports(uncounted)
	})

	t.Run("and does not report it as a probe", func(t *testing.T) {
		newSubdirectoryImport(t).doesNotReport(refused)
	})

	t.Run("refuses a symlink at the mount", func(t *testing.T) {
		newSymlinkedImport(t).reports(uncounted)
	})

	t.Run("and reports that one too, the shape nothing legitimate produces", func(t *testing.T) {
		newSymlinkedImport(t).reports(refused)
	})

	t.Run("refuses a file at the mount it cannot read", func(t *testing.T) {
		newUnreadableImport(t).reports(uncounted)
	})

	t.Run("and reports it, the shape that hid a short figure one tier down", func(t *testing.T) {
		newUnreadableImport(t).reports(refused)
	})

	t.Run("refuses a kk-flavor symlinked to the install, which would open the gate", func(t *testing.T) {
		f := newRootWithSymlinkedFlavor(t)
		f.newHomeWithoutFlavorMount()
		f.symlink(f.root+"/real-flavor", f.home+"/.kk-flavor")
		f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
		f.write(f.home+"/.claude/FOO.md", "one two three\n")
		f.reports(uncounted)
	})

	t.Run("refuses an import its carrier does not put at this mount", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n")
		f.appendTo(f.root+"/kk-flavor/inject.md", "@BAR.md\n")
		f.newHome()
		f.write(f.home+"/.claude/BAR.md", "one two three four five\n")
		f.reports(uncounted)
	})

	t.Run("treats a fenced mention in CLAUDE.md as prose, not as its import", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\n```\n@FOO.md\n```\n")
		f.appendTo(f.root+"/kk-flavor/inject.md", "@FOO.md\n")
		f.newHome()
		f.write(f.home+"/.claude/FOO.md", "one two three four five\n")
		f.reports(uncounted)
	})

	t.Run("treats a backticked mention in CLAUDE.md the same way", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\nnever write `@FOO.md` here\n")
		f.appendTo(f.root+"/kk-flavor/inject.md", "@FOO.md\n")
		f.newHome()
		f.write(f.home+"/.claude/FOO.md", "one two three four five\n")
		f.reports(uncounted)
	})

	t.Run("caps resolution attempts and names what it skipped", func(t *testing.T) {
		f := newRoot(t)
		f.newHome()
		var claudeMd strings.Builder
		claudeMd.WriteString("# Root\n\n")
		for i := 1; i <= 65; i++ {
			fmt.Fprintf(&claudeMd, "@f%d.md\n", i)
			f.write(fmt.Sprintf("%s/.claude/f%d.md", f.home, i), "one\n")
		}
		f.write(f.root+"/CLAUDE.md", claudeMd.String())
		f.reports(uncounted)
	})

	// Byte-order sorting is what fixes the two positions these cases turn on: `../x.md` first, because
	// `.` sorts below every letter, and `d01.md` 65th — one past the 64-attempt cap, with the 63 `b`
	// names filling it. Under a sort that puts punctuation away, `../x.md` itself lands past the cap
	// and both asserts flip.
	t.Run("reports the probe-shaped name the resolver did look at", func(t *testing.T) {
		newCappedImportSpread(t).reports("not counted: ../x.md")
	})

	t.Run("and carries no reason over to a name past the cap", func(t *testing.T) {
		newCappedImportSpread(t).doesNotReport("not counted: d01.md")
	})
}

// The fourth resolver, and the one the oracle cases in refs_test.go did not reach. A Read-always
// target is a link the reviewed branch wrote and `..` is in its charset, so asking the filesystem
// before testing containment tells that branch's author which files the reviewing machine holds:
// present came back `budget file refused`, absent came back `does not exist`. Both have to read the
// same now, and each case below asserts one half — asserting only the absent one passes against a
// checker that still probes.
func TestATraversalReadAlwaysTargetIsNotStatted(t *testing.T) {
	// One target outside the root that is there and one that is not. The refusal cuts the name it
	// echoes at 80 bytes, so both render as the same line — which is the point, and also why the
	// discriminating assertion is whether the other arm's wording appears at all.
	newProbe := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.write(f.base+"/present.md", "# here\n")
		up := "../../../../../../../../../../../../../../../../../../../../"
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [a]("+up+strings.TrimPrefix(f.base+"/present.md", "/")+")"+
				"\n- [b]("+up+strings.TrimPrefix(f.base+"/absent.md", "/")+")\n")
		return f
	}

	// The control: the Read-always list was read and a target was refused, so the silence below is a
	// refusal rather than a fixture that never reached the budget scan.
	t.Run("refuses a target that resolves outside the root (control for the case below)", func(t *testing.T) {
		newProbe(t).reports("budget file refused")
	})

	// The leaked bit itself. Present, the target came back refused; absent, it came back through the
	// other arm, and which of the two printed was the answer to the branch author's question.
	//
	// Matched on the head of that arm's line rather than on its trailing `does not exist`: these paths
	// run past the printer's 500-byte line cap, so the tail is cut and asserting on it passes whatever
	// the checker did. The subtest is named clear of the phrase too — t.TempDir() puts the subtest's
	// own name inside every path the run echoes.
	t.Run("and does not report either through the arm that says it is absent", func(t *testing.T) {
		newProbe(t).doesNotReport("inject.md lists '")
	})

	// The control. Without it, a checker that refused every listed target would pass the three cases
	// above as readily as one that only stops asking about paths outside the tree.
	t.Run("while an in-root Read-always target is still counted", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/real.md", "one two three\n")
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [a](standards/real.md)\n")
		f.doesNotReport("budget file refused")
	})
}

// PathExists answers false two ways, and the arm behind it called both of them absent. A Read-always
// target behind a directory this process cannot open is exactly where the router says it is, and the
// finding sent its reader hunting for a file nobody had removed — while the always-loaded figure went
// short by whatever that file holds, with the shortfall named as the tree's own defect.
//
// ecostats reports the same tree and split these two first; the answers have to match, because two
// detectors disagreeing about what a permission failure means is the hazard rather than the wording.
func TestAnUnreachableReadAlwaysTargetIsRefusedNotCalledAbsent(t *testing.T) {
	newUnreachable := func(t *testing.T) *fixture {
		skipUnlessModeDeniesRead(t, "an unreachable target cannot be built here")
		f := newRoot(t)
		shut := f.root + "/kk-flavor/standards/shut"
		f.mkdirAll(shut)
		f.write(shut+"/deep.md", "one two three\n")
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [a](standards/shut/deep.md)\n")
		f.chmod(shut, 0o000)
		// Restored, or t.TempDir's own cleanup cannot descend to remove it and fails the case on the
		// way out — a red that says nothing about what was asserted.
		f.t.Cleanup(func() { os.Chmod(shut, 0o755) })
		return f
	}

	t.Run("refuses a target it could not reach", func(t *testing.T) {
		newUnreachable(t).reports("budget file refused")
	})

	// Matched on the head of the absent arm's line because that wording opens the arm and no other
	// finding carries it. Not for the reason the sibling case above gives: these paths are short, and
	// the line lands around 240 bytes, well inside the printer's width bound.
	t.Run("and does not report it through the arm that says it is absent", func(t *testing.T) {
		newUnreachable(t).doesNotReport("inject.md lists '")
	})

	// The control, and the half that keeps the fix honest: a target nobody wrote is still absent, and
	// still says so. Without it, refusing every failed target would pass the two cases above.
	t.Run("while a target nobody wrote is still reported as absent", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [a](standards/nowhere.md)\n")
		f.reports("inject.md lists '")
	})
}

// The bound both refusals below quote a name under, restated here because the package under test does
// not export it, and a name long enough to run past it.
const (
	budgetMessageBound          = 80
	overEveryBudgetMessageBound = 200
)

// Both refusals name something the reviewed tree chose the length of, and the budget one carries the
// reason on its own tail — `<target>: <what Lstat said>`. Cut with nothing marking it, a reason stops
// being a reason and becomes a shorter wrong one: `permission denied` arrives as `permission de`, and
// a name cut at its colon arrives ending in `: `, which reads as "there was no reason" rather than as
// "the reason did not fit". ecostats' suite holds the same pair over the same two messages.
func TestACutRefusalSaysThatItWasCut(t *testing.T) {
	// A directory where the Read-always target should be a file: it exists, so this is not the absent
	// branch, and it is not regular, so containment refuses it and the refusal quotes the name. No
	// mode bit is involved, so the case runs for every user, root included.
	newLongRefusedTarget := func(t *testing.T) *fixture {
		t.Helper()
		f := newRoot(t)
		long := strings.Repeat("b", overEveryBudgetMessageBound)
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [core](standards/"+long+".md)\n")
		f.mkdirAll(f.root + "/kk-flavor/standards/" + long + ".md")
		return f
	}

	t.Run("refuses a budget file whose name runs past the bound (control)", func(t *testing.T) {
		newLongRefusedTarget(t).reports("budget file refused")
	})

	t.Run("and marks that name rather than printing a shorter wrong one", func(t *testing.T) {
		newLongRefusedTarget(t).reports("b" + shell.CutMarker)
	})

	// The other message on this path. An import name is the reviewed tree's to choose too, and this
	// one is the shape that carries a reason: a traversal.
	newLongRefusedImport := func(t *testing.T) *fixture {
		t.Helper()
		return newRootImporting(t, "../../"+strings.Repeat("e", overEveryBudgetMessageBound)+".md")
	}

	t.Run("refuses an import whose name runs past the bound (control)", func(t *testing.T) {
		newLongRefusedImport(t).reports(refused)
	})

	// Matched on the whole cut name under the refusal's own wording, never on the marker alone: a
	// refused import is also named in the uncounted-import note, which marks its own cut at a
	// different bound — so "the marker is somewhere in the output" passes through that other call
	// site whatever this one did. The mutation harness is where that was observed rather than argued.
	t.Run("and marks that name under the refusal, not only in the census note", func(t *testing.T) {
		kept := "../../" + strings.Repeat("e", budgetMessageBound-len("../../")-len(shell.CutMarker))
		newLongRefusedImport(t).reports("named but not counted: " + kept + shell.CutMarker)
	})
}

// A root whose CLAUDE.md imports one name, plus the home its mount resolves at. What each case below
// varies is only what it puts at that mount.
func newRootImporting(t *testing.T, imported string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@"+imported+"\n")
	f.newHome()
	return f
}

func newMountedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "FOO.md")
	f.write(f.home+"/.claude/FOO.md", "one two three\n")
	return f
}

// Two imports whose boundary character is not one byte: `é` is two and `—` is three.
func newMultiByteBoundaryImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\nsee é@alpha.md and —@beta.md\n")
	f.newHome()
	f.write(f.home+"/.claude/alpha.md", "one two three\n")
	f.write(f.home+"/.claude/beta.md", "four five six\n")
	return f
}

func newTraversalImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "../../escape.md")
	f.write(f.root+"/escape.md", "secret words here\n")
	return f
}

func newSubdirectoryImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "dir/file.md")
	f.mkdirAll(f.home + "/.claude/dir")
	f.write(f.home+"/.claude/dir/file.md", "one two three\n")
	return f
}

func newSymlinkedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "FOO.md")
	f.write(f.base+"/linked.md", "one two three\n")
	f.symlink(f.base+"/linked.md", f.home+"/.claude/FOO.md")
	return f
}

// The `isReadable` limb of the import refusal: a file at the mount this process cannot open. Root
// reads a mode-000 file regardless of the mode, and nothing else builds this condition — every other
// way of making the read fail (a missing path, a directory, a symlink) is refused by an earlier limb
// and never reaches this one. So the case declines rather than asserting against a file the tool
// reads happily.
func newUnreadableImport(t *testing.T) *fixture {
	t.Helper()
	skipUnlessModeDeniesRead(t, "an unreadable file at the mount cannot be built here")
	f := newRootImporting(t, "FOO.md")
	f.write(f.home+"/.claude/FOO.md", "one two three\n")
	f.chmod(f.home+"/.claude/FOO.md", 0o000)
	return f
}

func newCappedImportSpread(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newHome()
	var claudeMd strings.Builder
	claudeMd.WriteString("# Root\n\n")
	for i := 1; i <= 63; i++ {
		fmt.Fprintf(&claudeMd, "@b%02d.md\n", i)
		f.write(fmt.Sprintf("%s/.claude/b%02d.md", f.home, i), "one\n")
	}
	claudeMd.WriteString("@../x.md\n@d01.md\n")
	f.write(f.root+"/CLAUDE.md", claudeMd.String())
	f.write(f.home+"/.claude/d01.md", "one\n")
	return f
}
