// One mutation unit per suite set, not per file.
//
// What these pin is the reason for the grouping rather than the grouping itself: the harness runs a
// full uncached baseline per invocation, so the number of invocations IS the cost, and the only key
// that shares a baseline correctly is the set of suites it covers. A grouping that looked tidier —
// by the directory the mutated file sits in — puts files whose mutants run another package's suite
// into a unit whose baseline covers neither.
package gate

import (
	"slices"
	"strings"
	"testing"
)

// The listing the harness prints for `-units`: file, suites, mutant count, resolved path. The three
// eco-root rows are the real tree's, and they are why the key is the suite set — each of them is
// mutated against a different package's suite.
const unitsListing = "" +
	"shell.go\t./eco-check/\t2\t/repo/ai/tools/eco-check/shell.go\n" +
	"../eco-report/shell.go\t./eco-report/\t3\t/repo/ai/tools/eco-report/shell.go\n" +
	"tree.go\t./eco-check/\t7\t/repo/ai/tools/eco-check/tree.go\n" +
	"../eco-root/imports.go\t./eco-check/\t4\t/repo/ai/tools/eco-root/imports.go\n" +
	"../eco-root/contained.go\t./eco-stats/\t1\t/repo/ai/tools/eco-root/contained.go\n" +
	"../eco-root/eco-root.go\t./eco-root/\t2\t/repo/ai/tools/eco-root/eco-root.go\n" +
	"../shell/text.go\t./shell/,./eco-stats/\t5\t/repo/ai/tools/shell/text.go\n" +
	"../scratch_isolation_test.go\t./\t1\t/repo/ai/tools/scratch_isolation_test.go\n"

func grouped(t *testing.T) []mutantGroup {
	t.Helper()
	groups, err := groupMutants(unitsListing, "/repo")
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
	for _, want := range []string{"shell.go", "tree.go", "../eco-root/imports.go"} {
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
		"mutants:go:eco-check": "../eco-root/imports.go",
		"mutants:go:eco-stats": "../eco-root/contained.go",
		"mutants:go:eco-root":  "../eco-root/eco-root.go",
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
// and is called something rather than nothing: recordStem turns an id into a record's filename, and an
// empty id is the one stem that could collide with another empty one.
func TestAMutantWithNoSuiteIsTheModuleRootsAndIsNamed(t *testing.T) {
	group := groupNamed(t, "mutants:go:root")
	if len(group.inputs) < 2 || group.inputs[1] != "ai/tools" {
		t.Fatalf("keyed on %v, want the module root", group.inputs)
	}
	if recordStem(group.id) == recordStem("mutants:go:") {
		t.Fatal("the unnamed set flattens to the same record as an empty one")
	}
}

// A listing row with no resolved path refuses. The gate keys a verdict on the file's hash, and a row
// that will not say which file it means leaves nothing to hash — which would key the unit on its
// suites alone and report it fresh over a file that moved.
func TestARowWithNoResolvedPathRefuses(t *testing.T) {
	_, err := groupMutants("x.go\t./eco-check/\t1\t\n", "/repo")
	if err == nil || !strings.Contains(err.Error(), "no resolved path") {
		t.Fatalf("got %v, want a refusal naming the missing path", err)
	}
}

// A file the gate cannot safely put in a command refuses before any unit exists, rather than being
// joined into a `-file` argument with everything else.
func TestAFileTheGateCannotQuoteRefuses(t *testing.T) {
	_, err := groupMutants("a;rm -rf .\t./eco-check/\t1\t/repo/ai/tools/eco-check/a.go\n", "/repo")
	if err == nil || !strings.Contains(err.Error(), "cannot safely put in a command") {
		t.Fatalf("got %v, want a refusal", err)
	}
}

// Every listed file reaches exactly one unit. This is the property the saving must not cost: the
// harness selects mutants by matching these tokens exactly, so a file dropped from every group is its
// mutants silently not run, and a file in two groups is them run twice. Measured against the real
// tree at the time of the grouping: 62 files, 525 mutants, 18 units — and one grouped invocation over
// eco-report's 16 files selected 256 anchors, the exact sum of their per-file counts.
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
