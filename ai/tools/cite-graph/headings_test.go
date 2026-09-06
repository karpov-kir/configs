package main

import (
	"testing"
)

// check.sh truncates the CITATION to find a heading. Extending the citation to reach a longer heading
// is the inverse and accepts what check.sh refuses. Both directions accept the em-dash case, so a
// case built on that one cannot tell them apart.
func TestCitationIsTruncatedToAHeadingNotExtended(t *testing.T) {
	headings := map[string]bool{"Caller": true, "Phase 2": true}

	if got, ok := entersAHeading(headings, "Phase 2 — Assemble Context"); !ok || got != "Phase 2" {
		t.Errorf("truncation: got %q %v, want \"Phase 2\" true", got, ok)
	}
	if got, ok := entersAHeading(headings, "Caller of a skill"); !ok || got != "Caller" {
		t.Errorf("truncation: got %q %v, want \"Caller\" true", got, ok)
	}
	if got, ok := entersAHeading(headings, "Phase"); ok {
		t.Errorf("extension: %q resolved to %q, which check.sh cannot reach", "Phase", got)
	}
	if _, ok := entersAHeading(headings, "Call"); ok {
		t.Error("half a word satisfied a citation")
	}
}

// The two tools have to agree on what resolves. `ecocheck` registers a heading's em-dash prefix as an
// alias, so a citation truncating to it resolves there; this tool does not, so without the match
// below a compliant citation reads as broken.
func TestCitationTruncatingToAnEmDashPrefixResolves(t *testing.T) {
	headings := map[string]bool{"phase 2 — assemble context (progressive)": true}
	got, ok := entersAHeading(headings, "phase 2 — assemble context")
	if !ok || got != "phase 2 — assemble context (progressive)" {
		t.Fatalf("entersAHeading = %q, %v — want the full heading", got, ok)
	}
	if _, ok := entersAHeading(headings, "phase 9 — invented"); ok {
		t.Fatal("resolved a section no heading answers to")
	}
}

// A heading numbered `## 7. What a suite reports` is cited by its text alone, and eco-check resolves
// it that way. Without the same rule here the two detectors disagree about what resolves, which this
// file calls worse than matching wrong: the edge goes uncounted and the heading is reported UNENTERED
// while a file enters it. The citation is indistinguishable from an invented one in the output.
func TestCitationToANumberedHeadingResolves(t *testing.T) {
	headings := map[string]bool{"7. what a suite reports": true}
	got, ok := entersAHeading(headings, "what a suite reports")
	if !ok || got != "7. what a suite reports" {
		t.Fatalf("entersAHeading = %q, %v — want the full numbered heading", got, ok)
	}
	if got, ok := entersAHeading(headings, "7. what a suite reports"); !ok || got != "7. what a suite reports" {
		t.Fatalf("citing the number too = %q, %v — want the heading", got, ok)
	}
	if _, ok := entersAHeading(headings, "what a suite invented"); ok {
		t.Fatal("resolved a section no heading answers to")
	}
	plain := map[string]bool{"what a suite reports": true}
	if _, ok := entersAHeading(plain, "reports"); ok {
		t.Fatal("a bare fragment resolved against an unnumbered heading")
	}
}

func TestNumberedHeadingIsEnteredNotDangling(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/target.md", "# T\n\n## 7. What A Suite Reports\n\n## Plain Heading\n")
	write(t, root, "citer.md", "see `std/target.md` → **What A Suite Reports** for it\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 {
		t.Fatalf("edges = %+v, want 1 — the numbered heading was not entered", edges)
	}
	if edges[0].section != "7. What A Suite Reports" {
		t.Errorf("edge entered %q, want the full numbered heading", edges[0].section)
	}
}

