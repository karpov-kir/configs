// One mutation unit per suite set, not per file.
//
// What these pin is the reason for the grouping rather than the grouping itself: the harness runs a
// full uncached baseline per invocation, so the number of invocations IS the cost, and the only key
// that shares a baseline correctly is the set of suites it covers. A second half sits below it: what
// a unit is keyed on, which is every package its suites compile.
package gate

import (
	"slices"
	"strings"
	"testing"
)

// The listing the harness prints for `-units`: file, suites, mutant count, resolved path. The three
// eco-root rows are the real tree's, and they are why the key is the suite set — each of them is
// mutated against a different package's suite. The first column is spelled from `ai/tools`, which is
// what the harness prints and what it takes back on `-file`: two of this tree's files are called
// shell.go, and a bare name would name both.
const unitsListing = "" +
	"eco-check/shell.go\t./eco-check/\t2\t/repo/ai/tools/eco-check/shell.go\n" +
	"eco-report/shell.go\t./eco-report/\t3\t/repo/ai/tools/eco-report/shell.go\n" +
	"eco-check/tree.go\t./eco-check/\t7\t/repo/ai/tools/eco-check/tree.go\n" +
	"eco-root/imports.go\t./eco-check/\t4\t/repo/ai/tools/eco-root/imports.go\n" +
	"eco-root/contained.go\t./eco-stats/\t1\t/repo/ai/tools/eco-root/contained.go\n" +
	"eco-root/eco-root.go\t./eco-root/\t2\t/repo/ai/tools/eco-root/eco-root.go\n" +
	"shell/text.go\t./shell/,./eco-stats/\t5\t/repo/ai/tools/shell/text.go\n" +
	"scratch_isolation_test.go\t./\t1\t/repo/ai/tools/scratch_isolation_test.go\n"

// What each listed suite compiles, in the shape moduleImports returns and carrying the real tree's
// relationships: eco-report reaches repo-key through root.go, eco-stats reaches eco-check through its
// own harness_test.go. The real graph is read in inputs_test.go.
var suiteCompiles = map[string][]string{
	"ai/tools":            {"ai/tools"},
	"ai/tools/eco-check":  {"ai/tools/eco-check", "ai/tools/eco-root", "ai/tools/shell"},
	"ai/tools/eco-report": {"ai/tools/diffscan", "ai/tools/eco-report", "ai/tools/repo-key", "ai/tools/shell", "ai/tools/tree-fingerprint"},
	"ai/tools/eco-root":   {"ai/tools/eco-root", "ai/tools/shell"},
	"ai/tools/eco-stats":  {"ai/tools/eco-check", "ai/tools/eco-root", "ai/tools/eco-stats", "ai/tools/shell"},
	"ai/tools/shell":      {"ai/tools/shell"},
}

func grouped(t *testing.T) []mutantGroup {
	t.Helper()
	groups, err := groupMutants(unitsListing, "/repo", suiteCompiles)
	if err != nil {
		t.Fatalf("the listing did not group: %v", err)
	}
	return groups
}

func groupNamed(t *testing.T, id string) mutantGroup {
	t.Helper()
	for _, group := range grouped(t) {
		if group.id == id {
			return group
		}
	}
	t.Fatalf("no unit called %s", id)
	return mutantGroup{}
}

// The whole point: eight files become six units, because six distinct suite sets is six baselines.
// Per file it was eight invocations and so eight baselines; the two that go are eco-check's suite,
// which the per-file shape ran three times over to judge three files.
func TestMutantsGroupIntoOneUnitPerSuiteSet(t *testing.T) {
	groups := grouped(t)
	if len(groups) != 6 {
		var ids []string
		for _, g := range groups {
			ids = append(ids, g.id)
		}
		t.Fatalf("got %d unit(s) — %s — want 6", len(groups), strings.Join(ids, " "))
	}
	if files := groupNamed(t, "mutants:go:eco-check").files; len(files) != 3 {
		t.Fatalf("eco-check's unit runs %v, want all three of its files in one invocation", files)
	}
}

// The files reach the harness as one comma-separated `-file` argument, which is the whole saving: one
// invocation is one baseline. Each is checked against safeToken before the join, so the separator is
// the only comma in the argument.
func TestAGroupHandsTheHarnessEveryFileAtOnce(t *testing.T) {
	argument := strings.Join(groupNamed(t, "mutants:go:eco-check").files, ",")
	for _, want := range []string{"eco-check/shell.go", "eco-check/tree.go", "eco-root/imports.go"} {
		if !strings.Contains(argument, want) {
			t.Fatalf("-file %s does not name %s", argument, want)
		}
	}
	if strings.Count(argument, ",") != 2 {
		t.Fatalf("-file %s does not separate exactly three files", argument)
	}
}

