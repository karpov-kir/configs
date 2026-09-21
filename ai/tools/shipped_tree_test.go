// This file measures the shipped instruction tree against the shipped policy. These checks read
// directories under kk-flavor/ and fail on what the two disagree about.

// They live in this package with the other cases that read the shipped checkout, and the other ten
// files doing that point here for why.

// That gathering was once the only way to key them. go.mod sat at `ai/tools`, and Go keys a package's
// test cache on the module, hashing no file outside the module root. A case under
// `ai/tools/model-policy/` that walked kk-flavor/skills/ therefore answered `ok (cached)` over a tree
// that had moved, and a merge or a checkout is exactly when a tree has moved.

// go.mod is at the repository root now, so a case is keyed on what it opens wherever it sits, and the
// gathering is only a convention. `ai/kk-flavor/standards/testing.md` asks for one module at the
// repository root, and that is what took the constraint away.

// What is left in model-policy is every check a checkout could make no truer. The parser, Resolve,
// and the ceiling derivation run against a fixture tree carrying the defect the shipped tree lacks.
package tools_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

// The shipped tree these checks read, from the repository root — and the policy beside it, which
// shipped_policy_test.go reads too. Every path is derived from that root, so a suite run from another
// directory fails on the root and never on whichever literal was missed.
const (
	flavorTree        = repoRoot + "/ai/kk-flavor"
	skillsTree        = flavorTree + "/skills"
	workersTree       = flavorTree + "/workers"
	shippedPolicyPath = flavorTree + "/models.json"
)

// The Lanes table row the quality pass dispatches by. How a SKILL.md declares it runs is
// shell.RunsDeclaration, which the emitters in eco-guide share to price a skill from the same line. A
// second copy of that grammar here would let the census and the cost profile disagree about which
// skills the ceiling may ask anything.
var (
	// Column two names what fills the lane, which is a worker's path for most of them and a skill's
	// bare name for the three that kept a door. Both are reduced to the key the policy is written on.
	// That key is a path stripped of its tree prefix and of `.md`, so one pattern reads both and a
	// lane that changes home stays counted.
	lanesTableRow = regexp.MustCompile("(?m)^\\| *[a-z-]+ *\\| *`(?:~/\\.kk-flavor/workers/)?([a-z0-9/-]+?)(?:\\.md)?` *\\|")
)

// The policy is only a cost control surface while it covers everything that runs. So the check reads
// the tree, taking skills from their SKILL.md and workers from the directory that declares them. A
// hand-written list is what this replaces.
func TestEverySkillAndWorkerHasARow(t *testing.T) {
	policy := loadShippedPolicy(t)
	rows := nameSet(policy.TaskNames())
	workers := shippedWorkerFiles(t)
	for name := range workers {
		if !rows[name] {
			t.Errorf("worker %s has no row, so it would dispatch at its caller's tier", name)
		}
	}
	skills := skillBodies(t)
	for skill := range skills {
		if !rows[skill] {
			t.Errorf("skill %s has no row, so it would dispatch at its caller's tier", skill)
		}
	}
	owners := policy.PromptOwners()
	sessions := nameSet(policy.SessionTasks())
	for name := range rows {
		if sessions[name] {
			assertSessionRowReadsAModeFile(t, name)
			continue
		}
		// Every worker row resolves to a prompt, and the bare names are asked too. A top-level row claims
		// a prompt the way a sub-row does, and a name left unasked lets the flat worker grammar land a
		// row whose file was never written. model-policy.md asks for one row per skill and one per
		// dispatch site, and these forms are in its order.
		if workers[name] {
			continue
		}
		// Parse has already refused a row naming one that is not a worker, or one that names another's
		// prompt in turn. What is left is that the row named has a prompt to hand over.
		if target, named := owners[name]; named {
			if _, stillASkill := skills[target]; !workers[target] && !stillASkill {
				t.Errorf("row %s dispatches the prompt of %s, which has neither a file under kk-flavor/workers/ nor a SKILL.md", name, target)
			}
			continue
		}
		if _, stillASkill := skills[name]; stillASkill {
			continue
		}
		if name == toolBuiltWorker || strings.HasPrefix(name, toolBuiltWorker+"/") {
			continue
		}
		t.Errorf("worker row %s names no prompt: no file under kk-flavor/workers/, no worker whose prompt it names, no SKILL.md of its own, and it is not %s or one of its kinds, whose prompts a Go tool assembles", name, toolBuiltWorker)
	}
}

