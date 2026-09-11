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

// A worker as the page needs it. A worker carries no frontmatter, so there is no `description:` to
// quote the way a skill's card quotes one: what identifies it to a reader is its row's name, where
// the contract it dispatches is written, and the tier that row buys.
type worker struct {
	// The row's key in models.json, which for a worker holding its own prompt IS its path under
	// workers/ without `.md`. Keyed on the row rather than on the file because three of the four
	// forms a row resolves a prompt by own no file there, and a list built from the directory prices
	// fewer dispatch sites than the tree actually has.
	name string
	// Read off the name: a worker's own name carries no family prefix, so `idsd/` says the workflow
	// family, as does the `idsd-` prefix a row still keyed on a skill carries (ecosystem.md →
	// **Family direction**).
	family string
	// One sentence saying what this dispatch is for, from whichever of the prompt homes it has.
	summary string
	// Where the contract it runs is written, in the reader's own terms.
	prompt string
	// What the policy resolves for each client. Never empty: every row here came out of a parsed
	// policy, and model-policy refuses a client with neither a model nor an effort.
	claude string
	codex  string
}

// Every dispatch site the policy prices, ordered workflow family first and then alphabetically,
// matching the way the skills inventory prints.
//
// No audience exclusion, unlike readInventory. That filter answers "would this reader ever invoke
// it", and for a worker the answer is always no — nothing here has a door. What the list gives a
// reader is the tier map: what the tree spends when a skill hands a step away. A worker whose lane is
// maintainer-only is part of that bill like any other.
func readWorkers(root ecoroot.Root, rows []string, owners map[string]string, tiers func(task, client string) string) []worker {
	found := make([]worker, 0, len(rows))
	for _, name := range rows {
		summary, prompt := promptFor(root, name, owners)
		found = append(found, worker{
			name:    name,
			family:  workerFamily(name),
			summary: summary,
			prompt:  prompt,
			claude:  tiers(name, "claude"),
			codex:   tiers(name, "codex"),
		})
	}
	sort.Slice(found, func(a, b int) bool {
		if found[a].family != found[b].family {
			return found[a].family < found[b].family
		}
		return found[a].name < found[b].name
	})
	return found
}

// The four forms a row resolves its prompt by, in the order the policy's own checks take them: its
// own file under workers/, another row's prompt named in its `worker` field, a SKILL.md of its name
// during the migration window, and the one row whose prompt a Go tool assembles.
//
// The last branch is stated as what is observable — no prompt for this row is in the tree — rather
// than as "a tool assembles it". Only one row is legitimately in that state, and which rows may be
// is the policy census's question; a card asserting the legitimate reason would print that assertion
// over a row whose prompt file had simply gone missing, which is the one case a reader could
// otherwise catch here.
func promptFor(root ecoroot.Root, name string, owners map[string]string) (summary, prompt string) {
	file := shell.Join(shell.Join(root.Flavor(), "workers"), name+".md")
	if lines, err := readLines(file); err == nil {
		if brief := briefSummary(lines); brief != "" {
			return brief, "workers/" + name + ".md"
		}
	}
	if owner, named := owners[name]; named {
		ownerSummary, ownerPrompt := promptFor(root, owner, nil)
		return ownerSummary, ownerPrompt + ", dispatched as " + name
	}
	skill := shell.Join(shell.Join(root.Skills(), name), "SKILL.md")
	if lines, err := readLines(skill); err == nil {
		description := unquoteScalar(shell.FrontmatterDescription(lines))
		if stop := strings.Index(description, ". "); stop >= 0 {
			description = description[:stop+1]
		}
		if description != "" {
			return description, "skills/" + name + "/SKILL.md"
		}
	}
	return "No prompt in the tree holds this row's contract.", "not in the tree"
}

// A worker's family is the workflow one where its name says so — `idsd/` for a row holding its own
// prompt, `idsd-` for one still keyed on a skill — and any-repo otherwise. Keyed on the one prefix
// rather than on a list, so a row added under either spelling tomorrow is placed with no edit here.
func workerFamily(name string) string {
	if group, _, nested := strings.Cut(name, "/"); nested && group == "idsd" {
		return "idsd"
	}
	if strings.HasPrefix(name, "idsd-") {
		return "idsd"
	}
	return "kk"
}

// The first sentence of the brief's opening paragraph. Every worker opens `# <Name> brief` and then
// addresses the agent directly, so the first sentence is the one line that says what this agent is
// for. Taken up to the first sentence end rather than the whole paragraph, which runs to the return
// contract and is far longer than a card.
func briefSummary(lines []string) string {
	for i, line := range lines {
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		for _, next := range lines[i+1:] {
			next = strings.TrimSpace(next)
			if next == "" {
				continue
			}
			if stop := strings.Index(next, ". "); stop >= 0 {
				return next[:stop+1]
			}
			return next
		}
		return ""
	}
	return ""
}
