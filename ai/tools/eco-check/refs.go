package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

// The heads this scan's findings lead with, which report.go's rankTable ranks them on.
const (
	danglingLink           = "dangling link: "
	danglingHomeRef        = "dangling home ref: "
	danglingPathRef        = "dangling path ref: "
	unknownSkillReferenced = "unknown skill referenced: "
)

var (
	// Wider than the shell.LinkTargets budget.go uses: that one excludes `#`, because a budget target
	// with a fragment names no file to count, while a citation with one names a file *and* a section,
	// both checked here. So this admits the fragment and cuts it off itself below.
	markdownLinkPattern = regexp.MustCompilePOSIX(`\]\([^)]+\)`)
	homeRefPattern      = regexp.MustCompilePOSIX(`~/\.(kk-flavor|claude/skills)/[A-Za-z0-9._/-]+`)
	backtickedPathToken = regexp.MustCompilePOSIX(`^([A-Za-z0-9][A-Za-z0-9._/-]*/[A-Za-z0-9._-]+\.(sh|md)|[A-Za-z0-9][A-Za-z0-9._-]*\.sh|[A-Z][A-Z0-9]*(-[A-Z0-9]+)+\.md)$`)
	skillFamilyToken    = regexp.MustCompile(`\b(kk|idsd)-[a-z0-9-]+`)
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

// `~/.kk-flavor/...` and `~/.claude/skills/...` — how a skill reaches outside its own directory.
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

// The text inside each pair of backticks, outside fenced blocks.
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

// Our own skill namespaces: a name in prose must be a skill that exists.
func (c *checker) scanUnknownSkills() {
	var names []string
	for _, lines := range c.filesWithLines(c.root.Named(), "*.md", "*.yaml") {
		for _, line := range lines {
			for _, match := range skillFamilyToken.FindAllString(line, -1) {
				names = append(names, strings.TrimSuffix(match, "-"))
			}
		}
	}
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
