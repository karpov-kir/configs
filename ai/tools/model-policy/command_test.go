package modelpolicy

import (
	"encoding/json"
	"io/fs"
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

// The lines a SKILL.md declares its dispatch shape with, and the Lanes table row the quality pass
// dispatches by.
var (
	dispatchesLine = regexp.MustCompile(`(?m)^\*\*Dispatches:\*\*(.*)$`)
	backtickedSite = regexp.MustCompile("`([^`]+)`")
	runsMode       = regexp.MustCompile(`(?m)^\*\*Runs:\*\* *(dispatched|inline — (?:human|session-context|landing)) *$`)
	runsLine       = regexp.MustCompile(`(?m)^\*\*Runs:\*\*`)
	lanesTableRow  = regexp.MustCompile("(?m)^\\| *[a-z-]+ *\\| *`([a-z0-9-]+)` *\\|")
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
	rows := map[string]bool{}
	for _, name := range policy.TaskNames() {
		rows[name] = true
	}
	workers := shippedWorkerFiles(t)
	for name := range workers {
		if !rows[name] {
			t.Errorf("worker %s has no row, so it would dispatch at its caller's tier", name)
		}
	}
	for skill := range skillBodies(t) {
		if !rows[skill] {
			t.Errorf("skill %s has no row, so it would dispatch at its caller's tier", skill)
		}
	}
	declared := declaredDispatchSites(t)
	for name := range rows {
		skill, _, isSub := strings.Cut(name, "/")
		if !isSub {
			continue
		}
		// A worker's own file is its evidence, and it needs nothing else.
		if workers[name] {
			continue
		}
		// Nothing falls back to a skill's row any more, so every other sub-row stands on evidence of its
		// own: the **Dispatches:** entry its skill carries, because a worker spawn bills separately, or,
		// for a named path through one session that bills nothing of its own, the mode file it reads.
		if !declared[name] {
			mode := filepath.Join(skillsTree, skill, strings.TrimPrefix(name, skill+"/")+".md")
			if _, err := os.Stat(mode); err != nil {
				t.Errorf("sub-row %s has no worker file under kk-flavor/workers/, no **Dispatches:** entry in its skill, and no mode file at %s", name, mode)
			}
		}
	}
	for name := range declared {
		if !rows[name] {
			t.Errorf("dispatch site %s is declared but has no row, so it bills at its caller's tier", name)
		}
	}
}

// A session row is one nothing enforces, and which rows those are is derivable rather than a
// judgement: a model is set for the judge, for a declared dispatch site, and for the leaf skills
// kk-qualify's Lanes table dispatches; everything else runs in whatever session invoked it. The split
// stays hand-written so a reader of the policy can see it, and this checks it against the derivation.
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
		t.Fatalf("the Lanes table yielded %d leaf skills, so this proved nothing", len(enforced)-1)
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
	// A sub-row is a worker only when a skill declares it as one. A session sub-row is a named path
	// through the same session, so it takes the kind of the skill it belongs to.
	declaredSites := declaredDispatchSites(t)
	for name := range shippedWorkerFiles(t) {
		enforced[name] = true
	}
	for _, name := range policy.TaskNames() {
		skill, _, isSub := strings.Cut(name, "/")
		if !isSub {
			continue
		}
		if declaredSites[name] {
			enforced[name] = true
		} else if enforced[skill] {
			enforced[name] = true
		}
	}
	isSession := map[string]bool{}
	for _, name := range policy.SessionTasks() {
		isSession[name] = true
	}
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
// session row one tier down — see Resolve. Nine more keys are renamed in the steps after this one, so
// the refusal is the thing under test, not an edge case.
func TestAnUnlistedTaskIsRefusedRatherThanInherited(t *testing.T) {
	policy := loadShippedPolicy(t)
	own, err := policy.Resolve(Request{Client: "claude", Task: "kk-build/explore"})
	if err != nil || own.Requested.Model != "sonnet" || own.Kind != "worker" {
		t.Fatalf("a site with its own row = %+v, %v", own, err)
	}
	// Each of these has a listed ancestor and no row of its own: a phase of a real skill, and the
	// retired names of two workers this step renamed.
	for _, task := range []string{"kk-build/plan-the-change", "kk-patrol/scout", "kk-patrol/fixer", "kk-invented/phase"} {
		if decision, err := policy.Resolve(Request{Client: "claude", Task: task}); err == nil {
			t.Errorf("%s resolved to %+v instead of being refused", task, decision)
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
	rows := map[string]bool{}
	// The first segment of every sub-row, which is a task family whether or not it is a row itself.
	// `patrol/scout` and `patrol/fixer` carry no skill prefix, so keying on rows alone would leave the
	// whole family invisible to this scan.
	families := map[string]bool{}
	for _, name := range policy.TaskNames() {
		rows[name] = true
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

// Every dispatch site a skill declares, as the task name the policy would have to carry. Read off the
// **Dispatches:** line rather than the prose: every SKILL.md carries a frontmatter `description:`, so
// text-matching would make a row called "description" look declared. The line sits beside the prose
// that spawns the worker, so it moves with it.
func declaredDispatchSites(t *testing.T) map[string]bool {
	t.Helper()
	declared := map[string]bool{}
	for skill, body := range skillBodies(t) {
		line := dispatchesLine.FindSubmatch(body)
		if line == nil {
			continue
		}
		for _, site := range backtickedSite.FindAllSubmatch(line[1], -1) {
			declared[skill+"/"+string(site[1])] = true
		}
	}
	if len(declared) == 0 {
		t.Fatal("no skill declares a dispatch site, so every check against them proved nothing")
	}
	return declared
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

// The workers/ tree declares a worker: a file there is one, and its path is its task name. Not the
// only way yet — most rows under `workers` have no file and are declared inside the skill that spawns
// them — so this census is paired with the declared one. A directory cannot forget to mention itself,
// which is what a **Dispatches:** line beside the prose can always do.
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
