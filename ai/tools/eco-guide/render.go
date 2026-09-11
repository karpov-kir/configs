package ecoguide

import (
	"fmt"
	"sort"
	"strings"
)

// Every placeholder the template may carry. The set is closed in both directions: a template missing
// one silently drops that fact off the page, and one carrying a name not here would ship `{{typo}}` to
// a reader. Both are refused rather than rendered.
const (
	placeholderSkillCount  = "{{skill-count}}"
	placeholderFamilyCount = "{{family-count}}"
	placeholderTypedCount  = "{{human-typed-count}}"
	placeholderInventory   = "{{skill-inventory}}"
	placeholderWorkerCount = "{{worker-count}}"
	placeholderWorkers     = "{{worker-inventory}}"
)

var placeholders = []string{
	placeholderSkillCount,
	placeholderFamilyCount,
	placeholderTypedCount,
	placeholderInventory,
	placeholderWorkerCount,
	placeholderWorkers,
}

// The placeholders that must appear exactly once, because each prints a whole list and a template
// carrying one twice ships a page that reads as finished either way.
var placeholdersPrintedOnce = []string{placeholderInventory, placeholderWorkers}

// The CSS class each family's name is printed in, so the page's two colours mean the two families
// rather than being decoration. A family with no entry here prints unclassed and still reads fine.
var familyClass = map[string]string{"idsd": "i", "kk": "k"}

// render fills the hand-written template with the two generated inventories. Everything a reader
// reads as prose comes out of the template; everything about a particular skill comes out of its
// frontmatter, and everything about a particular worker out of its brief and its policy row.
// Nothing crosses that line, which is why a person editing the narrative never opens this file.
func render(template string, skills []skill, workers []worker) (string, error) {
	filled := stripAuthoringHeader(template)
	if err := checkNarrativeNames(filled, skills); err != nil {
		return "", err
	}
	for _, name := range placeholders {
		if !strings.Contains(filled, name) {
			return "", fmt.Errorf("the template carries no %s, so that part of the page would silently go missing", name)
		}
	}
	for _, name := range placeholdersPrintedOnce {
		if first := strings.Index(filled, name); strings.Contains(filled[first+len(name):], name) {
			return "", fmt.Errorf("the template carries %s more than once, so the page would list every entry twice", name)
		}
	}
	filled = strings.ReplaceAll(filled, placeholderSkillCount, fmt.Sprint(len(skills)))
	filled = strings.ReplaceAll(filled, placeholderFamilyCount, fmt.Sprint(countFamilies(skills)))
	filled = strings.ReplaceAll(filled, placeholderTypedCount, fmt.Sprint(countHumanTyped(skills)))
	filled = strings.ReplaceAll(filled, placeholderInventory, renderCards(skills))
	filled = strings.ReplaceAll(filled, placeholderWorkerCount, fmt.Sprint(len(workers)))
	filled = strings.ReplaceAll(filled, placeholderWorkers, renderWorkerCards(workers))
	if left := firstPlaceholder(filled); left != "" {
		return "", fmt.Errorf("the template carries %s, which nothing here fills — it would reach a reader as written", left)
	}
	return filled, nil
}

// The template opens with a comment addressed to whoever edits it — what the placeholders are, and
// what the generator will refuse. That is authoring guidance, not page content, so it is cut before
// anything else happens: it names every placeholder, and left in place the filler would have filled
// the documentation and printed the whole inventory a second time.
func stripAuthoringHeader(template string) string {
	if !strings.HasPrefix(template, "<!--") {
		return template
	}
	end := strings.Index(template, "-->")
	if end < 0 {
		return template
	}
	return strings.TrimLeft(template[end+len("-->"):], "\n")
}

func countFamilies(skills []skill) int {
	seen := map[string]bool{}
	for _, s := range skills {
		seen[s.family] = true
	}
	return len(seen)
}

func countHumanTyped(skills []skill) int {
	typed := 0
	for _, s := range skills {
		if s.humanTyped {
			typed++
		}
	}
	return typed
}

// One card per skill, under a heading per family. The heading is the family's prefix and nothing
// more: what a prefix means to a reader is stated once in the narrative, which is where a sentence
// about it can be edited without touching Go.
func renderCards(skills []skill) string {
	var out strings.Builder
	family := ""
	for _, s := range skills {
		if s.family != family {
			family = s.family
			fmt.Fprintf(&out, "    <div class=\"group-label\"><span>%s-*</span><i></i></div>\n", escapeText(family))
		}
		out.WriteString(card(s))
	}
	return strings.TrimRight(out.String(), "\n")
}

func card(s skill) string {
	var out strings.Builder
	out.WriteString("    <div class=\"lane\">\n")
	fmt.Fprintf(&out, "      <div class=\"lane-top\"><code class=\"%s\">%s</code>",
		familyClass[s.family], escapeText(s.name))
	if s.humanTyped {
		out.WriteString(`<span class="tag you">you type it</span>`)
	}
	out.WriteString("</div>\n")
	fmt.Fprintf(&out, "      <p>%s</p>\n", inlineCode(escapeText(s.description)))
	if s.argumentHint != "" {
		fmt.Fprintf(&out, "      <p class=\"hint\">takes: %s</p>\n", escapeText(s.argumentHint))
	}
	out.WriteString("    </div>\n")
	return out.String()
}