// The worker whose prompt a Go tool assembles. The census finds a file for every other one, and this
// row has none, so it passes by name. Naming it makes a second worker of this shape a decision
// somebody writes down. Its `<name>/<kind>` sub-rows are the same tool, assembled the same way, and
// price one kind apart for a question the tool's own tier cannot answer. The tree ships none today.
const toolBuiltWorker = "reader-judge"

// A session sub-row prices a named path through one session, and a worker row prices a spawn. So the
// mode file that path reads is its evidence. A session's own row is a skill, already checked against
// the skills tree.
func assertSessionRowReadsAModeFile(t *testing.T, name string) {
	t.Helper()
	skill, mode, isSub := strings.Cut(name, "/")
	if !isSub {
		return
	}
	path := filepath.Join(skillsTree, skill, mode+".md")
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		t.Errorf("session sub-row %s reads no mode file at %s, so nothing says which path through %s it prices", name, path, skill)
	}
}

// A model is set for the judge, for every worker file, for a row naming another worker's prompt, and
// for every leaf kk-qualify's Lanes table dispatches. Everything else runs in whatever session
// invoked it.

// A session row is one no mechanism enforces, and which rows those are is derivable. The split stays
// hand-written so a reader of the policy can see it, and this case checks it against the derivation.
func TestSessionRowsMatchWhatNothingEnforces(t *testing.T) {
	policy := loadShippedPolicy(t)
	skills := skillBodies(t)
	lanes, ok := skills["kk-qualify"]
	if !ok {
		t.Fatal("kk-qualify carries no readable SKILL.md, so the lane table cannot be read")
	}
	enforced := map[string]bool{"reader-judge": true}
	for _, row := range lanesTableRow.FindAllSubmatch(lanes, -1) {
		enforced[string(row[1])] = true
	}
	if len(enforced) < 5 {
		t.Fatalf("the Lanes table yielded %d lanes, so this proved nothing", len(enforced)-1)
	}
	// A skill's own `**Runs:**` line is authoritative: the lane table knows only the skills the quality
	// pass dispatches, and a skill dispatched elsewhere is invisible to it. Only `dispatched` puts a
	// row under workers — an orchestrator and a session both run in whatever session invoked them.
	for skill, mode := range declaredRunModes(t) {
		enforced[skill] = mode == "dispatched"
	}
	// A sub-row is a worker when it has a prompt of its own or names another worker's. A session sub-row
	// is a named path through the same session, so it takes the kind of the skill it belongs to.
	owners := policy.PromptOwners()
	for name := range shippedWorkerFiles(t) {
		enforced[name] = true
	}
	for _, name := range policy.TaskNames() {
		skill, _, isSub := strings.Cut(name, "/")
		if !isSub {
			continue
		}
		if _, named := owners[name]; named {
			enforced[name] = true
		} else if enforced[skill] {
			enforced[name] = true
		}
	}
	isSession := nameSet(policy.SessionTasks())
	for _, name := range policy.TaskNames() {
		if enforced[name] && isSession[name] {
			t.Errorf("%s is dispatched, so it belongs under workers, not sessions", name)
		}
		if !enforced[name] && !isSession[name] {
			t.Errorf("%s runs in its caller's session, so it belongs under sessions, not workers", name)
		}
	}
}

// Every skill declares how it runs, in a form this file can read. Without the line,
// TestSessionRowsMatchWhatNothingEnforces falls back to the lane table, which sees only the skills the
// quality pass dispatches. A skill that forgot it lands in whichever map its silence happens to imply,
// and the ceiling never asks it anything. The declarations, by skill.
func declaredRunModes(t *testing.T) map[string]string {
	t.Helper()
	declared := map[string]string{}
	for skill, body := range skillBodies(t) {
		mode, stated := shell.RunsDeclaration(shell.SplitLines(string(body)))
		if mode == "" {
			if stated {
				t.Errorf("%s declares how it runs in a form this cannot read; it is `dispatched`, `orchestrator`, or `holds — <reason>` naming one of converses, session-context, landing", skill)
			} else {
				t.Errorf("%s declares no **Runs:** line, so nothing says whether it holds work or hands every step away; it is `dispatched`, `orchestrator`, or `holds — <reason>` naming one of converses, session-context, landing", skill)
			}
			continue
		}
		declared[skill] = mode
	}
	return declared
}