// The numbered carve-out and the em-dash one compose, and eco-check composes them: it takes the
// numberless of every form it registers, the em-dash prefix included. Headings wearing both at once
// (`## 1. Trigger — how it gets invoked`) are where matching each carve-out alone leaves exactly the
// disagreement they were added to close.
func TestNumberedHeadingWithASubtitleResolvesByItsTextAlone(t *testing.T) {
	headings := map[string]bool{"1. trigger — how it gets invoked": true}

	for _, cited := range []string{
		"1. trigger — how it gets invoked",
		"trigger — how it gets invoked",
		"trigger — how it gets",
		"trigger —",
		"1. trigger",
		"trigger",
	} {
		got, ok := entersAHeading(headings, cited)
		if !ok || got != "1. trigger — how it gets invoked" {
			t.Errorf("entersAHeading(%q) = %q, %v — want the full heading", cited, got, ok)
		}
	}
	if got, ok := entersAHeading(headings, "1."); ok {
		t.Errorf("the leading number alone resolved to %q", got)
	}
	if got, ok := entersAHeading(headings, "invented"); ok {
		t.Errorf("a section no heading answers to resolved to %q", got)
	}
}

// Two headings in one file that answer to the same alias. Nothing in this tree does today, so the
// fixture is built rather than found — a defect surviving because no committed file happens to trigger
// it yet is one that ships. Repeated, because a single run of a coin flip proves nothing: Go
// randomises map iteration order per range, so one pass over an unsorted map passes about half the
// time.
//
// The edge keys on whatever comes back here, so an unstable winner means the same commit reports a
// different section entered and a different UNENTERED list on two runs — the shape already fixed twice
// in this package, at read.go's routerPath and at nameResolver.tails.
func TestTwoHeadingsSharingAnAliasResolveToTheSameOneEveryTime(t *testing.T) {
	// Both answer to `Caller` under the SAME alias — BeforeEmDash — which is what makes them
	// indistinguishable to the loop. Two headings colliding under DIFFERENT aliases prove nothing here:
	// headingAliases is the outer loop, so the first alias to match already picks the winner whatever
	// order the map is read in.
	headings := map[string]bool{
		"Caller — the skill's":        true,
		"Caller — the orchestrator's": true,
		"Queue":                       true,
	}
	first, ok := headingByAlias(headings, "Caller")
	if !ok {
		t.Fatalf("neither heading answered to the alias: %v", headings)
	}
	for range 200 {
		got, ok := headingByAlias(headings, "Caller")
		if !ok || got != first {
			t.Fatalf("headingByAlias returned %q (ok=%v), then %q — the winner came out of a map iteration", first, ok, got)
		}
	}
	// Which one it settles on is arbitrary — both genuinely answer to the run — so what is pinned is
	// that it is the sort's answer and not the map's. Byte order, so a reader can predict it.
	if first != "Caller — the orchestrator's" {
		t.Errorf("headingByAlias = %q, want the first in byte order of the two that collide", first)
	}
}

// The same collision reached through the resolver every citation goes through, so the stable winner is
// what an edge is actually keyed on rather than a property of a helper nothing calls that way.
func TestACitationIntoACollidingAliasKeysOnOneHeadingEveryTime(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/p.md", "# P\n\n## Caller — the skill's\n\n## Caller — the orchestrator's\n")
	write(t, root, "caller.md", "see `std/p.md` → **Caller**\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 {
		t.Fatalf("edges = %+v, want 1", edges)
	}
	want := edges[0].section
	for range 50 {
		_, again, _ := graph(t, root)
		if len(again) != 1 || again[0].section != want {
			t.Fatalf("the edge entered %q, then %+v — the same tree gave two answers", want, again)
		}
	}
}

func TestNumberedHeadingWithASubtitleIsEnteredNotDangling(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/skill.md", "# S\n\n## 1. Trigger — how it gets invoked\n\n## Plain Heading\n")
	write(t, root, "citer.md", "see `std/skill.md` → **Trigger** for it\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 {
		t.Fatalf("edges = %+v, want 1 — the numbered heading with a subtitle was not entered", edges)
	}
	if edges[0].section != "1. Trigger — how it gets invoked" {
		t.Errorf("edge entered %q, want the full heading", edges[0].section)
	}
}
