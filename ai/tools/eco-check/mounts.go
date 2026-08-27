package ecocheck

import "kk-flavor/tools/shell"

// The mounts every `~/...` citation is resolved through. Those citations are checked against *this
// checkout*, so a checkout that is not the installed one would report every one of them healthy.
func (c *checker) scanMounts() {
	flavorWant := shell.CanonicalDir(c.root.Flavor())
	flavorHave := shell.CanonicalDir(c.root.FlavorMount())
	switch {
	case flavorHave == "":
		c.add("flavor not mounted: $HOME/.kk-flavor is not a directory — every ~/.kk-flavor/ citation dangles at run time")
	case flavorHave != flavorWant:
		c.add("flavor mounted elsewhere: $HOME/.kk-flavor -> " + shell.Oneline(flavorHave) + ", not " + shell.Oneline(flavorWant))
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
