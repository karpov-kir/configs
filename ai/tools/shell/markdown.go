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

	// The number a heading is written under, `## 7. What a suite reports`. Matched against a heading
	// already in comparison form, where a whitespace run is one space, so a single space is exact.
	leadingHeadingNumber = regexp.MustCompilePOSIX(`^[0-9]+\. `)

	frontmatterRule    = regexp.MustCompilePOSIX(`^---[[:space:]]*$`)
	modelInvocationOff = regexp.MustCompilePOSIX(`^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$`)

	// The audience marker, shared with ai/bootstrap.sh — which reads it in awk, before any Go binary
	// on the machine exists, and cannot call in here. So the pattern is written twice, and
	// markdown_test.go holds the two spellings to each other.
	maintainerAudience = regexp.MustCompilePOSIX(`^audience:[[:space:]]*maintainer[[:space:]]*$`)
)

// LinkTargets is every `](target)` on one line, the parentheses stripped. Which *block* of a file it
// is applied to is the caller's — `ecoroot.ReadAlwaysTargets` is where the ecosystem's always-loaded
// tier decides that, for both tools at once.
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

// FrontmatterDescription is a SKILL.md's `description:` value — the routing text, and the only part
// of a skill loaded in every session.
func FrontmatterDescription(lines []string) string {
	return frontmatterField(lines, "description")
}

// FrontmatterName is a SKILL.md's `name:` value — what the loader invokes the skill by. Read through
// the same block walk as the description: two readers with two ideas of where frontmatter ends is one
// idea too many, and the looser of them takes a `name:` line in the body for a declaration.
func FrontmatterName(lines []string) string {
	return frontmatterField(lines, "name")
}

// FrontmatterValue is any other frontmatter field's value, for the readers that need one the two
// named accessors above do not cover — `argument-hint`, `audience`. Named ones stay for the two
// fields whose meaning to the loader is worth stating; a third accessor per field would only repeat
// this line.
func FrontmatterValue(lines []string, field string) string {
	return frontmatterField(lines, field)
}

// One field's value out of the frontmatter block. Anchored to line 1, so a `---` rule in the body does
// not open frontmatter.
func frontmatterField(lines []string, field string) string {
	value := ""
	scanFrontmatter(lines, func(line string) bool {
		rest, ok := strings.CutPrefix(line, field+":")
		if !ok {
			return false
		}
		value = strings.TrimLeft(rest, SpaceBytes)
		return true
	})
	return value
}

// True when a skill's frontmatter takes it out of the router, which is what makes its description
// cost no context in a session that never invokes it.
func IsOptedOutOfModelInvocation(lines []string) bool {
	return scanFrontmatter(lines, func(line string) bool {
		return modelInvocationOff.MatchString(AsciiLower(line))
	})
}

// True when a skill declares it exists to maintain this instruction tree rather than to work in any
// repository. `ai/bootstrap.sh --skip-maintainer-skills` leaves those unmounted, so an install that
// only uses the ecosystem does not carry their descriptions in every session.
//
// The audience is declared in the skill rather than listed in the script, which is what keeps the
// mount loop discovery: a maintainer-only skill added tomorrow is excluded without anyone editing
// either reader. Read through the pattern above, so this and ai/bootstrap.sh's awk agree about what
// the marker line looks like.
//
// Named for the marker it reads rather than for what a caller does with the answer: two builds
// arrived here at once and wrote this function twice under both names, and the marker is the half
// that cannot drift.
func IsMaintainerAudience(lines []string) bool {
	return scanFrontmatter(lines, func(line string) bool {
		return maintainerAudience.MatchString(AsciiLower(line))
	})
}

// Walks the frontmatter block and stops at the first line the reader accepts, reporting whether one
// did. The block opens on line 1 and nowhere else, so a `---` rule in the body cannot start one.
//
// The closing delimiter is found before any line is offered to the reader, rather than accepting on
// the way down. A block that never closes is not frontmatter, and the loader cannot read a
// declaration out of one either. Offering body lines as they pass would let the first `name:` in the
// prose answer for it — reported as a clean declaration where the skill cannot be invoked at all.
func scanFrontmatter(lines []string, accept func(string) bool) bool {
	if len(lines) == 0 || !IsFrontmatterDelimiter(lines[0]) {
		return false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if IsFrontmatterDelimiter(lines[i]) {
			end = i
			break
		}
	}
	if end < 0 {
		return false
	}
	for _, line := range lines[1:end] {
		if accept(line) {
			return true
		}
	}
	return false
}

// BeforeEmDash is a subtitled heading's text before its ` — `, and empty for a heading carrying no
// subtitle. A citation naming that run enters the heading, so both detectors accept it — which is
// why the rule is stated here rather than in each of them.
func BeforeEmDash(heading string) string {
	before, _, found := strings.Cut(heading, " — ")
	if !found {
		return ""
	}
	return strings.TrimSpace(before)
}

// WithoutLeadingNumber is a numbered heading's text without the `7. ` that opens it, and empty when
// the heading opens with no number — an empty key would answer a citation that named nothing.
func WithoutLeadingNumber(heading string) string {
	numberless := leadingHeadingNumber.ReplaceAllString(heading, "")
	if numberless == heading {
		return ""
	}
	return numberless
}
