package modelpolicy

// The shipped instruction tree, held against the shipped policy. These checks read directories under
// kk-flavor/ and fail on what the two disagree about; command_test.go beside them reads the policy
// document alone and never opens the tree. The split is that line — a check here needs a checkout to
// mean anything, and one there does not.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

// The shipped tree these checks read, from its one root — and the policy beside it, which
// command_test.go reads too. Every path is derived from that root, so a suite run from another
// directory fails on the root rather than on whichever literal was missed.
const (
	flavorTree        = "../../kk-flavor"
	skillsTree        = flavorTree + "/skills"
	workersTree       = flavorTree + "/workers"
	shippedPolicyPath = flavorTree + "/models.json"
)

// The Lanes table row the quality pass dispatches by. How a SKILL.md declares it runs is
// shell.RunsDeclaration, shared with the emitters in eco-guide that price a skill from the same
// line — a second copy of that grammar here would let the census and the cost profile disagree
// about which skills the ceiling may ask anything.
var (
	// Column two names what fills the lane, which is a worker's path for most of them and a skill's
	// bare name for the three that kept a door. Both are reduced to the key the policy is written
	// on: a path without its tree prefix and without `.md` IS that key, so one pattern reads both
	// and a lane that changes home stays counted.
	lanesTableRow = regexp.MustCompile("(?m)^\\| *[a-z-]+ *\\| *`(?:~/\\.kk-flavor/workers/)?([a-z0-9/-]+?)(?:\\.md)?` *\\|")
)

// The policy is only a cost control surface while it covers everything that runs, so it is checked
// against the tree rather than by hand: skills by their SKILL.md, workers by the directory that
// declares them.
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
		// Every worker row resolves to a prompt, and asked of the bare names too rather than sub-rows
		// alone: a top-level row claims a prompt exactly as a sub-row does, and leaving it unasked is
		// what would let the flat worker grammar land a row whose file was never written. The forms are
		// the ones model-policy.md → **One row per skill, one per dispatch site** names, in its order.
		if workers[name] {
			continue
		}
		// Parse has already refused a row naming one that is not a worker, or one that names another's
		// prompt in turn; what is left is that the row named has a prompt to hand over.
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

// The worker whose prompt is assembled in Go rather than read from the tree, so the census cannot
// find a file for it and must not demand one. Named rather than sniffed out, and a second worker of
// this shape is a decision somebody has to write down here.
//
// Its `<name>/<kind>` sub-rows are the same tool, and the prompt is assembled the same way. A sub-row
// exists where one kind is read by a different tier: comment-verdict asks a comprehension question,
// which the tier a delete vote is worth cannot answer.
const toolBuiltWorker = "reader-judge"

// A session sub-row prices a named path through one session rather than a spawn, so the mode file that
// path reads is its evidence. A session's own row is a skill, already checked against the skills tree.
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

