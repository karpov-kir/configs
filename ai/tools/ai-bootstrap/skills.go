package aibootstrap

import (
	"fmt"
	"os"
	"strings"

	"configs/ai/tools/installer"
	"configs/ai/tools/shell"
)

// The bucket every client shares, and the only mount here that is not a skill. An uninstall declares
// it only where this is the last client using it. A removal while another client still has skill
// mounts resolving through it would leave that client's whole set dangling.
func (run *invocation) declareBucket() {
	if run.isUninstall && run.anotherClientHoldsMounts() {
		run.mounting.Say("  kept     " + run.Home + "/.kk-flavor: another client still has skill mounts")
		return
	}
	run.mounting.AddConfig(run.Repo+"/kk-flavor", run.Home+"/.kk-flavor")
}

// Whether a mount directory other than this client's still holds a skill of this checkout's. The
// directory this run is uninstalling is skipped by identity, because CODEX_HOME is routinely an alias
// of the profile it names. A run that counted its own destination twice would keep the bucket for a
// client that is itself being removed.
func (run *invocation) anotherClientHoldsMounts() bool {
	prefix := run.Repo + "/kk-flavor/skills/"
	for _, directory := range run.allSkillMounts {
		if directory == run.skillsMount || isSameDirectory(directory, run.skillsMount) {
			continue
		}
		for _, link := range mountedLinks(directory) {
			if value, err := os.Readlink(link); err == nil && strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}

func (run *invocation) declareSkills() installer.SkillMounts {
	return run.mounting.AddSkillMounts(installer.SkillMountOptions{
		SkillsDirectory: run.Repo + "/kk-flavor/skills",
		MountParent:     run.skillsMount,
		Maintainer:      run.isMaintainer,
		Uninstalling:    run.isUninstall,
	})
}

// The tier is said out loud, after the mounts, so it reads beside them. A flag that quietly leaves
// skills out reads like a discovery loop that stopped finding them, and no line says which it was.
// The zero case is the same claim about work that never happened. A flag passed to a tree carrying
// no marked skill has to say it excluded none.
func (run *invocation) reportTier(found installer.SkillMounts) {
	if run.isMaintainer {
		return
	}
	if len(found.SkippedNames) == 0 {
		run.mounting.Say("  ok       no maintainer-only skill was there to exclude")
		return
	}
	run.mounting.Say(fmt.Sprintf("  skipped  %d maintainer-only skill(s): %s",
		len(found.SkippedNames), strings.Join(found.SkippedNames, " ")))
}

// Two ways to mount zero skills, and they send a reader to different places. An empty skills
// directory is a broken checkout. A flag that excluded every skill it found is a flag doing exactly
// what it says on a tree that holds only maintainer skills. The exit code is the same for both, so
// the wording is what tells them apart.
func (run *invocation) emptyTableRefusal(found installer.SkillMounts) string {
	if len(run.mounting.BulkMounts()) > 0 {
		return ""
	}
	directory := run.Repo + "/kk-flavor/skills/"
	if found.Found > 0 {
		return fmt.Sprintf("every skill under %s is maintainer-only, and a run without --maintainer "+
			"excluded all %d — nothing was mounted", directory, found.Found)
	}
	return "no skill directories under " + directory + " — nothing was mounted"
}

// Codex kept its skills under CODEX_HOME before the shared discovery directory existed, and machines
// set up then are still holding those links. Each is dropped once its replacement is in place and
// resolves to the same skill, so an interrupted migration leaves the client's skills reachable.
// UnmountTarget does the removing, so a link this checkout cannot prove it wrote is reported and left.
func (run *invocation) migrateLegacyCodexMounts() {
	if run.agent != codexAgent || isSameDirectory(run.legacyCodexSkills, run.skillsMount) {
		return
	}
	prefix := run.Repo + "/kk-flavor/skills/"
	for _, mounted := range mountedLinks(run.legacyCodexSkills) {
		value, err := os.Readlink(mounted)
		if err != nil || !strings.HasPrefix(value, prefix) {
			continue
		}
		replacement := run.skillsMount + "/" + shell.BaseName(mounted)
		// A symlink, and one resolving to the same skill. A real directory somebody put at the
		// replacement's path is no mount of this run's. A drop of the old link against it would leave
		// Codex with a directory that holds no skill.
		if shell.IsSymlink(replacement) && isSameDirectory(replacement, mounted) {
			run.mounting.UnmountTarget(mounted)
		}
	}
}
