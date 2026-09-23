package voicecheck

import (
	"testing"
)

// A hyphenated name is the repository's own where its tree spells it: a file, a directory, a flag on a
// line of code. A compound a comment alone spells is no name of the repository's.
func TestDerivedNamesAreWhatTheTreeSpells(t *testing.T) {
	r := newRepo(t)
	r.write("scripts/mcp-sync.sh", "echo sync\n")
	r.write("reader-judge/judge.go", "package readerjudge\n")
	r.write("cmd/run.go", "package main\n\n// A fake-live stream is one this comment coins.\nvar flag = \"--dry-run\"\n")
	r.commit("tree")
	got := DerivedNames(r.dir, r.git, "")
	for _, name := range []string{"mcp-sync", "reader-judge", "dry-run"} {
		if !got[name] {
			t.Errorf("%s is spelled by the tree and is not derived", name)
		}
	}
	if got["fake-live"] {
		t.Error("a compound only a comment spells was derived")
	}
}

// The coined-identifier check passes over a compound the tree spells hyphenated, and fires on one it
// spells only as a camelCase identifier.
func TestADerivedNameSilencesTheCoinedIdentifierCheck(t *testing.T) {
	lines := []string{"// Run it dry-run first, then read the dry-list.", "const dryRun = dryList.length;"}
	s := scanner{profile: ProfileComment, derived: map[string]bool{"dry-run": true}}
	var got []string
	for _, f := range s.scanSource("x.ts", lines, nil, lines) {
		if f.Check == checkCoinedIdent {
			got = append(got, f.Text)
		}
	}
	if len(got) != 1 || got[0] != "dry-list" {
		t.Fatalf("coined identifiers %v, want dry-list alone", got)
	}
}
