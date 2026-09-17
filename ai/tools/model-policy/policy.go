// Package modelpolicy resolves what one dispatch site may spend; it never launches or observes a model.
package modelpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// Settings is what a dispatch may set. Which half a transport carries differs, and a half it cannot
// carry drops in silence — model-policy.md holds the table.
type Settings struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

// assignment is one entry in either map. Rolls belongs here rather than beside the models because it
// is the same question asked in calls instead of tier: how many times this task may ask the model.
//
// Worker names another worker's prompt, for a row that dispatches an existing pass under its own
// accounting rather than owning a prompt. Without this field such a row would have to invent a prompt
// file to satisfy the directory census, which is the gate driving the tree instead of describing it.
type assignment struct {
	Codex  *Settings `json:"codex"`
	Claude *Settings `json:"claude"`
	Rolls  int       `json:"rolls,omitempty"`
	Worker string    `json:"worker,omitempty"`
}

func (a assignment) settingsFor(client string) *Settings {
	if client == "claude" {
		return a.Claude
	}
	return a.Codex
}

// Limits holds the counts that multiply a run's cost without changing any single call's price.
type Limits struct {
	IntentsInFlight int `json:"intents-in-flight"`
}

// maxRolls caps a hand-typed roll count. Rolls multiply model calls one for one, so a slipped digit
// is a tenfold bill with nothing else in the run to signal it. No vote over prose needs more than a
// few rolls; the ceiling is here to catch `30` typed for `3`.
const maxRolls = 9

// maxIntentsInFlight bounds the same hazard one level up: each intent in flight is a whole session.
const maxIntentsInFlight = 25

// Two maps rather than one with a marker: a flag at the end of a long row is a distinction a reader
// misses, and this one decides whether the settings can be acted on at all.
type document struct {
	Version int    `json:"version"`
	Limits  Limits `json:"limits"`
	// Each client's models, cheapest first. The file already decides what every row spends; this is
	// the only thing in it that says which of two rows spends more than the other, and nothing can
	// derive that from the names — `opus` and `gpt-6-astra` order by price, not alphabetically or by
	// length.
	//
	// Held per client rather than as one list of pairs, because the two clients are set
	// independently: a row names each client's model on its own, and the codex side carries an
	// effort beside it that buys the claude side nothing.
	Tiers    map[string][]string   `json:"tiers"`
	Sessions map[string]assignment `json:"sessions"`
	Workers  map[string]assignment `json:"workers"`
}

type Policy struct {
	content document
	digest  string
}
type Request struct {
	Client string
	Task   string
}
type Decision struct {
	Client string `json:"client"`
	Task   string `json:"task"`
	// Kind is "worker" when this row sets the model of a dispatch, and "session" when it only says what
	// tier the invoking session should have been started at. A caller reading "session" cannot act on
	// the settings — nothing can change the model of a session already running.
	Kind string `json:"kind"`
	// Worker is the prompt this row dispatches, named only where that is another row's — so a caller
	// resolving such a site learns which contract to hand the spawn, not only what it may spend.
	Worker       string   `json:"worker,omitempty"`
	Requested    Settings `json:"requested"`
	Rolls        int      `json:"rolls,omitempty"`
	PolicyDigest string   `json:"policy_digest"`
}

