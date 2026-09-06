// The cases that hold the two halves apart: the inventory comes from frontmatter and nothing else,
// and the narrative half cannot quietly outlive the skills it names.
//
// Every case builds its own root under t.TempDir() rather than reading ai/skills. A suite keyed on the
// real tree would go red every time someone edits a description, and Go's test cache cannot see files
// outside this module anyway — the gate unit is what reads the real tree.
package ecoguide

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A template holding every placeholder, so a case that cares about one is not also asserting the
// others exist. The narrative line is what the stale-name scan reads.
const fixtureTemplate = `<title>t</title>
<p>The pipeline runs idsd-ship, and kk-flavor is the bucket underneath.</p>
<div class="facts"><b>{{skill-count}}</b><b>{{family-count}}</b><b>{{human-typed-count}}</b></div>
<div class="lanes">
{{skill-inventory}}
</div>
`

type fixtureSkill struct {
	name        string
	frontmatter string
}

func newRoot(t *testing.T, template string, skills ...fixtureSkill) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"kk-flavor", "skills", "tools/eco-guide"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("fixture root: %v", err)
		}
	}
	for _, skill := range skills {
		dir := filepath.Join(root, "skills", skill.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("fixture skill %s: %v", skill.name, err)
		}
		body := "---\nname: " + skill.name + "\n" + skill.frontmatter + "---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatalf("fixture skill %s: %v", skill.name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, templateRelative), []byte(template), 0o644); err != nil {
		t.Fatalf("fixture template: %v", err)
	}
	return root
}

func run(t *testing.T, args ...string) (status int, output string) {
	t.Helper()
	var buffer bytes.Buffer
	status = Run("guide.sh", args, &buffer, &buffer)
	return status, buffer.String()
}

func generated(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, outputRelative))
	if err != nil {
		t.Fatalf("reading the generated guide: %v", err)
	}
	return string(body)
}

var shipped = fixtureSkill{"idsd-ship", "description: Ship one intent end-to-end.\nargument-hint: \"<arg> | done\"\n"}