// Grouped by the directory a file sits in, these three eco-root files would be one unit. They are
// mutated against three different suites, so that unit's baseline would cover none of them properly —
// and a baseline that does not cover the suite a mutant is judged by makes its verdict meaningless.
func TestFilesInOneDirectorySplitWhenTheirSuitesDiffer(t *testing.T) {
	for id, suite := range map[string]string{
		"mutants:go:eco-check": "eco-root/imports.go",
		"mutants:go:eco-stats": "eco-root/contained.go",
		"mutants:go:eco-root":  "eco-root/eco-root.go",
	} {
		if group := groupNamed(t, id); !slices.Contains(group.files, suite) {
			t.Errorf("%s is not in %s, whose suite is the one it is mutated against", suite, id)
		}
	}
}

// A unit is keyed on every suite directory it covers plus each mutated file, so a change to either
// re-runs it. The suite directory is why grouping costs no freshness: `eco-check/tree.go` sits inside
// `ai/tools/eco-check`, so every eco-check unit already re-ran whenever any of them did.
func TestAGroupIsKeyedOnItsSuitesAndItsFiles(t *testing.T) {
	held := map[string]bool{}
	for _, input := range groupNamed(t, "mutants:go:eco-check").inputs {
		held[input] = true
	}
	for _, want := range []string{
		"ai/tools/go-mutate",
		"ai/tools/eco-check",
		"ai/tools/eco-check/tree.go",
		"ai/tools/eco-root/imports.go",
	} {
		if !held[want] {
			t.Errorf("the unit is not keyed on %s", want)
		}
	}
}

// A unit is keyed on the packages its suites compile, not only on the directories they live in:
// editing repokey.go moves what every eco-report mutant is judged by, and that unit stayed fresh.
func TestAUnitIsKeyedOnThePackagesItsSuiteCompiles(t *testing.T) {
	group := groupNamed(t, "mutants:go:eco-report")
	for _, want := range []string{"ai/tools/repo-key", "ai/tools/shell", "ai/tools/tree-fingerprint"} {
		if !slices.Contains(group.inputs, want) {
			t.Errorf("the unit is not keyed on %s, which its suite compiles, so an edit there leaves "+
				"its verdict fresh over mutants that were never re-applied", want)
		}
	}
	// The other half, and why this reads the graph rather than widening every unit to the tree.
	if slices.Contains(group.inputs, "ai/tools/cadence") {
		t.Error("the unit is keyed on a package its suite never compiles, so it re-runs on edits that " +
			"cannot move its verdict")
	}
}

// A suite the graph says nothing about refuses the run: silence would key the unit on its directory.
func TestASuiteTheImportGraphDoesNotNameRefuses(t *testing.T) {
	_, err := groupMutants("x.go\t./eco-guide/\t1\t/repo/ai/tools/eco-guide/x.go\n", "/repo", suiteCompiles)
	if err == nil || !strings.Contains(err.Error(), "which packages") {
		t.Fatalf("got %v, want a refusal naming the suite whose imports are unknown", err)
	}
}

// A suite's own directory arrives twice — once as the suite, once out of its import closure — and a
// unit declaring an input twice makes a count read off `--units` disagree with `--why`.
func TestASuiteDeclaredByBothItsPathAndItsImportsReachesTheUnitOnce(t *testing.T) {
	group := groupNamed(t, "mutants:go:eco-check")
	if count(group.inputs, "ai/tools/eco-check") < 2 {
		t.Fatalf("the suite directory no longer arrives from both lists, so registering it below shows "+
			"the dedupe nothing: %v", group.inputs)
	}
	g := &gate{}
	g.add(group.id, "mutation", group.inputs, "run")
	if n := count(g.units[0].inputs, "ai/tools/eco-check"); n != 1 {
		t.Errorf("the unit declares its suite directory %d times", n)
	}
}

// The graph, read off a listing in `go list`'s own shape. What no `Deps` column carries is the test
// files' own imports, and they are how eco-stats' suite reaches eco-check and then eco-root.
func TestWhatASuiteCompilesCoversItsTestImportsTransitively(t *testing.T) {
	listing := "" +
		"kk-flavor/tools/shell\t/repo/ai/tools/shell\t\t\t\n" +
		"kk-flavor/tools/eco-root\t/repo/ai/tools/eco-root\tkk-flavor/tools/shell\t\t\n" +
		"kk-flavor/tools/eco-check\t/repo/ai/tools/eco-check\tkk-flavor/tools/eco-root kk-flavor/tools/shell\t\t\n" +
		"kk-flavor/tools/eco-stats\t/repo/ai/tools/eco-stats\tkk-flavor/tools/shell\tkk-flavor/tools/eco-check\tfmt\n"
	reached, err := moduleImports(listing, "/repo")
	if err != nil {
		t.Fatalf("reading the listing: %v", err)
	}
	got := reached["ai/tools/eco-stats"]
	for _, want := range []string{"ai/tools/eco-stats", "ai/tools/shell", "ai/tools/eco-check", "ai/tools/eco-root"} {
		if !slices.Contains(got, want) {
			t.Errorf("a test binary over ai/tools/eco-stats compiles %s, and the graph does not say so: %v", want, got)
		}
	}
	// The control: without this, a graph answering the whole module to everything passes the loop above.
	if got := reached["ai/tools/shell"]; len(got) != 1 || got[0] != "ai/tools/shell" {
		t.Errorf("a package importing nothing reached %v", got)
	}
}

