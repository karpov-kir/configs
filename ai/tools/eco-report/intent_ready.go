package ecoreport

import (
	"os"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// The intent-ready gate: the mechanical half of "is this ICE fit to build". The judgement half — a
// goal that reads two ways, an unpinned presentation, a technical choice nobody has made — is the
// grill's, and no script reaches it. What is here is what an agent otherwise scans by hand and
// eventually stops scanning: `~/.claude/skills/idsd-build/SKILL.md` → **Phase 1** is the contract.
//
// Exit 0 = ready, 1 = blocked with every reason printed, 2 = the check did not run.

// Sections the template defines as required. Reference data and follow-ups are optional, so an intent
// missing either is still buildable.
var requiredIntentSections = []string{"Constraints", "Success scenarios", "Failure scenarios"}

func (r *run) cmdIntentReady() {
	name := r.arg(1)
	// The name is joined into a path, so the slug charset is the whole of what keeps this read inside
	// intents/ — the same guard reportNameFor states at length.
	if name == "" || strings.HasPrefix(name, ".") || !isSlugCharset(name) {
		r.refuse("usage: report.sh intent-ready <NNN-slug>",
			"  the slug names <scratch>/intents/<NNN-slug>.md; it must be [0-9A-Za-z._-] and cannot start with a dot")
	}
	path := r.idsdDir + "/intents/" + name + ".md"
	if !shell.PathExists(path) && shell.IsRegularFile(r.idsdDir+"/archive/"+name+".md") {
		r.refuse("error: '" + name + "' is archived, so it is built rather than waiting to be built (" + r.idsdDir + "/archive/" + name + ".md)")
	}
	// A symlink is refused rather than followed for the reason the report template's is: what the check
	// then read is not the file the build will edit.
	if shell.IsSymlink(path) {
		r.refuse("error: " + path + " is a symlink — refusing to judge an intent through one")
	}
	if !shell.IsRegularFile(path) {
		r.refuse("error: no intent at " + path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		r.refuse("error: " + path + " cannot be read (" + err.Error() + ") — whether it is ready is unknown")
	}
	lines := shell.SplitLines(string(content))

	blocks := unfilledPlaceholders(lines)
	blocks = append(blocks, emptyRequiredSections(lines)...)
	blocks = append(blocks, unsignedCollaboration(path)...)
	blocks = append(blocks, r.unbuiltDependencies(lines)...)
	if len(blocks) > 0 {
		r.errLines(append([]string{"BLOCK (intent not ready): " + path}, blocks...)...)
		r.errLines("  Fold each answer into the ICE through idsd-intent, then re-run this. Building past it is how a placeholder ships as a requirement.")
		r.exit(1)
	}
	r.line("intent ready: %s — no placeholders, every required section filled, sign-off recorded, dependencies built", name)
}

// Template text the author never replaced. Scanned over the whole file, fenced blocks included: the
// gherkin skeleton is where `<name>` and `<state>` sit, and a scan that skipped fences would pass an
// untouched scenario block. An intent is business prose, so a literal angle bracket belongs in a code
// span, which is what stripCodeSpans exempts.
func unfilledPlaceholders(lines []string) []string {
	var found []string
	for i, line := range lines {
		if placeholder := firstPlaceholder(stripCodeSpans(line)); placeholder != "" {
			found = append(found, "  line "+strconv.Itoa(i+1)+": unfilled placeholder "+shell.Oneline(placeholder)+" — "+shell.Oneline(line))
		}
	}
	return found
}

// The first `<…>` that reads as a placeholder, or empty. A span opening on a space is not one, which
// is what keeps `returns in < 300ms and > 1s` from reading as one; an HTML comment is not one either.
func firstPlaceholder(text string) string {
	for start := 0; start < len(text); start++ {
		if text[start] != '<' {
			continue
		}
		end := strings.IndexByte(text[start+1:], '>')
		if end < 0 {
			return ""
		}
		body := text[start+1 : start+1+end]
		if body != "" && body[0] != ' ' && !strings.HasPrefix(body, "!--") {
			return "<" + body + ">"
		}
		start += end
	}
	return ""
}

// Backtick spans removed, so the text left is the prose alone. An unclosed backtick swallows the rest
// of the line, which is the safe direction: it hides candidates rather than inventing them.
func stripCodeSpans(line string) string {
	var out strings.Builder
	inCode := false
	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			out.WriteByte(line[i])
		}
	}
	return out.String()
}