// The description is the routing line each skill already writes about itself, so it reaches the page
// as written. A tool that summarised it would publish a second, vaguer claim about the same skill.
func TestADescriptionReachesThePageAsItsAuthorWroteIt(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped,
		fixtureSkill{"kk-build", "description: Take a settled requirement to a green tree — not the quality pass (kk-qualify).\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	want := "Take a settled requirement to a green tree — not the quality pass (kk-qualify)."
	if !strings.Contains(page, want) {
		t.Errorf("the page does not carry kk-build's own description\nwanted: %s\ngot:\n%s", want, page)
	}
	if !strings.Contains(page, `<code class="k">kk-build</code>`) {
		t.Errorf("the page does not name kk-build as an any-repo skill\n%s", page)
	}
	if !strings.Contains(page, "&lt;arg&gt; | done") {
		t.Errorf("the argument hint is missing or unescaped\n%s", page)
	}
}

// A YAML scalar that needed quoting arrives with the quotes and the backslashes still on it. Printed
// raw, the page shows a reader `"Build several \"at once\""`.
func TestAQuotedDescriptionLosesItsQuoting(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped,
		fixtureSkill{"kk-drive", `description: "Drive it — use for \"does this actually work?\"."` + "\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	if !strings.Contains(page, `Drive it — use for "does this actually work?".`) {
		t.Errorf("the quoted scalar was not unquoted\n%s", page)
	}
	if strings.Contains(page, `\"`) {
		t.Errorf("the page still carries YAML escapes\n%s", page)
	}
}

// The skills that maintain the instruction tree itself. To someone who just installed this they are
// noise, and a reader who reaches for one gets a skill that edits the skills.
func TestTheMaintainerOnlySkillsAreLeftOut(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped,
		fixtureSkill{"kk-ecosystem", "description: Refine what agents read.\naudience: maintainer\n"},
		fixtureSkill{"kk-skillcraft", "description: Review skills as skills.\naudience: maintainer\n"},
		fixtureSkill{"kk-reduce", "description: Shrink a whole ecosystem.\naudience: maintainer\n"},
		fixtureSkill{"kk-build", "description: Build it.\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	for _, excluded := range []string{"kk-ecosystem", "kk-skillcraft", "kk-reduce"} {
		if strings.Contains(page, excluded) {
			t.Errorf("%s reached an external reader's page\n%s", excluded, page)
		}
	}
	// Two skills counted, not four — the excluded ones must leave the figures too.
	if !strings.Contains(page, "<b>2</b>") {
		t.Errorf("the skill count still counts the maintainer-only skills\n%s", page)
	}
}

// The marker is what the exclusion should rest on, so it has to work before the three names above
// carry it. A skill declaring itself maintainer-only is left out whatever it is called.
func TestTheMaintainerMarkerExcludesASkillOnItsOwn(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped,
		fixtureSkill{"kk-tighten", "description: Tighten prose.\naudience: maintainer\n"},
		fixtureSkill{"kk-build", "description: Build it.\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if page := generated(t, root); strings.Contains(page, "kk-tighten") {
		t.Errorf("a skill marked audience: maintainer reached an external reader's page\n%s", page)
	}
}

// The frontmatter flag that drops a skill out of the router is the same fact a reader needs: nothing
// will reach for it on their behalf, so they have to type it.
func TestTheSkillsAHumanAlwaysTypesAreMarked(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped,
		fixtureSkill{"kk-foreman", "description: The front door.\ndisable-model-invocation: true\n"},
		fixtureSkill{"kk-build", "description: Build it.\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	foreman := cardFor(t, page, "kk-foreman")
	if !strings.Contains(foreman, "you type it") {
		t.Errorf("kk-foreman is not marked as one the human types\n%s", foreman)
	}
	if build := cardFor(t, page, "kk-build"); strings.Contains(build, "you type it") {
		t.Errorf("kk-build is marked as one the human types, and it is not\n%s", build)
	}
	if !strings.Contains(page, "<b>1</b>") {
		t.Errorf("the human-typed figure is not 1\n%s", page)
	}
}

func cardFor(t *testing.T, page, skill string) string {
	t.Helper()
	start := strings.Index(page, ">"+skill+"</code>")
	if start < 0 {
		t.Fatalf("%s has no card on the page\n%s", skill, page)
	}
	cardStart := strings.LastIndex(page[:start], `    <div class="lane">`)
	end := strings.Index(page[start:], "\n    </div>")
	if cardStart < 0 || end < 0 {
		t.Fatalf("%s's card has no boundaries\n%s", skill, page)
	}
	return page[cardStart : start+end]
}

// The gate's whole purpose. Regenerating into memory and finding the committed file already equal is
// the only clean answer; anything else has to be a failure that names the difference.
func TestCheckPassesOnlyWhenTheCommittedPageMatches(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped, fixtureSkill{"kk-build", "description: Build it.\n"})
	if status, output := run(t, root); status != 0 {
		t.Fatalf("writing the guide: exit %d\n%s", status, output)
	}

	status, output := run(t, "--check", root)
	if status != 0 {
		t.Fatalf("check on a freshly written guide: exit %d\n%s", status, output)
	}

	// The negative control: add a skill, and the committed page is now short of one.
	newRoot(t, fixtureTemplate) // no-op guard against the helper being the thing under test
	dir := filepath.Join(root, "skills", "kk-diagnose")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("adding a skill: %v", err)
	}
	body := "---\nname: kk-diagnose\ndescription: Find the cause.\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("adding a skill: %v", err)
	}

	status, output = run(t, "--check", root)
	if status != 1 {
		t.Fatalf("check over a stale guide: expected exit 1, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "kk-diagnose") {
		t.Errorf("the failure does not name the skill that changed\n%s", output)
	}
}

// A page nobody committed is the same staleness as a wrong one, and it must not read as clean.
func TestCheckFailsWhenNoPageIsCommitted(t *testing.T) {
	root := newRoot(t, fixtureTemplate, shipped, fixtureSkill{"kk-build", "description: Build it.\n"})
	status, output := run(t, "--check", root)
	if status != 1 {
		t.Fatalf("expected exit 1 for a missing page, got %d\n%s", status, output)
	}
	if !strings.Contains(output, outputRelative) {
		t.Errorf("the failure does not name the file that is missing\n%s", output)
	}
}

