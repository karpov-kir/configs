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

	// Any `audience:` line at all, so a value neither reader knows can be refused by name instead of
	// passing for an absent marker. Kept beside the pattern above because the two are one rule: the
	// marker has exactly one spelling, and everything else is a mistake somebody made on purpose.
	audienceDeclared = regexp.MustCompilePOSIX(`^audience:`)

	// How a skill declares it runs, in the body rather than the frontmatter: the line is a contract
	// between the skill and the model policy, and frontmatter is what the harness loads into every
	// session. The three forms are the whole grammar — `dispatched`, `orchestrator`, and `holds` with
	// one of three reasons — because the tier ceiling reads the answer and a fourth word it could not
	// place would exempt a skill by being unreadable.
	runsDeclaration = regexp.MustCompilePOSIX(`^\*\*Runs:\*\*[ ]*(dispatched|orchestrator|holds — (converses|session-context|landing))[ ]*$`)

	runsDeclared = regexp.MustCompilePOSIX(`^\*\*Runs:\*\*`)

	// Which contract a skill reads as its own delta and runs inside its own session, and when. Sixteen
	// edges in this tree do that and roughly thirty others name a second skill without doing it — to
	// point at it, or to name the stage after this one — and no reader can tell those apart from the
	// citation. So the one that costs money is declared, and everything else is a pointer by default.
	//
	// The `— <when>` is required because the declaration is the instruction: the prose at the firing
	// site names the contract's path and no longer says the read is inline, so a declaration with no
	// `when` leaves nothing in the file saying where the extension happens. Its text is free-form —
	// what it must name is a phase, a step or a mode of THIS skill, which no regex can judge. So the
	// only thing held here is that it is there: one character that is not whitespace. `[:space:]`
	// rather than a literal space, because a tab-only clause is an absent `when` that reads as a
	// present one, and that is the single direction this line can be wrong in and cost money.
	extendsDeclaration = regexp.MustCompilePOSIX(`^\*\*Extends:\*\* *([A-Za-z0-9][A-Za-z0-9._-]*) +— +([^[:space:]](.*[^[:space:]])?)[[:space:]]*$`)
	extendsDeclared    = regexp.MustCompilePOSIX(`^\*\*Extends:\*\*`)

	// Which layer a standard puts itself in. The three names are the whole grammar, for the reason
	// runsDeclaration's three forms are: the cycle check reads the answer, and a fourth word it could
	// not place would exempt a standard by being unreadable.
	//
	// Anchored and whole-word, because only one direction of a wrong regex is silent. Too tight — a
	// line ending in `\r` refused — reports a standard that declared as one nobody can read, which is
	// loud and fixed in seconds. Too loose, `**Layer:** basement` taken for `base`, files the standard
	// in a layer nobody wrote and the cycle check then judges it against the wrong neighbours without
	// a word.
	layerDeclaration = regexp.MustCompilePOSIX(`^\*\*Layer:\*\* *(` + strings.Join(Layers, "|") + `)[[:space:]]*$`)

	layerDeclared = regexp.MustCompilePOSIX(`^\*\*Layer:\*\*`)
)

// Layers are the three the standards divide into, ordered as ecosystem.md → **One home** writes them:
// base makes sense to a reader who has read no other layer, craft is making software, process is
// running the agent machine.
//
// Exported because the finding that refuses a fourth word names them, and a list written out at that
// finding is one a layer added here would never reach.
var Layers = []string{"base", "craft", "process"}

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
// repository. `ai/bootstrap.sh` without `--maintainer` leaves those unmounted, so an install that
// only uses the ecosystem does not carry their descriptions in every session.
//
// The audience is declared in the skill rather than listed in the script, which is what keeps the
// mount loop discovery: a maintainer-only skill added tomorrow is excluded without anyone editing
// either reader. Read through the pattern above, so this and ai/bootstrap.sh's awk agree about what
// the marker line looks like.
func IsMaintainerAudience(lines []string) bool {
	return scanFrontmatter(lines, func(line string) bool {
		return maintainerAudience.MatchString(AsciiLower(line))
	})
}

