package ecocheck_test

import (
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

func TestSkillDirectory(t *testing.T) {
	t.Run("fires on a skill directory holding no SKILL.md", func(t *testing.T) {
		newBrokenSkillDirs(t).reports(ecocheck.SkillDirWithoutSkillFile)
	})

	t.Run("fires when the frontmatter name is not the directory name", func(t *testing.T) {
		newBrokenSkillDirs(t).reports(ecocheck.SkillNameDirMismatch)
	})

	t.Run("fires on a SKILL.md carrying no description", func(t *testing.T) {
		newBrokenSkillDirs(t).reports(ecocheck.SkillWithoutDescription)
	})

	// A `name:` line in the body is not a declaration. The loader reads the frontmatter block, so a
	// reader with a looser idea of where that block is calls a skill reachable while the loader cannot
	// invoke it at all.
	t.Run("does not read a name: line in the body as the declaration", func(t *testing.T) {
		f := newRoot(t)
		f.mkdirAll(f.root + "/kk-flavor/skills/ghost")
		f.write(f.root+"/kk-flavor/skills/ghost/SKILL.md", "# Ghost\n\nname: ghost\n\ndescription: does a thing\n")
		f.reports(ecocheck.SkillNameDirMismatch)
	})
}

func TestAnUnreadableSkillFileIsNotReportedAsDeclaringNothing(t *testing.T) {
	// The mounted skill's SKILL.md carries no frontmatter at all, so readable it raises both findings.
	// That is what makes the two silences below a refusal rather than a compliant fixture.
	newUnreadableSkill := func(t *testing.T) *fixture {
		skipUnlessModeDeniesRead(t, "an unreadable SKILL.md cannot be built here")
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.chmod(f.root+"/kk-flavor/skills/kk-drive/SKILL.md", 0o000)
		return f
	}

	t.Run("names the file it could not read (control for the two below)", func(t *testing.T) {
		newUnreadableSkill(t).reports(ecocheck.FileCouldNotBeRead)
	})

	t.Run("does not claim it declares an empty name", func(t *testing.T) {
		newUnreadableSkill(t).doesNotReport(ecocheck.SkillNameDirMismatch)
	})

	t.Run("nor that it carries no description", func(t *testing.T) {
		newUnreadableSkill(t).doesNotReport(ecocheck.SkillWithoutDescription)
	})
}

// Unread and "routed, description empty" are the same line, which is the zero this pass exists to
// separate from the other one.
//
// The wording does not move — ecostats reports the same tree and leaves its own `of N skills` alone,
// carrying the unread fact beside the report instead. Here that fact is already a rank-1 finding, so
// what is fixed is only the claim to have measured something.
func TestAnUnreadableSkillFileIsNotCountedAsADescriptionThatWasRead(t *testing.T) {
	newTwoSkills := func(t *testing.T) *fixture {
		f := newRoot(t)
		for _, name := range []string{"kk-alpha", "kk-beta"} {
			f.mkdirAll(f.root + "/kk-flavor/skills/" + name)
			f.write(f.root+"/kk-flavor/skills/"+name+"/SKILL.md",
				"---\nname: "+name+"\ndescription: does a thing for the tree\n---\n")
		}
		return f
	}

	// The control. Both skills are read and both are counted, so the drop below is one file leaving
	// the figure rather than a fixture that never had two.
	t.Run("counts both skills while both can be read", func(t *testing.T) {
		newTwoSkills(t).reports("across 2 of 2 skills")
	})

	t.Run("drops the one it could not read out of what it claims to have measured", func(t *testing.T) {
		skipUnlessModeDeniesRead(t, "an unreadable SKILL.md cannot be built here")
		f := newTwoSkills(t)
		f.chmod(f.root+"/kk-flavor/skills/kk-beta/SKILL.md", 0o000)
		// The total still counts it: the skill is in the tree, which is what that number says.
		f.reports("across 1 of 2 skills")
	})
}

// The defect observed on 2026-09-16: an IDSD ship reached its landing stage twice and stopped both
// times, because Claude Code refuses a `disable-model-invocation` skill to every model caller and
// tells the caller not to reach the workflow by other means either. The stage's caller is a skill, so
// there was nobody to type the slash command it waited for.
func TestAStageItsOrchestratorCannotInvoke(t *testing.T) {
	// `marked` is the only variable: the extending caller and the stage are the same two files in every
	// case, so a silence below is the marker leaving and not a tree that lost its edge.
	newPipeline := func(t *testing.T, marked bool) *fixture {
		t.Helper()
		f := newRoot(t)
		f.mkdirAll(f.root + "/kk-flavor/skills/kk-ship")
		f.mkdirAll(f.root + "/kk-flavor/skills/kk-land")
		f.write(f.root+"/kk-flavor/skills/kk-ship/SKILL.md",
			"---\nname: kk-ship\ndescription: runs the pipeline end to end\n---\n\n"+
				"**Extends:** kk-land — the landing step, once the gate is clean\n")
		frontmatter := "---\nname: kk-land\ndescription: lands the change\n"
		if marked {
			frontmatter += "disable-model-invocation: true\n"
		}
		f.write(f.root+"/kk-flavor/skills/kk-land/SKILL.md", frontmatter+"---\n")
		return f
	}

	t.Run("fires on a marked skill another skill extends", func(t *testing.T) {
		newPipeline(t, true).reports(ecocheck.StageNothingCanInvoke)
	})

	// The control for the case above: same two files, same edge, marker gone. Without it the finding
	// could be coming from the `**Extends:**` line alone, and every stage in the tree would raise it.
	t.Run("stays silent once the marker is gone", func(t *testing.T) {
		newPipeline(t, false).doesNotReport(ecocheck.StageNothingCanInvoke)
	})

	// The marker's legitimate use, which this check must not take away: a skill the human always
	// initiates, that no other skill extends.
	t.Run("leaves a marked skill nothing extends alone", func(t *testing.T) {
		f := newRoot(t)
		f.mkdirAll(f.root + "/kk-flavor/skills/kk-retro")
		f.write(f.root+"/kk-flavor/skills/kk-retro/SKILL.md",
			"---\nname: kk-retro\ndescription: the human asks for this one by name\ndisable-model-invocation: true\n---\n")
		f.doesNotReport(ecocheck.StageNothingCanInvoke)
	})
}

func newBrokenSkillDirs(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.mkdirAll(f.root + "/kk-flavor/skills/orphan")
	f.mkdirAll(f.root + "/kk-flavor/skills/wrong-name")
	f.mkdirAll(f.root + "/kk-flavor/skills/no-desc")
	f.write(f.root+"/kk-flavor/skills/wrong-name/SKILL.md", "---\nname: misnamed\ndescription: does a thing\n---\n")
	f.write(f.root+"/kk-flavor/skills/no-desc/SKILL.md", "---\nname: no-desc\n---\n")
	return f
}
