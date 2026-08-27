package shell

import (
	"regexp"
	"strings"
)

// What the line-oriented tools read out of a markdown file: the link targets on a line, and the
// frontmatter block a SKILL.md opens with. The exact edges are the contract — which link forms the
// `grep -oE` matched, which `---` counts as a delimiter — so each is stated here once rather than
// re-derived wherever the question is put.
//
// Nothing here knows what a link or a description *means* to the ecosystem: which block of a file to
// scan, and where an `@import` resolves, belong to the tool asking.

var (
	// The link form `grep -oE '\]\([^)#]+\)'` matched, with the `sed 's/^](//; s/)$//'` behind it.
	linkTarget = regexp.MustCompilePOSIX(`\]\([^)#]+\)`)

	frontmatterRule    = regexp.MustCompilePOSIX(`^---[[:space:]]*$`)
	descriptionField   = regexp.MustCompilePOSIX(`^description:[[:space:]]*`)
	modelInvocationOff = regexp.MustCompilePOSIX(`^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$`)
)

// LinkTargets is every `](target)` on one line, the parentheses stripped. Which *block* of a file it
// is applied to is the caller's: check.sh reads a sed range, stats.sh an awk flag, and the two select
// different boundary lines on purpose.
func LinkTargets(line string) []string {
	var targets []string
	for _, match := range linkTarget.FindAllString(line, -1) {
		targets = append(targets, strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")"))
	}
	return targets
}

// IsFenceDelimiter is the ```-opened line — the `/^```/` every scan toggled its fence state on. The
// marker only, never the skipping: whether what a fence encloses is read is the caller's question,
// and ecocheck's direction scan reads inside one on purpose.
func IsFenceDelimiter(line string) bool {
	return strings.HasPrefix(line, "```")
}

// IsFrontmatterDelimiter is `/^---[[:space:]]*$/` — the delimiter line itself, and nothing that
// merely starts with one. Every reader that walks a frontmatter block asks this, so a `----` rule or
// a `--- x` line is admitted or refused the same way wherever the question is put.
func IsFrontmatterDelimiter(line string) bool {
	return frontmatterRule.MatchString(line)
}

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
		return modelInvocationOff.MatchString(asciiLower(line))
	})
}

// --- end shared:opted-out-of-model-invocation ---

// Walks the frontmatter block and stops at the first line the reader accepts, reporting whether one
// did. The block opens on line 1 and nowhere else, so a `---` rule in the body cannot start one.
func scanFrontmatter(lines []string, accept func(string) bool) bool {
	for i, line := range lines {
		if i == 0 && !IsFrontmatterDelimiter(line) {
			return false
		}
		if i > 0 && IsFrontmatterDelimiter(line) {
			return false
		}
		if accept(line) {
			return true
		}
	}
	return false
}
