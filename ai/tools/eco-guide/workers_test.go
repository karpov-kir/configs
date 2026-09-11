// The worker half of the inventory: one card per row the model policy prices, built from the prompt
// that row dispatches rather than from any frontmatter, since a worker declares none. The skills half
// and the page-level gates are in guide_test.go; these share that file's harness and add the two
// fixtures only the worker layer needs.
package ecoguide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The smallest policy the parser accepts, holding a row for every worker the fixtures write. The
// guide resolves its tiers through that parser, so a fixture policy is not optional scaffolding —
// without one the tool refuses, which is the behaviour TestAGuideWithNoPolicyRefuses pins.
const fixturePolicy = `{
  "version": 3,
  "limits": { "intents-in-flight": 3 },
  "sessions": { "idsd-ship": { "codex": { "model": "gpt-5.6-terra", "effort": "low" }, "claude": { "model": "sonnet" } } },
  "workers": {
    "code-review": { "codex": { "model": "gpt-6-astra", "effort": "high" }, "claude": { "model": "opus" } },
    "idsd/audit":  { "codex": { "model": "gpt-5.6-terra", "effort": "low" }, "claude": { "model": "sonnet" } },
    "cheap":       { "codex": { "model": "gpt-5.6-luna", "effort": "low" }, "claude": { "model": "haiku" } },
    "borrower":    { "codex": { "model": "gpt-6-astra", "effort": "high" }, "claude": { "model": "opus" }, "worker": "code-review" },
    "toolbuilt":   { "codex": { "model": "gpt-5.6-luna", "effort": "low" }, "claude": { "model": "haiku" } }
  }
}
`

// A worker as the tree holds one: `# <Name> brief` and then the sentence the card quotes.
type fixtureWorker struct {
	name  string
	brief string
}

// The worker every root carries unless a case names its own, so the common case does not restate the
// worker layer to assert something about skills.
var reviewer = fixtureWorker{"code-review", "You are one correctness review. The rest is the contract.\n"}

func writeWorkers(t *testing.T, root string, workers ...fixtureWorker) {
	t.Helper()
	for _, worker := range workers {
		file := filepath.Join(root, "kk-flavor", "workers", filepath.FromSlash(worker.name)+".md")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatalf("fixture worker %s: %v", worker.name, err)
		}
		body := "# " + worker.name + " brief\n\n" + worker.brief
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatalf("fixture worker %s: %v", worker.name, err)
		}
	}
}

// The worker inventory's card for one row. A name can carry a card in both inventories — a row still
// keyed on a skill is exactly that case — and the skills half prints first, so the page is cut to the
// worker layer before cardFor's markup-boundary lookup runs. Cut on the group label rather than on a
// count of `lanes` divs, which the narrative template is free to add more of.
func workerCardFor(t *testing.T, page, name string) string {
	t.Helper()
	layer := strings.Index(page, " workers</span>")
	if layer < 0 {
		t.Fatalf("the page carries no worker inventory\n%s", page)
	}
	return cardFor(t, page[layer:], name)
}

// A worker has no frontmatter, so its card is built from two sources that cannot be edited together:
// the brief's own first sentence, and the tier the model policy resolves for it. Both halves are
// asserted here, because a card carrying the name alone would look generated and say nothing.
func TestAWorkerCardCarriesItsBriefAndTheTierItsRowBuys(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped)

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	for _, want := range []string{
		`<code class="k">code-review</code>`,
		"You are one correctness review.",
		"tier: claude opus &middot; codex gpt-6-astra high",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the worker card is missing %q\n%s", want, page)
		}
	}
	// The sentence after the first one is the contract, which belongs in the brief and not on a card.
	if strings.Contains(page, "The rest is the contract.") {
		t.Errorf("the card printed past the brief's first sentence\n%s", page)
	}
}

// The tier is resolved through the policy rather than read off the worker, so a row that assigns a
// different model moves the page with no edit to the brief. Without this the card could be printing
// a constant that happens to match.
func TestAWorkerCardFollowsThePolicyRatherThanTheBrief(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped)
	writeWorkers(t, root, fixtureWorker{"unpriced", "You are the cheap one.\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	if !strings.Contains(page, "tier: claude haiku &middot; codex gpt-5.6-luna low") {
		t.Errorf("the cheap row's own tier did not reach its card\n%s", page)
	}
}

// A worker's name carries no family prefix, so its path is the only thing saying which family it is
// in. Grouping on the path is what keeps `workers/idsd/` with the workflow family.
func TestAWorkerTakesItsFamilyFromItsPath(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped)
	writeWorkers(t, root, fixtureWorker{"idsd/audit", "You are one audit of a whole intent set.\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	if !strings.Contains(page, `<code class="i">idsd/audit</code>`) {
		t.Errorf("the nested worker was not placed in the workflow family\n%s", page)
	}
	if !strings.Contains(page, "<span>idsd workers</span>") || !strings.Contains(page, "<span>kk workers</span>") {
		t.Errorf("the two worker families are not both labelled\n%s", page)
	}
}

