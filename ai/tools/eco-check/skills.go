package ecocheck

import (
	"io"

	"kk-flavor/tools/shell"
)

const (
	skillDirWithoutSkillFile = "skill dir without SKILL.md: "
	skillNameDirMismatch     = "skill name/dir mismatch"
	skillWithoutDescription  = "skill without a description: "
	audienceNothingReads     = "audience nothing reads"
)

// Each defect here makes a skill unreachable rather than merely mis-linked: the loader finds a skill
// by its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
func (c *checker) scanSkillDirectories() {
	for _, entry := range c.walkTree(c.root.Skills()).entries {
		if !entry.mode.IsDir() || shell.DirName(entry.path) != c.root.Skills() {
			continue
		}
		if !c.holdsRegularFile(shell.Join(entry.path, "SKILL.md")) {
			c.add(skillDirWithoutSkillFile + shell.Oneline(entry.path))
		}
	}
	for _, file := range c.filesNamed(c.root.Skills(), "SKILL.md") {
		lines, err := c.readLines(file)
		// A file nothing read declares nothing, and both findings below would state what it declares.
		// Unread, they came out as `declares ''` and `without a description` — two positive claims
		// about frontmatter that is very likely fine, aimed at the one reader who would go and check.
		// readLines has already named the file; that finding is the true one.
		if err != nil {
			continue
		}
		declared := shell.FrontmatterName(lines)
		if declared != shell.BaseName(shell.DirName(file)) {
			c.add(skillNameDirMismatch + ": " + shell.Oneline(file) + " declares '" + shell.Oneline(declared) + "'")
		}
		if shell.FrontmatterDescription(lines) == "" {
			c.add(skillWithoutDescription + shell.Oneline(file))
		}
		// Reported rather than read as an absent marker. `audience: maintainer` is the one value both
		// readers know; anything else installs the skill for everyone while the human who typed it
		// believes they marked it, and nothing on a correct-looking machine says otherwise. The
		// install refuses the same line for itself — an external machine has no copy of this check.
		if value, found := shell.UnknownAudience(lines); found {
			c.add(audienceNothingReads + ": " + shell.Oneline(file) + " declares '" + shell.Oneline(value) +
				"' — the only value is `audience: maintainer`, and this one leaves the skill installed for everyone")
		}
	}
}

// Every skill's description loads in every session too: the same tier, held to the same bar, and the
// only part of a skill no file in the router lists.
func (c *checker) reportDescriptionCensus(out io.Writer) {
	descriptionWords := 0
	routedSkills := 0
	skillTotal := 0
	for _, name := range c.skillDirNames() {
		file := c.skillFilePath(name)
		if !c.holdsRegularFile(file) {
			continue
		}
		skillTotal++
		lines, err := c.readLines(file)
		// Unread, this file looks exactly like a routed skill whose description says nothing: it
		// counted toward the figure's denominator and contributed zero words to the figure. readLines
		// has already named it at rank 1, so what is left to get right here is not counting it as a
		// description this run measured. The total above still counts it — the skill is in the tree,
		// which is what that number says; only the claim to have read its description goes.
		if err != nil || shell.IsOptedOutOfModelInvocation(lines) {
			continue
		}
		routedSkills++
		descriptionWords += len(shell.SplitFields(shell.FrontmatterDescription(lines)))
	}
	writeLinef(out, "always-loaded: %d words of skill description across %d of %d skills",
		descriptionWords, routedSkills, skillTotal)
}
