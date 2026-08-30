package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every skill ships a `SKILL.md`. Keyed on basename they weld into one node, and the graph then
// reports edges belonging to one file as if they belonged to all of them.
func TestSameBasenameInTwoDirectoriesStaysTwoNodes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/SKILL.md", "# A\n\nsee `standards/writing.md` → **Density**\n")
	write(t, root, "b/SKILL.md", "# B\n")
	write(t, root, "standards/writing.md", "# W\n\n## Density\n")

	defined, edges, _ := graph(t, root)
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
	if _, edges, _ := graph(t, root); len(edges) != 0 {
		t.Fatalf("guessed an ambiguous target: %+v", edges)
	}
}

// A citer holding the file whole is being precise about which rule, which the citation convention
// demands, not opening a door. Counting those as surface inverts the metric: the more precisely a
// file cites, the worse its target scores.
func TestCiterThatReadsTheFileWholeIsPrecisionNotADoor(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n\n## Queue\n")
	write(t, root, "reads-whole.md", "You run under `std/proto.md`. See `std/proto.md` → **Caller**.\n")
	write(t, root, "door.md", "See `std/proto.md` → **Queue**.\n")

	_, edges, _ := graph(t, root)
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
	if _, edges, _ := graph(t, root); len(edges) != 0 {
		t.Fatalf("edges = %+v, want none — one is a self-citation, one is fenced", edges)
	}
}

// A path citation resolves to the file it names, not to whatever shares its basename.
func TestPathCitationResolvesByPath(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/writing.md", "# W\n\n## Density\n")
	write(t, root, "other/writing.md", "# Other\n")
	write(t, root, "caller.md", "see `std/writing.md` → **Density**\n")
	_, edges, _ := graph(t, root)
	if len(edges) != 1 || edges[0].to != "std/writing.md" {
		t.Fatalf("edges = %+v, want one to std/writing.md", edges)
	}
}

// `X → **A** and → **B**` makes two claims and names the file once. Counting one makes the target's
// door surface read narrower than it is.
func TestChainedSectionBelongsToTheFileAlreadyNamed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n\n## Queue\n")
	write(t, root, "door.md", "Run under `std/proto.md` → **Caller** and → **Queue**.\n")

	_, edges, _ := graph(t, root)
	got := map[string]bool{}
	for _, e := range edges {
		got[e.section] = true
	}
	if len(edges) != 2 || !got["Caller"] || !got["Queue"] {
		t.Fatalf("edges = %+v, want both Caller and Queue", edges)
	}
}

// check.sh accepts the run before an em dash as an alias for the whole heading, so `**Budget**`
// resolves against `## Budget — the keep test`. Without the alias here, live citations read as
// entering nothing and the section reads as unentered.
func TestHeadingAliasBeforeAnEmDash(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/hw.md", "# HW\n\n## Budget — the keep test\n")
	write(t, root, "caller.md", "run `std/hw.md` → **Budget** over every sentence\n")

	defined, edges, _ := graph(t, root)
	// The alias resolves to the heading and is NOT a section of its own. Registering it as one keys
	// the edge on the alias, leaving the real heading reported UNENTERED while files enter it.
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

	_, edges, _ := graph(t, root)
	if len(edges) != 1 || edges[0].section != "1. Core philosophy" {
		t.Fatalf("edges = %+v, want only the real heading", edges)
	}
}

// Cutting a restatement replaces restated text with a citation, and a citation from a file that does
// not hold the target whole is a door, so de-duplication raises the door count by design. A citer
// entering one section wants one rule from a file it need not load; a citer entering several uses the
// file enough to read it whole, and only that is debt. Counted together, a correct de-duplication
// reads as a regression.
func TestADoorEnteringSeveralSectionsIsDistinguishable(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/p.md", "# P\n\n## Alpha\n\n## Beta\n")
	write(t, root, "spot.md", "one rule: `std/p.md` → **Alpha**\n")
	write(t, root, "deep.md", "`std/p.md` → **Alpha** and `std/p.md` → **Beta**\n")

	_, edges, _ := graph(t, root)
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

// A citation into a file the router marks read-always is not a door: every reader already holds the
// file, so the citation only says which rule. Without that, the always-read set reads as the widest
// surface in the tree.
func TestRouterReadAlwaysFilesAreHeldWholeByEveryone(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/inject.md", "# inject\n\n## Read always\n\n- [standards/w.md](standards/w.md)\n\n## Read on trigger\n\n- [standards/t.md](standards/t.md)\n")
	write(t, root, "kk-flavor/standards/w.md", "# W\n\n## Alpha\n\n## Beta\n")
	write(t, root, "kk-flavor/standards/t.md", "# T\n\n## Gamma\n")
	write(t, root, "caller.md", "`standards/w.md` → **Alpha**, `standards/w.md` → **Beta**, `standards/t.md` → **Gamma**\n")

	_, edges, _ := graph(t, root)
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

// "Run the skill in full, per its SKILL.md" names the kind of file, not one of them. check.sh drops
// such a basename before it reports anything.
func TestABasenameEveryLaneCarriesNamesAKindNotAFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "skills/one/SKILL.md", "# One\n")
	write(t, root, "skills/two/SKILL.md", "# Two\n")
	write(t, root, "kk-flavor/templates/spawn.md", "Run the skill in full, per its `SKILL.md` → **Steps**\n")

	_, edges, stderr := graph(t, root)
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

	_, edges, stderr := graph(t, root)
	if len(edges) != 0 {
		t.Fatalf("an ambiguous name must not be guessed: %+v", edges)
	}
	if !strings.Contains(stderr, "ambiguous") {
		t.Errorf("a name two files answer to is ambiguous, not a kind: %s", stderr)
	}
}

