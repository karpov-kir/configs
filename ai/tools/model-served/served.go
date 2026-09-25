// Package modelserved remembers which model an account serves for each model a row asks for, so a
// dispatch can ask for a model the account serves. An account can answer a model it does not allow
// with another: an organisation's team account answered `sonnet` and `haiku` with Opus, and a subagent
// asked for either ran on its parent session's model. A row keeps the model it intends, and the
// resolver dispatches the nearest tier at or above it that the account serves.
//
// model-check writes the set it probed, one entry per provider and account, with the probe date. An
// entry older than MaxAge reads as missing, and so does one probed under another account: a person
// with several accounts can switch, and an organisation can change what it allows. A call whose
// answering model contradicts its entry rewrites that record, so the set follows a change without a
// probe. The file is a cache: nothing tracks it, and without it every row dispatches as written.
package modelserved

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"configs/ai/tools/shell"
)

// MaxAge is how long a probed set stands. A day covers a working session, and an organisation's change
// to its allowed models reaches the next day's resolutions.
const MaxAge = 24 * time.Hour

// Entry is what one account serves through one provider: the model that answered each model asked for.
type Entry struct {
	Probed time.Time         `json:"probed"`
	Served map[string]string `json:"served"`
}

// Serves says the answering model is the one asked for. A row names an alias, such as `opus`, and the
// answer names the full model, such as `claude-opus-5-5`.
func Serves(requested, answered string) bool {
	return answered != "" && strings.Contains(strings.ToLower(answered), strings.ToLower(requested))
}

// Path is the cache file, under the user's cache directory.
func Path(lookup func(string) (string, bool)) string {
	if dir, set := lookup("XDG_CACHE_HOME"); set && dir != "" {
		return filepath.Join(dir, "kk-flavor", "models-served.json")
	}
	home, _ := lookup("HOME")
	return filepath.Join(home, ".cache", "kk-flavor", "models-served.json")
}

func key(provider, account string) string { return provider + " " + account }

// Load reads the cache. A missing or unreadable file is an empty cache.
func Load(path string) map[string]Entry {
	entries := map[string]Entry{}
	body, err := os.ReadFile(path)
	if err != nil {
		return entries
	}
	if json.Unmarshal(body, &entries) != nil {
		return map[string]Entry{}
	}
	return entries
}

func save(path string, entries map[string]Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	staging := path + ".tmp"
	if err := os.WriteFile(staging, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(staging, path)
}

// Replace stores what one probe of every model found for one provider and account.
func Replace(path, provider, account string, served map[string]string, now time.Time) error {
	entries := Load(path)
	entries[key(provider, account)] = Entry{Probed: now, Served: served}
	return save(path, entries)
}

// Lookup is the entry for this provider and account, and why there is none. The reason is empty where
// an entry stands.
func Lookup(path, provider, account string, now time.Time) (Entry, string) {
	entries := Load(path)
	entry, found := entries[key(provider, account)]
	switch {
	case found && now.Sub(entry.Probed) > MaxAge:
		return Entry{}, fmt.Sprintf("the served set for %s is from %s, past a day; run model-check", account,
			entry.Probed.Format("2006-01-02 15:04"))
	case found:
		return entry, ""
	}
	for other := range entries {
		if strings.HasPrefix(other, provider+" ") {
			return Entry{}, fmt.Sprintf("the served set is for %s, and this login is %s; run model-check",
				strings.TrimPrefix(other, provider+" "), account)
		}
	}
	return Entry{}, "no served set is kept; run model-check"
}

// Nearest is the model to dispatch for a requested one: the requested model where the account serves
// it, or the nearest tier above it that the account serves. A model the entry never probed, or one
// outside the tier order, dispatches as requested.
func Nearest(entry Entry, tiers []string, requested string) string {
	answered, probed := entry.Served[requested]
	if !probed || Serves(requested, answered) {
		return requested
	}
	at := -1
	for i, tier := range tiers {
		if tier == requested {
			at = i
		}
	}
	if at < 0 {
		return requested
	}
	for _, tier := range tiers[at+1:] {
		if Serves(tier, entry.Served[tier]) {
			return tier
		}
	}
	return requested
}

// Observe rewrites the record a call contradicts: a model newly served, or newly answered with another.
// It spends no probe, and it says what changed where it changed anything.
func Observe(path, provider, requested, answered string, account func() string) (string, error) {
	if answered == "" {
		return "", nil
	}
	entries := Load(path)
	contradicted := false
	for name, entry := range entries {
		if strings.HasPrefix(name, provider+" ") {
			if was, probed := entry.Served[requested]; probed && Serves(requested, was) != Serves(requested, answered) {
				contradicted = true
			}
		}
	}
	if !contradicted {
		return "", nil
	}
	login := account()
	entry, found := entries[key(provider, login)]
	if !found {
		return "", nil
	}
	was := entry.Served[requested]
	if Serves(requested, was) == Serves(requested, answered) {
		return "", nil
	}
	entry.Served[requested] = answered
	entries[key(provider, login)] = entry
	if err := save(path, entries); err != nil {
		return "", err
	}
	return fmt.Sprintf("model-served: %s now answers %s with %s, where it answered %s; the served set for %s follows",
		provider, requested, answered, was, login), nil
}

// Account names the Claude login a process with this environment runs on, as "email (org, plan)", or
// "" where none is signed in.
func Account(env []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), accountDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", "auth", "status", "--json")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var status struct {
		LoggedIn     bool   `json:"loggedIn"`
		Email        string `json:"email"`
		Org          string `json:"orgName"`
		Subscription string `json:"subscriptionType"`
	}
	if json.Unmarshal(out, &status) != nil || !status.LoggedIn {
		return ""
	}
	return shell.CutBytesMarked(shell.Oneline(fmt.Sprintf("%s (%s, %s)", status.Email, status.Org, status.Subscription)), 120)
}

// accountDeadline bounds the call that names the login.
const accountDeadline = 20 * time.Second