func Parse(raw []byte) (*Policy, error) {
	var p document
	if err := decodeStrict(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid model policy: %w", err)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canonical)
	return &Policy{content: p, digest: hex.EncodeToString(sum[:])}, nil
}

func (p *document) validate() error {
	if p.Version != 4 {
		return fmt.Errorf("unsupported model policy version %d", p.Version)
	}
	if p.Limits.IntentsInFlight < 1 || p.Limits.IntentsInFlight > maxIntentsInFlight {
		return fmt.Errorf("intents-in-flight is %d, outside 1..%d", p.Limits.IntentsInFlight, maxIntentsInFlight)
	}
	if len(p.Sessions) == 0 || len(p.Workers) == 0 {
		return fmt.Errorf("model policy needs both sessions and workers")
	}
	if err := p.validateTiers(); err != nil {
		return err
	}
	if err := p.validateSessions(); err != nil {
		return err
	}
	if err := p.validatePromptOwners(); err != nil {
		return err
	}
	return p.validateAssignments()
}

// The ordering is only usable if it covers what the rows actually name. So a model absent from its
// client's list is refused here, rather than left for whoever asks later to read as "unranked": a
// comparison that quietly answers "not higher" is how a ceiling passes what it should have stopped.
//
// A row naming no model is skipped here rather than refused, because validateAssignments refuses it
// a moment later with the sentence that says why — and this check has nothing to say about a row the
// ordering cannot rank in the first place.
func (p *document) validateTiers() error {
	// An extra key is inert today — both required clients must still be present and valid — but a
	// misspelling sits in the file reading as though it ranked something, and the next hand to edit
	// the real list leaves it behind. Refusing it makes the parser report the typo, rather than
	// someone eventually wondering why their tier never applied.
	for client := range p.Tiers {
		if !slices.Contains(dispatchClients, client) {
			return fmt.Errorf("the tier order names a client %q, which nothing dispatches to", client)
		}
	}
	for _, client := range dispatchClients {
		ordered, listed := p.Tiers[client]
		if !listed || len(ordered) == 0 {
			return fmt.Errorf("the policy orders no %s models, so nothing can say which of two rows spends more", client)
		}
		seen := map[string]bool{}
		for _, model := range ordered {
			if !validName(model) {
				return fmt.Errorf("%s tier %q is not a usable model name", client, model)
			}
			// The same refusal validateSettings applies to a model a row names. Nothing selects a
			// model out of this list today, so leaving the check out would be an asymmetry rather
			// than a hole — but the ceiling that will select from it is the whole reason the list
			// exists, and then an option-shaped entry becomes the argv token after `--model`.
			if strings.HasPrefix(model, "-") {
				return fmt.Errorf("%s tier %q starts with a dash; once something selects out of this order, it would reach the CLI as a flag rather than a model", client, model)
			}
			if seen[model] {
				return fmt.Errorf("%s lists %q at two tiers, so its rank is whichever one a reader stops at", client, model)
			}
			seen[model] = true
		}
		for name, task := range p.all() {
			settings := task.settingsFor(client)
			if settings == nil || settings.Model == "" {
				continue
			}
			if !seen[settings.Model] {
				return fmt.Errorf("task %q sets %s model %q, which the tier order does not rank", name, client, settings.Model)
			}
		}
	}
	// The clients are ranked separately and compared by rank, so two lists of different lengths make
	// one client's tier three mean something the other's cannot answer.
	if len(p.Tiers["codex"]) != len(p.Tiers["claude"]) {
		return fmt.Errorf("codex orders %d tiers and claude %d; a rank means nothing across two lists of different lengths",
			len(p.Tiers["codex"]), len(p.Tiers["claude"]))
	}
	return nil
}

// The two fields that price a dispatch are refused on a session, which is invoked rather than
// dispatched, and no name may sit in both maps.
func (p *document) validateSessions() error {
	for name, session := range p.Sessions {
		if _, both := p.Workers[name]; both {
			return fmt.Errorf("%q is listed as both a session and a worker", name)
		}
		if session.Rolls != 0 {
			return fmt.Errorf("session %q sets a roll count, but nothing dispatches it", name)
		}
		if session.Worker != "" {
			return fmt.Errorf("session %q names a worker prompt, but a session is invoked rather than dispatched", name)
		}
	}
	return nil
}

// What every row carries, whichever of the two maps it sits in.
func (p *document) validateAssignments() error {
	for name, task := range p.all() {
		if !validName(name) {
			return fmt.Errorf("invalid task name %q", name)
		}
		if task.Codex == nil || task.Claude == nil {
			return fmt.Errorf("task %q needs an entry for both codex and claude", name)
		}
		if task.Rolls < 0 || task.Rolls > maxRolls {
			return fmt.Errorf("task %q asks for %d rolls, outside 0..%d", name, task.Rolls, maxRolls)
		}
		// A vote decides by strict majority, so an even count is a unanimity requirement in a vote's
		// clothes: two rolls that split delete nothing, and the judge passes text it would have cut.
		if task.Rolls%2 == 0 && task.Rolls != 0 {
			return fmt.Errorf("task %q asks for %d rolls; an even count cannot break a tie, so use an odd one", name, task.Rolls)
		}
		// In dispatchClients' order rather than a map's, so a row invalid for both clients always
		// reports the same half first.
		for _, client := range dispatchClients {
			if err := validateSettings(client, *task.settingsFor(client)); err != nil {
				return fmt.Errorf("task %q: %w", name, err)
			}
		}
	}
	return nil
}

// A row naming another worker's prompt has to reach a real one in one hop. Unchecked, the field is a
// way to spell a row that names nothing — the directory census accepts it because it claims someone
// else's file, and no check ever opens that file.
func (p *document) validatePromptOwners() error {
	for name, task := range p.Workers {
		if task.Worker == "" {
			continue
		}
		if task.Worker == name {
			return fmt.Errorf("worker %q names itself as the prompt it dispatches, so it declares no prompt at all", name)
		}
		target, ok := p.Workers[task.Worker]
		if !ok {
			return fmt.Errorf("worker %q dispatches the prompt of %q, which is not a worker row", name, task.Worker)
		}
		// One hop, so the prompt a row names is always a row that owns one: a chain would let the file
		// this resolves to depend on reading two other rows, and a cycle would have no file at all.
		if target.Worker != "" {
			return fmt.Errorf("worker %q dispatches %q, which names %q's prompt in turn; name the prompt's owner directly", name, task.Worker, target.Worker)
		}
	}
	return nil
}

func (p *document) all() map[string]assignment {
	merged := make(map[string]assignment, len(p.Sessions)+len(p.Workers))
	for name, task := range p.Sessions {
		merged[name] = task
	}
	for name, task := range p.Workers {
		merged[name] = task
	}
	return merged
}

// An unknown task is refused rather than resolved to anything: a dispatch that omits its model takes
// the orchestrator's, so an unlisted task would bill at its parent's tier with nothing to say so.
func (p *Policy) Resolve(request Request) (Decision, error) {
	if !slices.Contains(dispatchClients, request.Client) {
		return Decision{}, fmt.Errorf("unknown client %q", request.Client)
	}
	// One exact lookup, and no fallback to the row of the skill a task's path starts with: that
	// fallback answers a dead worker name from its skill's session row, so the dispatch sets no model,
	// inherits its caller's tier and exits 0 with nothing saying the name is gone. Order between the
	// two maps is immaterial — validate refuses any name held by both — and a slice keeps it fixed.
	lookups := []struct {
		kind string
		rows map[string]assignment
	}{{"worker", p.content.Workers}, {"session", p.content.Sessions}}
	for _, lookup := range lookups {
		task, ok := lookup.rows[request.Task]
		if !ok {
			continue
		}
		requested := task.settingsFor(request.Client)
		return Decision{
			Client:       request.Client,
			Task:         request.Task,
			Kind:         lookup.kind,
			Worker:       task.Worker,
			Requested:    *requested,
			Rolls:        task.Rolls,
			PolicyDigest: p.digest,
		}, nil
	}
	// The name, and where a right one comes from: a stale caller's first need is the live name rather than
	// a row, since "add it" alone sends them to write a second row for a worker that already has one, which
	// the two-way file check refuses. Conditional, because three row shapes own no file under workers/.
	return Decision{}, fmt.Errorf("the policy assigns no model to %q; if this names a worker whose key was renamed, the live one is its path under kk-flavor/workers/ without the .md — otherwise add a row rather than letting the dispatch inherit its parent's", request.Task)
}

// SessionTasks answers which rows set no dispatch, so a check can compare them against the skills tree.
func (p *Policy) SessionTasks() []string {
	names := make([]string, 0, len(p.content.Sessions))
	for name := range p.content.Sessions {
		names = append(names, name)
	}
	return names
}

// WorkerTasks answers which rows are dispatched, so a reader of the policy can be built from the
// rows themselves rather than from the files that happen to hold a prompt. Only one of the four ways
// a row resolves its prompt leaves a file under workers/, so a list keyed on that directory silently
// prices fewer dispatch sites than exist.
func (p *Policy) WorkerTasks() []string {
	names := make([]string, 0, len(p.content.Workers))
	for name := range p.content.Workers {
		names = append(names, name)
	}
	return names
}

// PromptOwners answers which rows dispatch another row's prompt, keyed by the row and valued by the
// row that owns it, so a check can resolve such a site to the file holding its contract without
// parsing the policy a second time.
func (p *Policy) PromptOwners() map[string]string {
	named := map[string]string{}
	for name, task := range p.content.Workers {
		if task.Worker != "" {
			named[name] = task.Worker
		}
	}
	return named
}

// Limits is how a skill reads a count from here rather than parsing the file itself — the policy is
// meant to be the one place a cost multiplier is decided, and a second parser is a second answer.
func (p *Policy) Limits() Limits {
	return Limits{IntentsInFlight: p.content.Limits.IntentsInFlight}
}

func (p *Policy) Digest() string {
	return p.digest
}

// TierOf ranks one client's model, cheapest at 0. The bool is not a courtesy: a caller comparing two
// rows has to tell "cheaper" from "not ranked at all". validateTiers leaves one way to reach that
// second answer — asking about a model no row names.
func (p *Policy) TierOf(client, model string) (int, bool) {
	for rank, name := range p.content.Tiers[client] {
		if name == model {
			return rank, true
		}
	}
	return 0, false
}

// TopTier names the most expensive model one client has, which is the ceiling an orchestrator may not
// reach. Derived from the order rather than pinned anywhere, so adding a tier above the current one
// moves the ceiling with it instead of leaving a gate guarding a rank that is no longer the top.
//
// The bool is the same answer TierOf's is: a caller comparing a row against the top has to tell "not
// at the top" from "there is no top here". validateTiers leaves one way to reach that second answer —
// asking about a client dispatchClients does not name. An empty string in its place would compare
// equal to nothing and report a clean ceiling over a tree nothing had measured.
func (p *Policy) TopTier(client string) (string, bool) {
	ordered := p.content.Tiers[client]
	if len(ordered) == 0 {
		return "", false
	}
	return ordered[len(ordered)-1], true
}

// OrchestratorsAtTheCeiling names every skill in `declared` that calls itself an orchestrator and is
// priced at the top tier, as `<client>/<skill>`, sorted. An orchestrator claims every substantive step
// is dispatched; the most expensive row is what the work a session keeps costs, so a skill holding
// both says two things that cannot both be true and neither file says which one to believe.
//
// Given the declarations rather than reading them, so a caller can hand it a tree that has the defect.
// A gate only ever run against a tree that passes is one nobody has watched fail.
//
// Exported because the check that runs it over the shipped tree lives in `ai/tools`: that tree is
// outside this module, and Go keys a package's test cache on the module, so a case here that read
// kk-flavor/skills/ would answer `ok (cached)` over declarations that had changed underneath the run.
// The fixture case beside this file is what holds the derivation itself.
//
// Every row names a model for every client — validateSettings refuses one that does not — so every
// orchestrator ranks and this needs no arm for a row it cannot judge.
//
// The unranked arm below is the other half of that, and it is unreachable on purpose rather than by
// luck: it walks the same dispatchClients validateTiers demands a non-empty order for, so every
// client asked about here is one Parse refused to leave unranked. It stays because a client added to
// that list with no order behind it would otherwise make this report an empty ceiling and pass.
func (p *Policy) OrchestratorsAtTheCeiling(declared map[string]string) ([]string, error) {
	var atTop []string
	for _, client := range dispatchClients {
		top, ranked := p.TopTier(client)
		if !ranked {
			return nil, fmt.Errorf("the policy orders no %s tiers, so nothing here knows which model is the top one", client)
		}
		for skill, mode := range declared {
			if mode != "orchestrator" {
				continue
			}
			decision, err := p.Resolve(Request{Client: client, Task: skill})
			if err != nil {
				return nil, fmt.Errorf("%s declares itself an orchestrator and the policy does not price it: %w", skill, err)
			}
			if decision.Requested.Model == top {
				atTop = append(atTop, client+"/"+skill)
			}
		}
	}
	slices.Sort(atTop)
	return atTop, nil
}

// TaskNames answers what the policy covers, so a check can compare it against the dispatch sites that
// exist rather than trusting a hand-kept list.
func (p *Policy) TaskNames() []string {
	all := p.content.all()
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	return names
}

// Selection is one thing this file asserts a provider will run, and not a bare model name: the same
// string means different things to the two CLIs, and codex does not offer the same effort on every
// model. Measured 2026-09-15 — gpt-5.6-terra takes `ultra` and gpt-5.6-luna does not, so what a
// provider runs or refuses is the whole selection.
type Selection struct {
	// Which part of the file asserts it: the task the row belongs to, or tierOrigin where only the
	// order holds it. A row wins where both do, because the row is the side that can carry an effort.
	Origin string
	Client string
	Model  string
	Effort string
}

// tierOrigin stands where a task name would, and cannot be mistaken for one: validName bars
// whitespace, so no row is keyed like this.
const tierOrigin = "tier order"

// selectionKey is a selection minus where it came from — one question to a provider, however many
// places in the file ask it.
type selectionKey struct{ client, model, effort string }

// rowModel is a model one client's rows name, at whatever effort they name it.
type rowModel struct{ client, model string }

// Selections lists every model selection this file asserts, for a check that resolves each one
// against the provider that would run it.
//
// Two sources, and either one alone is incomplete. The rows are the only place an effort exists, so
// nothing else says what a dispatch actually sends. The order is checked one way only — validateTiers
// refuses a row naming a model the order does not rank, never the reverse — so the order may rank a
// model no row names yet. That is still a name this file asserts, and still the one a ceiling compares
// a row against, so walking the rows alone would leave it unasked.
//
// The two are not concatenated, though. A tier name whose model some row already names for that
// client is left out: every dispatch of that model carries the effort its row sets, so asking about
// the bare name would probe a selection this file never sends. Where no row reaches the name, the
// bare model is the whole of what the file says about it, and it stays.
//
// Deduplicated on client, model and effort together: two rows naming one model at one effort are one
// question to ask, and one model at two efforts is two. Ordered rows first by task name, then the
// order's own cheapest-first names, so a report reads the same way twice.
func (p *Policy) Selections() []Selection {
	var found []Selection
	seen := map[selectionKey]bool{}
	keep := func(candidate Selection) {
		key := selectionKey{client: candidate.Client, model: candidate.Model, effort: candidate.Effort}
		if seen[key] {
			return
		}
		seen[key] = true
		found = append(found, candidate)
	}
	dispatched := map[rowModel]bool{}
	rows := p.content.all()
	for _, name := range slices.Sorted(maps.Keys(rows)) {
		row := rows[name]
		for _, client := range dispatchClients {
			settings := row.settingsFor(client)
			dispatched[rowModel{client: client, model: settings.Model}] = true
			keep(Selection{Origin: name, Client: client, Model: settings.Model, Effort: settings.Effort})
		}
	}
	for _, client := range dispatchClients {
		for _, model := range p.content.Tiers[client] {
			if dispatched[rowModel{client: client, model: model}] {
				continue
			}
			keep(Selection{Origin: tierOrigin, Client: client, Model: model})
		}
	}
	return found
}

func validateSettings(client string, settings Settings) error {
	// Every row names a model for every client. An effort alone reads as a decision and is not one: on
	// claude it changes nothing the caller can rely on — subagent dispatch drops it, and the CLI takes
	// one without the model spending differently — and on codex, which does carry one, the
	// spawn then takes whatever model the caller was running, which is the silent inheritance this
	// whole file exists to remove. It also leaves the row outside the tier order, so nothing can
	// compare it against a ceiling and an orchestrator priced that way is judged by nothing.
	if settings.Model == "" {
		return fmt.Errorf("%s names no model, so the run takes its caller's and no tier can be read from the row", client)
	}
	if !validName(settings.Model) {
		return fmt.Errorf("%s model holds whitespace or control characters", client)
	}
	// A model becomes the argv token straight after `--model`, where a `--` separator cannot shield it
	// the way it shields a positional. An option-shaped name would reach the child process as a flag
	// of this file's choosing, `--dangerously-skip-permissions` among them.
	if strings.HasPrefix(settings.Model, "-") {
		return fmt.Errorf("%s model %q starts with a dash, which would reach the CLI as a flag rather than a model", client, settings.Model)
	}
	if !validEffort(client, settings.Effort) {
		return fmt.Errorf("unsupported %s effort %q", client, settings.Effort)
	}
	return nil
}

// A task name is read as a path by everything that resolves a row to the prompt it dispatches — a
// worker's file under workers/, a skill's SKILL.md — so what is legal here is what is legal as a
// relative path inside the tree. `/` has to be allowed, because `patrol/scout` is a real row; that is
// what makes the rest of this necessary. So a name is refused unless every segment between the
// slashes names something — never empty, never `.`, never `..`.
//
// Without the segment rule a row keyed `../../../x` resolves to a file outside the checkout, and the
// tools that read it are not all silent about what they found: the field guide prints a worker's
// first sentence onto a committed page. Validating here rather than at each reader is the point —
// there are four such readers and a fifth is a step away.
func validName(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}
	if strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// The two CLIs a row is written for. Named once because everything that walks both clients reads it —
// the tier orders that must exist, the halves of a row validated, Resolve, Selections, a ceiling
// check — and those only stay in step while they read the same list.
var dispatchClients = []string{"codex", "claude"}

// DispatchClients is that list, for the shipped-tree checks in `ai/tools`. They live there because the
// tree is outside this module and Go's test cache cannot see it; they still have to walk the same two
// clients as everything here, and a second list written out beside them is one that drifts.
func DispatchClients() []string { return slices.Clone(dispatchClients) }

// The efforts both CLIs answer to, and the three codex carries on its own.
var (
	sharedEfforts = []string{"low", "medium", "high", "xhigh", "max"}
	codexEfforts  = []string{"none", "minimal", "ultra"}
)

func validEffort(client, effort string) bool {
	if effort == "" {
		return true
	}
	if slices.Contains(sharedEfforts, effort) {
		return true
	}
	return client == "codex" && slices.Contains(codexEfforts, effort)
}

func decodeStrict[T any](raw []byte, target *T) error {
	if len(raw) > 1<<20 {
		return fmt.Errorf("JSON exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := checkValue(decoder, reflect.TypeOf(*target), 0); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("trailing JSON value")
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
