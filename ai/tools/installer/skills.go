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
	// every session context through its `description:`, which loads whether or not the skill is ever
	// invoked, so an install that is not maintaining the tree should not carry them.
	Maintainer bool
	// Uninstalling takes the marked skills whatever Maintainer says. The tier a machine or a project
	// was installed with is nowhere on disk, so filtering on an uninstall would build a removal table
	// for the tier being asked for now rather than the one that wrote the mounts — `--maintainer` in,
	// plain out, and the marked skills stay mounted while the run reports ok. UnmountTarget removes only
	// a symlink resolving under the checkout, so widening the table cannot reach anything this checkout
	// did not write.
	Uninstalling bool
}

// SkillMounts is what one discovery pass found, for the caller to report on.
//
// Found counts every skill directory, marked or not. Finding none mounts nothing in silence, which is
// why a caller refuses on an empty table and reads Found to say which of the two ways it got there: a
// tree holding no skill, or a tier that excluded every one it had.
type SkillMounts struct {
	Found        int
	SkippedNames []string
}

// AddSkillMounts declares every skill under a directory as a bulk mount. Discovery rather than a list:
// a skill added tomorrow is mounted without anyone editing either installer.
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
		// Asked whatever the flags say: a marker nothing reads is wrong on a maintainer's machine too,
		// and the run that installs it is the last moment anyone looks at that line. Mounting continues,
		// so the tree behaves as it does today and the non-zero exit is what carries the news.
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

// A skill's frontmatter lines, or none. An unreadable SKILL.md declares nothing, which is the same
// answer as a skill that declared nothing — and is why an audience nobody can read is reported by the
// caller above rather than assumed.
func skillFrontmatter(file string) []string {
	content, err := os.ReadFile(file)
	if err != nil || len(content) > shell.MaxFileBytes {
		return nil
	}
	return shell.SplitLines(string(content))
}