func TestEverySkillDeclaresHowItRuns(t *testing.T) {
	declared := declaredRunModes(t)
	if len(declared) != len(skillBodies(t)) {
		t.Fatalf("%d skills declare how they run out of %d", len(declared), len(skillBodies(t)))
	}
}

func TestNoOrchestratorHoldsTheTopTier(t *testing.T) {
	declared := declaredRunModes(t)
	orchestrators := 0
	for _, mode := range declared {
		if mode == "orchestrator" {
			orchestrators++
		}
	}
	if orchestrators == 0 {
		t.Fatal("no skill declares itself an orchestrator, so this proved nothing")
	}
	atTop, err := loadShippedPolicy(t).OrchestratorsAtTheCeiling(declared)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range atTop {
		t.Errorf("%s is declared an orchestrator and priced at the top tier — find the work that tier is paying for: either it is real and the skill is a session naming which reason holds it, or it was dispatched already and the row never came down", at)
	}
}

// A stale reference to a renamed worker key names a task the policy leaves unassigned, and Resolve
// catches that only at the moment the dispatch runs. So the tree may carry none. Slash-shaped tokens
// only. A bare name is a skill or worker TestEverySkillAndWorkerHasARow already covers, and anything
// holding a `.` or a placeholder is a path or a template.
func TestNoFileNamesATaskThePolicyDoesNotAssign(t *testing.T) {
	policy := loadShippedPolicy(t)
	rows := nameSet(policy.TaskNames())
	// The first segment of every sub-row, which is a task family whether or not it is a row itself.
	// `patrol/scout` and `patrol/fixer` carry no skill prefix, and keying on rows alone leaves the
	// whole family invisible to this scan.
	families := map[string]bool{}
	for name := range rows {
		if family, _, isSub := strings.Cut(name, "/"); isSub {
			families[family] = true
		}
	}
	shaped := regexp.MustCompile("`([a-z0-9][a-z0-9-]*(?:/[a-z0-9][a-z0-9-]*)+)`")
	scanned, checked := 0, 0
	for _, tree := range []string{flavorTree, repoRoot + "/ai/tools"} {
		if err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			// Only regular files: os.ReadFile follows a symlink, so a committed link to /dev/zero hangs
			// this gate and one pointing outside the repo gets its tokens echoed into the failure line.
			if !entry.Type().IsRegular() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".md", ".go", ".sh", ".json":
			default:
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, hit := range shaped.FindAllSubmatch(body, -1) {
				name := string(hit[1])
				first, _, _ := strings.Cut(name, "/")
				// A task reference is one whose first segment the policy already uses that way: its own
				// row, or the family a sub-row sits in. Everything else is a path sharing the shape.
				if !rows[first] && !families[first] && !rows[name] {
					continue
				}
				checked++
				if !rows[name] {
					t.Errorf("%s names task %q, which the policy does not assign — the dispatch that reads this would be refused", path, name)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if scanned == 0 || checked == 0 {
		t.Fatalf("scanned %d files and checked %d task names, so this proved nothing", scanned, checked)
	}
}

// Every skill in the tree, by name, with the body of its SKILL.md. These checks count skills from
// this census alone, and cannot disagree about which directories are skills.
func skillBodies(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(skillsTree)
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if body, ok := readSkillFile(entry.Name()); ok {
			bodies[entry.Name()] = body
		}
	}
	if len(bodies) == 0 {
		t.Fatal("found no skills to check, so every check against them proved nothing")
	}
	return bodies
}

// One skill's SKILL.md, and the only place these checks read one. Regular files only: os.ReadFile
// follows a symlink, so a committed link to /dev/zero hangs the check and one pointing outside the
// repo gets its tokens echoed into a failure line. A directory whose SKILL.md is something else is
// therefore no skill, and every census stays on the same set.
func readSkillFile(skill string) ([]byte, bool) {
	path := filepath.Join(skillsTree, skill, "SKILL.md")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	body, err := os.ReadFile(path)
	return body, err == nil
}

// The workers/ tree declares a worker: a file there is one, and its path is its task name. A directory
// cannot forget to mention itself, which a line beside the prose that spawns the worker always could.
// Two row shapes still have no file here: one naming another worker's prompt, and a worker that is
// still a mounted skill.
func shippedWorkerFiles(t *testing.T) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	if err := filepath.WalkDir(workersTree, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		rel, err := filepath.Rel(workersTree, path)
		if err != nil {
			return err
		}
		// A `tests/` directory holds probes a human runs against a worker, the way
		// skills/idsd-reactor/tests does. A human runs a probe and a dispatch never reaches it. A row
		// for one would price a worker that stays idle, and this census would go on asking for it.
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		for _, part := range strings.Split(name, "/") {
			if part == "tests" {
				return nil
			}
		}
		found[name] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("found no worker files, so every check against them proved nothing")
	}
	return found
}

// A reader cannot check this edge from the citation. `ecosystem.md` says extension, sequencing and
// orientation all name a second file the same way, so the declaration has to tell the cost surface. A
// declaration no case verifies is a claim that rots into a wrong number.

// This case holds the extension declaration to the tree it describes. It can be wrong two ways and
// both are here. One names a skill that is absent, and one names a skill this file never reads. The
// second costs money, because a stale `**Extends:**` left behind after the citation moved keeps
// billing that contract's dispatches to this row forever.
func TestEveryDeclaredExtensionNamesASkillThisOneActuallyReads(t *testing.T) {
	skills := skillBodies(t)
	for skill, body := range skills {
		declared, stated := shell.ExtendsDeclarations(shell.SplitLines(string(body)))
		if stated && len(declared) == 0 {
			t.Errorf("%s declares an extension in a form this cannot read; it is `**Extends:** <skill> — <when>`, and the when is required", skill)
			continue
		}
		for _, target := range declared {
			if _, exists := skills[target]; !exists {
				t.Errorf("%s declares it extends %s, and no skill by that name has a SKILL.md", skill, target)
				continue
			}
			// The scan reads the whole skill directory, not just SKILL.md. `kk-pr` declares the pass it
			// extends and cites it from `review.md` and `address-review.md`, which are the modes that
			// run it.
			if !skillDirCites(t, skill, target) {
				t.Errorf("%s declares it extends %s but no file of it cites ~/.kk-flavor/skills/%s/ — a declaration outliving its citation bills that contract's dispatches to this row for nothing", skill, target, target)
			}
		}
	}
}

// True when any markdown under one skill's directory names another skill by path.
func skillDirCites(t *testing.T, skill, target string) bool {
	t.Helper()
	found := false
	dir := filepath.Join(skillsTree, skill)
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(body), "~/.kk-flavor/skills/"+target+"/") {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	return found
}

// Every skill that declares it, named, so the set cannot quietly grow or shrink. A change to the set
// takes an edit here. Extension is the expensive edge, and the only edge the tree has to be told
// about. So which skills claim it is a fact worth pinning by hand.
func TestExactlyTheKnownSkillsDeclareAnExtension(t *testing.T) {
	want := map[string]string{
		"idsd-build":    "kk-build, kk-grill, idsd-charter, idsd-intent",
		"idsd-charter":  "kk-grill",
		"idsd-finalize": "idsd-qualify, idsd-charter",
		"idsd-intent":   "kk-grill, idsd-charter",
		"idsd-qualify":  "kk-qualify",
		"idsd-reactor":  "kk-handoff",
		"idsd-ship":     "idsd-build, idsd-intent, idsd-qualify, idsd-finalize",
		"kk-pr":         "kk-qualify",
	}
	got := map[string]string{}
	for skill, body := range skillBodies(t) {
		if declared, _ := shell.ExtendsDeclarations(shell.SplitLines(string(body))); len(declared) > 0 {
			got[skill] = strings.Join(declared, ", ")
		}
	}
	for skill, target := range want {
		if got[skill] != target {
			t.Errorf("%s extends %q, wanted %q", skill, got[skill], target)
		}
	}
	for skill, target := range got {
		if _, known := want[skill]; !known {
			t.Errorf("%s newly declares it extends %s; add it here, and check --cost still reads as one run's bill", skill, target)
		}
	}
}
