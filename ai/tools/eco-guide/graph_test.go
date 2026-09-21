// The two emitters: the workflow map and one skill's per-run tier profile. What these hold is the
// pricing rule — which edges spend a row and which do not — because that rule is the only thing
// either answer is made of and it is invisible in the output once it is wrong.
//
// Own fixture tree and own policy, like every case beside them: a suite keyed on the real tree would
// redden whenever someone moved a citation, and the tree-against-policy census lives in
// model-policy's tree_test.go, which is the check that is supposed to fail on that.
package ecoguide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A policy with both homes filled and one row in each place the classifier has to tell apart: a
// plain worker, a worker row keyed on a skill that kept its door, a skill, and a session row keyed
// under a skill, which is that skill's own cheaper mode.
const graphPolicy = `{
  "version": 4,
  "limits": { "intents-in-flight": 3 },
  "tiers": {
    "codex":  ["gpt-5.6-luna", "gpt-5.6-terra", "gpt-6-astra"],
    "claude": ["haiku", "sonnet", "opus"]
  },
  "sessions": {
    "idsd-ship":    { "codex": { "model": "gpt-6-astra", "effort": "high" }, "claude": { "model": "opus" } },
    "kk-qualify":   { "codex": { "model": "gpt-5.6-terra", "effort": "low" }, "claude": { "model": "sonnet" } },
    "kk-pr":        { "codex": { "model": "gpt-6-astra", "effort": "high" }, "claude": { "model": "opus" } },
    "kk-pr/refine": { "codex": { "model": "gpt-5.6-luna", "effort": "low" }, "claude": { "model": "haiku" } }
  },
  "workers": {
    "code-review":    { "codex": { "model": "gpt-6-astra", "effort": "high" }, "claude": { "model": "opus" } },
    "refactor":       { "codex": { "model": "gpt-5.6-terra", "effort": "low" }, "claude": { "model": "sonnet" } },
    "build/explore":  { "codex": { "model": "gpt-5.6-luna", "effort": "low" }, "claude": { "model": "haiku" } },
    "kk-edit":        { "codex": { "model": "gpt-5.6-luna", "effort": "low" }, "claude": { "model": "haiku" } }
  }
}
`

// A root holding the fixture policy, whatever skill files a case writes, and the worker prompts
// behind the rows. Separate from newRoot because these cases care about the citations inside a
// skill's files and not at all about its frontmatter, which is the only thing newRoot varies.
func newGraphRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"kk-flavor/skills", "kk-flavor/workers", "tools/eco-guide"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("fixture root: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "kk-flavor", "models.json"), []byte(graphPolicy), 0o644); err != nil {
		t.Fatalf("fixture policy: %v", err)
	}
	// A template, so the page path stays runnable from this root too and a case can assert that the
	// emitters do not need it by deleting it.
	if err := os.WriteFile(filepath.Join(root, templateRelative), []byte(fixtureTemplate), 0o644); err != nil {
		t.Fatalf("fixture template: %v", err)
	}
	writeWorkers(t, root, fixtureWorker{"code-review", "one review\n"}, fixtureWorker{"refactor", "one refactor\n"},
		fixtureWorker{"build/explore", "one exploration\n"}, fixtureWorker{"kk-edit", "one edit pass\n"})
	for name, body := range files {
		file := filepath.Join(root, "kk-flavor", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatalf("fixture file %s: %v", name, err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatalf("fixture file %s: %v", name, err)
		}
	}
	return root
}

// A SKILL.md body: the declaration the emitters read, then whatever the case wants cited.
func skillBody(mode, body string) string {
	return "---\nname: x\ndescription: One line.\n---\n\n**Runs:** " + mode + "\n\n" + body + "\n"
}

// The same, declaring which contract this skill reads as its own delta. Separate from skillBody so a
// case that does not care about extension cannot accidentally declare one — which is the whole point
// of the declaration: nothing is an extension unless it says so. The `— <when>` the grammar requires
// is supplied here rather than by every case, because no case turns on what it says.
func skillExtending(mode, extends, body string) string {
	return "---\nname: x\ndescription: One line.\n---\n\n**Runs:** " + mode +
		"\n\n**Extends:** " + extends + " — the phase that reads it\n\n" + body + "\n"
}

