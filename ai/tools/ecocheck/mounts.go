package ecocheck

// The mounts every `~/...` citation is resolved through. Those citations are checked against *this
// checkout*, so a checkout that is not the installed one would report every one of them healthy.
func (c *checker) scanMounts() {
	flavorWant := canonicalDir(c.flavor)
	flavorHave := canonicalDir(join(c.home, ".kk-flavor"))
	switch {
	case flavorHave == "":
		c.add("flavor not mounted: $HOME/.kk-flavor is not a directory — every ~/.kk-flavor/ citation dangles at run time")
	case flavorHave != flavorWant:
		c.add("flavor mounted elsewhere: $HOME/.kk-flavor -> " + oneline(flavorHave) + ", not " + oneline(flavorWant))
	}

	skillsMount := join(c.home, ".claude/skills")
	if !isDir(skillsMount) {
		c.add("skills not mounted: " + skillsMount + " is not a directory — no skill here is loadable and every ~/.claude/skills/ citation dangles")
		return
	}
	for _, name := range c.skillDirNames() {
		mountWant := canonicalDir(join(c.skills, name))
		mountHave := canonicalDir(join(skillsMount, name))
		switch {
		case mountHave == "":
			c.add("skill not mounted: " + join(skillsMount, oneline(name)) + " is missing — the skill exists here and cannot be invoked")
		case mountHave != mountWant:
			c.add("skill mounted elsewhere: " + join(skillsMount, oneline(name)) + " -> " + oneline(mountHave) + ", not " + oneline(mountWant))
		}
	}
}
