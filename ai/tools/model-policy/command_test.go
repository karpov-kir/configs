package modelpolicy

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The shipped tree these checks read, from its one root. Every path below is derived from it, so a
// suite run from another directory fails on the root rather than on whichever literal was missed.
const (
	flavorTree        = "../../kk-flavor"
	skillsTree        = flavorTree + "/skills"
	workersTree       = flavorTree + "/workers"
	shippedPolicyPath = flavorTree + "/models.json"
)

// The line a SKILL.md declares how it runs with, and the Lanes table row the quality pass dispatches by.
var (
	runsMode = regexp.MustCompile(`(?m)^\*\*Runs:\*\* *(dispatched|inline — (?:human|session-context|landing)) *$`)
	runsLine = regexp.MustCompile(`(?m)^\*\*Runs:\*\*`)
	// Column two names what fills the lane, which is a worker's path for most of them and a skill's
	// bare name for the three that kept a door. Both are reduced to the key the policy is written
	// on: a path without its tree prefix and without `.md` IS that key, so one pattern reads both
	// and a lane that changes home stays counted.
	lanesTableRow = regexp.MustCompile("(?m)^\\| *[a-z-]+ *\\| *`(?:~/\\.kk-flavor/workers/)?([a-z0-9/-]+?)(?:\\.md)?` *\\|")
)

func TestCommandEmitsRequestedSettingsAndNothingObserved(t *testing.T) {
	config := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(config, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errors strings.Builder
	args := []string{"--config", config, "--client", "claude", "--task", "bloat-judge"}
	if code := Run(Command{Args: args, Stdout: &out, Stderr: &errors}); code != 0 {
		t.Fatalf("command=%d: %s", code, errors.String())
	}
	var decision Decision
	if err := json.Unmarshal([]byte(out.String()), &decision); err != nil {
		t.Fatal(err)
	}
	if decision.Requested.Model != "haiku" || decision.Rolls != 3 || strings.Contains(out.String(), "observed") {
		t.Fatalf("wrong requested evidence: %s", out.String())
	}
}

func TestCommandRefusesAnUnassignedTask(t *testing.T) {
	var out, errors strings.Builder
	args := []string{"--config", shippedPolicyPath, "--client", "claude", "--task", "kk-invented"}
	if code := Run(Command{Args: args, Stdout: &out, Stderr: &errors}); code != 2 || out.Len() != 0 {
		t.Fatalf("unassigned task result=%d %s", code, out.String())
	}
	if !strings.Contains(errors.String(), "assigns no model") {
		t.Fatalf("unhelpful refusal: %s", errors.String())
	}
}

// The shipped policy is the cost control surface, so every task in it must resolve for both clients.
func TestEveryShippedTaskResolvesForBothClients(t *testing.T) {
	policy := loadShippedPolicy(t)
	names := policy.TaskNames()
	if len(names) < 2 {
		t.Fatalf("shipped policy covers %d tasks", len(names))
	}
	for _, name := range names {
		for _, client := range []string{"codex", "claude"} {
			decision, err := policy.Resolve(Request{Client: client, Task: name})
			if err != nil {
				t.Fatalf("%s/%s: %v", client, name, err)
			}
			if client == "claude" && decision.Requested.Model == "" {
				t.Fatalf("%s/%s names no model, so it would bill at its parent's tier", client, name)
			}
			if decision.Requested.Model == "" && decision.Requested.Effort == "" {
				t.Fatalf("%s/%s sets neither a model nor an effort", client, name)
			}
		}
	}
}

// The judge is the only task a tool resolves on its own, so its assignment is the one the shipped
// policy cannot lose without a gate going quiet.
func TestShippedPolicyKeepsTheJudgeCheapAndVoting(t *testing.T) {
	policy := loadShippedPolicy(t)
	for client, want := range map[string]Settings{
		"claude": {Model: "haiku"},
		"codex":  {Model: "gpt-5.6-luna", Effort: "low"},
	} {
		decision, err := policy.Resolve(Request{Client: client, Task: "bloat-judge"})
		if err != nil || decision.Requested != want {
			t.Fatalf("judge %s = %+v, %v; want %+v", client, decision.Requested, err, want)
		}
		if decision.Rolls < 1 {
			t.Fatalf("judge %s votes with %d rolls", client, decision.Rolls)
		}
	}
}

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
		if name == toolBuiltWorker {
			continue
		}
		t.Errorf("worker row %s names no prompt: no file under kk-flavor/workers/, no worker whose prompt it names, no SKILL.md of its own, and it is not %s, the one row a Go tool assembles the prompt for", name, toolBuiltWorker)
	}
}