// The block the graph prints for one name, from its heading to the blank line that ends it. Cut this
// way rather than by counting lines, so a case asserting one node's edges does not also pin how many
// edges its neighbours have.
func graphBlock(t *testing.T, output, name string) string {
	t.Helper()
	at := strings.Index(output, "\n"+name+"  [")
	if at < 0 {
		at = strings.Index(output, "\n"+name+"\n")
	}
	if at < 0 {
		t.Fatalf("the map names no node %q\n%s", name, output)
	}
	rest := output[at+1:]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}

// The classifier's whole job, written the way the tree writes it. `kk-edit` has a `workers` row and
// a `skills/` path, the shape the three door-keeping lanes have, and `kk-qualify` names all three in
// column two of its Lanes table **by bare name** — the spelling that table defines as "a door you
// invoke". This case used to spell that edge `~/.kk-flavor/skills/kk-edit/SKILL.md`, which occurs
// zero times in the tree. The fixture matched no file in the repository, so the rule it named went
// unexercised. Two of the three appeared in the map anyway, off column *three*'s `scripts/*.sh`
// path cut at the first slash, and `kk-diagnose` — which owns no script — appeared nowhere at all.
func TestASkillsPathWithAWorkerRowIsADispatchAndNotARead(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", strings.Join([]string{
			"| Lane | What fills it | Scanner to run |",
			"|---|---|---|",
			"| edit | `kk-edit` | `~/.kk-flavor/skills/kk-edit/scripts/voice-check.sh` |",
			"| code-review | `~/.kk-flavor/workers/code-review.md` | — |",
		}, "\n")),
		"skills/kk-pr/SKILL.md": skillBody("holds — landing", "The pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	block := graphBlock(t, output, "kk-qualify")
	if !strings.Contains(block, "dispatches kk-edit") {
		t.Errorf("a lane named by bare name in the Lanes table is a dispatch, and the map missed it\n%s", block)
	}
	if strings.Contains(block, "reads      kk-edit") {
		t.Errorf("kk-edit was classified by its directory rather than by the map its row is in\n%s", block)
	}
	if !strings.Contains(block, "dispatches code-review") {
		t.Errorf("the table's worker-path rows stopped being read\n%s", block)
	}
	// The other half of the same rule, so this case cannot pass by calling every edge a dispatch.
	if !strings.Contains(graphBlock(t, output, "kk-pr"), "reads      kk-qualify") {
		t.Errorf("a skills path with a sessions row is a read, and the map says otherwise\n%s", output)
	}
}

// The script path in the row above must not be what produced that edge. A lane's scripts are cited
// as `~/.kk-flavor/skills/<skill>/scripts/<x>.sh` from 18 sites in the tree, and reading the first
// segment of one put an opus dispatch on the bill of every skill that runs a shell script.
func TestASkillsScriptPathIsNoEdge(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md": skillBody("holds — landing",
			"Run `~/.kk-flavor/skills/kk-edit/scripts/voice-check.sh` over the diff."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if block := graphBlock(t, output, "kk-pr"); strings.Contains(block, "kk-edit") {
		t.Errorf("a lane's script was priced as a dispatch of the lane that owns it\n%s", block)
	}
}

// A `workers/` path that is not a prompt is a script the worker owns. It costs nothing of its own —
// it runs inside the dispatch that already paid for the prompt beside it — and an edge here would
// put a row on the bill that the policy does not even have.
func TestAWorkersPathThatNamesNoPromptIsNoEdge(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator",
			"Run `~/.kk-flavor/workers/refactor/dup-literals.sh` before the lane."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if block := graphBlock(t, output, "kk-qualify"); strings.Contains(block, "refactor") {
		t.Errorf("a script beside a worker was counted as a dispatch of that worker\n%s", block)
	}
}