// A set naming two suites keys on both and says so in its name, so `--units` reads as what it runs.
func TestASetOfTwoSuitesCarriesBoth(t *testing.T) {
	group := groupNamed(t, "mutants:go:shell+eco-stats")
	held := map[string]bool{}
	for _, input := range group.inputs {
		held[input] = true
	}
	if !held["ai/tools/shell"] || !held["ai/tools/eco-stats"] {
		t.Fatalf("keyed on %v, want both suites", group.inputs)
	}
}

// A mutant naming no suite keys on the whole module, which is what it keyed on before it was grouped,
// and is called something rather than nothing (see suiteSetName).
func TestAMutantWithNoSuiteIsTheModuleRootsAndIsNamed(t *testing.T) {
	group := groupNamed(t, "mutants:go:root")
	// Membership, not position: both sibling cases read `inputs` as a set, and reordering it without
	// changing what the unit keys on is not a defect this case should report.
	if !slices.Contains(group.inputs, "ai/tools") {
		t.Fatalf("keyed on %v, want the module root among them", group.inputs)
	}
	if recordStem(group.id) == recordStem("mutants:go:") {
		t.Fatal("the unnamed set flattens to the same record as an empty one")
	}
}

// A listing row with no resolved path refuses. The gate keys a verdict on the file's hash, and a row
// that will not say which file it means leaves nothing to hash — which would key the unit on its
// suites alone and report it fresh over a file that moved.
func TestARowWithNoResolvedPathRefuses(t *testing.T) {
	_, err := groupMutants("x.go\t./eco-check/\t1\t\n", "/repo", suiteCompiles)
	if err == nil || !strings.Contains(err.Error(), "no resolved path") {
		t.Fatalf("got %v, want a refusal naming the missing path", err)
	}
}

// A file the gate cannot safely put in a command refuses before any unit exists, rather than being
// joined into a `-file` argument with everything else.
func TestAFileTheGateCannotQuoteRefuses(t *testing.T) {
	_, err := groupMutants("a;rm -rf .\t./eco-check/\t1\t/repo/ai/tools/eco-check/a.go\n", "/repo", suiteCompiles)
	if err == nil || !strings.Contains(err.Error(), "cannot safely put in a command") {
		t.Fatalf("got %v, want a refusal", err)
	}
}

// Every listed file reaches exactly one unit. This is the property the saving must not cost: the
// harness selects mutants by matching these tokens exactly, so a file dropped from every group is its
// mutants silently not run, and a file in two groups is them run twice. Measured against the real tree
// too: eco-report's 16 files in one invocation selected 256 mutants, the sum of their per-file counts.
func TestEveryFileLandsInExactlyOneUnit(t *testing.T) {
	seen := map[string]int{}
	for _, group := range grouped(t) {
		for _, file := range group.files {
			seen[file]++
		}
	}
	for _, line := range strings.Split(unitsListing, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 || fields[0] == "" {
			continue
		}
		if seen[fields[0]] != 1 {
			t.Errorf("%s reaches %d unit(s), want exactly 1", fields[0], seen[fields[0]])
		}
	}
}

// A short row refuses rather than being skipped. Skipping one drops the mutants it named out of the
// table with nothing said, so the run measures less than its own summary reports — and a count is the
// only tell, which nobody has a reason to compare. An empty listing is already refused; a truncated one
// is that same failure arriving a line at a time.
func TestATruncatedMutantRowRefuses(t *testing.T) {
	listing := "x.go\t./eco-check/\t1\t/repo/ai/tools/eco-check/x.go\n" +
		"y.go\t./eco-check/\n"
	_, err := groupMutants(listing, "/repo", suiteCompiles)
	if err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("got %v, want a refusal naming the row it could not read", err)
	}
}

// The same for the `go list` listing the import graph is built from: a package dropped here leaves
// every unit that compiles it keyed on less than its command reads.
func TestATruncatedPackageRowRefuses(t *testing.T) {
	listing := "kk-flavor/tools/shell\t/repo/ai/tools/shell\t\t\t\n" +
		"kk-flavor/tools/eco-root\t/repo/ai/tools/eco-root\n"
	_, err := moduleImports(listing, "/repo")
	if err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("got %v, want a refusal naming the row it could not read", err)
	}
}

// A blank line is not a truncated row: a listing ends in one, and refusing it would refuse every run.
func TestABlankLineIsNotATruncatedRow(t *testing.T) {
	listing := "kk-flavor/tools/shell\t/repo/ai/tools/shell\t\t\t\n\n"
	if _, err := moduleImports(listing, "/repo"); err != nil {
		t.Fatalf("a trailing blank line refused the listing: %v", err)
	}
}