// The whole-file read and the section citation on one line. The citer holds the file whole, so the
// citation is precision rather than a door. A crude scan treats any line carrying an arrow as having
// no bare mention, which silently turns every such citer into surface.
func TestTheWholeFileReadAndItsCitationShareOneLine(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n")
	write(t, root, "door.md", "You run under `std/proto.md` as an orchestrator: `std/proto.md` → **Caller**; read it.\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 || edges[0].section != "Caller" {
		t.Fatalf("edges = %+v, want one entering at Caller", edges)
	}
	if !edges[0].precision {
		t.Error("the bare mention shares the citation's line, so the citer holds the file whole")
	}
}

// An edge keys on the heading matched, not the string cited. Keyed on the citation, a section reached
// through truncation or the em-dash alias is reported UNENTERED while files are entering it.
func TestEdgeKeysOnTheHeadingNotTheCitation(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/h.md", "# H\n\n## Budget — the keep test\n")
	write(t, root, "caller.md", "run `std/h.md` → **Budget** over every sentence\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 {
		t.Fatalf("edges = %+v, want 1", edges)
	}
	if edges[0].section != "Budget — the keep test" {
		t.Fatalf("edge keyed on %q, want the heading it matched", edges[0].section)
	}
}

// An unentered section is a finding only where readers enter by section. Every file the router names
// is entered whole on its trigger, so its unentered sections say nothing, and listing them buries the
// files that are reached by citation alone.
func TestRouterLoadedFilesAreNotReportedUnentered(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/inject.md", "# inject\n\n## Read on trigger\n\n| when | read |\n| --- | --- |\n| coding | [standards/routed.md](standards/routed.md) |\n")
	write(t, root, "kk-flavor/standards/routed.md", "# R\n\n## Never Cited\n")
	write(t, root, "kk-flavor/standards/delta.md", "# D\n\n## Also Never Cited\n")

	defined, _, _ := graph(t, root)
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

// A citation wraps, and a formatter is what wraps it. Read one line at a time it is invisible: the
// edge goes uncounted, the target's door surface reads narrower than it is, and the section it
// enters is reported UNENTERED while a file enters it.
func TestCitationWrappedAcrossALineBreak(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/p.md", "# P\n\n## Alpha\n\n## Beta\n\n## Two Words\n")
	write(t, root, "after-arrow.md", "see `std/p.md` \u2192\n**Alpha**\n")
	write(t, root, "before-arrow.md", "see `std/p.md`\n\u2192 **Beta**\n")
	write(t, root, "inside-name.md", "see `std/p.md` \u2192 **Two\nWords**\n")

	_, edges, _ := graph(t, root)
	entered := map[string]string{}
	for _, e := range edges {
		entered[e.from] = e.section
	}
	for from, want := range map[string]string{
		"after-arrow.md":  "Alpha",
		"before-arrow.md": "Beta",
		"inside-name.md":  "Two Words",
	} {
		if entered[from] != want {
			t.Errorf("%s entered %q, want %q", from, entered[from], want)
		}
	}
	if len(edges) != 3 {
		t.Fatalf("edges = %+v, want 3", edges)
	}
}

// A paragraph break is not a wrap. Joining across one would let a file named in one paragraph answer
// an arrow in the next, which is an edge nobody wrote.
func TestABlankLineIsNotAWrap(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/p.md", "# P\n\n## Alpha\n")
	write(t, root, "caller.md", "see `std/p.md`\n\n\u2192 **Alpha**\n")

	if _, edges, _ := graph(t, root); len(edges) != 0 {
		t.Fatalf("edges = %+v, want none across the paragraph break", edges)
	}
}

// The router is a node, and a node keys on its path. Found by basename, any `inject.md` a lane
// committed answers for it — and which one answered came out of a map iteration, so one tree gave
// different numbers on different runs. Repeated, because a single run of a coin flip proves nothing.
func TestTheRouterIsFoundByItsPathNotItsBasename(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/inject.md", "# inject\n\n## Read always\n\n- [standards/real.md](standards/real.md)\n")
	write(t, root, "kk-flavor/standards/real.md", "# R\n\n## Alpha\n")
	write(t, root, "kk-flavor/standards/other.md", "# O\n\n## Beta\n")
	write(t, root, "skills/decoy/inject.md",
		"# decoy\n\n## Read always\n\n- [../../kk-flavor/standards/other.md](../../kk-flavor/standards/other.md)\n")

	defined, _, _ := graph(t, root)
	for range 50 {
		always, _ := routerSets(root, defined)
		if !always["kk-flavor/standards/real.md"] || always["kk-flavor/standards/other.md"] {
			t.Fatalf("the read-always tier came from %v, not from the router at %s", always, routerPath)
		}
	}
}

// A cited path that names no file must not be answered by its last segment. Answered that way, the
// graph reports an edge to a file the citation never named — which is the whole of what keying on a
// basename costs.
func TestAPathThatNamesNoFileIsNotAnsweredByItsBasename(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/standards/writing.md", "# W\n\n## Density\n")
	write(t, root, "caller.md", "see `made/up/writing.md` \u2192 **Density**\n")

	_, edges, stderr := graph(t, root)
	if len(edges) != 0 {
		t.Fatalf("a basename answered for a path the citation never named: %+v", edges)
	}
	if !strings.Contains(stderr, "no such path") {
		t.Errorf("a citation this drops has to say so: %s", stderr)
	}
}

// The other half: a path no file carries verbatim still resolves to the file it is the tail of, which
// is how `standards/writing.md` reaches the real one and why a live dependency is counted.
func TestAPathCitationResolvesToTheFileItIsTheTailOf(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kk-flavor/standards/writing.md", "# W\n\n## Density\n")
	write(t, root, "caller.md", "see `standards/writing.md` \u2192 **Density**\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 || edges[0].to != "kk-flavor/standards/writing.md" {
		t.Fatalf("edges = %+v, want one to kk-flavor/standards/writing.md", edges)
	}
}

// The path is what disambiguates. Two files answer to the basename and only one is the tail of the
// cited path, so the citation names exactly one file — read as a basename it named two and was
// dropped.
func TestThePathDecidesWhenTheBasenameCannot(t *testing.T) {
	root := t.TempDir()
	write(t, root, "one/standards/writing.md", "# W\n\n## Density\n")
	write(t, root, "two/other/writing.md", "# W2\n\n## Density\n")
	write(t, root, "caller.md", "see `standards/writing.md` \u2192 **Density**\n")

	_, edges, _ := graph(t, root)
	if len(edges) != 1 || edges[0].to != "one/standards/writing.md" {
		t.Fatalf("edges = %+v, want one to one/standards/writing.md", edges)
	}
}

// A heading reaches the report exactly as the file spelled it, which is why the sink above has to
// sanitise: nothing between the read and the print does.
func TestAHeadingReachesTheGraphUnsanitised(t *testing.T) {
	root := t.TempDir()
	write(t, root, "b.md", "# B\n\n## Dead\x1b[2KCLEAN\n")
	defined, _, _ := graph(t, root)
	if !defined["b.md"]["Dead\x1b[2KCLEAN"] {
		t.Fatalf("the control byte was dropped before the sink: %v", defined["b.md"])
	}
}

// A path the walk cannot read is named on stderr. Every figure the report prints counts over the files
// that were read, so one dropped in silence moves all of them and nothing says it went.
func TestAPathTheWalkCannotReadIsReported(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-tree")
	_, _, stderr := graph(t, missing)
	if !strings.Contains(stderr, "could not read") || !strings.Contains(stderr, "no-such-tree") {
		t.Errorf("stderr %q names no unreadable path", stderr)
	}
}

// The stderr line and the count are two different reaches. `cite-graph.sh` and CI branch on the exit
// code, which is taken from the count, and neither reads prose — so a skip that only ever reached
// stderr arrived at every automated caller as a clean run over a tree that was never opened.
func TestAPathTheWalkCannotReadIsAlsoCounted(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-tree")
	if skipped, stderr := skippedUnder(t, missing); skipped != 1 {
		t.Fatalf("skipped = %d, want 1; stderr was %q", skipped, stderr)
	}
}

// A symlinked directory is where the silence was total. Walk stats with Lstat, so the link is not a
// directory, and the `.md` suffix filter used to drop it before the regular-file guard could speak:
// the whole subtree left every figure with nothing on stderr at all. `~/.kk-flavor` is such a link and
// so is every `~/.claude/skills/*`, so this is the shape the installed layout is made of.
func TestASymlinkedDirectoryIsReportedAndCounted(t *testing.T) {
	root := t.TempDir()
	away := t.TempDir()
	write(t, away, "b.md", "# B\n\n## Beta\n")
	write(t, root, "a.md", "# A\n\n## Alpha\n")
	if err := os.Symlink(away, filepath.Join(root, "std")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}

	skipped, stderr := skippedUnder(t, root)
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1 — the subtree behind the link went unread; stderr was %q", skipped, stderr)
	}
	if !strings.Contains(stderr, "links to a directory") || !strings.Contains(stderr, "std") {
		t.Errorf("stderr %q does not name the link it did not follow", stderr)
	}
}