// A skill's modes are files beside its SKILL.md, read inline by the same session. What `review.md`
// dispatches is what `kk-pr` dispatches, so a scan that stopped at SKILL.md would price the mode
// that does the most work as reaching nothing at all.
func TestACitationInASiblingFileIsTheSkillsOwnEdge(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md":  skillBody("holds — landing", "Modes are beside this file."),
		"skills/kk-pr/review.md": "The review dispatches `~/.kk-flavor/workers/code-review.md` over the diff.\n",
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if block := graphBlock(t, output, "kk-pr"); !strings.Contains(block, "dispatches code-review") {
		t.Errorf("a mode file's dispatch is its skill's dispatch, and the map missed it\n%s", block)
	}
}

// Bare prose naming a skill routes a human to a door; it is not this skill reading that contract.
// Counted, kk-foreman — which is a routing table and almost nothing else — would carry the whole
// tree on its profile, and every cost figure downstream of a router would be wrong upward.
func TestASkillNamedInBareProseIsNoEdge(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md": skillBody("holds — landing",
			"| A PR to review | kk-qualify, alone |\nThen kk-edit over the text."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	block := graphBlock(t, output, "kk-pr")
	if strings.Contains(block, "kk-qualify") || strings.Contains(block, "kk-edit") {
		t.Errorf("a name in prose was read as an edge, so a router would carry the tree\n%s", block)
	}
}

// The pricing asymmetry, and the reason `--cost` exists at all. Extension runs the second contract
// inside the first session, so what that contract dispatches is on this skill's bill; a dispatch is
// already priced at the row on the line, so folding in what it dispatches would charge one run
// twice. Both halves are here, because a walk that followed everything and one that followed
// nothing each pass half of this.
func TestCostFollowsAnExtensionAndStopsAtADispatch(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/idsd-ship/SKILL.md": skillExtending("holds — converses", "kk-qualify",
			"Its delta over `~/.kk-flavor/skills/kk-qualify/SKILL.md`."),
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator",
			"Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})
	// Reached only through a dispatch, so what it dispatches must NOT appear on idsd-ship's bill.
	writeWorkers(t, root, fixtureWorker{"code-review",
		"One review. Dispatch `~/.kk-flavor/workers/build/explore.md` when the cause is unknown.\n"})

	status, output := run(t, "--cost", "idsd-ship", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "dispatch code-review") {
		t.Errorf("a read's dispatches bill at the reading session, and this one is missing\n%s", output)
	}
	if strings.Contains(output, "build/explore") {
		t.Errorf("the walk carried on through a dispatch, charging one run for a second run's rows\n%s", output)
	}
	if !strings.Contains(output, "session  idsd-ship") {
		t.Errorf("the profile does not price the session itself\n%s", output)
	}
}

