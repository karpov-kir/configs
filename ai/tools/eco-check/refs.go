package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

const (
	danglingLink           = "dangling link: "
	danglingHomeRef        = "dangling home ref: "
	danglingPathRef        = "dangling path ref: "
	unknownSkillReferenced = "unknown skill referenced: "
	malformedSkillName     = "malformed skill name: "
)

var (
	// Wider than the shell.LinkTargets budget.go uses: that one excludes `#`, because a budget target
	// with a fragment names no file to count, while a citation with one names a file *and* a section,
	// both checked here. So this admits the fragment and cuts it off itself below.
	markdownLinkPattern = regexp.MustCompilePOSIX(`\]\([^)]+\)`)
	// `claude/skills` is here so a citation written through the MOUNT is matched and then found
	// dangling: resolveRef has no arm for that prefix, deliberately, because the tree cites itself
	// through `~/.kk-flavor/skills/` and the mount is where a skill is loaded from, not where it is
	// cited from. Matching it is the enforcement; dropping it from here would pass such a ref silently.
	homeRefPattern      = regexp.MustCompilePOSIX(`~/\.(kk-flavor|claude/skills|agents/skills|codex/skills)/[A-Za-z0-9._/-]+`)
	backtickedPathToken = regexp.MustCompilePOSIX(`^([A-Za-z0-9][A-Za-z0-9._/-]*/[A-Za-z0-9._-]+\.(sh|md)|[A-Za-z0-9][A-Za-z0-9._-]*\.sh|[A-Z][A-Z0-9]*(-[A-Z0-9]+)+\.md)$`)
	// What follows the family prefix stays inside the lane grammar — isNotLaneNameRune admits ASCII
	// alphanumerics and `._-`, and nothing else — because a name this scan reports has to be one the
	// rest of the tree could hold.
	//
	// Widening the character class to every Unicode letter was tried and reverted. It made the scan
	// echo `kk-cоde-review`, with a Cyrillic о, as a name that renders exactly like a real skill.
	// Such a finding reads as the checker being broken, and takes that run's true findings down with
	// it. The truncation the widening was meant to fix is answered in scanUnknownSkills instead,
	// which reports the token as malformed rather than naming a path nothing can hold.
	skillFamilyToken = regexp.MustCompile(`\b(kk|idsd)-[a-z0-9-]+`)
	// Matches the character just past a family token: the one that would have continued the name,
	// had the grammar above admitted it. Letters, marks and digits only, so that the character reads
	// as part of a name someone typed rather than as the sentence around it.
	continuesAName = regexp.MustCompile(`^[\p{L}\p{M}\p{N}]`)
)

// Relative markdown links, resolved against the linking file's own directory. A template's links
// resolve where it is emitted (a project's `.idsd/`), so a bare sibling name is unverifiable and
// passes; only a traversal out of the emitted directory is checkable there.
func (c *checker) scanDanglingLinks() {
	for file, lines := range c.filesWithLines(c.root.Named(), "*.md") {
		c.reportDanglingLinks(file, lines)
	}
}

func (c *checker) reportDanglingLinks(file string, lines []string) {
	isTemplate := strings.Contains(file, "/templates/")
	dir := shell.DirName(file)
	for _, line := range lines {
		for _, match := range markdownLinkPattern.FindAllString(line, -1) {
			link := strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")")
			if strings.HasPrefix(link, "http") || strings.HasPrefix(link, "mailto:") ||
				strings.HasPrefix(link, "#") || strings.HasPrefix(link, "~") {
				continue
			}
			target, _, _ := strings.Cut(link, "#")
			// A target resolving outside the root is not stat'ed — see tree.go's underRoot. It falls
			// through to the finding below, so the report says the same thing whether or not the
			// reviewing machine holds that file.
			if c.existsUnderRoot(shell.Join(dir, target)) {
				continue
			}
			if isTemplate && !isTraversal(target) {
				continue
			}
			c.add(danglingLink + shell.Oneline(file) + " -> " + shell.Oneline(link))
		}
	}
}

func isTraversal(target string) bool {
	return strings.HasPrefix(target, "/") || strings.HasPrefix(target, "../") ||
		strings.Contains(target, "/../") || target == ".."
}

func (c *checker) scanHomeRefs() {
	var refs []string
	for _, lines := range c.filesWithLines(c.root.Named(), "*.md", "*.sh") {
		for _, line := range lines {
			for _, match := range homeRefPattern.FindAllString(line, -1) {
				// The trailing run of sentence punctuation is where the prose ended, not part of
				// the path.
				refs = append(refs, strings.TrimRight(match, ".,;:"))
			}
		}
	}
	for _, ref := range shell.SortUnique(refs) {
		if c.resolveRef("", ref) == "" {
			c.add(danglingHomeRef + shell.Oneline(ref))
		}
	}
}

