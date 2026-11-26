package installer

import (
	"os"

	"configs/ai/tools/shell"
)

// AddSkillMounts declares every skill directory under skillsDirectory as a bulk mount at
// mountParent, one target per skill named after its directory. Discovery, so a skill added tomorrow
// is mounted without anyone editing the installer.
func (r *Run) AddSkillMounts(skillsDirectory, mountParent string) {
	entries, err := os.ReadDir(skillsDirectory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		directory := shell.Join(skillsDirectory, entry.Name())
		if shell.IsDir(directory) {
			r.AddBulk(directory, shell.Join(mountParent, entry.Name()))
		}
	}
}