// The policy is an input the page cannot be generated without, so a tree with no usable one refuses
// rather than shipping a page whose every tier reads `—`. Exit 2: a check that did not run is not a
// clean one. Two ways to have no policy, asserted on their own messages, because they refuse from
// different branches and a single case would leave whichever branch it did not reach unproven.
func TestAGuideWithNoUsablePolicyRefuses(t *testing.T) {
	for name, wreck := range map[string]func(string) error{
		"no policy at all": func(path string) error { return os.Remove(path) },
		"a policy that does not parse": func(path string) error {
			return os.WriteFile(path, []byte("{\"version\": 3, \"sessions\": {}}\n"), 0o644)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := newRoot(t, fixtureTemplate, shipped)
			if err := wreck(filepath.Join(root, "kk-flavor", "models.json")); err != nil {
				t.Fatalf("wrecking the fixture policy: %v", err)
			}

			status, output := run(t, root)
			if status != 2 {
				t.Fatalf("expected exit 2, got %d\n%s", status, output)
			}
			if !strings.Contains(output, "the model policy at") || !strings.Contains(output, "NOT generated") {
				t.Errorf("the refusal does not name the policy as the reason\n%s", output)
			}
		})
	}
}

// Both inventories are held to appearing once, not just the skills one: a template carrying the
// worker placeholder twice would print the whole layer twice and read as finished either way.
func TestASecondWorkerInventoryPlaceholderIsRefused(t *testing.T) {
	root := newRoot(t, fixtureTemplate+"\n{{worker-inventory}}\n", shipped)

	status, output := run(t, root)
	if status != 1 {
		t.Fatalf("expected exit 1 for a repeated worker placeholder, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "more than once") {
		t.Errorf("the refusal does not name the repetition\n%s", output)
	}
}

// The list is keyed on the policy's rows, not on the files under workers/. Only one of the four ways
// a row resolves its prompt leaves a file there, so a list built from the directory would price
// fewer dispatch sites than the tree has — and would do it silently, since every card it did print
// would look right.
func TestEveryRowGetsACardEvenWithNoFileOfItsOwn(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped)

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	for _, want := range []string{
		`<code class="k">code-review</code>`, // its own file
		`<code class="k">borrower</code>`,    // another row's prompt
		`<code class="k">toolbuilt</code>`,   // no prompt in the tree at all
		`<code class="k">cheap</code>`,
		`<code class="i">idsd/audit</code>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the dispatch site %q has no card\n%s", want, page)
		}
	}
	if got := strings.Count(page, `<span class="tag">dispatched</span>`); got != 5 {
		t.Errorf("cards printed = %d, want one per worker row (5)\n%s", got, page)
	}
}

// A card says where the contract it buys is written, because for most rows that is not a file named
// after the row. A reader told this list is the tree's cost needs to be able to go and read what is
// being bought.
func TestACardNamesWhereItsPromptLives(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped)

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	for _, want := range []string{
		"prompt: workers/code-review.md",
		"prompt: workers/code-review.md, dispatched as borrower",
		"prompt: not in the tree",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("no card names the prompt home %q\n%s", want, page)
		}
	}
	// A row running another's prompt describes that prompt, not itself — it has nothing of its own.
	if got := strings.Count(page, "You are one correctness review."); got != 2 {
		t.Errorf("the borrowed contract's summary appears %d times, want 2\n%s", got, page)
	}
}

// A row whose prompt file has gone missing must not read like the one row that legitimately holds no
// prompt in the tree. Both say what is observable — nothing here holds this contract — so the page
// never asserts a reason it cannot check, and the reader can see which rows are in that state.
func TestARowWhoseFileIsGoneSaysSoRatherThanClaimingATool(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped)
	if err := os.Remove(filepath.Join(root, "kk-flavor", "workers", "code-review.md")); err != nil {
		t.Fatalf("removing the fixture worker: %v", err)
	}

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	if strings.Contains(page, "You are one correctness review.") {
		t.Errorf("a deleted prompt still reached the page\n%s", page)
	}
	// The row and the one borrowing its prompt both lose their summary; the fixture's other three
	// rows never had a file, so every row now reads the same way — which is the point. Nothing on the
	// page offers a reason for it, because the page cannot tell a deletion from a tool-built row.
	if got := strings.Count(page, "No prompt in the tree holds this row's contract."); got != 5 {
		t.Errorf("rows reading as prompt-less = %d, want all 5\n%s", got, page)
	}
	for _, claim := range []string{"assembled", "assembled in Go", "by the tool"} {
		if strings.Contains(page, claim) {
			t.Errorf("the page asserts %q about a row whose prompt it merely could not find\n%s", claim, page)
		}
	}
}

// A row still keyed on a skill is part-way through moving, and its card reads that skill's own
// description rather than going blank.
func TestARowStillPointingAtASkillReadsThatSkill(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped,
		fixtureSkill{"kk-edit", "description: Cut outward text to what it must say. Nothing else.\n"})
	// Keyed on the map's own opening rather than on a sibling row: keyed on a neighbour, renaming
	// that neighbour makes this Replace a silent no-op, the policy still parses, and the case reds
	// over a row it never added.
	policy := strings.Replace(fixturePolicy, `"workers": {`,
		`"workers": {
    "kk-edit": { "codex": { "model": "gpt-5.6-terra", "effort": "low" }, "claude": { "model": "sonnet" } },`, 1)
	if err := os.WriteFile(filepath.Join(root, "kk-flavor", "models.json"), []byte(policy), 0o644); err != nil {
		t.Fatalf("fixture policy: %v", err)
	}

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	if !strings.Contains(page, "prompt: skills/kk-edit/SKILL.md") {
		t.Errorf("the migrating row does not name the skill holding its prompt\n%s", page)
	}
	card := workerCardFor(t, page, "kk-edit")
	if !strings.Contains(card, "Cut outward text to what it must say.") {
		t.Errorf("the migrating row did not take the skill's own description\n%s", card)
	}
	if strings.Contains(card, "Nothing else.") {
		t.Errorf("the card printed past the description's first sentence\n%s", card)
	}
}
