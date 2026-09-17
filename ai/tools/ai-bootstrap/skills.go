package aibootstrap

import (
	"fmt"
	"os"
	"strings"

	"configs/ai/tools/installer"
	"configs/ai/tools/shell"
)

// The bucket every client shares, and the one mount that is not a skill. On an uninstall it is
// declared only when this is the last client using it: removing it while another client still has
// skill mounts resolving through it would leave that client's whole set dangling.
func (run *invocation) declareBucket() {
	if run.isUninstall && run.anotherClientHoldsMounts() {
		run.mounting.Say("  kept     " + run.Home + "/.kk-flavor: another client still has skill mounts")
		return
	}
	run.mounting.AddConfig(run.Repo+"/kk-flavor", run.Home+"/.kk-flavor")
}

// Whether a mount directory other than this client's still holds a skill of this checkout's. The
// directory this run is uninstalling is passed over by identity rather than by name: CODEX_HOME is
// routinely an alias of the profile it names, and a run that counted its own destination twice would
// keep the bucket for a client that is itself being removed.
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

// Said out loud, and after the mounts so it reads beside them. A flag that quietly leaves skills out
// is indistinguishable from a discovery loop that stopped finding them: the machine ends up short of
// skills with nothing in the run saying why. The zero case is the same claim about work that did not
// happen — a flag passed to a tree carrying no marked skill has to say it excluded nothing.
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

// Two ways to mount no skill, and they send a reader to different places: a skills directory with
// nothing in it is a broken checkout, while a flag that excluded every skill it found is a flag doing
// exactly what it says on a tree that has nothing else. The exit code is the same for both, so the
// wording is the only thing telling them apart.
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
// set up then are still holding those links. Each is dropped only once its replacement is in place and
// resolves to the same skill, so an interrupted migration leaves the old mount working rather than the
// client with no skill at all.
//
// UnmountTarget is what does the removing, so a link this checkout cannot prove it wrote is reported
// and left — the same promise the rest of the run makes.
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
		// replacement's path is not this run's mount, and dropping the old link against it would leave
		// Codex with a directory nobody wrote and no skill behind it.
		if shell.IsSymlink(replacement) && isSameDirectory(replacement, mounted) {
			run.mounting.UnmountTarget(mounted)
		}
	}
}