// The one worker whose prompt is assembled in Go rather than read from the tree, so the census cannot
// find a file for it and must not demand one. Named rather than sniffed out: it is the only row of its
// kind, and a second would be a decision somebody has to write down here.
const toolBuiltWorker = "bloat-judge"

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
	enforced := map[string]bool{"bloat-judge": true}
	for _, row := range lanesTableRow.FindAllSubmatch(lanes, -1) {
		enforced[string(row[1])] = true
	}
	if len(enforced) < 5 {
		t.Fatalf("the Lanes table yielded %d lanes, so this proved nothing", len(enforced)-1)
	}
	// A skill's own **Runs:** line is authoritative where it has one: the lane table knows only the
	// skills the quality pass dispatches, and a skill dispatched elsewhere is invisible to it.
	declaredRuns := 0
	for skill, body := range skills {
		// `inline` must name which of the three reasons it is legitimate for, so that the claim is
		// arguable from the file rather than only by a human reading the skill. Dispatched needs no
		// reason: it is the default.
		mode := runsMode.FindSubmatch(body)
		if mode == nil {
			if runsLine.Match(body) {
				t.Errorf("%s declares how it runs in a form this cannot read; inline needs one of human, session-context or landing", skill)
			}
			continue
		}
		declaredRuns++
		enforced[skill] = string(mode[1]) == "dispatched"
	}
	if declaredRuns == 0 {
		t.Fatal("no skill declares how it runs, so the split rests on the lane table alone")
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

// A task the policy does not list is refused, including one whose path starts with a skill that has a
// row. That ancestor fallback used to answer, which is what let a renamed worker keep resolving to a
// session row one tier down — see Resolve. A dozen live-looking names sit below, so the refusal is the
// thing under test, not an edge case.
func TestAnUnlistedTaskIsRefusedRatherThanInherited(t *testing.T) {
	policy := loadShippedPolicy(t)
	own, err := policy.Resolve(Request{Client: "claude", Task: "build/explore"})
	if err != nil || own.Requested.Model != "sonnet" || own.Kind != "worker" {
		t.Fatalf("a site with its own row = %+v, %v", own, err)
	}
	// Each of these has a listed ancestor and no row of its own: a phase of a real skill, and the retired
	// names of renamed workers — every one of them a skill prefix the resolver would have answered from.
	for _, task := range []string{
		"kk-build/plan-the-change", "kk-invented/phase",
		"kk-patrol/scout", "kk-patrol/fixer", "kk-build/explore", "kk-grill/facts",
		"kk-reduce/over-cut", "kk-reduce/arbitrate", "kk-reduce/fan-out",
		"kk-reduce/reconcile", "kk-reduce/converge", "kk-reduce/repair",
		"idsd-qualify/reconcile",
	} {
		if decision, err := policy.Resolve(Request{Client: "claude", Task: task}); err == nil {
			t.Errorf("%s resolved to %+v instead of being refused", task, decision)
		}
	}
}

// The rows that name another worker's prompt, pinned. The field is the first mechanism that can run a
// protected lane's contract at a cheap row's tier, and nothing in the policy orders the tiers — so a
// downgrade added as a new row reads like an ordinary cheap site rather than an edit to the protected
// one. Pinning the set is what makes adding one a decision somebody had to write down; the ceiling
// that would compare the two tiers needs an order the file does not carry yet.
var shippedPromptOwners = map[string]string{
	"reduce/fan-out": "kk-ecosystem",
	"reduce/repair":  "kk-edit",
}

func TestOnlyThePinnedRowsNameAnotherWorkersPrompt(t *testing.T) {
	if named := loadShippedPolicy(t).PromptOwners(); !maps.Equal(named, shippedPromptOwners) {
		t.Errorf("rows naming another worker's prompt = %v; want %v — add it here with its reason, or drop the field", named, shippedPromptOwners)
	}
}

// A row naming another worker's prompt owns none itself, so what it resolves to is the whole of its
// contract: its own tier, and the name of the worker whose prompt the spawn is handed.
func TestARowNamingAnotherWorkersPromptResolvesToBoth(t *testing.T) {
	policy := loadShippedPolicy(t)
	owners := policy.PromptOwners()
	if len(owners) == 0 {
		t.Fatal("no row names another worker's prompt, so this proved nothing")
	}
	for name, target := range owners {
		decision, err := policy.Resolve(Request{Client: "claude", Task: name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if decision.Kind != "worker" || decision.Worker != target {
			t.Errorf("%s resolved as %s dispatching %q; want worker dispatching %q", name, decision.Kind, decision.Worker, target)
		}
		if _, err := policy.Resolve(Request{Client: "claude", Task: target}); err != nil {
			t.Errorf("%s names %s's prompt, which the policy does not assign: %v", name, target, err)
		}
	}
}

func TestInstalledPolicyFollowsTheInvokedMount(t *testing.T) {
	dir := t.TempDir()
	scripts := filepath.Join(dir, "owned", "kk-flavor", "scripts")
	if err := os.MkdirAll(scripts, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scripts, "model-policy.sh")
	if err := os.WriteFile(script, []byte("script"), 0700); err != nil {
		t.Fatal(err)
	}
	mount := filepath.Join(dir, "mount")
	if err := os.Symlink(script, mount); err != nil {
		t.Fatal(err)
	}
	got, err := InstalledPath(mount)
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(canonical, "owned", "kk-flavor", "models.json")
	if err != nil || got != want {
		t.Fatalf("mounted policy=%q, %v; want %q", got, err, want)
	}
	if _, err := InstalledPath(filepath.Join(dir, "unrelated")); err == nil {
		t.Fatal("guessed a policy from an unrelated location")
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

// The policy's names as a set: every check below asks whether one name is among them rather than
// walking the list.
func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func loadShippedPolicy(t *testing.T) *Policy {
	t.Helper()
	policy, err := Load(shippedPolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	return policy
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
