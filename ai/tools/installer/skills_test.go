package installer_test

// Which skills an install mounts, and the question that decides it. Each skill's own frontmatter
// says who it is for.

// Discovery, so a skill added tomorrow is mounted without anyone editing an installer. That is also
// why every case here counts what the fixture holds, and never a number written down.

import (
	"strings"
	"testing"

	"configs/ai/tools/installer"
)

const markedSkill = "---\nname: kk-ecosystem\ndescription: maintains this tree\naudience: maintainer\n---\n"

// A tree holding one public skill and one marked for maintainers. Both sets are needed in every
// case, because a tier that excluded every skill and a tier that excluded none each pass half.
func newSkillTree(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.newSkill("kk-build")
	f.MkdirAll(f.repo + "/skills/kk-ecosystem")
	f.Write(f.repo+"/skills/kk-ecosystem/SKILL.md", markedSkill)
	return f
}

func (f *fixture) discoverSkills(options installer.RunOptions, skills installer.SkillMountOptions) (*installer.Run, installer.SkillMounts) {
	f.t.Helper()
	options.BulkLabel = "skills"
	run := f.newRun(options)
	skills.SkillsDirectory = f.repo + "/skills"
	skills.MountParent = f.skillsMount()
	found := run.AddSkillMounts(skills)
	run.Mount()
	f.ExpectNoBreach(run.Breaches())
	return run, found
}

// Each maintainer-only skill costs every session context through its `description:`, which loads
// even for a skill that is never invoked. An install that maintains no tree should leave them out.
func TestAMaintainerOnlySkillIsLeftOutUnlessItIsAskedFor(t *testing.T) {
	t.Parallel()
	t.Run("a default run mounts the public skill and skips the marked one", func(t *testing.T) {
		f := newSkillTree(t)

		run, found := f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{})

		f.expectRefusals(run, 0)
		f.ExpectLinkTo(f.skillsMount()+"/kk-build", f.repo+"/skills/kk-build")
		f.ExpectAbsent(f.skillsMount() + "/kk-ecosystem")
		if found.Found != 2 || len(found.SkippedNames) != 1 {
			t.Errorf("discovery found %d skill(s) and skipped %v, wanted 2 and one marked name",
				found.Found, found.SkippedNames)
		}
	})

	// The opt-in, and the half that proves the flag still reaches them: a default that excluded
	// everything could not pass both cases.
	t.Run("and --maintainer mounts it like any other", func(t *testing.T) {
		f := newSkillTree(t)

		_, found := f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{Maintainer: true})

		f.ExpectLinkTo(f.skillsMount()+"/kk-ecosystem", f.repo+"/skills/kk-ecosystem")
		if len(found.SkippedNames) != 0 {
			t.Errorf("--maintainer skipped %v, wanted nothing", found.SkippedNames)
		}
	})

	// The tier a machine was installed with is written down nowhere. An uninstall that re-applied the
	// filter would build its removal table for the tier being asked for NOW. Pass `--maintainer` in
	// and plain out, and exactly the marked skills stay mounted while the run reports ok.
	t.Run("and an uninstall takes them whatever the flags say", func(t *testing.T) {
		f := newSkillTree(t)
		f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{Maintainer: true})

		run := f.newRun(installer.RunOptions{BulkLabel: "skills"})
		run.AddSkillMounts(installer.SkillMountOptions{
			SkillsDirectory: f.repo + "/skills",
			MountParent:     f.skillsMount(),
			Uninstalling:    true,
		})
		run.Unmount()
		f.ExpectNoBreach(run.Breaches())

		f.ExpectAbsent(f.skillsMount() + "/kk-ecosystem")
		f.ExpectAbsent(f.skillsMount() + "/kk-build")
	})
}

// The marker check answers "not marked" to a misspelling and to a skill with an empty declaration,
// and those two mean opposite things. The misspelling installs for everyone, whoever typed it
// believes they marked it, and the machine looks healthy throughout.
func TestAnAudienceNoReaderKnowsIsReportedAndStillMounted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.MkdirAll(f.repo + "/skills/kk-typo")
	f.Write(f.repo+"/skills/kk-typo/SKILL.md",
		"---\nname: kk-typo\ndescription: a skill\naudience: maintainr\n---\n")

	run, found := f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{})

	f.expectRefusals(run, 1)
	f.ExpectSaid("maintainr")
	f.ExpectSaid("audience: maintainer")
	// The mount still happens, so the tree behaves as it does today and the non-zero exit carries the
	// news.
	f.ExpectLinkTo(f.skillsMount()+"/kk-typo", f.repo+"/skills/kk-typo")
	if found.Found != 1 {
		t.Errorf("discovery found %d skill(s), wanted 1", found.Found)
	}
}

// The audience value is echoed back to whoever typed it, so it is text a branch chose reaching the
// terminal. It is file CONTENT, where a skill's directory name is a filename. That is the single
// route a raw 0x9b can take on a filesystem that refuses an invalid UTF-8 filename.

// 0x9b is CSI as a single byte, which a terminal in 8-bit mode acts on. It is the byte the shell
// could never reach. Bash leaves it, and catching it needs the decoding a `[[:cntrl:]]` substitution
// cannot do.
func TestARawCsiInAnAudienceValueNeverReachesTheTerminal(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.MkdirAll(f.repo + "/skills/kk-typo")
	const csi = "\x9b"
	f.Write(f.repo+"/skills/kk-typo/SKILL.md",
		"---\nname: kk-typo\ndescription: a skill\naudience: maintainr"+csi+"[2K\n---\n")

	run, _ := f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{})

	// The control. The assertion that follows is otherwise equally satisfied by a run that never read
	// the declaration at all.
	f.expectRefusals(run, 1)
	f.ExpectSaid("maintainr")
	if strings.Contains(f.Said(), csi) {
		t.Errorf("a raw CSI out of the declared audience reached the terminal:\n%q", f.Said())
	}
}

// A pass that finds none writes no mount and stays silent about it. A caller refuses on an empty
// table and reads Found to tell the two causes apart.

// The exit is the same either way, and the wording alone separates a tree holding no skill from a
// tier that excluded all it had. A false diagnosis sends the reader looking for files that are all
// there.
func TestDiscoveryTellsAnEmptyTreeFromATierThatExcludedEverything(t *testing.T) {
	t.Parallel()
	t.Run("a tree holding no skill finds none", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.repo + "/skills")

		_, found := f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{})

		if found.Found != 0 || len(found.SkippedNames) != 0 {
			t.Errorf("an empty tree found %d skill(s) and skipped %v, wanted none of either",
				found.Found, found.SkippedNames)
		}
	})

	t.Run("while a tree whose every skill is marked finds them and skips them", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.repo + "/skills/kk-ecosystem")
		f.Write(f.repo+"/skills/kk-ecosystem/SKILL.md", markedSkill)

		_, found := f.discoverSkills(installer.RunOptions{}, installer.SkillMountOptions{})

		if found.Found != 1 || len(found.SkippedNames) != 1 {
			t.Errorf("an all-marked tree found %d skill(s) and skipped %v, wanted 1 and one name",
				found.Found, found.SkippedNames)
		}
	})
}