// Text-node escaping: the three bytes that could open a tag or an entity, and no others. Quotes and
// apostrophes are left alone deliberately — every description here is full of them, and `&#39;`
// through the source of a page someone may read as a file helps nobody.
func escapeText(text string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(text)
}

// A description writes a path or a flag between backticks, the way the rest of this tree does. Paired
// backticks become code spans; an odd one left over stays the character it is, since guessing where
// an unclosed span ends would move where the emphasis falls.
func inlineCode(text string) string {
	parts := strings.Split(text, "`")
	if len(parts)%2 == 0 {
		return text
	}
	var out strings.Builder
	for i, part := range parts {
		if i%2 == 1 {
			out.WriteString("<code>" + part + "</code>")
			continue
		}
		out.WriteString(part)
	}
	return out.String()
}

func firstPlaceholder(text string) string {
	open := strings.Index(text, "{{")
	if open < 0 {
		return ""
	}
	close := strings.Index(text[open:], "}}")
	if close < 0 {
		return text[open:min(open+40, len(text))]
	}
	return text[open : open+close+2]
}

// The narrative half is hand-written, so nothing regenerates it — but it can still be held to the
// tree. Every skill it names by hand has to be one the page actually lists. That is how a
// hand-maintained catalogue goes wrong: a skill is renamed or retired, and the prose keeps sending
// readers to a name nothing answers to.
//
// A name the exclusion above removed fails here too, and should: the narrative must not point an
// external reader at a skill their copy of the page never lists.
func checkNarrativeNames(template string, skills []skill) error {
	listed := map[string]bool{}
	for _, s := range skills {
		listed[s.name] = true
	}
	var stale []string
	seen := map[string]bool{}
	for _, name := range skillNamesIn(template) {
		if listed[name] || notASkill[name] || seen[name] {
			continue
		}
		seen[name] = true
		stale = append(stale, name)
	}
	if len(stale) == 0 {
		return nil
	}
	sort.Strings(stale)
	return fmt.Errorf("the narrative names %s, which the page does not list — the skill was renamed, "+
		"retired, or is one an external reader should not be sent to; edit the template", strings.Join(stale, ", "))
}

// Names that open with a family prefix and are not skills. `kk-flavor` is the bucket the standards
// live in, which the narrative names on purpose.
var notASkill = map[string]bool{"kk-flavor": true}

// Every `kk-<word>` and `idsd-<word>` in the template. A match must start a word, which is what keeps
// the CSS custom properties out: `--idsd-soft` and `--kk-edge` are preceded by a dash, and reading
// them as skill names would make every template stale on sight.
func skillNamesIn(template string) []string {
	var names []string
	for _, prefix := range []string{"kk-", "idsd-"} {
		at := 0
		for {
			found := strings.Index(template[at:], prefix)
			if found < 0 {
				break
			}
			start := at + found
			at = start + len(prefix)
			if start > 0 && isNameByte(template[start-1]) {
				continue
			}
			end := at
			for end < len(template) && isNameByte(template[end]) {
				end++
			}
			if end == at {
				continue
			}
			names = append(names, strings.TrimRight(template[start:end], "-"))
		}
	}
	return names
}

func isNameByte(b byte) bool {
	return b == '-' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// One card per worker, under a heading per family, matching the skills inventory's shape so the two
// lists read as two halves of one tree.
func renderWorkerCards(workers []worker) string {
	var out strings.Builder
	family := ""
	for _, w := range workers {
		if w.family != family {
			family = w.family
			fmt.Fprintf(&out, "    <div class=\"group-label\"><span>%s workers</span><i></i></div>\n", escapeText(family))
		}
		out.WriteString(workerCard(w))
	}
	return strings.TrimRight(out.String(), "\n")
}

// The tier is printed for both clients because the two are set independently and a reader comparing
// cost needs the pair. Neither can be blank: every row here came out of a parsed policy, and
// model-policy's validateSettings refuses a client with neither a model nor an effort — so there is
// no guard for it in this package, which would be a second answer to a question already settled.
//
// Where the prompt lives is printed beside it, because a reader who has just been told this row is
// what the tree spends will want to read the contract being bought, and for most of these rows that
// is not a file named after the row.
func workerCard(w worker) string {
	var out strings.Builder
	out.WriteString("    <div class=\"lane\">\n")
	fmt.Fprintf(&out, "      <div class=\"lane-top\"><code class=\"%s\">%s</code><span class=\"tag\">dispatched</span></div>\n",
		familyClass[w.family], escapeText(w.name))
	fmt.Fprintf(&out, "      <p>%s</p>\n", inlineCode(escapeText(w.summary)))
	fmt.Fprintf(&out, "      <p class=\"hint\">tier: claude %s &middot; codex %s &middot; prompt: %s</p>\n",
		escapeText(w.claude), escapeText(w.codex), escapeText(w.prompt))
	out.WriteString("    </div>\n")
	return out.String()
}