// A link to something this tool was never going to read is not a hole in its promise. Counting it
// would exit 2 over a tree that was read whole, which retires the refusal by making it cry wolf.
func TestALinkToANonMarkdownFileIsNotASkip(t *testing.T) {
	root := t.TempDir()
	away := t.TempDir()
	write(t, away, "script.sh", "echo hi\n")
	write(t, root, "a.md", "# A\n\n## Alpha\n")
	if err := os.Symlink(filepath.Join(away, "script.sh"), filepath.Join(root, "script.sh")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}

	if skipped, stderr := skippedUnder(t, root); skipped != 0 {
		t.Fatalf("skipped = %d, want 0 — a linked .sh was never this tool's to read; stderr was %q", skipped, stderr)
	}
}

// A `.md` link resolving to nothing is a file the tree names and this cannot read. Silent, it leaves
// the citations into it coming back as a manufactured `no such path`.
func TestADanglingMarkdownSymlinkIsCounted(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md", "# A\n\n## Alpha\n")
	if err := os.Symlink(filepath.Join(root, "gone.md"), filepath.Join(root, "b.md")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}

	skipped, stderr := skippedUnder(t, root)
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1; stderr was %q", skipped, stderr)
	}
	if !strings.Contains(stderr, "b.md") {
		t.Errorf("stderr %q does not name the dangling link", stderr)
	}
}