// The value on an `audience:` line that neither reader recognises, and whether there was one.
//
// Returned rather than passed over. `audience: maintainr` matches no marker, so a reader that only
// asks "is this the marker" answers no and the skill installs for everyone — a declaration the human
// wrote, silently ignored, leaving them with a skill they believe is marked and is not. That is the
// same failure `ai/tools/reader-judge/deadline.go` refuses an unrecognised override line for, and it
// is worse here: the mistake is invisible on a machine where the install looks correct.
//
// `maintainer` is the only value there is. Adding a second one means teaching both readers, which is
// what this refusal makes unavoidable rather than optional.
func UnknownAudience(lines []string) (string, bool) {
	value, found := "", false
	scanFrontmatter(lines, func(line string) bool {
		lowered := AsciiLower(line)
		if !audienceDeclared.MatchString(lowered) || maintainerAudience.MatchString(lowered) {
			return false
		}
		// The raw value, not the lowered one: it is echoed back to whoever typed it, and a reader
		// hunting `Maintainr` in their file should find what they wrote.
		value, found = strings.TrimSpace(line[len("audience:"):]), true
		return true
	})
	return value, found
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

// RunsDeclaration is how a skill says it runs — `dispatched`, `orchestrator`, or `holds — <reason>`
// — and whether the line was there at all. Body text, so the whole file is scanned rather than the
// frontmatter block.
//
// Three returns rather than two, for the reason UnknownAudience has three: `**Runs:** human` matched
// no form for a year after that word was retired, and a reader asking only "which of the three is
// it" would have read every such skill as declaring nothing and priced it by whatever its silence
// implied. `declared` says a line is there; `mode` empty beside it is a line nobody can read.
func RunsDeclaration(lines []string) (mode string, declared bool) {
	for _, line := range lines {
		if !runsDeclared.MatchString(line) {
			continue
		}
		declared = true
		if found := runsDeclaration.FindStringSubmatch(line); found != nil {
			return found[1], true
		}
	}
	return "", declared
}

// RunsHoldsReason is the reason a `holds` declaration names, and empty for every other mode. The
// three reasons are the standing ones in model-policy.md; which of them a skill claims is what says
// whether its tier is the work it keeps or a hole nobody has offloaded yet.
func RunsHoldsReason(mode string) string {
	_, reason, found := strings.Cut(mode, " — ")
	if !found {
		return ""
	}
	return reason
}

// ExtendsDeclarations is every skill this one reads as its own delta and runs inline, and whether any
// `**Extends:**` line was there at all. Body text, like RunsDeclaration beside it.
//
// The declaration exists because the citation cannot carry it. `ecosystem.md` → **Three kinds, two
// homes** names three ways one skill can name another without dispatching it — extension, sequencing
// and orientation — and only extension puts the second contract's dispatches on the first's bill.
// Read off the path alone they are identical, so a cost surface either guesses or is told; this is
// being told.
//
// Declared-but-unreadable is reported the way RunsDeclaration reports it: a line nobody can parse is
// not an absent one, and reading it as silence would price the extension as free. A line missing its
// `— <when>` is one of those: the grammar is `**Extends:** <skill> — <when>`, and the when is what the
// prose used to carry before the declaration became the only place the edge is stated.
func ExtendsDeclarations(lines []string) (extends []string, declared bool) {
	for _, line := range lines {
		if !extendsDeclared.MatchString(line) {
			continue
		}
		declared = true
		if found := extendsDeclaration.FindStringSubmatch(line); found != nil {
			extends = append(extends, found[1])
		}
	}
	return extends, declared
}

// LayerDeclaration is the layer a standard puts itself in — one of Layers — and whether a
// `**Layer:**` line was there at all. Body text, like RunsDeclaration beside it: the rule puts the
// line first in the file, and anchoring the scan there would report a standard that declared one line
// lower as one that abstains. Where the line sits is not what either reader wants from it.
//
// Two returns for RunsDeclaration's reason. `**Layer:** core` matches no name, and a reader asking
// only "which of the three is it" would answer that the standard declares nothing — which is what the
// tree's one unlayered-file finding already means, so the mistake would arrive under the wrong name
// and the fix under the wrong instruction.
func LayerDeclaration(lines []string) (layer string, declared bool) {
	for _, line := range lines {
		if !layerDeclared.MatchString(line) {
			continue
		}
		declared = true
		if found := layerDeclaration.FindStringSubmatch(line); found != nil {
			return found[1], true
		}
	}
	return "", declared
}

func UnknownLayer(lines []string) (string, bool) {
	for _, line := range lines {
		if !layerDeclared.MatchString(line) || layerDeclaration.MatchString(line) {
			continue
		}
		return strings.Trim(line[len("**Layer:**"):], SpaceBytes), true
	}
	return "", false
}

// ListMarker reads the list marker a line opens on. It returns the empty string for a bullet, the
// digits for an ordered item, or false when no space or tab follows the marker.
func ListMarker(line string) (string, bool) {
	number, rest := "", ""
	switch {
	case strings.HasPrefix(line, "-"), strings.HasPrefix(line, "*"), strings.HasPrefix(line, "+"):
		rest = line[1:]
	default:
		digits := 0
		for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits+1 > len(line) || (line[digits] != '.' && line[digits] != ')') {
			return "", false
		}
		number, rest = line[:digits], line[digits+1:]
	}
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	return number, true
}