// Backticked in-repo paths — `scripts/report.sh`, `templates/ice-template.md`, `AGENT-BRIEF.md`.
// Fenced blocks are skipped. The shapes stay narrow on purpose: a bare lowercase `*.md` is as often
// a file a project owns (`charter.md`, `roadmap.md`), so only SHOUTY-with-a-hyphen is matched.
func (c *checker) scanPathRefs() {
	for file, lines := range c.filesWithLines(c.root.Named(), "*.md", "*.sh") {
		dir := shell.DirName(file)
		// A skill cites its own tooling from the skill root (`scripts/report.sh`) even in a file
		// that sits under `scripts/`, so resolve from both.
		skillRoot := dir
		if rest, ok := strings.CutPrefix(file, c.root.Skills()+"/"); ok {
			first, _, _ := strings.Cut(rest, "/")
			skillRoot = shell.Join(c.root.Skills(), first)
		}
		var tokens []string
		for _, span := range backtickedSpans(lines) {
			if backtickedPathToken.MatchString(span) {
				tokens = append(tokens, span)
			}
		}
		for _, token := range shell.SortUnique(tokens) {
			if c.refExists(dir, token) {
				continue
			}
			if skillRoot != dir && c.refExists(skillRoot, token) {
				continue
			}
			c.add(danglingPathRef + shell.Oneline(file) + " -> " + shell.Oneline(token))
		}
	}
}

func backtickedSpans(lines []string) []string {
	var spans []string
	for _, line := range unfenced(lines) {
		spans = append(spans, delimitedSpans(line, "`")...)
	}
	return spans
}

// The text inside each pair of the given delimiter on one line. Split on the delimiter rather than
// shrinking the line: rebuilding the tail on every hit is quadratic in a line length the tree
// chooses, and one committed multi-megabyte line would stall the whole check.
func delimitedSpans(line, delimiter string) []string {
	var spans []string
	parts := strings.Split(line, delimiter)
	for k := 0; k <= len(parts)-3; {
		if parts[k+1] != "" {
			spans = append(spans, parts[k+1])
			k += 2
			continue
		}
		k++
	}
	return spans
}

func (c *checker) scanUnknownSkills() {
	var names, malformed []string
	for _, lines := range c.filesWithLines(c.root.Named(), "*.md", "*.yaml") {
		for _, line := range lines {
			for _, at := range skillFamilyToken.FindAllStringIndex(line, -1) {
				match := line[at[0]:at[1]]
				// The grammar stopped this token mid-name. Reporting the ASCII half it did match as
				// the name is what sent a reader to `skills/kk-driv/SKILL.md` for a `kk-drivé`
				// directory — a path nothing can hold — so the token goes to `malformed` and is
				// never resolved as a path.
				if continuesAName.MatchString(line[at[1]:]) {
					malformed = append(malformed, match)
					continue
				}
				names = append(names, strings.TrimSuffix(match, "-"))
			}
		}
	}
	c.reportMalformedSkillNames(malformed)
	for _, name := range shell.SortUnique(names) {
		// `kk-flavor` is the shared layer, not a skill.
		if name == "kk-flavor" || c.holdsRegularFile(c.skillFilePath(name)) {
			continue
		}
		// Two readings, and the scan cannot tell them apart: a misspelled skill, or prose that
		// happens to wear the family's shape (`kk-drive-verified`). Suppressing a token whose
		// prefix is a real skill would mask the first — `kk-drives` is exactly that shape — so the
		// message carries both readings instead.
		c.add(unknownSkillReferenced + shell.Oneline(name) + " — no skills/" + shell.Oneline(name) +
			"/SKILL.md. If this is prose rather than a skill, reword it so it does not read as one")
	}
}

// A name carrying a character the lane grammar does not admit. These are reported apart from the
// unknown-skill findings, and without the offending name, on purpose: a homoglyph — `kk-cоde-review`
// with a Cyrillic о — echoed whole reads as a skill that exists, so a message carrying it argues the
// checker is wrong. What a reader can act on is the ASCII part the grammar did accept, plus the fact
// that what follows it is outside the grammar. That is all this prints.
func (c *checker) reportMalformedSkillNames(malformed []string) {
	for _, prefix := range shell.SortUnique(malformed) {
		c.add(malformedSkillName + shell.Oneline(prefix) + "… — the character after this prefix is " +
			"outside the ASCII letters, digits and hyphens a skill name may use. No skill can carry " +
			"it, so reword the line or spell the name the way its directory does")
	}
}