// The scenario as it was observed: a directory the process cannot open. The mode bit is the only
// mechanism that produces this exact failure, so the case probes whether it really denies this process
// and says why it declined rather than asserting against a directory root reads happily.
func TestAnUnreadableDirectoryIsCounted(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md", "# A\n\n## Alpha\n")
	write(t, root, "shut/b.md", "# B\n\n## Beta\n")
	shut := filepath.Join(root, "shut")
	if err := os.Chmod(shut, 0o000); err != nil {
		t.Skipf("cannot drop the mode bits here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(shut, 0o755) })
	if _, err := os.ReadDir(shut); err == nil {
		t.Skip("mode 000 does not deny this process — root or CAP_DAC_OVERRIDE, so there is no refusal to observe")
	}

	skipped, stderr := skippedUnder(t, root)
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1; stderr was %q", skipped, stderr)
	}
	if !strings.Contains(stderr, "permission denied") {
		t.Errorf("stderr %q does not name the refusal", stderr)
	}
}

// `filepath.Walk` stats with Lstat, so a `.md` symlink is not a regular file and the read guard drops
// it. Dropping it in silence is the defect: the file leaves every figure this tool prints, and a live
// citation into it comes back as a manufactured `no such path`. The installed layout is a symlink
// farm — `~/.kk-flavor` is one and every `~/.claude/skills/*` is one — and `cite-graph.sh` promises
// every `.md` under the root is read, so the skip has to reach the reader.
func TestASymlinkedMarkdownFileIsReportedNotDropped(t *testing.T) {
	root := t.TempDir()
	write(t, root, "real/b.md", "# B\n\n## Beta\n")
	write(t, root, "std/a.md", "# A\n\n## Alpha\n")
	if err := os.Symlink(filepath.Join(root, "real/b.md"), filepath.Join(root, "std/b.md")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	var errOut bytes.Buffer
	read(root, &errOut)
	if !strings.Contains(errOut.String(), "std/b.md") {
		t.Fatalf("the symlinked .md was dropped with nothing said; stderr was %q", errOut.String())
	}
}
