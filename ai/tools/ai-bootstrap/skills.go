package aibootstrap

import (
	"os"
	"strings"
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

// Whether a distinct mount directory still holds a skill from this checkout.
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
