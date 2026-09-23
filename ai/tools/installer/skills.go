package installer

import (
	"os"

	"configs/ai/tools/shell"
)

// SkillMountOptions is one discovery pass over a skills directory.
type SkillMountOptions struct {
	// SkillsDirectory holds one directory per skill.
	SkillsDirectory string
	// MountParent is where each skill is mounted, one target per skill named after its directory.
	MountParent string
	// Maintainer asks for the skills that exist only to maintain this instruction tree. Each costs
	// every session context through its `description:`, which loads even for a skill that is never
	// invoked. An install that maintains no tree should leave them out.
	Maintainer bool

	// The tier a machine or a project was installed with is nowhere on disk. A filter on an uninstall
	// builds a removal table for the tier being asked for now, and the tier that wrote the mounts goes
	// unconsulted. Pass `--maintainer` on the way in and plain on the way out, and the marked skills
	// stay mounted while the run reports ok.

	// UnmountTarget removes only a symlink resolving under the checkout, so a wider table still reaches
	// only what this checkout wrote.

	// Uninstalling: the marked skills come out whatever Maintainer says.
	Uninstalling bool
}

// Found counts every skill directory, marked or plain. A pass that finds none writes no mount and
// stays silent about it. A caller refuses on an empty table and reads Found to tell the two causes
// apart: a tree holding no skill, or a tier that excluded all it had.

// SkillMounts is what one discovery pass found, for the caller to report on.
type SkillMounts struct {
	Found        int
	SkippedNames []string
}

// AddSkillMounts declares every skill under a directory as a bulk mount. Discovery, so a skill added
// tomorrow is mounted without anyone editing either installer.
func (r *Run) AddSkillMounts(options SkillMountOptions) SkillMounts {
	takeMaintainerOnly := options.Maintainer || options.Uninstalling
	found := SkillMounts{}
	entries, err := os.ReadDir(options.SkillsDirectory)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		directory := shell.Join(options.SkillsDirectory, entry.Name())
		if !shell.IsDir(directory) {
			continue
		}
		found.Found++
		lines := skillFrontmatter(shell.Join(directory, "SKILL.md"))
		// The question is asked whatever the flags say. A marker no reader knows is wrong on a
		// maintainer's machine too, and the run that installs it is the last moment anyone looks at that
		// line. The mount still happens, so the tree behaves as it does today and the non-zero exit
		// carries the news.
		if value, unknown := shell.UnknownAudience(lines); unknown {
			r.Refuse(entry.Name() + " declares 'audience: " + value + "' in " + directory +
				"/SKILL.md, which no reader knows — the only value is 'audience: maintainer', " +
				"and as written the skill installs for everyone")
		}
		if !takeMaintainerOnly && shell.IsMaintainerAudience(lines) {
			found.SkippedNames = append(found.SkippedNames, entry.Name())
			continue
		}
		r.AddBulk(directory, shell.Join(options.MountParent, entry.Name()))
	}
	return found
}

// An unreadable SKILL.md gives the same empty answer as a skill that declared no audience.
// AddSkillMounts therefore reports an audience it cannot read instead of assuming one.

// Returns a skill's frontmatter lines, or none.
func skillFrontmatter(file string) []string {
	content, err := os.ReadFile(file)
	if err != nil || len(content) > shell.MaxFileBytes {
		return nil
	}
	return shell.SplitLines(string(content))
}
