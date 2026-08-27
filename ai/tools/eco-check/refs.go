package ecocheck

import (
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

var (
	// Wider than shell.LinkTargets, which this package also uses one file away in budget.go: that one
	// excludes `#` because a budget target with a fragment names no file to count, while a citation
	// with one names a file *and* a section, both of which are checked here — so this admits the
	// fragment and cuts it off itself below. Two readings of one form, on purpose.
	markdownLinkPattern = regexp.MustCompilePOSIX(`\]\([^)]+\)`)
	homeRefPattern      = regexp.MustCompilePOSIX(`~/\.(kk-flavor|claude/skills)/[A-Za-z0-9._/-]+`)
	backtickedPathToken = regexp.MustCompilePOSIX(`^([A-Za-z0-9][A-Za-z0-9._/-]*/[A-Za-z0-9._-]+\.(sh|md)|[A-Za-z0-9][A-Za-z0-9._-]*\.sh|[A-Z][A-Z0-9]*(-[A-Z0-9]+)+\.md)$`)
	headingLine         = regexp.MustCompilePOSIX(`^#+[[:space:]]`)
	headingMarker       = regexp.MustCompilePOSIX(`^#+[[:space:]]*`)
	skillFamilyToken    = regexp.MustCompile(`\b(kk|idsd)-[a-z0-9-]+`)
	// The cited file at the head of a `<file> → <Section>`, in its three written forms.
	trailingLinkTarget = regexp.MustCompilePOSIX(`\]\([^()]*\)$`)
	trailingBarePath   = regexp.MustCompilePOSIX(`[A-Za-z0-9._/-]+$`)
	markdownFileTail   = regexp.MustCompilePOSIX(`[A-Za-z0-9]\.md$`)
	sectionCutPoint    = regexp.MustCompilePOSIX(`[():;,.!?"]`)
)

// Relative markdown links, resolved against the linking file's own directory. A template's links
// resolve where it is emitted (a project's `.idsd/`), so a bare sibling name is unverifiable and
// passes; only a traversal out of the emitted directory is checkable there.
func (c *checker) scanDanglingLinks() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md") {
		isTemplate := strings.Contains(file, "/templates/")
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		dir := shell.DirName(file)
		for _, line := range lines {
			for _, match := range markdownLinkPattern.FindAllString(line, -1) {
				link := strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")")
				if strings.HasPrefix(link, "http") || strings.HasPrefix(link, "mailto:") ||
					strings.HasPrefix(link, "#") || strings.HasPrefix(link, "~") {
					continue
				}
				target, _, _ := strings.Cut(link, "#")
				if shell.PathExists(shell.Join(dir, target)) {
					continue
				}
				if isTemplate && !isTraversal(target) {
					continue
				}
				c.add("dangling link: " + shell.Oneline(file) + " -> " + shell.Oneline(link))
			}
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
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
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
			c.add("dangling home ref: " + shell.Oneline(ref))
		}
	}
}

// Backticked in-repo paths — `scripts/report.sh`, `templates/ice-template.md`, `AGENT-BRIEF.md`.
// Fenced blocks are skipped. The shapes stay narrow on purpose: a bare lowercase `*.md` is as often
// a file a project owns (`charter.md`, `roadmap.md`), so only SHOUTY-with-a-hyphen is matched.
func (c *checker) scanPathRefs() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
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
			c.add("dangling path ref: " + shell.Oneline(file) + " -> " + shell.Oneline(token))
		}
	}
}

// The text inside each pair of backticks, outside fenced blocks. Split on the tick rather than
// shrinking the line: rebuilding the tail on every hit is quadratic in a line length the tree
// chooses, and one committed multi-megabyte line would stall the whole check.
func backtickedSpans(lines []string) []string {
	var spans []string
	inFence := false
	for _, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		parts := strings.Split(line, "`")
		for k := 0; k <= len(parts)-3; {
			if parts[k+1] != "" {
				spans = append(spans, parts[k+1])
				k += 2
				continue
			}
			k++
		}
	}
	return spans
}

type citation struct {
	src         string
	line        int
	path        string
	section     string
	isDelimited bool
}

// The other half of a `<file> → <Section>` citation: the heading it names must still be there. An
// arrow counts only when the text before it resolves to a real markdown file, which keeps prose
// arrows ("intent → build") out.
func (c *checker) scanCitations() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		for _, cited := range citationsIn(file, lines) {
			c.reportCitation(cited)
		}
	}
}

