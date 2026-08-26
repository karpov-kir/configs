package ecocheck_test

// Tests for the checker's guards, one subtest per case of
// `~/.claude/skills/kk-ecosystem/scripts/check-test.sh`, carrying that case's name verbatim so the two
// suites compare name for name. A change to a scan needs a case here, and a case that cannot fail
// proves nothing — `ai/tools/mutate/` is what shows one can.
//
// Where the shell version ran several asserts against one fixture, each of them is its own subtest
// that rebuilds the tree to the state that assert saw. A pair reading "(control for the case below)"
// and the case after it are exactly that: the same tree without and with the hostile file.

import (
	"fmt"
	"strings"
	"testing"
)

func TestDirectionScan(t *testing.T) {
	t.Run("fires on a standard citing into a SKILL.md", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/x.md", "the mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n")
		f.reports(cites)
	})

	t.Run("fires on the root CLAUDE.md citing into a SKILL.md", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "see `~/.claude/skills/kk-drive/SKILL.md` for the mechanics\n")
		f.reports(cites)
	})

	t.Run("reports through the bounded findings path, not raw on stdout", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/x.md", "the mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n")
		f.reportedViaFindings(cites)
	})

	// The fourth form is not legal: ecosystem.md → **One home** bans a file the skill owns too. It is
	// quiet *here* only because the fixture mounts no skill — in any tree that holds `idsd-qualify`,
	// this scan reports the path and the bare-name scan below reports the name.
	t.Run("stays quiet on the four forms that are not a path into a SKILL.md", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/legal.md",
			"a glob names the set: `~/.claude/skills/*/SKILL.md`\n"+
				"a bare name is not a path: run it per its SKILL.md\n"+
				"a placeholder: `~/.claude/skills/<skill name>/SKILL.md`\n"+
				"a file the skill owns that is not its body: `~/.claude/skills/idsd-qualify/templates/retro-spawn-prompt.md`\n")
		f.doesNotReport(cites)
	})

	t.Run("does not read a violation through a symlinked CLAUDE.md", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.base+"/outside.md", "see `~/.claude/skills/kk-drive/SKILL.md`\n")
		f.symlink(f.base+"/outside.md", f.root+"/CLAUDE.md")
		f.doesNotReport(cites)
	})

	// The bare-name half of the same rule. A name counts only when a skill answers to it, so the
	// fixture has to mount one. The negative case is the same prose with nothing mounted: it pins that
	// gate, not the prose's legality — the unknown-skill scan reports that prose anyway.
	t.Run("fires on a standard naming a skill that exists", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "spawn `kk-drive` before any lens reads it\n")
		f.reports(names)
	})

	// The match keeps the trailing hyphen a token ends on, which a glob in prose produces. Without the
	// strip the name matches no skill directory and the violation goes unreported.
	t.Run("strips the trailing hyphen a glob leaves on a lane name", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "every `kk-drive-*` invocation is the same lane\n")
		f.reports(names)
	})

	// The suffix class carries `.` as well, so the match keeps the full stop of a sentence that ends
	// on the lane name — the commonest way prose names one. The hyphen case above does not reach it:
	// the strip has to take the whole trailing run.
	t.Run("strips the full stop a sentence ending on a lane name leaves", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "the lane that owns this one is kk-drive.\n")
		f.reports(names)
	})

	t.Run("the names scan stays quiet on a token no skill answers to", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/x.md", "spawn `kk-drive` before any lens reads it\n")
		f.doesNotReport(names)
	})

	t.Run("stays quiet on kk-flavor, which is the shared layer itself", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "a template under `kk-flavor/` is this same layer, not a lane\n")
		f.doesNotReport(names)
	})

	t.Run("fires on a skill named outside the kk-* and idsd-* families", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("sonar-api")
		f.write(f.root+"/kk-flavor/standards/x.md", "route it through `sonar-api`\n")
		f.reports(names)
	})

	// The case above already covers a lane named outside the `kk-*`/`idsd-*` families. No script is
	// mounted at the cited path here: this scan matches the shape of a path, it never resolves one.
	t.Run("fires on a path into a lane that is not a SKILL.md", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-humanize")
		f.write(f.root+"/kk-flavor/standards/x.md", "read `"+laneScriptRef+"`\n")
		f.reports(cites)
	})

	t.Run("stays quiet on a hyphenated compound built off a real lane name", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "a `kk-drive-verified` claim is not a lane\n")
		f.doesNotReport(names)
	})

	// One NUL byte made grep call the file binary, and it printed `Binary file … matches` or, on GNU
	// grep >= 3.5, nothing at all: every grep in the shell scan then read no violation out of a file
	// find had handed them, while the did-not-run guard stayed quiet because the file was still
	// walked. Go has no notion of a binary file, so these three cases pin that the hazard stays shut.
	t.Run("reads a lane name past a NUL byte that makes grep call the file binary", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "# Rule\n\x00\nspawn `kk-drive` before any lens reads it\n")
		f.reports(names)
	})

	t.Run("and reads a cited SKILL.md path past one too", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/x.md", "# Rule\n\x00\nthe mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n")
		f.reports("kk-drive/SKILL.md — move the rule")
	})

	// The third grep needed its own case: the two above cover the cites and names greps, and `-a` was
	// per-grep, so dropping it from this one alone would have left both of them green.
	t.Run("and reads a lane basename past one too", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.write(f.root+"/kk-flavor/standards/x.md", "# Rule\n\x00\nrun `comment-density.sh` before any lens reads it\n")
		f.reports(basenames)
	})

	// The cited path is echoed whole. One trailing segment stops it at `.../kk-humanize/scripts` and
	// drops the file the citation was about, which is the half that says what to go and move.
	t.Run("echoes a cited path whole, not truncated at one segment", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.write(f.root+"/kk-flavor/standards/x.md", "read `"+laneScriptRef+"`\n")
		f.reports("kk-humanize/scripts/comment-density.sh — move the rule")
	})

	// The third shape: a lane's file named by basename alone, carrying neither a lane name nor a path.
	t.Run("fires on a lane's file named by its basename alone", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.write(f.root+"/kk-flavor/standards/x.md", "run `comment-density.sh` before any lens reads it\n")
		f.reports(basenames)
	})

	// Uniqueness is the gate. A basename every lane carries names the kind of file, not one lane's
	// copy, so two mounted skills are the fixture: with one, `SKILL.md` resolves uniquely and this
	// case cannot fail.
	t.Run("stays quiet on a basename more than one lane carries", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.newMountedSkill("kk-humanize")
		f.write(f.root+"/kk-flavor/standards/x.md", "a bare name is not a path: run it per its SKILL.md\n")
		f.doesNotReport(basenames)
	})

	// A name the shared layer carries is not in the set. Here nothing under skills/ carries it either,
	// so it never enters the set; the subtraction cases below cover the half where a lane does.
	t.Run("stays quiet on a shared-layer sibling's own basename", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/writing.md", "# Writing\n")
		f.write(f.root+"/kk-flavor/standards/x.md", "the shape is `writing.md`\n")
		f.doesNotReport(basenames)
	})

	t.Run("does not report a basename that reached it inside a cited path", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.write(f.root+"/kk-flavor/standards/x.md", "read `"+laneScriptRef+"`\n")
		f.doesNotReport(basenames)
	})

	// The basename set is built from paths that cannot be split on a newline: a committed filename
	// holding one would reach a line-oriented reader as two names, and each half is a forgery the
	// reviewed tree chose. This is the muting half — the head becomes a second copy of a real
	// basename, the uniqueness gate drops it, and a genuine violation goes quiet. The control comes
	// first: without the hostile file, the finding must be there to lose.
	t.Run("reports a lane's basename (control for the case below)", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.write(f.root+"/kk-flavor/standards/x.md", "run `comment-density.sh` before any lens reads it\n")
		f.reports(basenames)
	})

	t.Run("a newline in a committed filename cannot mute a real basename finding", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.write(f.root+"/kk-flavor/standards/x.md", "run `comment-density.sh` before any lens reads it\n")
		f.newFileWithNewlineName(f.root+"/skills/kk-humanize/scripts/x\ncomment-density.sh",
			"not a script", "the basename forgery cases")
		f.reports(basenames)
	})

	// The forging half of the same split: the tail is a basename no lane carries, so a standard naming
	// a file nothing under skills/ holds is reported against a file the branch never touched. The name
	// is one the shared layer does not carry either, or the subtraction cases below would silence this
	// one for free.
	t.Run("a newline in a committed filename cannot forge a basename finding", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.write(f.root+"/kk-flavor/standards/x.md", "run `report.sh` at the close\n")
		f.newFileWithNewlineName(f.root+"/skills/kk-drive/q\nreport.sh", "x", "the basename forgery cases")
		f.doesNotReport(basenames)
	})

	// The shared layer's own basenames are subtracted from the set. The reviewed tree fills skills/,
	// so one committed file named after a standard would otherwise report every standard citing that
	// sibling. Control first, again: the same lane file fires while the shared layer carries no file
	// by that name.
	t.Run("reports a lane file the shared layer has no counterpart for (control)", func(t *testing.T) {
		newSubtractionFixture(t, false).reports(basenames)
	})

	t.Run("and calls nothing unchecked while one tier alone carries the name", func(t *testing.T) {
		newSubtractionFixture(t, false).doesNotReport(unchecked)
	})

	t.Run("a lane file cannot forge a finding against a standard citing its own sibling", func(t *testing.T) {
		newSubtractionFixture(t, true).doesNotReport(basenames)
	})

	// Subtracting the name narrows the scan, and a narrowing nothing says out loud is the mute this
	// reports: any `.md` committed under kk-flavor/ named after a lane file would otherwise buy
	// silence for free.
	t.Run("and says the name went unchecked instead of going quiet", func(t *testing.T) {
		newSubtractionFixture(t, true).reports(unchecked)
	})

	// The unchecked-name notice is bounded like every other shape here: the tree picks how many times
	// an ambiguous name appears, and each mention costs work to sanitise it.
	t.Run("bounds what the unchecked-name notice emits", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.mkdirAll(f.root + "/skills/kk-drive/notes")
		f.write(f.root+"/skills/kk-drive/notes/writing.md", "# notes\n")
		f.write(f.root+"/kk-flavor/standards/writing.md", "# Writing\n")
		f.floodWithLine(f.root+"/kk-flavor/standards/x.md", 45, "the shape is `writing.md`")
		f.reports(unchecked + ": " + f.root + "/kk-flavor/standards/x.md — 40 already shown")
	})

	// This half walks the tree per finding on top of the sanitising every hit costs, so its bound is
	// the one that matters most of the three.
	t.Run("bounds what the basename half of the scan emits", func(t *testing.T) {
		f := newRoot(t)
		f.newLaneWithScript()
		f.floodWithLine(f.root+"/kk-flavor/standards/x.md", 45, "run `comment-density.sh` now")
		f.reports(basenames + ": " + f.root + "/kk-flavor/standards/x.md — 40 already shown")
	})

	// The bound on what each shape of the scan emits. Every finding costs work to sanitise its hit, so
	// an unbounded emit lets one committed file turn this scan into tens of thousands of them.
	//
	// The file is pinned, not just the tail: leading with it is what sorts the notice ahead of that
	// file's own hits, so the printer's per-rank cap drops those before it drops the notice.
	t.Run("bounds what the names half of the scan emits", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.floodWithLine(f.root+"/kk-flavor/standards/x.md", 45, "spawn `kk-drive` now")
		f.reports(names + ": " + f.root + "/kk-flavor/standards/x.md — 40 already shown")
	})

	// The same bound over two files, each holding fewer hits than the cap. It can only fire while the
	// budget outlives one file. Which of the two files the notice names depends on the order the walk
	// hands them over, so the assert leaves that free. The fixture mounts no skill, so the names half
	// stays at zero and this tail can only have come from the cites half.
	t.Run("bounds the cites half across files, not once per file", func(t *testing.T) {
		f := newRoot(t)
		for i := 1; i <= 45; i++ {
			f.appendTo(fmt.Sprintf("%s/kk-flavor/standards/x%d.md", f.root, i%2),
				"the mechanics are `path/to/other/SKILL.md`\n")
		}
		f.reports("— 40 already shown across the shared layer")
	})

	// `kk-flavor` names the shared layer, not a lane. The reviewed tree picks what is in `skills/`, so
	// the exclusion cannot rest on no skill answering to that name.
	t.Run("excludes kk-flavor even when the tree commits a skill by that name", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-flavor")
		f.write(f.root+"/kk-flavor/standards/x.md", "a template under `kk-flavor/` is this same layer, not a lane\n")
		f.doesNotReport(names)
	})

	t.Run("a root with no violation and no symlink does not trip the guard", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n")
		f.doesNotReport(neverRan)
	})

	t.Run("reports itself when a symlinked kk-flavor leaves it nothing to walk", func(t *testing.T) {
		newSymlinkedFlavorViolation(t).reports(neverRan)
	})

	t.Run("and does not read the violation behind that symlink", func(t *testing.T) {
		newSymlinkedFlavorViolation(t).doesNotReport(cites)
	})
}

