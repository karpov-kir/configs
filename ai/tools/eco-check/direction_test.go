package ecocheck_test

// The direction scan: the shared layer never cites into a lane, never names one, and never reaches
// into one by basename — plus the guard that fires when a symlinked kk-flavor left it nothing to walk.
// Two cases here cover scans outside that block, kept beside it because that is where the shell suite
// this was ported from grouped them.

import (
	"fmt"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
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

	// The four forms that wear the shape of a path into a lane, one case each. Written as four cases
	// and not as one file carrying four lines, because merged they hid two defects that a reader cannot
	// see and a single assertion cannot separate: a form quiet for no reason at all is
	// indistinguishable from one a guard keeps quiet, and a form quiet because the fixture was too thin
	// to reach the scan is indistinguishable from both. Each case below goes red when its own guard is
	// removed, and nothing else here does that work for it.
	//
	// The two path-shaped forms turn on one guard: the run before `/SKILL.md` must open on a name
	// character, or a glob's or a placeholder's bare `/SKILL.md` tail matches on its own.
	t.Run("stays quiet on a glob over the lanes, which names no one lane", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/legal.md", "a glob names the set: `~/.claude/skills/*/SKILL.md`\n")
		f.doesNotReport(cites)
	})

	t.Run("stays quiet on a placeholder segment, which names no lane either", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/legal.md", "a placeholder: `~/.claude/skills/<skill name>/SKILL.md`\n")
		f.doesNotReport(cites)
	})

	// The third form, a bare `SKILL.md`, carries no path at all, so the cites scan is not what governs
	// it and asserting against that scan observes nothing. The basename scan is, and its own guard is
	// uniqueness: "stays quiet on a basename more than one lane carries" below is that form's case,
	// with the control that says its fixture reaches the scan.
	//
	// The fourth form, which the merged case asserted silence for and should not have: ecosystem.md →
	// **One home** bans a path into a lane whatever the file at the end of it is — "not a section, not
	// a file it owns, not a script it ships" — and a template is a file the lane owns. It was quiet
	// only because that fixture mounted no skill, so the assertion held on a tree where the rule had
	// nothing to be broken against, and went on holding while every real tree broke it.
	t.Run("fires on a path into a template a lane owns", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("idsd-qualify")
		f.write(f.root+"/kk-flavor/standards/x.md",
			"the template is `~/.claude/skills/idsd-qualify/templates/qualify-report-template.md`\n")
		f.reports(cites)
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

	// The scans outside the direction block need to read past a NUL just as much, and the shell
	// version's greps had no `-a`: one committed NUL made BSD grep answer `Binary file X matches`,
	// which both replaced the real finding and put a tree-chosen path inside the text of one. An agent
	// drafts PR comments from these, so that is an injection, not only a miss. Two scans stand for the
	// four, one per grep shape the shell version used.
	t.Run("reads a markdown link past a NUL byte", func(t *testing.T) {
		f := newNulByteFile(t)
		f.reports(ecocheck.DanglingLink + f.root + "/kk-flavor/standards/nul.md -> nowhere.md")
	})

	t.Run("and reads a skill name past one, rather than reporting grep's own notice as the name", func(t *testing.T) {
		newNulByteFile(t).reports(ecocheck.UnknownSkillReferenced + "kk-nonesuch")
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
	// case cannot fail. The third lane file is what makes the fixture reach the scan at all — the scan
	// is entered only where some lane basename is unique, and a tree of nothing but `SKILL.md` has
	// none, so without it this case is quiet because no basename was looked at. The control after it
	// is what says otherwise.
	t.Run("stays quiet on a basename more than one lane carries", func(t *testing.T) {
		f := newTwoLaneTree(t)
		f.write(f.root+"/kk-flavor/standards/x.md", "a bare name is not a path: run it per its SKILL.md\n")
		f.doesNotReport(basenames)
	})

	t.Run("while firing on this same tree's unique lane basename (control for the case above)", func(t *testing.T) {
		f := newTwoLaneTree(t)
		f.write(f.root+"/kk-flavor/standards/x.md", "the density gate is `comment-density.sh`\n")
		f.reports(basenames)
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
		newLaneFileOnlyALaneCarries(t).reports(basenames)
	})

	t.Run("and calls nothing unchecked while one tier alone carries the name", func(t *testing.T) {
		newLaneFileOnlyALaneCarries(t).doesNotReport(unchecked)
	})

	t.Run("a lane file cannot forge a finding against a standard citing its own sibling", func(t *testing.T) {
		newLaneFileTheSharedLayerAlsoCarries(t).doesNotReport(basenames)
	})

	// Subtracting the name narrows the scan, and a narrowing nothing says out loud is the mute this
	// reports: any `.md` committed under kk-flavor/ named after a lane file would otherwise buy
	// silence for free.
	t.Run("and says the name went unchecked instead of going quiet", func(t *testing.T) {
		newLaneFileTheSharedLayerAlsoCarries(t).reports(unchecked)
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

// A lane file whose basename nothing in the shared layer answers to, so the name stays in the
// checked set and a standard naming it is a finding.
func newLaneFileOnlyALaneCarries(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newMountedSkill("kk-drive")
	f.mkdirAll(f.root + "/skills/kk-drive/notes")
	f.write(f.root+"/skills/kk-drive/notes/writing.md", "# notes\n")
	f.write(f.root+"/kk-flavor/standards/x.md", "the shape is `writing.md`\n")
	return f
}

// The same tree with the shared layer carrying that basename too, which subtracts it from the checked
// set — the shape one committed file would otherwise buy silence with.
func newLaneFileTheSharedLayerAlsoCarries(t *testing.T) *fixture {
	t.Helper()
	f := newLaneFileOnlyALaneCarries(t)
	f.write(f.root+"/kk-flavor/standards/writing.md", "# Writing\n")
	return f
}

// One standard carrying a dangling link and an unknown skill name, with a NUL byte after both.
func newNulByteFile(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/standards/nul.md", "see [x](nowhere.md) and kk-nonesuch\n\x00\n")
	return f
}

func newSymlinkedFlavorViolation(t *testing.T) *fixture {
	t.Helper()
	f := newRootWithSymlinkedFlavor(t)
	f.write(f.root+"/real-flavor/standards/x.md", "see `~/.claude/skills/kk-drive/SKILL.md`\n")
	f.write(f.root+"/CLAUDE.md", "# Root\n")
	return f
}
