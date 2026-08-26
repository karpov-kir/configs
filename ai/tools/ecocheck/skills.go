package ecocheck

import (
	"io"
	"regexp"
)

var (
	frontmatterRule    = regexp.MustCompilePOSIX(`^---[[:space:]]*$`)
	descriptionField   = regexp.MustCompilePOSIX(`^description:[[:space:]]*`)
	modelInvocationOff = regexp.MustCompilePOSIX(`^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$`)
	declaredNameField  = regexp.MustCompilePOSIX(`^name: *`)
)

// Each defect here makes a skill unreachable rather than merely mis-linked: the loader finds a skill
// by its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
func (c *checker) scanSkillDirectories() {
	// A skill is its directory plus a SKILL.md; a directory without one is invisible to the loader.
	for _, entry := range c.walkTree(c.skills).entries {
		if !entry.mode.IsDir() || dirName(entry.path) != c.skills {
			continue
		}
		if !isRegularFile(join(entry.path, "SKILL.md")) {
			c.add("skill dir without SKILL.md: " + oneline(entry.path))
		}
	}
	// A skill's frontmatter name is how it is invoked; a mismatch with its directory makes it
	// unreachable.
	for _, file := range c.filesNamed(c.skills, "SKILL.md") {
		lines, _ := c.readLines(file)
		declared := declaredSkillName(lines)
		if declared != baseName(dirName(file)) {
			c.add("skill name/dir mismatch: " + oneline(file) + " declares '" + oneline(declared) + "'")
		}
		if frontmatterDescription(lines) == "" {
			c.add("skill without a description: " + oneline(file))
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

// A SKILL.md's `description:` value — the routing text, and the only part of a skill loaded in every
// session. Anchored to line 1, so a `---` rule in the body does not open frontmatter.
func frontmatterDescription(lines []string) string {
	value := ""
	scanFrontmatter(lines, func(line string) bool {
		if !descriptionField.MatchString(line) {
			return false
		}
		value = descriptionField.ReplaceAllString(line, "")
		return true
	})
	return value
}

func isOptedOutOfModelInvocation(lines []string) bool {
	return scanFrontmatter(lines, func(line string) bool {
		return modelInvocationOff.MatchString(asciiLower(line))
	})
}

// Walks the frontmatter block and stops at the first line the reader accepts, reporting whether one
// did. The block opens on line 1 and nowhere else, so a `---` rule in the body cannot start one.
func scanFrontmatter(lines []string, accept func(string) bool) bool {
	for i, line := range lines {
		if i == 0 && !frontmatterRule.MatchString(line) {
			return false
		}
		if i > 0 && frontmatterRule.MatchString(line) {
			return false
		}
		if accept(line) {
			return true
		}
	}
	return false
}

// awk's tolower under LC_ALL=C, which touches ASCII and nothing else.
func asciiLower(text string) string {
	out := []byte(text)
	for i, b := range out {
		if b >= 'A' && b <= 'Z' {
			out[i] = b + 'a' - 'A'
		}
	}
	return string(out)
}

// Every skill's description loads in every session too: the same tier, held to the same bar, and the
// only part of a skill no file in the router lists.
func (c *checker) reportDescriptionCensus(out io.Writer) {
	descriptionWords := 0
	routedSkills := 0
	skillTotal := 0
	for _, name := range c.skillDirNames() {
		file := join(join(c.skills, name), "SKILL.md")
		if !isRegularFile(file) {
			continue
		}
		skillTotal++
		lines, _ := c.readLines(file)
		if isOptedOutOfModelInvocation(lines) {
			continue
		}
		routedSkills++
		descriptionWords += len(splitFields(frontmatterDescription(lines)))
	}
	writeLinef(out, "always-loaded: %d words of skill description across %d of %d skills",
		descriptionWords, routedSkills, skillTotal)
}