// Where a row came from is most of what makes the profile readable: a bill a reader did not expect
// is nearly always one their skill inherited, and naming the chain is what lets them decide whether
// the extension or the tier is the thing to change.
func TestCostNamesTheChainAnInheritedRowArrivedThrough(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/idsd-ship/SKILL.md": skillExtending("holds — converses", "kk-pr",
			"Its delta over `~/.kk-flavor/skills/kk-pr/SKILL.md`."),
		"skills/kk-pr/SKILL.md": skillExtending("holds — landing", "kk-qualify",
			"The pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`."),
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/refactor.md`."),
	})

	status, output := run(t, "--cost", "idsd-ship", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "(through kk-pr → kk-qualify, which it extends)") {
		t.Errorf("the chain a row arrived through is missing, so an unexpected bill names no cause\n%s", output)
	}
}

// Two skills that read each other is an ordinary shape — a pipeline and its stage each orient the
// reader toward the other — so the walk has to end on its own rather than on a depth limit somebody
// tuned.
func TestAnExtensionCycleTerminates(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md":      skillExtending("holds — landing", "kk-qualify", "See `~/.kk-flavor/skills/kk-qualify/SKILL.md`."),
		"skills/kk-qualify/SKILL.md": skillExtending("orchestrator", "kk-pr", "Back to `~/.kk-flavor/skills/kk-pr/SKILL.md`, and dispatch `~/.kk-flavor/workers/refactor.md`."),
	})

	status, output := run(t, "--cost", "kk-pr", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "dispatch refactor") {
		t.Errorf("the walk did not reach past the cycle\n%s", output)
	}
}

// A session row keyed under a skill is that skill's cheaper mode, which has no SKILL.md and declares
// nothing on purpose. Labelled as a skill that forgot its line, the map reports the tree's own
// design as a defect on every single run, and a real missing declaration stops standing out.
func TestASessionRowKeyedUnderASkillReadsAsThatSkillsMode(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md": skillBody("holds — landing", "Modes are priced apart."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	block := graphBlock(t, output, "kk-pr/refine")
	if !strings.Contains(block, "[mode of kk-pr]") {
		t.Errorf("a mode row is not a skill that forgot its declaration\n%s", block)
	}
	if strings.Contains(block, "declares nothing") {
		t.Errorf("the mode row was reported as an undeclared skill\n%s", block)
	}
}

// A skill with no readable declaration is the case the label above must not swallow — the ceiling
// reads that line, and a skill whose silence goes unremarked is one nothing can ask anything.
func TestASkillWithNoDeclarationSaysSoRatherThanGoingBlank(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md": "---\nname: kk-pr\ndescription: One line.\n---\n\nNo declaration here.\n",
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(graphBlock(t, output, "kk-pr"), "declares nothing") {
		t.Errorf("a skill declaring nothing is not reported as such\n%s", output)
	}
}

func TestCostRefusesANameNoRowHolds(t *testing.T) {
	root := newGraphRoot(t, map[string]string{"skills/kk-pr/SKILL.md": skillBody("holds — landing", "x")})

	status, output := run(t, "--cost", "kk-nothing", root)
	if status != 2 {
		t.Fatalf("expected exit 2 for an unknown name, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "has a row in models.json") {
		t.Errorf("the refusal does not say what was looked for\n%s", output)
	}
}

func TestCostNamesNoSkillAtAllIsRefused(t *testing.T) {
	root := newGraphRoot(t, map[string]string{"skills/kk-pr/SKILL.md": skillBody("holds — landing", "x")})

	if status, output := run(t, root, "--cost"); status != 2 {
		t.Fatalf("expected exit 2 for a --cost with no value, got %d\n%s", status, output)
	}
}

// Three flags, three different things on one stdout. Any order chosen here would be this tool's
// opinion about which of them the caller meant, printed as though it were the answer.
func TestTwoEmitFlagsAreRefusedRatherThanOrdered(t *testing.T) {
	root := newGraphRoot(t, map[string]string{"skills/kk-pr/SKILL.md": skillBody("holds — landing", "x")})

	status, output := run(t, "--check", "--graph", root)
	if status != 2 {
		t.Fatalf("expected exit 2 for two emit flags, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "name one") {
		t.Errorf("the refusal does not say what to do instead\n%s", output)
	}
}

// The cost question is the one asked while the tree is being edited, so it must not rest on the
// narrative template or on the committed page — neither of which it reads a byte of.
func TestTheEmittersRunWithNoNarrativeTemplate(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})
	if err := os.Remove(filepath.Join(root, templateRelative)); err != nil {
		t.Fatalf("removing the fixture template: %v", err)
	}

	if status, output := run(t, "--graph", root); status != 0 {
		t.Fatalf("--graph needs the template it does not read: %d\n%s", status, output)
	}
	if status, output := run(t, "--cost", "kk-qualify", root); status != 0 {
		t.Fatalf("--cost needs the template it does not read: %d\n%s", status, output)
	}
	// And the page path still refuses, naming the template it could not read rather than emitting a
	// guide without it.
	if status, output := run(t, root); status != 2 ||
		!strings.Contains(output, "narrative template") || !strings.Contains(output, templateRelative) {
		t.Errorf("the page path no longer refuses a missing template by name: %d\n%s", status, output)
	}
}

// Every node on the map is a priced row, both ways. A node with no row would print a tier of "no
// tier" and read as a bug in the policy; a row with no node would be spend the map never showed.
func TestTheMapIsExactlyThePricedRows(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
		// A directory with no row at all: present in the tree, absent from the policy, and so absent
		// from a map built from rows.
		"skills/kk-unpriced/SKILL.md": skillBody("orchestrator", "Nothing prices this."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "workflow map — 4 skills, 4 workers") {
		t.Errorf("the map counts something other than the policy's rows\n%s", output)
	}
	if strings.Contains(output, "kk-unpriced") {
		t.Errorf("a directory with no row reached the map, so the map prices what the policy does not\n%s", output)
	}
	if strings.Contains(output, noTier) {
		t.Errorf("a node printed no tier, which a map built from priced rows cannot produce\n%s", output)
	}
}

// A worker that dispatches another worker is a real shape, and the map's second half is the only
// place it is visible: the first half shows what a door reaches, never what a leaf goes on to spend.
func TestAWorkerThatDispatchesAnotherIsOnTheMap(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})
	writeWorkers(t, root, fixtureWorker{"code-review", "One review. Hand the duplicates to `~/.kk-flavor/workers/refactor.md`.\n"})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "the dispatch sites underneath") {
		t.Errorf("the map has no second half, so a worker's own spend is invisible\n%s", output)
	}
	if !strings.Contains(graphBlock(t, output, "code-review"), "dispatches refactor") {
		t.Errorf("a worker's dispatch of another worker is missing\n%s", output)
	}
}

// A refusal names what did not happen, and the three paths do not produce the same thing. Told "the
// guide was NOT generated", someone who asked what a skill costs goes looking for a page they never
// mentioned, and the message stops being the fastest way to the actual fault.
//
// The page path's own wording is TestAGuideWithNoUsablePolicyRefuses, over this same wrecked policy.
func TestARefusalNamesWhichOfTheThreeDidNotHappen(t *testing.T) {
	for _, one := range []struct {
		args []string
		want string
	}{
		{args: []string{"--graph"}, want: "the map was NOT emitted"},
		{args: []string{"--cost", "kk-pr"}, want: "the cost profile was NOT emitted"},
	} {
		root := newGraphRoot(t, map[string]string{"skills/kk-pr/SKILL.md": skillBody("holds — landing", "x")})
		if err := os.Remove(filepath.Join(root, "kk-flavor", "models.json")); err != nil {
			t.Fatalf("wrecking the fixture policy: %v", err)
		}

		status, output := run(t, append(one.args, root)...)
		if status != 2 {
			t.Fatalf("expected exit 2 with no policy, got %d\n%s", status, output)
		}
		if !strings.Contains(output, one.want) {
			t.Errorf("%v refused without saying what it failed to produce\nwanted: %s\ngot: %s", one.args, one.want, output)
		}
	}
}

// The conformance gate's sharpest finding, kept as a case under the mechanism that now answers it.
// A skill naming another is extending it, sequencing it, or pointing at it, and only the first bills
// here. An earlier version could not tell them apart and treated all three as extension, which billed
// `idsd-ship` for four stages it sequences and called the total a ceiling. The edge is declared now,
// so the rule is simply that an undeclared citation is not followed.
func TestACitationWithoutTheDeclarationIsNotBilled(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/idsd-ship/SKILL.md": skillBody("holds — converses",
			"The stage after this is `~/.kk-flavor/skills/kk-qualify/SKILL.md`; dispatch `~/.kk-flavor/workers/build/explore.md` first."),
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})

	status, output := run(t, "--cost", "idsd-ship", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if !strings.Contains(output, "dispatch build/explore") {
		t.Errorf("a dispatch the skill names itself is missing from its profile\n%s", output)
	}
	if strings.Contains(output, "code-review") {
		t.Errorf("a sequenced stage's dispatches were billed to the run that merely names it\n%s", output)
	}
	if !strings.Contains(output, "1 row(s) one run can reach") {
		t.Errorf("the count includes rows that are not this run's\n%s", output)
	}
	// And the same tree with the declaration added bills it — so this case cannot pass by a walk
	// that follows nothing at all.
	declared := newGraphRoot(t, map[string]string{
		"skills/idsd-ship/SKILL.md": skillExtending("holds — converses", "kk-qualify",
			"Its delta over `~/.kk-flavor/skills/kk-qualify/SKILL.md`; dispatch `~/.kk-flavor/workers/build/explore.md` first."),
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})
	if _, with := run(t, "--cost", "idsd-ship", declared); !strings.Contains(with, "dispatch code-review") {
		t.Errorf("a declared extension's dispatches are not on the extending session's bill\n%s", with)
	}
}

// A door a human types answers what it costs; a worker nobody types refuses, because its one line is
// already printed beside every skill that dispatches it. The three lanes that kept their doors sit on
// the worker side of the policy and the door side of that question.
//
// The profile it does answer is also where this file holds a worker row whose contract is not
// `workers/<name>.md`. Six of this tree's rows are in that state — three doors, two borrowing another's
// prompt, one a Go tool assembles — and read only at the first home every one of them is a leaf that
// dispatches nothing.
func TestADoorKeepingLaneAnswersItsCostAndADoorlessWorkerRefuses(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-edit/SKILL.md": skillBody("dispatched", "Hand structure to `~/.kk-flavor/workers/refactor.md`."),
	})
	// The door-keeping shape is a `workers` row whose contract is a SKILL.md, so the fixture's
	// stand-in prompt under workers/ has to go or the row resolves to that instead.
	if err := os.Remove(filepath.Join(root, "kk-flavor", "workers", "kk-edit.md")); err != nil {
		t.Fatalf("removing the fixture worker file: %v", err)
	}

	status, output := run(t, "--cost", "kk-edit", root)
	if status != 0 {
		t.Fatalf("a lane with a door refuses to price itself: %d\n%s", status, output)
	}
	if !strings.Contains(output, "dispatch refactor") {
		t.Errorf("the door-keeping lane's own dispatch is missing\n%s", output)
	}
	status, output = run(t, "--cost", "code-review", root)
	if status != 2 || !strings.Contains(output, "worker with no door") {
		t.Errorf("a worker nobody types was priced as though it had a door: %d\n%s", status, output)
	}
}

// A worker a skill dispatches on its own face, and also reaches through a contract it extends, is
// reported as this skill's own — with no chain beside it. Recording the last reach instead would
// print `(through kk-qualify, which it extends)` next to a dispatch the skill names itself, sending
// a reader to the wrong file to change it.
func TestARowReachedBothWaysIsReportedAsTheSkillsOwn(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/idsd-ship/SKILL.md": skillExtending("holds — converses", "kk-qualify",
			"Its delta over `~/.kk-flavor/skills/kk-qualify/SKILL.md`, and it dispatches `~/.kk-flavor/workers/code-review.md` itself."),
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})

	status, output := run(t, "--cost", "idsd-ship", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "dispatch code-review") {
			continue
		}
		if strings.Contains(line, "through") {
			t.Errorf("a worker the skill dispatches on its own face is attributed to a contract it extends\n%s", line)
		}
		return
	}
	t.Errorf("the profile does not price code-review at all\n%s", output)
}

// The walk went recursive with this change, and a recursive walk over a tree the caller merely named
// can be pointed out of it. A `.md` symlink puts an out-of-tree file's citations into the map; the
// FIFO case is the same gate and cannot be driven here without hanging the suite if it regresses.
func TestASymlinkedMarkdownFileIsNotWalked(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md": skillBody("holds — landing", "Nothing cited here."),
	})
	outside := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(outside, []byte("Dispatch `~/.kk-flavor/workers/code-review.md`.\n"), 0o644); err != nil {
		t.Fatalf("fixture outsider: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "kk-flavor", "skills", "kk-pr", "linked.md")); err != nil {
		t.Skipf("this filesystem does not take symlinks: %v", err)
	}

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	if block := graphBlock(t, output, "kk-pr"); strings.Contains(block, "code-review") {
		t.Errorf("a symlink carried an out-of-tree file's citations into the map\n%s", block)
	}
}

// A session row keyed under a skill is one file, not a directory. Walking
// `skills/kk-pr/refine/` finds nothing, so the cheap mode read as dispatching nothing while its
// citations were billed to the expensive row beside it.
func TestAModeRowReadsItsOwnFileAndNotItsSkillsDirectory(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-pr/SKILL.md":  skillBody("holds — landing", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
		"skills/kk-pr/refine.md": "This mode dispatches `~/.kk-flavor/workers/refactor.md` and nothing else.\n",
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	mode := graphBlock(t, output, "kk-pr/refine")
	if !strings.Contains(mode, "dispatches refactor") {
		t.Errorf("a mode row read no file of its own, so its dispatches are priced nowhere\n%s", mode)
	}
	if strings.Contains(mode, "code-review") {
		t.Errorf("a mode row picked up its skill's other files\n%s", mode)
	}
}

// The borrowing shape: a row with no file of its own, running the prompt its `worker` field names.
// What that prompt dispatches is what this row dispatches, at this row's tier — and the file also
// points at its own assets. Those pointers are the prompt owner's self-citations whoever runs them;
// dropped against the borrowing row's name instead, they left the tree as dispatches and billed the
// borrower for a second run of the very contract it is.
func TestABorrowedPromptsSelfCitationsAreNotTheBorrowersDispatches(t *testing.T) {
	policy := strings.Replace(graphPolicy, `"refactor":       {`,
		`"lender/repair":  { "codex": { "model": "gpt-5.6-luna", "effort": "low" }, "claude": { "model": "haiku" }, "worker": "kk-edit" },
    "refactor":       {`, 1)
	root := newGraphRoot(t, map[string]string{
		"skills/kk-edit/SKILL.md": skillBody("dispatched",
			"My own branch is `~/.kk-flavor/skills/kk-edit/humanize.md`; hand structure to `~/.kk-flavor/workers/refactor.md`."),
	})
	if err := os.WriteFile(filepath.Join(root, "kk-flavor", "models.json"), []byte(policy), 0o644); err != nil {
		t.Fatalf("fixture policy: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "kk-flavor", "workers", "kk-edit.md")); err != nil {
		t.Fatalf("removing the fixture worker file: %v", err)
	}

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	block := graphBlock(t, output, "lender/repair")
	if strings.Contains(block, "dispatches kk-edit") {
		t.Errorf("a row was billed for dispatching the contract it runs, at a second row\n%s", block)
	}
	if !strings.Contains(block, "dispatches refactor") {
		t.Errorf("the borrowed prompt's real dispatch is missing\n%s", block)
	}
}

// `guide.sh --cost "$skill"` with the variable unset is a request this cannot answer, not an absent
// flag. Read back off the value alone it looked like neither, and the run fell through to
// overwriting the committed page — from a command whose own contract says it writes no file.
func TestCostWithAnEmptyValueRefusesAndWritesNothing(t *testing.T) {
	root := newGraphRoot(t, map[string]string{"skills/kk-pr/SKILL.md": skillBody("holds — landing", "x")})
	page := filepath.Join(root, outputRelative)
	if err := os.WriteFile(page, []byte("committed\n"), 0o644); err != nil {
		t.Fatalf("fixture page: %v", err)
	}

	status, output := run(t, "--cost", "", root)
	if status != 2 {
		t.Fatalf("expected exit 2 for an empty --cost value, got %d\n%s", status, output)
	}
	held, err := os.ReadFile(page)
	if err != nil || string(held) != "committed\n" {
		t.Errorf("the committed page was overwritten by a command that writes no file: %q, %v", string(held), err)
	}
}

// Every priced row prints its tier. Only the skills half carried one at first, which left the map
// silent about 22 of its 40 rows — while `--cost`'s refusal on a worker redirects the reader here
// for exactly that number, and the header counts them.
func TestEveryPricedRowCarriesItsTier(t *testing.T) {
	root := newGraphRoot(t, map[string]string{
		"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/code-review.md`."),
	})

	status, output := run(t, "--graph", root)
	if status != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", status, output)
	}
	// `build/explore` has a row, no outgoing edge, and nothing anywhere cites it — the shape that
	// used to vanish from the map entirely.
	block := graphBlock(t, output, "build/explore")
	if !strings.Contains(block, "runs at") {
		t.Errorf("a worker row nothing cites is absent from the map, or prints no tier\n%s", output)
	}
	if strings.Contains(output, noTier) {
		t.Errorf("a priced row printed no tier\n%s", output)
	}
	// And the tier rides on the dispatch line, which is where the refusal on a worker sends a reader.
	// Asserted as "both on one line" rather than on the exact run of spaces between them, so a column
	// widening by one name does not redden a case about what the line carries.
	carried := false
	for _, one := range strings.Split(graphBlock(t, output, "kk-qualify"), "\n") {
		if strings.Contains(one, "dispatches code-review") && strings.Contains(one, "claude opus") {
			carried = true
		}
	}
	if !carried {
		t.Errorf("a dispatch line does not carry the tier it buys\n%s", output)
	}
}

