// Package modelpolicy resolves what one dispatch site may spend; it never launches or observes a model.
package modelpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// Settings is what a dispatch may set. Which half a transport carries differs, and one it cannot
// carry it drops in silence — model-policy.md holds the table.
type Settings struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

// assignment is one entry in either map. Rolls belongs here rather than beside the models because it
// is the same question asked in calls instead of tier: how many times this task may ask the model.
type assignment struct {
	Codex  *Settings `json:"codex"`
	Claude *Settings `json:"claude"`
	Rolls  int       `json:"rolls,omitempty"`
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
	Version  int                   `json:"version"`
	Limits   Limits                `json:"limits"`
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
	Kind         string   `json:"kind"`
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
	if p.Version != 3 {
		return fmt.Errorf("unsupported model policy version %d", p.Version)
	}
	if p.Limits.IntentsInFlight < 1 || p.Limits.IntentsInFlight > maxIntentsInFlight {
		return fmt.Errorf("intents-in-flight is %d, outside 1..%d", p.Limits.IntentsInFlight, maxIntentsInFlight)
	}
	if len(p.Sessions) == 0 || len(p.Workers) == 0 {
		return fmt.Errorf("model policy needs both sessions and workers")
	}
	for name, session := range p.Sessions {
		if _, both := p.Workers[name]; both {
			return fmt.Errorf("%q is listed as both a session and a worker", name)
		}
		if session.Rolls != 0 {
			return fmt.Errorf("session %q sets a roll count, but nothing dispatches it", name)
		}
	}
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
		for client, settings := range map[string]*Settings{"codex": task.Codex, "claude": task.Claude} {
			if err := validateSettings(client, *settings); err != nil {
				return fmt.Errorf("task %q: %w", name, err)
			}
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
	if request.Client != "codex" && request.Client != "claude" {
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
		requested := task.Codex
		if request.Client == "claude" {
			requested = task.Claude
		}
		return Decision{
			Client:       request.Client,
			Task:         request.Task,
			Kind:         lookup.kind,
			Requested:    *requested,
			Rolls:        task.Rolls,
			PolicyDigest: p.digest,
		}, nil
	}
	return Decision{}, fmt.Errorf("the policy assigns no model to %q; add it rather than letting the dispatch inherit its parent's", request.Task)
}

// SessionTasks answers which rows set no dispatch, so a check can compare them against the skills tree.
func (p *Policy) SessionTasks() []string {
	names := make([]string, 0, len(p.content.Sessions))
	for name := range p.content.Sessions {
		names = append(names, name)
	}
	return names
}

// Limits is how a skill reads a count from here rather than parsing the file itself — the policy is
// meant to be the one place a cost multiplier is decided, and a second parser is a second answer.
func (p *Policy) Limits() Limits {
	return Limits{IntentsInFlight: p.content.Limits.IntentsInFlight}
}

func (p *Policy) Digest() string {
	return p.digest
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

func validateSettings(client string, settings Settings) error {
	if settings.Model == "" && settings.Effort == "" {
		return fmt.Errorf("%s needs a model, an effort, or both", client)
	}
	// Claude's transports carry a model and have nowhere to put an effort, so an effort alone would be
	// a row that reads as a saving and changes nothing about the bill.
	if client == "claude" && settings.Model == "" {
		return fmt.Errorf("claude needs a model: it has no per-dispatch effort, so an effort alone would not change what runs")
	}
	if settings.Model != "" && !validName(settings.Model) {
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

func validName(value string) bool {
	return value != "" && len(value) <= 200 && !strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

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
