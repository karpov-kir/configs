package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func graph(t *testing.T, root string) (map[string]map[string]bool, []edge) {
	t.Helper()
	defined, edges, err := read(root)
	if err != nil {
		t.Fatal(err)
	}
	return defined, edges
}

// The regression that made the first version of this tool print a chain nobody walks. Twenty-two
// skills each ship a `SKILL.md`; keyed on basename they weld into one node, and the graph then
// reports edges belonging to one file as if they belonged to all of them.
func TestSameBasenameInTwoDirectoriesStaysTwoNodes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/SKILL.md", "# A\n\nsee `standards/writing.md` → **Density**\n")
	write(t, root, "b/SKILL.md", "# B\n")
	write(t, root, "standards/writing.md", "# W\n\n## Density\n")

	defined, edges := graph(t, root)
	if _, ok := defined["a/SKILL.md"]; !ok {
		t.Fatalf("nodes keyed wrong: %v", defined)
	}
	if _, ok := defined["b/SKILL.md"]; !ok {
		t.Fatalf("second SKILL.md was merged away: %v", defined)
	}
	if len(edges) != 1 || edges[0].from != "a/SKILL.md" {
		t.Fatalf("edges = %+v, want one from a/SKILL.md", edges)
	}
}

// An ambiguous bare name is dropped rather than attributed to a guess.
func TestAmbiguousBasenameIsNotCounted(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/SKILL.md", "# A\n")
	write(t, root, "b/SKILL.md", "# B\n")
	write(t, root, "c/doc.md", "# C\n\nsee `SKILL.md` → **Somewhere**\n")
	if _, edges := graph(t, root); len(edges) != 0 {
		t.Fatalf("guessed an ambiguous target: %+v", edges)
	}
}

// The correction that changes what the fan-out number means. A citer holding the file whole is being
// precise about which rule — which the citation convention demands — not opening a door. Counting
// those as surface inverts the metric: the more precisely a file cites, the worse its target scores.
func TestCiterThatReadsTheFileWholeIsPrecisionNotADoor(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n\n## Queue\n")
	write(t, root, "reads-whole.md", "You run under `std/proto.md`. See `std/proto.md` → **Caller**.\n")
	write(t, root, "door.md", "See `std/proto.md` → **Queue**.\n")

	_, edges := graph(t, root)
	got := map[string]bool{}
	for _, e := range edges {
		got[e.from] = e.precision
	}
	if len(edges) != 2 {
		t.Fatalf("edges = %+v, want 2", edges)
	}
	if !got["reads-whole.md"] {
		t.Error("a citer that also names the file bare was counted as a door")
	}
	if got["door.md"] {
		t.Error("a citer that only ever cites a section was counted as precision")
	}
}

func TestSelfCitationAndFencesAreNotEdges(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/one.md", "# One\n\n## Here\n\nsee `std/one.md` → **Here**\n"+
		"```\nsee `std/two.md` → **There**\n```\n")
	write(t, root, "std/two.md", "# Two\n\n## There\n")
	if _, edges := graph(t, root); len(edges) != 0 {
		t.Fatalf("edges = %+v, want none — one is a self-citation, one is fenced", edges)
	}
}

// A path citation resolves to the file it names, not to whatever shares its basename.
func TestPathCitationResolvesByPath(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/writing.md", "# W\n\n## Density\n")
	write(t, root, "other/writing.md", "# Other\n")
	write(t, root, "caller.md", "see `std/writing.md` → **Density**\n")
	_, edges := graph(t, root)
	if len(edges) != 1 || edges[0].to != "std/writing.md" {
		t.Fatalf("edges = %+v, want one to std/writing.md", edges)
	}
}

// `X → **A** and → **B**` makes two claims and names the file once. Counting one made the target's
// door surface read narrower than it is — and the second claim is the one a reader is most likely to
// miss, because nothing repeats the file name in front of it.
func TestChainedSectionBelongsToTheFileAlreadyNamed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n\n## Queue\n")
	write(t, root, "door.md", "Run under `std/proto.md` → **Caller** and → **Queue**.\n")

	_, edges := graph(t, root)
	got := map[string]bool{}
	for _, e := range edges {
		got[e.section] = true
	}
	if len(edges) != 2 || !got["Caller"] || !got["Queue"] {
		t.Fatalf("edges = %+v, want both Caller and Queue", edges)
	}
}

// check.sh accepts the run before an em dash as an alias for the whole heading, so a citation writing
// `**Budget**` resolves against `## Budget — the keep test`. Without the alias here, three live
// citations read as entering nothing and the section reads as unentered.
func TestHeadingAliasBeforeAnEmDash(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/hw.md", "# HW\n\n## Budget — the keep test\n")
	write(t, root, "caller.md", "run `std/hw.md` → **Budget** over every sentence\n")

	defined, edges := graph(t, root)
	// The alias resolves to the heading and is NOT a section of its own. Registering it as one keyed
	// the edge on the alias, so the real heading stayed reported UNENTERED while files entered it.
	if defined["std/hw.md"]["Budget"] {
		t.Error("the alias was registered as a section in its own right")
	}
	if !defined["std/hw.md"]["Budget — the keep test"] {
		t.Fatalf("the heading itself is missing: %v", defined["std/hw.md"])
	}
	if len(edges) != 1 || edges[0].section != "Budget — the keep test" {
		t.Fatalf("edges = %+v, want one keyed on the heading it matched", edges)
	}
}

