package installer_test

// Which skills an install mounts: every skill directory the tree holds. Discovery, so a skill added
// tomorrow is mounted without anyone editing an installer.

import (
	"testing"

	"configs/ai/tools/installer"
)

const markedSkill = "---\nname: kk-ecosystem\ndescription: maintains this tree\naudience: maintainer\n---\n"

// A tree holding one plain skill, one marked for maintainers, and a stray file beside them.
func newSkillTree(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.newSkill("kk-build")
	f.MkdirAll(f.repo + "/skills/kk-ecosystem")
	f.Write(f.repo+"/skills/kk-ecosystem/SKILL.md", markedSkill)
	f.Write(f.repo+"/skills/README.md", "not a skill\n")
	return f
}

func (f *fixture) skillRun() *installer.Run {
	f.t.Helper()
	run := f.newRun(installer.RunOptions{BulkLabel: "skills"})
	run.AddSkillMounts(f.repo+"/skills", f.skillsMount())
	return run
}

// The `audience: maintainer` marker no longer decides a mount. The marked skill is the control: a
// filter left behind would drop it and still mount the plain one.
func TestEverySkillDirectoryIsMountedAndNothingElse(t *testing.T) {
	t.Parallel()
	f := newSkillTree(t)
	run := f.skillRun()
	run.Mount()
	f.ExpectNoBreach(run.Breaches())

	f.expectRefusals(run, 0)
	f.ExpectLinkTo(f.skillsMount()+"/kk-build", f.repo+"/skills/kk-build")
	f.ExpectLinkTo(f.skillsMount()+"/kk-ecosystem", f.repo+"/skills/kk-ecosystem")
	f.ExpectAbsent(f.skillsMount() + "/README.md")
}

func TestAnUninstallTakesEverySkillItMounted(t *testing.T) {
	t.Parallel()
	f := newSkillTree(t)
	f.skillRun().Mount()

	run := f.skillRun()
	run.Unmount()
	f.ExpectNoBreach(run.Breaches())

	f.ExpectAbsent(f.skillsMount() + "/kk-ecosystem")
	f.ExpectAbsent(f.skillsMount() + "/kk-build")
}

// A missing skills directory declares nothing, which the caller reports as an empty table.
func TestAMissingSkillsDirectoryDeclaresNoMount(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	run := f.newRun(installer.RunOptions{BulkLabel: "skills"})

	run.AddSkillMounts(f.repo+"/nowhere", f.skillsMount())

	if mounts := run.BulkMounts(); len(mounts) != 0 {
		t.Errorf("a missing skills directory declared %d mount(s), wanted none", len(mounts))
	}
}
