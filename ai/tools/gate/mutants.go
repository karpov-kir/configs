// The gate's Go mutation units: how the harness's own listing becomes units, why one unit covers a set
// of suites rather than a single file, and what each of them is keyed on.
package gate

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"kk-flavor/tools/shell"
)

func (g *gate) discoverGoMutants(imports map[string][]string) int {
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
	groups, err := groupMutants(listing, g.root, imports)
	if err != nil {
		return g.fail("%s", err)
	}
	for _, group := range groups {
		g.add(group.id, "mutation", group.inputs,
			g.goMutateBinary+" -file "+shellQuote(strings.Join(group.files, ",")))
	}
	return 0
}

// Import path, directory, transitive imports, then the two kinds of test file's direct imports —
// `Deps` being transitive is why the closure below walks only the test ones.
const goListFields = "{{.ImportPath}}\t{{.Dir}}\t{{join .Deps \" \"}}\t{{join .TestImports \" \"}}\t{{join .XTestImports \" \"}}"

// One `go list` over the whole module rather than one per suite set: sixteen separate calls measured
// 2.8s against 0.25s for this one, and `--units` takes about a second in total.
//
// Not `-e`, which answers for a package whose imports will not resolve with a `Deps` list short of
// exactly the ones it could not read. Go's own words reach the refusal: what breaks this is a source
// file nothing can parse, and the reader needs that error rather than a summary of it.
func (g *gate) listModulePackages() (string, error) {
	cmd := exec.Command("go", "list", "-f", goListFields, "./...")
	cmd.Dir = filepath.Join(g.root, goTree)
	out, err := cmd.Output()
	if err != nil {
		said := ""
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			said = ": " + shell.Oneline(string(exit.Stderr))
		}
		return "", fmt.Errorf("go list could not read %s's import graph, so the gate cannot say which packages a suite compiles — nothing ran%s", goTree, said)
	}
	return string(out), nil
}

// Which of this module's packages a `go test` over one directory compiles, keyed and valued by
// repository-relative directory: the package, everything it imports transitively, and each package its
// test files import together with everything THOSE reach.
//
// A mutation verdict is a claim about mutants its suite killed, so every package compiled into that
// suite's binary is content the verdict was decided over. Keyed on the suite's directory alone,
// editing an imported package left it fresh: eco-report's suite compiles repo-key, and repokey.go
// moved without touching one declared input. `extStubs` closes the same hole outside the module.
//
// Filtered to this module by the listing itself, so no import path prefix is written down here.
func moduleImports(listing, root string) (map[string][]string, error) {
	home := map[string]string{}
	deps := map[string][]string{}
	fromTests := map[string][]string{}
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 || fields[0] == "" {
			continue
		}
		dir, err := filepath.Rel(root, fields[1])
		if err != nil || strings.HasPrefix(dir, "..") {
			return nil, fmt.Errorf("the package %s lives at %s, which is outside the repository, so the gate cannot key a unit on it — nothing ran", fields[0], fields[1])
		}
		home[fields[0]] = dir
		deps[fields[0]] = strings.Fields(fields[2])
		fromTests[fields[0]] = append(strings.Fields(fields[3]), strings.Fields(fields[4])...)
	}
	if len(home) == 0 {
		return nil, fmt.Errorf("go list named no package under %s, so every mutation unit would be keyed on its own directory alone — nothing ran", goTree)
	}
	reached := make(map[string][]string, len(home))
	for pkg, dir := range home {
		compiled := append([]string{pkg}, deps[pkg]...)
		// What a test file imports is compiled in, and so is everything it reaches — but not ITS tests.
		for _, imported := range fromTests[pkg] {
			if _, ours := home[imported]; !ours {
				continue
			}
			compiled = append(compiled, imported)
			compiled = append(compiled, deps[imported]...)
		}
		var dirs []string
		for _, one := range compiled {
			if where, ours := home[one]; ours {
				dirs = append(dirs, where)
			}
		}
		reached[dir] = shell.SortUnique(dirs)
	}
	return reached, nil
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
//
// `imports` says what each suite compiles, and a suite missing from it refuses the run: a unit keyed
// on less than its command reads is the stale green this file exists not to serve.
func groupMutants(listing, root string, imports map[string][]string) ([]mutantGroup, error) {
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
			dirs := suiteDirs(suites)
			inputs := append([]string{goTree + "/go-mutate"}, dirs...)
			for _, dir := range dirs {
				compiles, known := imports[dir]
				if !known {
					return nil, fmt.Errorf("the gate cannot say which packages %s imports, so its unit would be keyed on less than the suite compiles — nothing ran", dir)
				}
				inputs = append(inputs, compiles...)
			}
			index = len(groups)
			at[suites] = index
			groups = append(groups, mutantGroup{id: "mutants:go:" + suiteSetName(suites), inputs: inputs})
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