// A bolded list item matches the `→ **Section**` shape and is not a heading, so a citation naming one
// resolves to nothing. Counting it adds a door to a section that does not exist.
func TestCitationToABoldedListItemIsNotADoor(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/t.md", "# T\n\n## 1. Core philosophy\n\n4. **Cheapest level first.** Push it down.\n")
	write(t, root, "caller.md", "see `std/t.md` → **Cheapest level first.** and `std/t.md` → **1. Core philosophy**\n")

	_, edges := graph(t, root)
	if len(edges) != 1 || edges[0].section != "1. Core philosophy" {
		t.Fatalf("edges = %+v, want only the real heading", edges)
	}
}

// The distinction that decides whether a door is debt. Cutting a restatement replaces restated text
// with a citation, and a citation from a file that does not hold the target whole is a door — so
// de-duplication raises the door count by design. A citer entering one section wants one rule from a
// file it need not load; a citer entering several uses the file enough to read it whole, and only
// that one is debt. Counting them together made a correct de-duplication look like a regression.
func TestADoorEnteringSeveralSectionsIsDistinguishable(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/p.md", "# P\n\n## Alpha\n\n## Beta\n")
	write(t, root, "spot.md", "one rule: `std/p.md` → **Alpha**\n")
	write(t, root, "deep.md", "`std/p.md` → **Alpha** and `std/p.md` → **Beta**\n")

	_, edges := graph(t, root)
	reach := map[string]map[string]bool{}
	for _, e := range edges {
		if e.precision {
			t.Fatalf("neither citer holds the file whole: %+v", e)
		}
		if reach[e.from] == nil {
			reach[e.from] = map[string]bool{}
		}
		reach[e.from][e.section] = true
	}
	if len(reach["spot.md"]) != 1 {
		t.Fatalf("spot citer entered %d sections, want 1", len(reach["spot.md"]))
	}
	if len(reach["deep.md"]) != 2 {
		t.Fatalf("deep citer entered %d sections, want 2", len(reach["deep.md"]))
	}
}

// A file the router marks read-always is held whole by every reader on every task, so no citation
// into it is a door. Read-whole detection from a bare mention in the citer cannot see that — nothing
// mentions the file, because the router loaded it — so without this the always-read set reads as the
// widest surface in the tree, which is the opposite of what being always-read means.
func TestRouterReadAlwaysFilesAreHeldWholeByEveryone(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/inject.md", "# inject\n\n## Read always\n\n- [standards/w.md](standards/w.md)\n\n## Read on trigger\n\n- [standards/t.md](standards/t.md)\n")
	write(t, root, "kk-flavor/standards/w.md", "# W\n\n## Alpha\n\n## Beta\n")
	write(t, root, "kk-flavor/standards/t.md", "# T\n\n## Gamma\n")
	write(t, root, "caller.md", "`standards/w.md` → **Alpha**, `standards/w.md` → **Beta**, `standards/t.md` → **Gamma**\n")

	_, edges := graph(t, root)
	for _, e := range edges {
		switch filepath.Base(e.to) {
		case "w.md":
			if !e.precision {
				t.Errorf("a read-always file was entered as a door at **%s**", e.section)
			}
		case "t.md":
			if e.precision {
				t.Errorf("a trigger-loaded file was counted as held whole at **%s**", e.section)
			}
		}
	}
	if len(edges) != 3 {
		t.Fatalf("edges = %+v, want 3", edges)
	}
}

// The tool's own stderr during one read, which is where the difference lives: a kind and a genuine
// ambiguity both resolve to nothing, and only the notice tells them apart.
func graphWithStderr(t *testing.T, root string) ([]edge, string) {
	t.Helper()
	real := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	_, edges, readErr := read(root)
	os.Stderr = real
	w.Close()
	var captured bytes.Buffer
	if _, err := io.Copy(&captured, r); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return edges, captured.String()
}

// "Run the skill in full, per its SKILL.md" names the kind of file, not one of them. check.sh drops
// such a basename before it reports anything, and this printed five dangling-reference notices for
// the same references — two detectors disagreeing about what resolves, which is invisible until
// someone reads both.
func TestABasenameEveryLaneCarriesNamesAKindNotAFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "skills/one/SKILL.md", "# One\n")
	write(t, root, "skills/two/SKILL.md", "# Two\n")
	write(t, root, "kk-flavor/templates/spawn.md", "Run the skill in full, per its `SKILL.md` → **Steps**\n")

	edges, stderr := graphWithStderr(t, root)
	if len(edges) != 0 {
		t.Fatalf("a kind names no file, so it opens no door: %+v", edges)
	}
	if strings.Contains(stderr, "ambiguous") {
		t.Errorf("a generic reference was reported as a dangling one: %s", stderr)
	}
}

