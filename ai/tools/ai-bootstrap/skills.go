package aibootstrap

import (
	"os"
	"strings"

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

func (run *invocation) declareSkills() {
	run.mounting.AddSkillMounts(run.Repo+"/kk-flavor/skills", run.skillsMount)
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
