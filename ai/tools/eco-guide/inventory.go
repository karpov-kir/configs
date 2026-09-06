package ecoguide

import (
	"os"
	"sort"
	"strings"

	ecoroot "kk-flavor/tools/eco-root"
	"kk-flavor/tools/shell"
)

// A skill as the page needs it, and nothing else. Every field is read off the skill's own frontmatter,
// which is the same block the loader routes on — so the page cannot describe a skill differently from
// the way it is actually reached.
type skill struct {
	name string
	// The prefix before the first dash: `kk` for the any-repo family, `idsd` for the workflow one.
	// ecosystem.md → **Conventions a new file joins** makes that prefix the contract, so the page can
	// group on it without a table in here.
	family string
	// The routing line, verbatim. Each one already says what the skill is and what it is not, which is
	// exactly what a reader choosing between two neighbours needs.
	description string
	// What the skill expects as an argument, empty where it declares none.
	argumentHint string
	// `disable-model-invocation: true` — nothing will reach for this skill on the reader's behalf, so
	// the reader types it.
	humanTyped bool
}

// Every skill an external reader should see, ordered the way the page prints them: workflow family
// first, then any-repo, alphabetical inside each. The order is a property of the tree, so two runs
// over one tree produce one page and the gate has something stable to diff.
//
// A directory with no readable SKILL.md, and one whose frontmatter declares no description, are both
// skipped rather than printed empty: eco-check is what reports those, and a card saying nothing about
// a skill is worse for a reader than no card.
//
// A skill declaring `audience: maintainer` is left out. Those exist to maintain this instruction tree,
// so to someone who just installed it they are noise, and one they reach for edits the skills rather
// than their code. The marker is the only thing deciding it — no list of names here — so a skill
// joining or leaving that set is one frontmatter line and no change to this tool.
func readInventory(root ecoroot.Root) ([]skill, error) {
	entries, err := os.ReadDir(root.Skills())
	if err != nil {
		return nil, err
	}
	var found []skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lines, err := readLines(shell.Join(shell.Join(root.Skills(), entry.Name()), "SKILL.md"))
		if err != nil || shell.IsMaintainerAudience(lines) {
			continue
		}
		description := unquoteScalar(shell.FrontmatterDescription(lines))
		if description == "" {
			continue
		}
		found = append(found, skill{
			name:         entry.Name(),
			family:       familyOf(entry.Name()),
			description:  description,
			argumentHint: unquoteScalar(shell.FrontmatterValue(lines, "argument-hint")),
			humanTyped:   shell.IsOptedOutOfModelInvocation(lines),
		})
	}
	sort.Slice(found, func(a, b int) bool {
		if found[a].family != found[b].family {
			return found[a].family < found[b].family
		}
		return found[a].name < found[b].name
	})
	return found, nil
}

func familyOf(name string) string {
	prefix, _, found := strings.Cut(name, "-")
	if !found {
		return name
	}
	return prefix
}

func readLines(path string) ([]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(body), "\n"), nil
}

// A YAML scalar as a person wrote it. A description holding a colon or a leading quote has to be
// quoted in the file, and printed raw it reaches the page as `"Ship it — use for \"ship it\"."`.
//
// Only the two quoting forms YAML actually uses here, and only when the quote closes the value: an
// unbalanced quote is left exactly as written rather than half-stripped, because a value nobody can
// parse must not be reported as one that parsed.
func unquoteScalar(value string) string {
	value = strings.TrimRight(value, " \t")
	if len(value) < 2 {
		return value
	}
	switch {
	case strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		inner := value[1 : len(value)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		return strings.ReplaceAll(inner, `\\`, `\`)
	case strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'"):
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}