// The other side of the same gate, and the reason it is "every lane" rather than "more than one
// file": with a single lane, that lane's whole contents would qualify as kinds, and a name shared
// between it and the shared layer is exactly the ambiguity worth reporting.
func TestOneLaneIsNotEveryLane(t *testing.T) {
	root := t.TempDir()
	write(t, root, "skills/only/notes.md", "# Only\n")
	write(t, root, "kk-flavor/notes.md", "# Shared\n\n## Density\n")
	write(t, root, "caller.md", "see `notes.md` → **Density**\n")

	edges, stderr := graphWithStderr(t, root)
	if len(edges) != 0 {
		t.Fatalf("an ambiguous name must not be guessed: %+v", edges)
	}
	if !strings.Contains(stderr, "ambiguous") {
		t.Errorf("a name two files answer to is ambiguous, not a kind: %s", stderr)
	}
}

// The whole-file read and the section citation on one line — "You run under `<file>` as an
// orchestrator (→ **Orchestrators**); read it". The citer holds the file whole, so the citation is
// precision rather than a door. A crude scan gets this wrong by treating any line with an arrow as
// carrying no bare mention, which silently turns every such citer into surface.
func TestTheWholeFileReadAndItsCitationShareOneLine(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n")
	write(t, root, "door.md", "You run under `std/proto.md` as an orchestrator: `std/proto.md` → **Caller**; read it.\n")

	_, edges := graph(t, root)
	if len(edges) != 1 || edges[0].section != "Caller" {
		t.Fatalf("edges = %+v, want one entering at Caller", edges)
	}
	if !edges[0].precision {
		t.Error("the bare mention shares the citation's line, so the citer holds the file whole")
	}
}

// check.sh truncates the CITATION to find a heading. Extending the citation to reach a longer heading
// is the inverse and accepts what check.sh refuses. Both directions accept the em-dash case, which is
// how the inversion sat behind a comment claiming it implemented check.sh's rule.
func TestCitationIsTruncatedToAHeadingNotExtended(t *testing.T) {
	headings := map[string]bool{"Caller": true, "Phase 2": true}

	if got, ok := entersAHeading(headings, "Phase 2 — Assemble Context"); !ok || got != "Phase 2" {
		t.Errorf("truncation: got %q %v, want \"Phase 2\" true", got, ok)
	}
	// Prose runs on past the heading it names, so check.sh accepts this and the old code refused it.
	if got, ok := entersAHeading(headings, "Caller of a skill"); !ok || got != "Caller" {
		t.Errorf("truncation: got %q %v, want \"Caller\" true", got, ok)
	}
	// The inverse: a citation SHORTER than the heading. The old code extended it and resolved; check.sh
	// cannot, because truncating "Phase" reaches nothing.
	if got, ok := entersAHeading(headings, "Phase"); ok {
		t.Errorf("extension: %q resolved to %q, which check.sh cannot reach", "Phase", got)
	}
	if _, ok := entersAHeading(headings, "Call"); ok {
		t.Error("half a word satisfied a citation")
	}
}

// An edge keys on the heading matched, not the string cited. Keyed on the citation, a section reached
// through truncation or the em-dash alias is reported UNENTERED while files are entering it — which
// is what `human-writing.md` → **Budget** did while three files cited it.
func TestEdgeKeysOnTheHeadingNotTheCitation(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/h.md", "# H\n\n## Budget — the keep test\n")
	write(t, root, "caller.md", "run `std/h.md` → **Budget** over every sentence\n")

	_, edges := graph(t, root)
	if len(edges) != 1 {
		t.Fatalf("edges = %+v, want 1", edges)
	}
	if edges[0].section != "Budget — the keep test" {
		t.Fatalf("edge keyed on %q, want the heading it matched", edges[0].section)
	}
}

// An unentered section is a finding only where readers enter by section. Every file the router names
// is entered whole on its trigger, so its unentered sections say nothing — and listing them buried
// the one file that is reached by citation alone under thirty-nine that are not.
func TestRouterLoadedFilesAreNotReportedUnentered(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/inject.md", "# inject\n\n## Read on trigger\n\n| when | read |\n| --- | --- |\n| coding | [standards/routed.md](standards/routed.md) |\n")
	write(t, root, "kk-flavor/standards/routed.md", "# R\n\n## Never Cited\n")
	write(t, root, "kk-flavor/standards/delta.md", "# D\n\n## Also Never Cited\n")

	defined, _ := graph(t, root)
	_, routed := routerSets(root, defined)

	if !routed["kk-flavor/standards/routed.md"] {
		t.Error("a file named in the trigger table was not counted as router-loaded")
	}
	if routed["kk-flavor/standards/delta.md"] {
		t.Error("a file the router never names was counted as router-loaded")
	}
	if !routed["kk-flavor/inject.md"] {
		t.Error("the router itself must not report its own headings as unentered")
	}
}
