package shell

import "regexp"

var (
	frontmatterRule    = regexp.MustCompilePOSIX(`^---[[:space:]]*$`)
	descriptionField   = regexp.MustCompilePOSIX(`^description:[[:space:]]*`)
	modelInvocationOff = regexp.MustCompilePOSIX(`^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$`)
)

// --- shared:frontmatter-description ---
// FrontmatterDescription is a SKILL.md's `description:` value — the routing text, and the only part
// of a skill loaded in every session. Anchored to line 1, so a `---` rule in the body does not open
// frontmatter.
func FrontmatterDescription(lines []string) string {
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

// --- end shared:frontmatter-description ---

// --- shared:opted-out-of-model-invocation ---
// True when a skill's frontmatter takes it out of the router, which is what makes its description
// cost no context in a session that never invokes it.
func IsOptedOutOfModelInvocation(lines []string) bool {
	return scanFrontmatter(lines, func(line string) bool {
		return modelInvocationOff.MatchString(AsciiLower(line))
	})
}

// --- end shared:opted-out-of-model-invocation ---

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