// A lane file and, optionally, the shared-layer sibling that subtracts its name from the checked set.
func newSubtractionFixture(t *testing.T, sharedCarriesTheName bool) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newMountedSkill("kk-drive")
	f.mkdirAll(f.root + "/skills/kk-drive/notes")
	f.write(f.root+"/skills/kk-drive/notes/writing.md", "# notes\n")
	f.write(f.root+"/kk-flavor/standards/x.md", "the shape is `writing.md`\n")
	if sharedCarriesTheName {
		f.write(f.root+"/kk-flavor/standards/writing.md", "# Writing\n")
	}
	return f
}

func newSymlinkedFlavorViolation(t *testing.T) *fixture {
	t.Helper()
	f := newRootWithSymlinkedFlavor(t)
	f.write(f.root+"/real-flavor/standards/x.md", "see `~/.claude/skills/kk-drive/SKILL.md`\n")
	f.write(f.root+"/CLAUDE.md", "# Root\n")
	return f
}

func TestImportResolvedAtTheMount(t *testing.T) {
	t.Run("counts an import the mount really holds", func(t *testing.T) {
		newMountedImport(t).doesNotReport(uncounted)
	})

	t.Run("and folds it into the budget's file count", func(t *testing.T) {
		newMountedImport(t).reports("across 3 files")
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

	t.Run("shows a tampered-check finding through a flood of link findings", func(t *testing.T) {
		f := newRoot(t)
		var flood strings.Builder
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&flood, "[x](nope%d.md)\n", i)
		}
		f.write(f.root+"/kk-flavor/standards/flood.md", flood.String())
		f.write(f.root+"/skills/notexec.sh", "#!/usr/bin/env bash\n")
		f.reports("script not executable")
	})

	// `filler` is what makes the needle flood-only: `flavor mounted elsewhere` alone also matches the
	// genuine rank-1 finding this fixture's $HOME may raise, and the assertion would then compare the
	// real finding against itself.
	t.Run("ranks a real finding above a flood whose link targets forge a mount finding", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/skills/notexec.sh", "#!/bin/sh\necho hi\n")
		f.chmod(f.root+"/skills/notexec.sh", 0o644)
		var flood strings.Builder
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&flood, "[x](nope%03d flavor mounted elsewhere filler)\n", i)
		}
		f.write(f.root+"/kk-flavor/standards/flood.md", flood.String())
		f.ranksAbove("script not executable", "flavor mounted elsewhere filler")
	})

	t.Run("surfaces the did-not-run guard under a flood that sorts ahead of it", func(t *testing.T) {
		f := newRootWithSymlinkedFlavor(t)
		var flood strings.Builder
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&flood, "[x](nope%03d.md)\n", i)
		}
		f.write(f.root+"/real-flavor/standards/flood.md", flood.String())
		f.reports(neverRan)
	})

	t.Run("a newline in a committed path cannot start a forged finding line", func(t *testing.T) {
		f := newRoot(t)
		f.newFileWithNewlineName(f.root+"/kk-flavor/standards/a\nsyntax: FORGED.md",
			"[x](nowhere.md)", "the forged-finding-line case")
		forged, output := f.countLinesStartingWith("syntax: FORGED")
		if forged != 0 {
			t.Errorf("a committed path started %d forged finding line(s)\n%s", forged, indent(output))
		}
	})

	t.Run("shows a syntax error under a flood of its own priority tier", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/skills/broken.sh", "if then\n")
		f.chmod(f.root+"/skills/broken.sh", 0o755)
		for i := 1; i <= 300; i++ {
			path := fmt.Sprintf("%s/skills/notexec%d.sh", f.root, i)
			f.write(path, "#!/bin/sh\necho ok\n")
			f.chmod(path, 0o644)
		}
		f.reports("syntax: ")
	})

	t.Run("shows a rank-1 finding under a flood of the gravest class", func(t *testing.T) {
		newGravestClassFlood(t).reports(neverRan)
	})

	t.Run("and says how many of the flooding class it withheld", func(t *testing.T) {
		newGravestClassFlood(t).reports("of this class, suppressed")
	})
}

func newMountedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
	f.newHome()
	f.write(f.home+"/.claude/FOO.md", "one two three\n")
	return f
}

func newTraversalImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@../../escape.md\n")
	f.newHome()
	f.write(f.root+"/escape.md", "secret words here\n")
	return f
}

func newSubdirectoryImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@dir/file.md\n")
	f.newHome()
	f.mkdirAll(f.home + "/.claude/dir")
	f.write(f.home+"/.claude/dir/file.md", "one two three\n")
	return f
}

func newSymlinkedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
	f.newHome()
	f.write(f.base+"/linked.md", "one two three\n")
	f.symlink(f.base+"/linked.md", f.home+"/.claude/FOO.md")
	return f
}

func newUnreadableImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
	f.newHome()
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

func newGravestClassFlood(t *testing.T) *fixture {
	t.Helper()
	f := newRootWithSymlinkedFlavor(t)
	for i := 1; i <= 120; i++ {
		path := fmt.Sprintf("%s/skills/broken%d.sh", f.root, i)
		f.write(path, "if then\n")
		f.chmod(path, 0o755)
	}
	return f
}

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

// Each defect below makes a skill unreachable rather than merely mis-linked: the loader finds a skill
// by its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
func TestSkillDirectory(t *testing.T) {
	t.Run("fires on a skill directory holding no SKILL.md", func(t *testing.T) {
		newBrokenSkillDirs(t).reports("skill dir without SKILL.md")
	})

	t.Run("fires when the frontmatter name is not the directory name", func(t *testing.T) {
		newBrokenSkillDirs(t).reports("skill name/dir mismatch")
	})

	t.Run("fires on a SKILL.md carrying no description", func(t *testing.T) {
		newBrokenSkillDirs(t).reports("skill without a description")
	})
}

