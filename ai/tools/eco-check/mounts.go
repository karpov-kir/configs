package ecocheck

import "kk-flavor/tools/shell"

// The mounts every `~/...` citation resolves through, checked against *this checkout*. Anywhere that
// is not the install — a clone, a PR review's worktree, a CI runner with a bare $HOME — the mounts
// point at somebody else's tree or at nothing, and every finding below would be about that rather than
// about the tree under review. Hence the IsInstalled gate; ecostats gates its own mount figure on the
// same question (budget.go → mountedOutside).
//
// Past the gate, $HOME/.kk-flavor resolves to this tree by definition, so there is no flavor-mount
// comparison here: `flavor not mounted` and `flavor mounted elsewhere` would both be restating the
// gate's own condition and could never fire. What is left is the half the gate does not answer —
// whether this install's own skills are reachable at the mount.
func (c *checker) scanMounts() {
	if !c.root.IsInstalled() {
		return
	}
	skillsMount := c.root.SkillsMount()
	if !shell.IsDir(skillsMount) {
		c.add("skills not mounted: " + skillsMount + " is not a directory — no skill here is loadable and every ~/.claude/skills/ citation dangles")
		return
	}
	for _, name := range c.skillDirNames() {
		mountWant := shell.CanonicalDir(shell.Join(c.root.Skills(), name))
		mountHave := shell.CanonicalDir(shell.Join(skillsMount, name))
		switch {
		case mountHave == "":
			c.add("skill not mounted: " + shell.Join(skillsMount, shell.Oneline(name)) + " is missing — the skill exists here and cannot be invoked")
		case mountHave != mountWant:
			c.add("skill mounted elsewhere: " + shell.Join(skillsMount, shell.Oneline(name)) + " -> " + shell.Oneline(mountHave) + ", not " + shell.Oneline(mountWant))
		}
	}
}