// A required section absent, or present with nothing under it. Emptiness is checked separately from
// placeholders because deleting the placeholder is the obvious way to "answer" one.
func emptyRequiredSections(lines []string) []string {
	var blocks []string
	for _, section := range requiredIntentSections {
		filled, present := sectionHasContent(lines, section)
		switch {
		case !present:
			blocks = append(blocks, "  no '## "+section+"' section — the template defines it as required")
		case !filled:
			blocks = append(blocks, "  '## "+section+"' is empty")
		}
	}
	return blocks
}

func sectionHasContent(lines []string, section string) (filled, present bool) {
	inSection := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				return filled, true
			}
			inSection = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "## ")), section)
			continue
		}
		if inSection && strings.TrimSpace(line) != "" {
			filled = true
		}
	}
	return filled, inSection
}

// A pair-authored intent with nobody's name against it. The frontmatter's own comment is not a value,
// so both fields are read through yamlValue.
func unsignedCollaboration(path string) []string {
	if yamlValue(path, "collaborative") != "true" || yamlValue(path, "approved-by") != "" {
		return nil
	}
	return []string{"  collaborative: true with an empty approved-by — the intent needs its collaborator's sign-off before it is built"}
}

// `depends-on` edges whose target has not shipped. Direction is the point: building on an intent that
// is still draft means building on a contract that can still change under it.
func (r *run) unbuiltDependencies(lines []string) []string {
	var blocks []string
	for _, number := range dependsOnNumbers(lines) {
		file, where := r.intentFileNumbered(number)
		switch {
		case where == "archive":
			continue
		case file == "":
			blocks = append(blocks, "  depends-on "+number+" names no intent under "+r.idsdDir+"/intents/ or /archive/")
		case yamlValue(file, "status") != "built":
			blocks = append(blocks, "  depends-on "+number+" is not built yet ("+file+" is "+shell.Oneline(yamlValue(file, "status"))+") — build that one first")
		}
	}
	return blocks
}

// Every `depends-on` edge's number, read from the frontmatter alone so a body line quoting the
// relation is not mistaken for an edge.
func dependsOnNumbers(lines []string) []string {
	var numbers []string
	inFrontmatter := false
	for i, line := range lines {
		if i > 0 && shell.IsFrontmatterDelimiter(line) {
			break
		}
		if i == 0 {
			inFrontmatter = shell.IsFrontmatterDelimiter(line)
			continue
		}
		if !inFrontmatter || !strings.Contains(line, "depends-on") {
			continue
		}
		if number := firstNumber(line[strings.Index(line, "depends-on")+len("depends-on"):]); number != "" {
			numbers = append(numbers, number)
		}
	}
	return shell.SortUnique(numbers)
}

func firstNumber(text string) string {
	for _, field := range shell.SplitFields(text) {
		digits := strings.TrimLeft(strings.TrimRight(field, ".,;:"), "#")
		if digits != "" && strings.Trim(digits, "0123456789") == "" {
			return digits
		}
	}
	return ""
}

// The intent file an edge's number names, and which directory it came from. Numbers are the stable
// half of a slug, so a renamed intent is still found.
func (r *run) intentFileNumbered(number string) (path, where string) {
	for _, dir := range []string{"intents", "archive"} {
		entries, err := os.ReadDir(r.idsdDir + "/" + dir)
		if err != nil {
			// Absent is "nothing here", which the caller reports as an edge naming no intent. Unreadable
			// is a different fact, and read as the same one it turns a real dependency into a bad link.
			if os.IsNotExist(err) {
				continue
			}
			r.refuse("error: could not read " + r.idsdDir + "/" + dir + " (" + err.Error() + ") — whether depends-on " + number + " is built is unknown")
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, number+"-") && strings.HasSuffix(name, ".md") &&
				shell.IsRegularFile(r.idsdDir+"/"+dir+"/"+name) {
				return r.idsdDir + "/" + dir + "/" + name, dir
			}
		}
	}
	return "", ""
}

// fieldValue with YAML's trailing comment removed, because the ICE template documents three of its
// own fields in one — `approved-by:` carries its explanation, and read raw that explanation is a
// sign-off.
func yamlValue(path, field string) string {
	value := fieldValue(path, field)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	if comment := strings.Index(value, " #"); comment >= 0 {
		value = value[:comment]
	}
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
}