// A session row is one nothing enforces, and which rows those are is derivable rather than a
// judgement: a model is set for the judge, for every worker file, for a row naming another worker's
// prompt, and for every leaf kk-qualify's Lanes table dispatches; everything else runs in whatever
// session invoked it. The split stays hand-written so a reader of the policy can see it, and this
// checks it against the derivation.
func TestSessionRowsMatchWhatNothingEnforces(t *testing.T) {
	policy := loadShippedPolicy(t)
	skills := skillBodies(t)
	lanes, ok := skills["kk-qualify"]
	if !ok {
		t.Fatal("kk-qualify carries no readable SKILL.md, so the lane table cannot be read")
	}
	enforced := map[string]bool{"reader-judge": true, "reader-judge/comment-verdict": true}
	for _, row := range lanesTableRow.FindAllSubmatch(lanes, -1) {
		enforced[string(row[1])] = true
	}
	if len(enforced) < 5 {
		t.Fatalf("the Lanes table yielded %d lanes, so this proved nothing", len(enforced)-1)
	}
	// A skill's own **Runs:** line is authoritative: the lane table knows only the skills the quality
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

// Every skill declares how it runs, in a form this file can read. Missing, the derivation below falls
// back to the lane table, which sees only the skills the quality pass dispatches — so a skill that
// forgot the line lands in whichever map its silence happens to imply, and the ceiling never asks it
// anything. The declarations, by skill.
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

// The ceiling. An orchestrator claims every substantive step is dispatched; the most expensive row is
// what the work a session keeps costs, so a skill holding both says two things that cannot both be
// true and neither file says which one to believe.
//
// Given the policy and the declarations rather than reading either, so the case below can hand it a
// tree that has the defect. A gate only ever run against a tree that passes is one nobody has watched
// fail.
//
// Every row names a model for every client — validateSettings refuses one that does not — so every
// orchestrator ranks and this needs no arm for a row it cannot judge.
//
// The unranked arm below is the other half of that, and it is unreachable on purpose rather than by
// luck: it walks the same dispatchClients validateTiers demands a non-empty order for, so every
// client asked about here is one Parse refused to leave unranked. It stays because a client added to
// that list with no order behind it would otherwise make this report an empty ceiling and pass.
func orchestratorsAtTheCeiling(policy *Policy, declared map[string]string) ([]string, error) {
	var atTop []string
	for _, client := range dispatchClients {
		top, ranked := policy.TopTier(client)
		if !ranked {
			return nil, fmt.Errorf("the policy orders no %s tiers, so nothing here knows which model is the top one", client)
		}
		for skill, mode := range declared {
			if mode != "orchestrator" {
				continue
			}
			decision, err := policy.Resolve(Request{Client: client, Task: skill})
			if err != nil {
				return nil, fmt.Errorf("%s declares itself an orchestrator and the policy does not price it: %w", skill, err)
			}
			if decision.Requested.Model == top {
				atTop = append(atTop, client+"/"+skill)
			}
		}
	}
	sort.Strings(atTop)
	return atTop, nil
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
	atTop, err := orchestratorsAtTheCeiling(loadShippedPolicy(t), declared)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range atTop {
		t.Errorf("%s is declared an orchestrator and priced at the top tier — find the work that tier is paying for: either it is real and the skill is a session naming which reason holds it, or it was dispatched already and the row never came down", at)
	}
}

// The ceiling against a tree that has the defect, which the shipped one does not. Both clients are
// asserted: the two orders share no model name, so a check that read one list and compared against the
// other's top would report nothing and look green.
func TestTheCeilingCatchesAnOrchestratorAtTheTopTier(t *testing.T) {
	policy, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	atTop, err := orchestratorsAtTheCeiling(policy, map[string]string{"kk-build": "orchestrator"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"claude/kk-build", "codex/kk-build"}; !slices.Equal(atTop, want) {
		t.Fatalf("at the ceiling: %v; want %v", atTop, want)
	}
	// The same row declared for the work it keeps is no finding at all — the ceiling reads the
	// declaration, never the tier alone.
	atTop, err = orchestratorsAtTheCeiling(policy, map[string]string{"kk-build": "holds — converses"})
	if err != nil || len(atTop) != 0 {
		t.Fatalf("a session at the top tier was reported: %v, %v", atTop, err)
	}
}

// A stale reference to a renamed worker key names a task nothing assigns, and Resolve catches that
// only at the moment the dispatch runs — so the tree may not carry one. Slash-shaped tokens only: a
// bare name is a skill or worker the completeness checks above already cover, and anything holding a
// `.` or a placeholder is a path or a template rather than a task.
func TestNoFileNamesATaskThePolicyDoesNotAssign(t *testing.T) {
	policy := loadShippedPolicy(t)
	rows := nameSet(policy.TaskNames())
	// The first segment of every sub-row, which is a task family whether or not it is a row itself.
	// `patrol/scout` and `patrol/fixer` carry no skill prefix, so keying on rows alone would leave the
	// whole family invisible to this scan.
	families := map[string]bool{}
	for name := range rows {
		if family, _, isSub := strings.Cut(name, "/"); isSub {
			families[family] = true
		}
	}
	shaped := regexp.MustCompile("`([a-z0-9][a-z0-9-]*(?:/[a-z0-9][a-z0-9-]*)+)`")
	scanned, checked := 0, 0
	for _, tree := range []string{flavorTree, "../../tools"} {
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

// Every skill in the tree, by name, with the body of its SKILL.md — the one census these checks count
// skills from, so no two of them disagree about which directories are skills.
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
// repo gets its tokens echoed into a failure line. A directory whose SKILL.md is not a regular file
// is therefore not a skill, which keeps every census on the same set.
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
		found[strings.TrimSuffix(filepath.ToSlash(rel), ".md")] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("found no worker files, so every check against them proved nothing")
	}
	return found
}

// The extension declaration, held to the tree it describes. It is the one edge a reader cannot check
// from the citation — `ecosystem.md` → **Three kinds, two homes** says extension, sequencing and
// orientation all name a second file the same way — so the cost surface is told rather than guessing,
// and a declaration nothing verifies is a claim that rots into a wrong number.
//
// Two ways it can be wrong and both are here: naming a skill that does not exist, and naming one this
// file never reads. The second is the one that costs money — a stale `**Extends:**` left behind after
// the citation moved keeps billing that contract's dispatches to this row forever.
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
			// Read across the whole skill directory, not just SKILL.md: `kk-pr` declares the pass it
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

// Every skill that declares it, named, so the set cannot quietly grow or shrink without someone saying
// so here. Extension is the expensive edge and the only one the tree has to be told about, so which
// skills claim it is a fact worth pinning rather than deriving.
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
