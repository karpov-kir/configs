package main

import (
	"os"
	"path/filepath"
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
	if !defined["std/hw.md"]["Budget"] || !defined["std/hw.md"]["Budget — the keep test"] {
		t.Fatalf("both the heading and its alias must be defined: %v", defined["std/hw.md"])
	}
	if len(edges) != 1 || edges[0].section != "Budget" {
		t.Fatalf("edges = %+v, want one entering at Budget", edges)
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
