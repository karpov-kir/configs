package ecoreport

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// The intent-ready gate: the mechanical half of "is this ICE fit to build". The judgement half — a
// goal that reads two ways, an unpinned presentation, a technical choice nobody has made — is the
// grill's, and no script reaches it. What is here is what an agent otherwise scans by hand and
// eventually stops scanning: `~/.kk-flavor/skills/idsd-build/SKILL.md` → **Phase 1** is the contract.
//
// Exit 0 = ready, 1 = blocked with every reason printed, 2 = the check did not run.

// Sections the template defines as required. Reference data and follow-ups are optional, so an intent
// missing either is still buildable.
var requiredIntentSections = []string{"Constraints", "Success scenarios", "Failure scenarios"}

// The relations this intent's own frontmatter can draw whose target is asked to exist and nothing
// more. `depends-on` is not one of them: unbuiltDependencies asks it for a BUILT target, which is a
// strictly stronger question. `blocks` says the other intent waits on this one, so its target being
// unbuilt is the normal case, and `extends` draws no build-order edge at all.
var existenceOnlyRelations = []string{"blocks", "extends"}

func (r *run) cmdIntentReady() {
	name := r.arg(1)
	// The name is joined into a path, so the slug charset is the whole of what keeps this read inside
	// intents/<slug>/ — the same guard reportNameFor states at length.
	// The number is required, not decoration: every link between intents is drawn on it, so a name
	// without one leaves the reverse scan below with nothing to look for. Refusing beats running three
	// of the four checks and printing the line that says all four passed.
	number := leadingNumber(name)
	if name == "" || strings.HasPrefix(name, ".") || !isSlugCharset(name) || number == "" {
		r.refuse("usage: report.sh intent-ready <NNN-slug>",
			"  the slug names <scratch>/intents/<NNN-slug>/; it must be [0-9A-Za-z._-], cannot start with a dot, and must open with the intent's number")
	}
	path := r.shipDir(name) + "/" + intentName
	if !shell.PathExists(path) && shell.IsRegularFile(r.archiveDir(name)+"/"+intentName) {
		r.refuse("error: '" + name + "' is archived, so it is built rather than waiting to be built (" + r.archiveDir(name) + "/" + intentName + ")")
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

	reasons := unfilledPlaceholders(lines)
	reasons = append(reasons, emptyRequiredSections(lines)...)
	reasons = append(reasons, r.unbuiltDependencies(lines)...)
	reasons = append(reasons, r.unresolvedLinks(lines)...)
	reasons = append(reasons, r.unbuiltBlockers(number)...)
	if len(reasons) > 0 {
		r.errLines(append([]string{"BLOCK (intent not ready): " + path}, reasons...)...)
		r.errLines("  Fold each answer into the ICE through idsd-intent, then re-run this. Building past it is how a placeholder ships as a requirement.")
		r.exit(1)
	}
	r.line("intent ready: %s — no placeholders, every required section filled, dependencies built, every link naming a real intent, every sibling declaring it goes first is built too", name)
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
		// Only a comment swallows what is inside it. A span opening on a space is not a placeholder, but
		// the `>` that closed it is as likely a real placeholder's: `stays < 300ms for <state>` closes the
		// space-span on `<state>`'s own bracket, and skipping there steps over the one thing this looks for.
		if strings.HasPrefix(body, "!--") {
			start += end
		}
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
	var reasons []string
	for _, section := range requiredIntentSections {
		filled, present := sectionHasContent(lines, section)
		switch {
		case !present:
			reasons = append(reasons, "  no '## "+section+"' section — the template defines it as required")
		case !filled:
			reasons = append(reasons, "  '## "+section+"' is empty")
		}
	}
	return reasons
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

// `depends-on` edges whose target has not shipped. Direction is the point: building on an intent that
// is still draft means building on a contract that can still change under it.
func (r *run) unbuiltDependencies(lines []string) []string {
	var reasons []string
	for _, number := range linkNumbers(lines, "depends-on") {
		file, where := r.intentFileNumbered("depends-on", number)
		status := yamlValue(file, "status")
		switch {
		case where == "archive":
			continue
		case file == "":
			reasons = append(reasons, "  depends-on "+number+" names no intent under "+r.idsdDir+"/intents/ or /archive/")
		case status != "built":
			// The path is collapsed as well as the status: it carries a ship FOLDER's name, and a folder
			// name is not a value this tool wrote — `init` holds the slug charset, but a folder authored by
			// idsd-intent or arriving on someone else's branch answers to nothing here.
			reasons = append(reasons, "  depends-on "+number+" is not built yet ("+shell.Oneline(file)+" is "+shell.Oneline(status)+") — build that one first")
		}
	}
	return reasons
}

// The intent's own forward edges whose target is nowhere. Only existence is asked, for the reason
// existenceOnlyRelations states. Nothing resolved these before: `blocks` was read out of the SIBLINGS'
// files by unbuiltBlockers, which is the reverse direction, and `extends` was read nowhere at all — so
// an edge onto a number nobody wrote cleared this gate and died at idsd-finalize, inside the merge
// slot, after the build.
func (r *run) unresolvedLinks(lines []string) []string {
	var reasons []string
	for _, relation := range existenceOnlyRelations {
		for _, number := range linkNumbers(lines, relation) {
			if file, _ := r.intentFileNumbered(relation, number); file == "" {
				reasons = append(reasons, "  "+relation+" "+number+" names no intent under "+r.idsdDir+"/intents/ or /archive/")
			}
		}
	}
	return reasons
}

// The numbers one relation names, read from the frontmatter alone so a body line quoting the relation
// is not mistaken for an edge, and from the head of a link entry alone so the why half of a
// neighbouring entry is not either. `- depends-on 002 — 002 blocks 003, so this lands after both`
// declares one edge and mentions a second; read as two, the phantom arrives from a SIBLING's file and
// blocks a build for a reason the blocked intent's own author cannot clear from their own file.
func linkNumbers(lines []string, relation string) []string {
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
		if !inFrontmatter {
			continue
		}
		if number := firstNumber(linkEntryFor(line, relation)); number != "" {
			numbers = append(numbers, number)
		}
	}
	return shell.SortUnique(numbers)
}

// What a `links:` entry says after its relation, or empty where the line does not declare that
// relation. The template's shape is `  - blocks 003 — why`, and the bullet is optional because a
// hand-written frontmatter drops it as often as not. The character after the relation has to end it,
// or `blocks-after 3` would be read as `blocks 3`.
func linkEntryFor(line, relation string) string {
	entry := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	if !strings.HasPrefix(entry, relation) {
		return ""
	}
	rest := entry[len(relation):]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' && rest[0] != ':' {
		return ""
	}
	return rest
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
func (r *run) intentFileNumbered(relation, number string) (path, where string) {
	for _, dir := range []string{"intents", "archive"} {
		entries, err := os.ReadDir(r.idsdDir + "/" + dir)
		if err != nil {
			// Absent is "nothing here", which the caller reports as an edge naming no intent. Unreadable
			// is a different fact, and read as the same one it turns a real dependency into a bad link.
			if os.IsNotExist(err) {
				continue
			}
			// The question this leaves unanswered is the one the CALL SITE was asking, and the two
			// differ: depends-on wants the target built, the existence-only relations want it to be
			// there at all. Stating the weaker question at the depends-on site would understate what
			// an unreadable directory costs.
			unknown := "names a real intent"
			if relation == "depends-on" {
				unknown = "is built"
			}
			r.refuse("error: could not read " + r.idsdDir + "/" + dir + " (" + err.Error() + ") — whether " + relation + " " + number + " " + unknown + " is unknown")
		}
		for _, entry := range entries {
			name := entry.Name()
			// A ship is a folder now, and the intent inside it is always named the same — so the number is
			// matched against the folder rather than against a filename carrying the slug.
			intent := r.idsdDir + "/" + dir + "/" + name + "/" + intentName
			if strings.HasPrefix(name, number+"-") && shell.IsRegularFile(intent) {
				return intent, dir
			}
		}
	}
	return "", ""
}

// fieldValue with YAML's trailing comment removed, because the ICE template documents its own fields
// inline — `status:` carries its explanation, and a blanked value read raw is that explanation.
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

// Reverse edges. A sibling declaring `blocks <this intent>` has said it ships first, and its own
// frontmatter is the only place that edge exists — nothing obliges this intent to carry a mirroring
// `depends-on`, and in practice it does not. Archived siblings are built, so only `intents/` is scanned.
func (r *run) unbuiltBlockers(number string) []string {
	dir := r.idsdDir + "/intents"
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Absent means no siblings to read. Unreadable is a different fact, and read as the same one it
		// turns "someone declared they go first" into silence.
		if os.IsNotExist(err) {
			return nil
		}
		r.refuse("error: could not read " + dir + " (" + err.Error() + ") — whether a sibling blocks " + number + " is unknown")
	}
	var reasons []string
	for _, entry := range entries {
		sibling := entry.Name()
		if strings.HasPrefix(sibling, number+"-") {
			continue
		}
		path := dir + "/" + sibling + "/" + intentName
		// The guard cmdIntentReady gives the intent under judgement, for the same reason. Refused rather
		// than skipped — a skip drops a real blocker and clears the gate on a file nobody looked at.
		if shell.IsSymlink(path) {
			r.refuse("error: " + shell.Oneline(path) + " is a symlink — refusing to judge whether it blocks " + number + " through one")
		}
		if !shell.IsRegularFile(path) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			// Collapsed for the reason the block line below is: this path carries a ship FOLDER's name, and
			// nobody here chose it. The error text is collapsed too, because it quotes the same path back.
			r.refuse("error: " + shell.Oneline(path) + " cannot be read (" + shell.Oneline(err.Error()) + ") — whether it blocks " + number + " is unknown")
		}
		if !slices.Contains(linkNumbers(shell.SplitLines(string(content)), "blocks"), number) {
			continue
		}
		if status := yamlValue(path, "status"); status != "built" {
			reasons = append(reasons, "  "+shell.Oneline(sibling)+" declares blocks "+number+" and is "+shell.Oneline(status)+" — build that one first")
		}
	}
	return reasons
}

// The `NNN` a ship folder starts with, empty for a name that does not carry one. The trailing `-` is
// required: without it `29-base` and `290-base` both answer to a sibling's `blocks 29`.
func leadingNumber(name string) string {
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	if end == 0 || end >= len(name) || name[end] != '-' {
		return ""
	}
	return name[:end]
}