// The narrative is hand-written, so nothing regenerates it. What can be checked is that it never names
// a skill the tree no longer ships — the way a hand-maintained page goes wrong.
func TestTheNarrativeCannotNameASkillThatIsGone(t *testing.T) {
	root := newRoot(t, fixtureTemplate, fixtureSkill{"kk-build", "description: Build it.\n"})
	status, output := run(t, root)
	if status != 1 {
		t.Fatalf("expected exit 1 when the narrative names a missing skill, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "idsd-ship") {
		t.Errorf("the refusal does not name the stale mention\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(root, outputRelative)); err == nil {
		t.Error("a page was written despite the narrative naming a skill that is gone")
	}
}

// A placeholder someone deleted while editing the narrative would drop the whole inventory off the
// page, and the page would still look finished.
func TestAMissingPlaceholderIsRefused(t *testing.T) {
	withoutInventory := strings.Replace(fixtureTemplate, "{{skill-inventory}}", "", 1)
	root := newRoot(t, withoutInventory, shipped)
	status, output := run(t, root)
	if status != 1 {
		t.Fatalf("expected exit 1 for a template missing a placeholder, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "{{skill-inventory}}") {
		t.Errorf("the refusal does not name the missing placeholder\n%s", output)
	}
}

// The real template opens with a comment telling whoever edits it what the placeholders are — so the
// comment names every one of them. Filled rather than cut, it printed the whole inventory a second
// time and shipped the authoring notes to the reader, and the page looked finished both times.
func TestTheAuthoringCommentIsNotPartOfThePage(t *testing.T) {
	header := "<!--\n  Edit this, then rebuild. Placeholders: {{skill-count}} and {{skill-inventory}}.\n-->\n"
	root := newRoot(t, header+fixtureTemplate, shipped, fixtureSkill{"kk-build", "description: Build it.\n"})

	if status, output := run(t, root); status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	page := generated(t, root)
	if strings.Contains(page, "Edit this, then rebuild") {
		t.Errorf("the authoring comment reached the page\n%s", page)
	}
	if strings.Count(page, `>kk-build</code>`) != 1 {
		t.Errorf("kk-build is listed %d times, not once\n%s", strings.Count(page, `>kk-build</code>`), page)
	}
}

// Two inventories would list every skill twice, and the page would read as finished either way.
func TestASecondInventoryPlaceholderIsRefused(t *testing.T) {
	root := newRoot(t, fixtureTemplate+"{{skill-inventory}}\n", shipped)
	status, output := run(t, root)
	if status != 1 {
		t.Fatalf("expected exit 1 for a repeated inventory placeholder, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "twice") {
		t.Errorf("the refusal does not say what would go wrong\n%s", output)
	}
}

// An unresolved placeholder would ship `{{typo}}` to a reader.
func TestAnUnknownPlaceholderIsRefused(t *testing.T) {
	root := newRoot(t, fixtureTemplate+"<p>{{lane-count}}</p>\n", shipped)
	status, output := run(t, root)
	if status != 1 {
		t.Fatalf("expected exit 1 for an unknown placeholder, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "{{lane-count}}") {
		t.Errorf("the refusal does not name the unknown placeholder\n%s", output)
	}
}

// A root that is not a checkout, and a template that is not there, are both "it did not run".
func TestWhatCouldNotRunIsNotAPass(t *testing.T) {
	status, output := run(t, filepath.Join(t.TempDir(), "nowhere"))
	if status != 2 {
		t.Fatalf("expected exit 2 for a root that is not a checkout, got %d\n%s", status, output)
	}

	root := newRoot(t, fixtureTemplate, shipped)
	if err := os.Remove(filepath.Join(root, templateRelative)); err != nil {
		t.Fatalf("removing the template: %v", err)
	}
	status, output = run(t, root)
	if status != 2 {
		t.Fatalf("expected exit 2 with no template, got %d\n%s", status, output)
	}
	if !strings.Contains(output, templateRelative) {
		t.Errorf("the refusal does not name the template it could not read\n%s", output)
	}
}

// Two runs over one tree produce one page. Without that the gate reports drift on every commit.
func TestGenerationIsDeterministic(t *testing.T) {
	skills := []fixtureSkill{
		shipped,
		{"kk-build", "description: Build it.\n"},
		{"kk-drive", "description: Drive it.\n"},
		{"idsd-intent", "description: Author an intent.\n"},
	}
	root := newRoot(t, fixtureTemplate, skills...)
	if status, output := run(t, root); status != 0 {
		t.Fatalf("first run: exit %d\n%s", status, output)
	}
	first := generated(t, root)
	if status, output := run(t, root); status != 0 {
		t.Fatalf("second run: exit %d\n%s", status, output)
	}
	if second := generated(t, root); first != second {
		t.Error("two runs over one tree wrote two different pages")
	}
	// Grouped by family, workflow first, alphabetical inside each — so a card's position is a property
	// of the tree and not of the order the directory listing answered in.
	order := []string{"idsd-intent", "idsd-ship", "kk-build", "kk-drive"}
	at := -1
	for _, name := range order {
		next := strings.Index(first, ">"+name+"</code>")
		if next <= at {
			t.Fatalf("%s is out of order on the page\n%s", name, first)
		}
		at = next
	}
}
