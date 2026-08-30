package ecocheck_test

// The skill directory itself.

import (
	"testing"
)

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

	// A `name:` line in the body is not a declaration. The loader reads the frontmatter block, so a
	// reader with a looser idea of where that block is calls a skill reachable while the loader cannot
	// invoke it at all.
	t.Run("does not read a name: line in the body as the declaration", func(t *testing.T) {
		f := newRoot(t)
		f.mkdirAll(f.root + "/skills/ghost")
		f.write(f.root+"/skills/ghost/SKILL.md", "# Ghost\n\nname: ghost\n\ndescription: does a thing\n")
		f.reports("skill name/dir mismatch")
	})
}

// A SKILL.md the open refuses declares nothing, so both findings above would be stating what it
// declares. Unread, it was reported as declaring an empty name and as carrying no description — two
// positive claims about frontmatter that is almost certainly fine, aimed at the one reader who would
// go and check it.
func TestAnUnreadableSkillFileIsNotReportedAsDeclaringNothing(t *testing.T) {
	// The mounted skill's SKILL.md carries no frontmatter at all, so readable it raises both findings.
	// That is what makes the two silences below a refusal rather than a compliant fixture.
	newUnreadableSkill := func(t *testing.T) *fixture {
		skipUnlessModeDeniesRead(t, "an unreadable SKILL.md cannot be built here")
		f := newRoot(t)
		f.newMountedSkill("kk-drive")
		f.chmod(f.root+"/skills/kk-drive/SKILL.md", 0o000)
		return f
	}

	// The control: the run does say the file went unread, so the silences below are a refusal and not
	// a fixture that never reached the scan.
	t.Run("names the file it could not read (control for the two below)", func(t *testing.T) {
		newUnreadableSkill(t).reports("file could not be read")
	})

	t.Run("does not claim it declares an empty name", func(t *testing.T) {
		newUnreadableSkill(t).doesNotReport("skill name/dir mismatch")
	})

	t.Run("nor that it carries no description", func(t *testing.T) {
		newUnreadableSkill(t).doesNotReport("skill without a description")
	})
}

// The census counted a SKILL.md it could not read as one it had read: the file entered the
// denominator and contributed zero words, so `19 of 22` stayed put while the figure beside it went 34
// words short. Unread and "routed, description empty" are the same line, which is the zero this pass
// exists to separate from the other one.
//
// The wording does not move — ecostats reports the same tree and leaves its own `of N skills` alone,
// carrying the unread fact beside the report instead. Here that fact is already a rank-1 finding, so
// what is fixed is only the claim to have measured something.
func TestAnUnreadableSkillFileIsNotCountedAsADescriptionThatWasRead(t *testing.T) {
	newTwoSkills := func(t *testing.T) *fixture {
		f := newRoot(t)
		for _, name := range []string{"kk-alpha", "kk-beta"} {
			f.mkdirAll(f.root + "/skills/" + name)
			f.write(f.root+"/skills/"+name+"/SKILL.md",
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
		f.chmod(f.root+"/skills/kk-beta/SKILL.md", 0o000)
		// The total still counts it: the skill is in the tree, which is what that number says.
		f.reports("across 1 of 2 skills")
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
