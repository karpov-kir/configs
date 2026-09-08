// Package modelpolicy resolves requested settings; it never launches or observes a model.
package modelpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

type Settings struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}
type Origin struct {
	Client string `json:"client"`
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}
type profile struct {
	Source string    `json:"source,omitempty"`
	Codex  *Settings `json:"codex,omitempty"`
	Claude *Settings `json:"claude,omitempty"`
}
type role struct {
	Profile string   `json:"profile"`
	Uses    []string `json:"uses"`
}
type document struct {
	Version  int                `json:"version"`
	Profiles map[string]profile `json:"profiles"`
	Roles    map[string]role    `json:"roles"`
}
type Policy struct {
	content document
	digest  string
}
type Request struct {
	Client           string
	Role             string
	Origin           *Origin
	Transport        string
	FromOriginalTask bool
	PolicyDigest     string
}
type Decision struct {
	Client       string   `json:"client"`
	Role         string   `json:"role"`
	Profile      string   `json:"profile"`
	Transport    string   `json:"transport"`
	Source       string   `json:"source"`
	Requested    Settings `json:"requested"`
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
	if p.Version != 1 {
		return fmt.Errorf("unsupported model policy version %d", p.Version)
	}
	if len(p.Profiles) == 0 || len(p.Roles) == 0 {
		return fmt.Errorf("model policy needs profiles and roles")
	}
	for name, profile := range p.Profiles {
		if !validName(name) {
			return fmt.Errorf("invalid profile name %q", name)
		}
		if profile.Source != "" {
			if profile.Source != "task-origin" || profile.Codex != nil || profile.Claude != nil {
				return fmt.Errorf("profile %q must select task-origin alone", name)
			}
			continue
		}
		if profile.Codex == nil || profile.Claude == nil {
			return fmt.Errorf("profile %q needs explicit codex and claude settings", name)
		}
		for client, settings := range map[string]*Settings{"codex": profile.Codex, "claude": profile.Claude} {
			if err := validateSettings(client, *settings); err != nil {
				return fmt.Errorf("profile %q: %w", name, err)
			}
		}
	}
	uses := map[string]string{}
	for name, role := range p.Roles {
		if !validName(name) {
			return fmt.Errorf("invalid role name %q", name)
		}
		profile, ok := p.Profiles[role.Profile]
		if !ok {
			return fmt.Errorf("role %q names unknown profile %q", name, role.Profile)
		}
		if (name == "implement" || name == "correctness" || name == "security") && profile.Source != "task-origin" {
			return fmt.Errorf("protected role %q must use task-origin", name)
		}
		if len(role.Uses) == 0 {
			return fmt.Errorf("role %q needs at least one use", name)
		}
		for _, use := range role.Uses {
			if !validName(use) {
				return fmt.Errorf("role %q has invalid use %q", name, use)
			}
			if held, ok := uses[use]; ok {
				return fmt.Errorf("use %q belongs to both %q and %q", use, held, name)
			}
			uses[use] = name
		}
	}
	return nil
}

func (p *Policy) Resolve(request Request) (Decision, error) {
	if request.Client != "codex" && request.Client != "claude" {
		return Decision{}, fmt.Errorf("unknown client %q", request.Client)
	}
	if request.Transport != "native" && request.Transport != "cli" {
		return Decision{}, fmt.Errorf("transport must be native or cli")
	}
	if request.PolicyDigest != "" && request.PolicyDigest != p.digest {
		return Decision{}, fmt.Errorf("model policy digest changed")
	}
	role, ok := p.content.Roles[request.Role]
	if !ok {
		return Decision{}, fmt.Errorf("unknown role %q", request.Role)
	}
	if request.Origin != nil {
		if request.Origin.Client != request.Client {
			return Decision{}, fmt.Errorf("origin client does not match requested client")
		}
		if request.Origin.Model != "" && !validName(request.Origin.Model) {
			return Decision{}, fmt.Errorf("invalid origin model")
		}
		if !validEffort(request.Client, request.Origin.Effort) {
			return Decision{}, fmt.Errorf("invalid origin effort %q", request.Origin.Effort)
		}
	}
	d := Decision{Client: request.Client, Role: request.Role, Profile: role.Profile, Transport: request.Transport, PolicyDigest: p.digest}
	profile := p.content.Profiles[role.Profile]
	if profile.Source != "task-origin" {
		settings := profile.Codex
		if request.Client == "claude" {
			settings = profile.Claude
		}
		d.Source = "profile"
		d.Requested = *settings
		return d, nil
	}
	if request.Origin != nil && request.Origin.Model != "" && request.Origin.Effort != "" {
		d.Source = "task-origin"
		d.Requested = Settings{Model: request.Origin.Model, Effort: request.Origin.Effort}
		return d, nil
	}
	if request.Transport == "native" && request.FromOriginalTask {
		d.Source = "original-task-native-inheritance"
		return d, nil
	}
	return Decision{}, fmt.Errorf("role %q requires the original task model and effort, or native dispatch directly from that original task", request.Role)
}

func validateSettings(client string, settings Settings) error {
	if !validName(settings.Model) {
		return fmt.Errorf("%s profile needs a nonempty model without whitespace or control characters", client)
	}
	if !validEffort(client, settings.Effort) {
		return fmt.Errorf("unsupported %s effort %q", client, settings.Effort)
	}
	return nil
}

func validName(value string) bool {
	return value != "" && len(value) <= 200 && !strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

func validEffort(client, effort string) bool {
	if effort == "" {
		return true
	}
	allowed := "low medium high xhigh max"
	if client == "codex" {
		allowed += " none minimal ultra"
	}
	for _, value := range strings.Fields(allowed) {
		if effort == value {
			return true
		}
	}
	return false
}

func ParseOrigin(raw []byte) (*Origin, error) {
	var origin Origin
	if err := decodeStrict(raw, &origin); err != nil {
		return nil, fmt.Errorf("invalid origin: %w", err)
	}
	if origin.Client != "codex" && origin.Client != "claude" {
		return nil, fmt.Errorf("origin needs an explicit codex or claude client")
	}
	if origin.Model != "" && !validName(origin.Model) {
		return nil, fmt.Errorf("invalid origin model")
	}
	if !validEffort(origin.Client, origin.Effort) {
		return nil, fmt.Errorf("invalid origin effort")
	}
	return &origin, nil
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
