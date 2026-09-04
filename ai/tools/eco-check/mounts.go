package ecocheck

import (
	"io"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// The mounts every `~/...` citation resolves through, checked against *this checkout*. Anywhere that
// is not the install — a clone, a PR review's worktree, a CI runner with a bare $HOME — the mounts
// point at somebody else's tree or at nothing, and every finding below would be about that rather than
// about the tree under review. Hence the IsInstalled gate; ecostats gates its own mount figure on the
// same question (budget.go → mountedOutside).
//
// The gate holds for the reverse half below as well. A worktree's mounts point into the main checkout,
// so none of them is under the worktree's own root. Set those aside and a scan run there has nothing
// left to select.
//
// Past the gate, $HOME/.kk-flavor resolves to this tree by definition, so there is no flavor-mount
// comparison here: `flavor not mounted` and `flavor mounted elsewhere` would both be restating the
// gate's own condition and could never fire. What is left is the half the gate does not answer —
// whether this install's own skills are reachable at the mount, asked in both directions, because
// neither direction can see the defect the other one is about.
func (c *checker) scanMounts() {
	if !c.root.IsInstalled() {
		return
	}
	skillsMount := c.root.SkillsMount()
	if !shell.IsDir(skillsMount) {
		c.add("skills not mounted: " + skillsMount + " is not a directory — no skill here is loadable and every ~/.claude/skills/ citation dangles")
		return
	}
	skillNames := c.skillDirNames()
	for _, name := range skillNames {
		mountWant := shell.CanonicalDir(shell.Join(c.root.Skills(), name))
		mountHave := shell.CanonicalDir(shell.Join(skillsMount, name))
		switch {
		case mountHave == "":
			c.add("skill not mounted: " + shell.Join(skillsMount, shell.Oneline(name)) + " is missing — the skill exists here and cannot be invoked")
		case mountHave != mountWant:
			c.add("skill mounted elsewhere: " + shell.Join(skillsMount, shell.Oneline(name)) + " -> " + shell.Oneline(mountHave) + ", not " + shell.Oneline(mountWant))
		}
	}
	c.scanMountsWithoutSkills(skillsMount, skillNames)
}

// A run that checked no mount prints byte for byte what a run that checked every one of them prints,
// and check.sh puts `wiring: clean` over both. Its own header refuses exactly that:
// a check that did not run is not a clean one. Absence of this line says the scan did run.
func (c *checker) reportSkippedMountScan(out io.Writer) {
	if c.root.IsInstalled() {
		return
	}
	writeLinef(out, "mounts: skipped — this checkout is not the install, so nothing here was checked about ~/.claude/skills/ in either direction")
}

// The mounts that outlived their skills. The forward loop iterates the directories this tree has, so a
// mount whose directory was deleted has nothing left to iterate from and the forward loop never
// reaches it. ai/bootstrap.sh mounts every directory under skills/ and unmounts none, so a deletion that
// lands leaves its symlink behind at $HOME for good.
//
// Only a mount pointing straight into this tree's own skills/ is spoken about. $HOME legitimately
// carries skills from other checkouts, and one of those — resolving or dangling — is not this tree's
// to report.
//
// A skill deleted on a branch is not a defect, and it stays quiet on the path the work actually takes:
// that branch is checked out in a worktree, where the mount still resolves through the main checkout,
// which still holds the directory. Checked out in the install itself it does report. The forward half
// has the same window the other way round: a branch that adds a skill reads as one not mounted until
// ai/bootstrap.sh runs again.
//
// It takes the forward loop's own list rather than reading skills/ a second time, so the two halves
// cannot disagree about which skills this tree has.
func (c *checker) scanMountsWithoutSkills(skillsMount string, skillNames []string) {
	entries, err := os.ReadDir(skillsMount)
	if err != nil {
		return
	}
	skillDirs := map[string]bool{}
	for _, name := range skillNames {
		skillDirs[name] = true
	}
	skillsHere := shell.CanonicalDir(c.root.Skills())
	for _, entry := range entries {
		name := entry.Name()
		mountPath := shell.Join(skillsMount, name)
		target, isSymlink := mountTarget(mountPath)
		if !isSymlink {
			continue
		}
		// Empty is a target whose own parent is gone — a mount into a checkout that was deleted. It
		// dangles just like the mount this scan is for, and is still not this tree's to report.
		mountedInto := shell.CanonicalDir(shell.DirName(target))
		if mountedInto == "" || mountedInto != skillsHere {
			continue
		}
		// A name this tree still has is the forward loop's to report, and it has already said whether
		// that mount is missing or points elsewhere. Reported here too, one broken mount would be two
		// findings.
		if skillDirs[name] {
			continue
		}
		// A mount still resolving to a directory has something to load, whatever name it carries, so it
		// is not this finding.
		if shell.IsDir(mountPath) {
			continue
		}
		c.add("mount without a skill: " + shell.Join(skillsMount, shell.Oneline(name)) + " -> " + shell.Oneline(target) +
			", which this checkout does not have — remove it once the deletion has landed; ai/bootstrap.sh drops no mount of its own")
	}
}

// Where a mount points, resolved the way the kernel resolves it. False means the path is not a symlink
// at all, which is a directory somebody put under the mount by hand rather than a mount.
func mountTarget(mountPath string) (string, bool) {
	target, err := os.Readlink(mountPath)
	if err != nil {
		return "", false
	}
	// A relative target resolves against the directory holding the link; CanonicalDir would otherwise
	// hand it to filepath.Abs and resolve it against this process's working directory. Left that way
	// the same mount reads as this tree's when the tool is run from the root and as nobody's when it
	// is run from anywhere else.
	if !strings.HasPrefix(target, "/") {
		target = shell.Join(shell.DirName(mountPath), target)
	}
	return target, true
}
