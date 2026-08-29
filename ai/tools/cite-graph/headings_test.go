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
	// Prose runs on past the heading it names, so check.sh accepts this.
	if got, ok := entersAHeading(headings, "Caller of a skill"); !ok || got != "Caller" {
		t.Errorf("truncation: got %q %v, want \"Caller\" true", got, ok)
	}
	// The inverse: a citation SHORTER than the heading. Extension resolves it; check.sh cannot,
	// because truncating "Phase" reaches nothing.
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
	// A section that truly does not exist is still refused: no heading's prefix answers to it.
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

// End to end: the numbered heading is entered, so the edge is counted and the section is not
// reported UNENTERED. The unit case above cannot see either of those.
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
// numberless of every form it registers, the em-dash prefix included. Four headings in this tree wear
// both at once (`## 1. Trigger — how it gets invoked`), so matching each carve-out alone leaves
// exactly the disagreement they were added to close.
func TestNumberedHeadingWithASubtitleResolvesByItsTextAlone(t *testing.T) {
	headings := map[string]bool{"1. trigger — how it gets invoked": true}

	// Every form eco-check registers for that heading, plus the truncations it accepts on the way.
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
	// The number alone names no section, and neither does an invented one.
	if got, ok := entersAHeading(headings, "1."); ok {
		t.Errorf("the leading number alone resolved to %q", got)
	}
	if got, ok := entersAHeading(headings, "invented"); ok {
		t.Errorf("a section no heading answers to resolved to %q", got)
	}
}

// End to end: the edge is counted and keys on the heading, so the section is not reported UNENTERED
// while a file enters it. The unit case above sees neither.
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
