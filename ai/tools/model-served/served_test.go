package modelserved

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var tiers = []string{"haiku", "sonnet", "opus"}

// probedToday is an account that serves opus and answers the two lower tiers with it.
func probedToday(t *testing.T) (string, time.Time) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models-served.json")
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	err := Replace(path, "claude", "a@example.invalid (Org, team)", map[string]string{
		"haiku": "claude-opus-5-5[1m]", "sonnet": "claude-opus-5-5[1m]", "opus": "claude-opus-5-5",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	return path, now
}

// A row asking for a model the account does not serve dispatches the nearest tier above it that the
// account serves, where a subagent would otherwise fall back to its parent's model.
func TestARowTheAccountDoesNotServeDispatchesTheNearestServedTier(t *testing.T) {
	path, now := probedToday(t)
	entry, why := Lookup(path, "claude", "a@example.invalid (Org, team)", now)
	if why != "" {
		t.Fatalf("no entry: %s", why)
	}
	for requested, want := range map[string]string{"haiku": "opus", "sonnet": "opus", "opus": "opus", "fable": "fable"} {
		if got := Nearest(entry, tiers, requested); got != want {
			t.Errorf("%s dispatched as %s, want %s", requested, got, want)
		}
	}
}

// A set probed under another login says nothing about this one: a person can switch accounts.
func TestASetFromAnotherAccountIsNotUsed(t *testing.T) {
	path, now := probedToday(t)
	if _, why := Lookup(path, "claude", "b@example.invalid (Other, max)", now); !strings.Contains(why, "a@example.invalid") {
		t.Fatalf("got %q, want the account the set is for named", why)
	}
}

// A set older than a day reads as missing, since an organisation can change what it allows.
func TestASetPastADayReadsAsMissing(t *testing.T) {
	path, now := probedToday(t)
	if _, why := Lookup(path, "claude", "a@example.invalid (Org, team)", now.Add(MaxAge+time.Minute)); !strings.Contains(why, "past a day") {
		t.Fatalf("got %q, want the set called stale", why)
	}
}

// A call answering a tier the set records as substituted rewrites that record, with no probe spent.
func TestACallTheSetContradictsRewritesIt(t *testing.T) {
	path, now := probedToday(t)
	account := func() string { return "a@example.invalid (Org, team)" }
	said, err := Observe(path, "claude", "sonnet", "claude-sonnet-5", account)
	if err != nil || !strings.Contains(said, "now answers sonnet with claude-sonnet-5") {
		t.Fatalf("said %q, %v", said, err)
	}
	entry, _ := Lookup(path, "claude", "a@example.invalid (Org, team)", now)
	if got := Nearest(entry, tiers, "sonnet"); got != "sonnet" {
		t.Fatalf("sonnet dispatched as %s after the account began serving it", got)
	}
	// A call agreeing with the set changes nothing and names no account.
	if said, _ := Observe(path, "claude", "opus", "claude-opus-5-5", func() string { t.Fatal("asked for the account"); return "" }); said != "" {
		t.Fatalf("an agreeing call said %q", said)
	}
}
