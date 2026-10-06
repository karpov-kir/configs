package commentpass

import (
	"fmt"
	"regexp"
	"strings"
)

// userPrompt is one file's call: the file numbered, the material by id, the reviewer's notes on the
// file, and the reply's grammar. The page goes in as the system prompt.
func userPrompt(path string, lines []string, m material, notes []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "File %s, numbered:\n\n", path)
	for n, line := range lines {
		fmt.Fprintf(&b, "%4d  %s\n", n+1, line)
	}
	b.WriteString("\nThe change touched these. Decide each, and touch nothing else in the file.\n")
	for _, c := range m.candidates {
		on := "nothing"
		if c.decl > 0 {
			on = fmt.Sprintf("line %d: %s", c.decl, strings.TrimSpace(lines[c.decl-1]))
		}
		fmt.Fprintf(&b, "%s  the comment on lines %d-%d, which sits on %s\n", c.id, c.first, c.last, on)
	}
	for _, p := range m.places {
		fmt.Fprintf(&b, "%s  line %d, which has no comment: %s\n", p.id, p.line, strings.TrimSpace(lines[p.line-1]))
	}
	if len(notes) > 0 {
		b.WriteString("\nThe reviewer's notes on this file, each to act on:\n")
		for _, note := range notes {
			b.WriteString(note + "\n")
		}
	}
	b.WriteString(replyGrammar)
	return b.String()
}

// replyGrammar is the reply the tool parses. A decision that writes is followed by the comment's lines,
// in the file's comment syntax, up to the next id line.
const replyGrammar = `
Reply with one line per id above, and nothing else:
  <id> keep: <reason>
  <id> remove: <reason>
  <id> rewrite: <reason>
  <id> add: <reason>
  <id> skip: <reason>
c ids take keep, remove or rewrite; p ids take add or skip. After a rewrite or an add line, give the
comment's lines in this file's comment syntax, without indentation, and then the next id line. Give no
code, no fence and no other text.
`

// decision is one id's answer.
type decision struct {
	id     string
	verb   string
	reason string
	text   []string
}

var reDecision = regexp.MustCompile(`^([cp]\d+) (keep|remove|rewrite|add|skip): (.+)$`)

// parseReply reads the model's reply against the material. Every id is answered once with a verb its
// kind takes, and a writing verb carries text. Anything else refuses the file.
func parseReply(reply string, m material) ([]decision, error) {
	want := map[string]string{}
	for _, c := range m.candidates {
		want[c.id] = "c"
	}
	for _, p := range m.places {
		want[p.id] = "p"
	}
	var out []decision
	seen := map[string]bool{}
	for _, raw := range strings.Split(strings.TrimSpace(stripFence(reply)), "\n") {
		line := strings.TrimRight(raw, " \t\r")
		if d := reDecision.FindStringSubmatch(strings.TrimSpace(line)); d != nil {
			id, verb := d[1], d[2]
			kind, ok := want[id]
			switch {
			case !ok:
				return nil, fmt.Errorf("the reply answers %s, which the call did not offer", id)
			case seen[id]:
				return nil, fmt.Errorf("the reply answers %s twice", id)
			case kind == "c" && (verb == "add" || verb == "skip"), kind == "p" && (verb == "keep" || verb == "remove" || verb == "rewrite"):
				return nil, fmt.Errorf("the reply answers %s with %s, which a %s id does not take", id, verb, kind)
			}
			seen[id] = true
			out = append(out, decision{id: id, verb: verb, reason: d[3]})
			continue
		}
		if len(out) == 0 {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return nil, fmt.Errorf("the reply opens on %q, which is no id line", line)
		}
		last := &out[len(out)-1]
		if last.verb != "rewrite" && last.verb != "add" {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return nil, fmt.Errorf("%s %s carries text, which only rewrite and add take", last.id, last.verb)
		}
		last.text = append(last.text, line)
	}
	for id := range want {
		if !seen[id] {
			return nil, fmt.Errorf("the reply leaves %s unanswered", id)
		}
	}
	for i := range out {
		for len(out[i].text) > 0 && strings.TrimSpace(out[i].text[len(out[i].text)-1]) == "" {
			out[i].text = out[i].text[:len(out[i].text)-1]
		}
		if (out[i].verb == "rewrite" || out[i].verb == "add") && len(out[i].text) == 0 {
			return nil, fmt.Errorf("%s %s carries no comment text", out[i].id, out[i].verb)
		}
	}
	return out, nil
}

// stripFence takes off a code fence a reply may wrap itself in.
func stripFence(reply string) string {
	text := strings.TrimSpace(reply)
	if strings.HasPrefix(text, "```") {
		if end := strings.Index(text, "\n"); end >= 0 {
			text = text[end+1:]
		}
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	}
	return text
}
