package ecocheck

import (
	"io"
	"regexp"

	"kk-flavor/tools/shell"
)

var declaredNameField = regexp.MustCompilePOSIX(`^name: *`)

// Each defect here makes a skill unreachable rather than merely mis-linked: the loader finds a skill
// by its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
func (c *checker) scanSkillDirectories() {
	// A skill is its directory plus a SKILL.md; a directory without one is invisible to the loader.
	for _, entry := range c.walkTree(c.skills).entries {
		if !entry.mode.IsDir() || shell.DirName(entry.path) != c.skills {
			continue
		}
		if !shell.IsRegularFile(shell.Join(entry.path, "SKILL.md")) {
			c.add("skill dir without SKILL.md: " + shell.Oneline(entry.path))
		}
	}
	// A skill's frontmatter name is how it is invoked; a mismatch with its directory makes it
	// unreachable.
	for _, file := range c.filesNamed(c.skills, "SKILL.md") {
		lines, _ := c.readLines(file)
		declared := declaredSkillName(lines)
		if declared != shell.BaseName(shell.DirName(file)) {
			c.add("skill name/dir mismatch: " + shell.Oneline(file) + " declares '" + shell.Oneline(declared) + "'")
		}
		if shell.FrontmatterDescription(lines) == "" {
			c.add("skill without a description: " + shell.Oneline(file))
		}
	}
}

// Read from lines 2 to 10, where a skill's frontmatter puts it.
func declaredSkillName(lines []string) string {
	for i := 1; i < len(lines) && i < 10; i++ {
		if declaredNameField.MatchString(lines[i]) {
			return declaredNameField.ReplaceAllString(lines[i], "")
		}
	}
	return ""
}

// Every skill's description loads in every session too: the same tier, held to the same bar, and the
// only part of a skill no file in the router lists.
func (c *checker) reportDescriptionCensus(out io.Writer) {
	descriptionWords := 0
	routedSkills := 0
	skillTotal := 0
	for _, name := range c.skillDirNames() {
		file := shell.Join(shell.Join(c.skills, name), "SKILL.md")
		if !shell.IsRegularFile(file) {
			continue
		}
		skillTotal++
		lines, _ := c.readLines(file)
		if shell.IsOptedOutOfModelInvocation(lines) {
			continue
		}
		routedSkills++
		descriptionWords += len(shell.SplitFields(shell.FrontmatterDescription(lines)))
	}
	writeLinef(out, "always-loaded: %d words of skill description across %d of %d skills",
		descriptionWords, routedSkills, skillTotal)
}