// A declaration nobody can parse is the one input both emitters must refuse rather than price. Read
// as silence it is free: an unreadable `**Extends:**` drops its contract's whole subtree off the
// bill, and an unreadable `**Runs:**` leaves a skill with no mode, which the graph then prints as a
// lane that keeps nothing. Either way a reader is handed a number with no sign that a line was
// skipped — and `--cost` is asked precisely while a tree is being edited, which is when a
// half-written declaration exists.
//
// Over the shipped tree the policy suite already refuses both. This is the same guarantee for the
// other checkouts, which is every place `--cost <skill> <root>` is actually pointed.
func TestABrokenDeclarationRefusesRatherThanPricingItFree(t *testing.T) {
	for _, one := range []struct {
		name    string
		body    string
		wanting string
	}{
		{"an extension with no when", "---\nname: x\ndescription: One line.\n---\n\n**Runs:** holds — converses\n\n" +
			"**Extends:** kk-qualify\n\nThe pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`.\n",
			"`**Extends:** <skill> — <when>`"},
		{"an extension whose when is only whitespace", "---\nname: x\ndescription: One line.\n---\n\n**Runs:** holds — converses\n\n" +
			"**Extends:** kk-qualify — \t\n\nThe pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`.\n",
			"`**Extends:** <skill> — <when>`"},
		{"a runs line naming a fourth mode", "---\nname: x\ndescription: One line.\n---\n\n**Runs:** inline\n\n" +
			"Dispatch `~/.kk-flavor/workers/refactor.md`.\n",
			"declares how it runs in a form this cannot read"},
	} {
		t.Run(one.name, func(t *testing.T) {
			root := newGraphRoot(t, map[string]string{
				"skills/kk-pr/SKILL.md":      one.body,
				"skills/kk-qualify/SKILL.md": skillBody("orchestrator", "Dispatch `~/.kk-flavor/workers/refactor.md`."),
			})
			// Both emitters, because they share the one reader: a refusal reaching only the map would
			// leave the profile — the answer a human acts on — pricing the same file free.
			for _, argv := range [][]string{{"--graph", root}, {"--cost", "kk-pr", root}} {
				status, output := run(t, argv...)
				if status == 0 {
					t.Errorf("%v exited 0 over an unreadable declaration, so it priced one free\n%s", argv, output)
				}
				if !strings.Contains(output, one.wanting) {
					t.Errorf("%v does not say which declaration it could not read, wanting %q\n%s", argv, one.wanting, output)
				}
				if !strings.Contains(output, "kk-pr") {
					t.Errorf("%v does not name the file, so nobody can find the line\n%s", argv, output)
				}
			}
		})
	}
}
