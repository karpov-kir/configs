// The gate's Go mutation units: how the harness's own listing becomes units, and why one unit covers
// a set of suites rather than a single file.
package gate

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (g *gate) discoverGoMutants() int {
	g.goMutateBinary = "ai/tools/go-mutate/go-mutate"
	build := exec.Command("go", "build", "-o", "go-mutate/go-mutate", "./go-mutate")
	build.Dir = filepath.Join(g.root, "ai", "tools")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintln(g.errOut, "gate.sh: go-mutate does not build, so its units cannot be listed and its verdicts cannot be trusted — nothing ran")
		g.errOut.Write(out)
		return 2
	}
	listing, err := g.capture(filepath.Join(g.root, g.goMutateBinary), "-units")
	if err != nil {
		return g.fail("go-mutate could not list its units — nothing ran")
	}
	if strings.TrimSpace(listing) == "" {
		return g.fail("go-mutate listed no units — read this as the harness broken, never as nothing to check")
	}
	groups, err := groupMutants(listing, g.root)
	if err != nil {
		return g.fail("%s", err)
	}
	for _, group := range groups {
		g.add(group.id, "mutation", group.inputs,
			g.goMutateBinary+" -file "+shellQuote(strings.Join(group.files, ",")))
	}
	return 0
}

// One group of mutants and the unit it becomes: the files handed to the harness in one invocation, and
// the paths whose hashes the verdict is keyed on.
type mutantGroup struct {
	id     string
	files  []string
	inputs []string
}

// groupMutants turns the harness's `-units` listing into one unit per SUITE SET rather than one per
// file.
//
// Every invocation of the harness runs a full uncached baseline over the suites its mutants name
// before it runs a single mutant. A unit per file therefore paid that baseline once per file: sixteen
// times over eco-report's 53-second suite and seventeen over eco-check's 18-second one, about 1080s of
// a 2230s cold gate spent proving the same two suites green. Mutants that name the same suites share a
// baseline, so grouping on that pays each one once.
//
// The suite set and not the package directory, because the two come apart: `eco-root/imports.go` is
// mutated against eco-check's suite and `eco-root/contained.go` against eco-stats'. Grouped by
// directory, those two would share a unit whose baseline covers neither of them.
//
// It costs no freshness. A unit was already keyed on every suite directory its mutants name, and a
// file under `ai/tools/eco-report` is inside `ai/tools/eco-report` — so a change anywhere in that
// package re-ran all sixteen of its units before this and re-runs the one unit after it.
//
// The baseline cannot be left to Go's test cache instead, which would keep a unit per file: the cache
// serves a green over a suite that a change from outside the module has already broken. Measured, and
// recorded in IDEAS.md.
func groupMutants(listing, root string) ([]mutantGroup, error) {
	var groups []mutantGroup
	at := map[string]int{}
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 || fields[0] == "" {
			continue
		}
		file, suites, resolved := fields[0], fields[1], fields[3]
		// Checked per file, before the join: the gate builds a command from these, and the comma that
		// separates them here must be the only one in the argument.
		if err := safeToken("mutant file", file); err != nil {
			return nil, err
		}
		if resolved == "" {
			return nil, fmt.Errorf("the mutation harness listed %s with no resolved path, so the gate cannot say which file the unit is keyed on — nothing ran", file)
		}
		index, held := at[suites]
		if !held {
			index = len(groups)
			at[suites] = index
			groups = append(groups, mutantGroup{
				id:     "mutants:go:" + suiteSetName(suites),
				inputs: append([]string{goTree + "/go-mutate"}, suiteDirs(suites)...),
			})
		}
		groups[index].files = append(groups[index].files, file)
		groups[index].inputs = append(groups[index].inputs,
			strings.TrimPrefix(strings.TrimPrefix(resolved, root), "/"))
	}
	return groups, nil
}

// Where the suites of one set live, relative to the repository. A mutant naming no suite keys on the
// whole module, which is what a unit with no suite of its own keyed on before it was grouped.
func suiteDirs(suites string) []string {
	var dirs []string
	for _, suite := range strings.Split(suites, ",") {
		if suite == "" {
			continue
		}
		dirs = append(dirs, strings.TrimSuffix(goTree+"/"+strings.TrimPrefix(suite, "./"), "/"))
	}
	return dirs
}

// What a suite set is called. Read off suiteDirs so the name and the inputs cannot disagree about
// which suites a group covers. A set naming no suite comes back "root" rather than empty: recordStem
// flattens an id to a record filename, and an empty one is the single stem two such sets would
// collide on.
func suiteSetName(suites string) string {
	var names []string
	for _, dir := range suiteDirs(suites) {
		name := strings.TrimPrefix(strings.TrimPrefix(dir, goTree), "/")
		if name == "" {
			name = "root"
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "root"
	}
	return strings.Join(names, "+")
}