func newBrokenSkillDirs(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.mkdirAll(f.root + "/skills/orphan")
	f.mkdirAll(f.root + "/skills/wrong-name")
	f.mkdirAll(f.root + "/skills/no-desc")
	f.write(f.root+"/skills/wrong-name/SKILL.md", "---\nname: misnamed\ndescription: does a thing\n---\n")
	f.write(f.root+"/skills/no-desc/SKILL.md", "---\nname: no-desc\n---\n")
	return f
}

func TestScriptTestPosition(t *testing.T) {
	t.Run("fires on a script naming neither a test nor an untested reason", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("lonely.sh", "#!/usr/bin/env bash\n# Does a thing.\ntrue")
		f.reports(noPosition)
	})

	t.Run("fires on a header naming a -test.sh that is not in the tree", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("claims.sh", "#!/usr/bin/env bash\n# A change here needs a case in claims-test.sh beside it.\ntrue")
		f.reports(missingTest)
	})

	t.Run("accepts a header whose named test exists", func(t *testing.T) {
		newCoveredScript(t).doesNotReport(missingTest)
	})

	t.Run("a named existing test is a declared position", func(t *testing.T) {
		newCoveredScript(t).doesNotReport(noPosition)
	})

	t.Run("accepts an explicit untested: declaration with a reason", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("waived.sh", "#!/usr/bin/env bash\n# untested: a four-line wrapper whose only failure mode is the exec bit.\ntrue")
		f.doesNotReport(noPosition)
	})

	t.Run("a bare untested: with no reason does not clear the check", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("bare.sh", "#!/usr/bin/env bash\n# untested:\ntrue")
		f.reports(noPosition)
	})

	// The harness is exempt: asking a test file to name its own test makes every one of them a finding.
	t.Run("asks nothing of -test.sh and -mutate.sh themselves", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("harness-test.sh", "#!/usr/bin/env bash\ntrue")
		f.newScript("harness-mutate.sh", "#!/usr/bin/env bash\ntrue")
		f.doesNotReport(noPosition)
	})

	// Header-scoped on purpose: a suite a script merely mentions in its body would read as coverage.
	t.Run("a -test.sh named below the header does not count as declared", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("body.sh", "#!/usr/bin/env bash\n# Does a thing.\nset -u\n# see also body-test.sh\ntrue")
		f.reports(noPosition)
	})

	// The cap that keeps a crafted header from turning one scan into thousands of whole-tree walks. It
	// has to *report*, never quietly read less than it looks like it read.
	t.Run("reports a header naming more suites than it reads", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("greedy.sh", "#!/usr/bin/env bash\n"+
			"# see n1-test.sh n2-test.sh n3-test.sh n4-test.sh n5-test.sh n6-test.sh\n"+
			"# and n7-test.sh n8-test.sh n9-test.sh n10-test.sh n11-test.sh n12-test.sh\ntrue")
		f.reports("names more suites than the scan reads")
	})

	// The bound on the header read. A declaration past 200 lines is not seen, which is correct, and it
	// still has to be *reported* rather than pass as declared.
	t.Run("a declaration past the header bound does not clear the check", func(t *testing.T) {
		f := newRoot(t)
		var buried strings.Builder
		buried.WriteString("#!/usr/bin/env bash\n")
		for line := 1; line <= 205; line++ {
			fmt.Fprintf(&buried, "# padding %d\n", line)
		}
		buried.WriteString("# untested: this reason sits past the 200-line bound and cannot clear the check\n")
		buried.WriteString("true\n")
		f.write(f.root+"/skills/buried.sh", buried.String())
		f.chmod(f.root+"/skills/buried.sh", 0o755)
		f.reports(noPosition)
	})

	// The suite list is built from filenames the reviewed tree chose. A newline in one splits a
	// basename in two, the tail reads as a suite that exists, and a header naming an absent suite then
	// passes. The control case comes first: without the hostile file, the finding must be there to
	// lose.
	t.Run("reports a named suite that is absent (control for the case below)", func(t *testing.T) {
		newGhostSuiteScript(t, false).reports(missingTest)
	})

	t.Run("a newline in a filename cannot forge the suite that satisfies a header", func(t *testing.T) {
		newGhostSuiteScript(t, true).reports(missingTest)
	})

	t.Run("a suite name starting with a dash is still checked", func(t *testing.T) {
		newDashSuiteScript(t).reports(missingTest)
	})

	t.Run("and grep never dumps its usage into the findings", func(t *testing.T) {
		newDashSuiteScript(t).doesNotReport("unrecognized option")
	})

	t.Run("nor its usage banner", func(t *testing.T) {
		newDashSuiteScript(t).doesNotReport("Usage: grep")
	})
}

func newCoveredScript(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("covered.sh", "#!/usr/bin/env bash\n# A change here needs a case in covered-test.sh beside it.\ntrue")
	f.newScript("covered-test.sh", "#!/usr/bin/env bash\ntrue")
	return f
}

func newGhostSuiteScript(t *testing.T, forgeTheSuite bool) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("tool.sh", "#!/usr/bin/env bash\n# a change here needs a case in ghost-test.sh\ntrue")
	if forgeTheSuite {
		f.newFileWithNewlineName(f.root+"/skills/x\nghost-test.sh", "not a suite", "the forged-suite-name case")
	}
	return f
}

func newDashSuiteScript(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("dash.sh", "#!/usr/bin/env bash\n# a change here needs a case in --test.sh\ntrue")
	return f
}