func (c *checker) reportCitation(cited citation) {
	position := shell.Oneline(cited.src) + ":" + strconv.Itoa(cited.line)
	target := c.resolveRef(shell.DirName(cited.src), cited.path)
	if target == "" {
		c.add("unresolvable citation path: " + position + " -> " + shell.Oneline(cited.path))
		return
	}
	// Reported even when the section resolves today: undelimited is how it stops resolving in
	// silence. An undelimited name truncates at the first comma, so half a heading satisfies the
	// citation and a rename then breaks it without a word.
	if !cited.isDelimited {
		c.add("undelimited section citation: " + position + " -> " + shell.Oneline(cited.path) +
			" → " + shell.Oneline(cited.section) + " is not wrapped in ** or backticks")
	}
	// The cited path resolved through whatever the reviewed tree pointed it at, and pathExists
	// follows a symlink: `evil.md -> /dev/zero` or a committed FIFO makes the read below never
	// return. Reported rather than skipped — a target nothing read must not be indistinguishable
	// from a checked one.
	if !shell.IsRegularFile(target) {
		c.add("citation target is not a regular file: " + position + " -> " + shell.Oneline(cited.path) +
			" — it was NOT read")
		return
	}
	headings := c.markdownHeadings(target)
	// Prose runs on past the heading it names, so accept the longest leading run that is a heading.
	want := plainText(cited.section)
	for want != "" {
		if headings[want] {
			return
		}
		cut := strings.LastIndexByte(want, ' ')
		if cut < 0 {
			break
		}
		want = want[:cut]
	}
	c.add("dangling section ref: " + position + " -> " + shell.Oneline(cited.path) + " → " + shell.Oneline(cited.section))
}

// Every `#` heading in a markdown file, in comparison form. Fenced blocks are skipped.
func (c *checker) markdownHeadings(path string) map[string]bool {
	headings := map[string]bool{}
	lines, err := c.readLines(path)
	if err != nil {
		return headings
	}
	inFence := false
	for _, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if inFence || !headingLine.MatchString(line) {
			continue
		}
		heading := plainText(headingMarker.ReplaceAllString(line, ""))
		headings[heading] = true
		// A heading may carry a subtitle after an em dash and a citation names only the run before
		// it, so accept that run too. Cut at the em dash and nowhere else: a trailing run, or a
		// word-by-word prefix, would let half a heading satisfy a citation.
		if i := strings.Index(heading, " — "); i >= 0 {
			headings[heading[:i]] = true
		}
	}
	return headings
}

func citationsIn(file string, lines []string) []citation {
	var found []citation
	inFence := false
	for lineNumber, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		segments := strings.Split(line, "→")
		for i := 1; i < len(segments); i++ {
			path := citedPath(segments[i-1])
			if path == "" {
				continue
			}
			section, isDelimited := citedSection(segments[i])
			if section == "" {
				continue
			}
			found = append(found, citation{
				src:         file,
				line:        lineNumber + 1,
				path:        path,
				section:     section,
				isDelimited: isDelimited,
			})
		}
	}
	return found
}

// The cited file: a markdown link, a backticked path, or a bare filename. Empty unless it is a
// markdown file, which is what keeps a prose arrow out.
func citedPath(before string) string {
	before = strings.TrimRight(before, shell.SpaceBytes)
	path := ""
	switch {
	case trailingLinkTarget.MatchString(before):
		open := strings.LastIndex(before, "](")
		path = strings.TrimSuffix(before[open+2:], ")")
	case strings.HasSuffix(before, "`"):
		head := before[:len(before)-1]
		if tick := strings.LastIndexByte(head, '`'); tick >= 0 {
			path = head[tick+1:]
		}
	default:
		path = trailingBarePath.FindString(before)
	}
	path, _, _ = strings.Cut(path, "#")
	if !markdownFileTail.MatchString(path) {
		return ""
	}
	return path
}

// Whether the name arrived inside `**` or backticks is the whole decision here: read exactly, or
// guessed at by the fallback. ecosystem.md → **Conventions a new file joins** requires the
// delimited form for that reason.
func citedSection(after string) (section string, isDelimited bool) {
	after = strings.TrimLeft(after, shell.SpaceBytes)
	if rest, ok := strings.CutPrefix(after, "**"); ok {
		if end := strings.Index(rest, "**"); end > 0 {
			section = rest[:end]
		}
	} else if rest, ok := strings.CutPrefix(after, "`"); ok {
		if end := strings.IndexByte(rest, '`'); end > 0 {
			section = rest[:end]
		}
	}
	isDelimited = section != ""
	if section == "" {
		section = after
		if cut := sectionCutPoint.FindStringIndex(section); cut != nil {
			section = section[:cut[0]]
		}
		if dash := strings.Index(section, "—"); dash > 0 {
			section = section[:dash]
		}
	}
	section = strings.NewReplacer("`", "", "*", "").Replace(section)
	section = headingMarker.ReplaceAllString(section, "")
	return strings.Trim(section, shell.SpaceBytes), isDelimited
}

// Our own skill namespaces — a name in prose must be a skill that exists.
func (c *checker) scanUnknownSkills() {
	var names []string
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.yaml") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		for _, line := range lines {
			for _, match := range skillFamilyToken.FindAllString(line, -1) {
				names = append(names, strings.TrimSuffix(match, "-"))
			}
		}
	}
	for _, name := range shell.SortUnique(names) {
		// `kk-flavor` is the shared layer, not a skill.
		if name == "kk-flavor" || shell.IsRegularFile(shell.Join(shell.Join(c.root.Skills(), name), "SKILL.md")) {
			continue
		}
		// Two readings, and the scan cannot tell them apart: a misspelled skill, or prose that
		// happens to wear the family's shape (`kk-drive-verified`). Suppressing a token whose
		// prefix is a real skill would mask the first — `kk-drives` is exactly that shape — so the
		// message carries both readings instead.
		c.add("unknown skill referenced: " + shell.Oneline(name) + " — no skills/" + shell.Oneline(name) +
			"/SKILL.md. If this is prose rather than a skill, reword it so it does not read as one")
	}
}
