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
