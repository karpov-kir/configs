// Which of this module's packages a build or a suite over one directory compiles. The `guide` unit is
// keyed on that set: hand-listed, it named eco-guide, eco-root and shell but not `cmd/eco-guide` — the
// main package the binary is built from — so editing main.go left the verdict fresh over a tool
// nothing rebuilt.
package gate

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"kk-flavor/tools/shell"
)

// Import path, directory, transitive imports, then the two kinds of test file's direct imports —
// `Deps` being transitive is why the closure below walks only the test ones.
const goListFields = "{{.ImportPath}}\t{{.Dir}}\t{{join .Deps \" \"}}\t{{join .TestImports \" \"}}\t{{join .XTestImports \" \"}}"

// One `go list` over the whole module rather than one per package: sixteen separate calls measured
// 2.8s against 0.25s for this one.
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
		return "", fmt.Errorf("go list could not read %s's import graph, so the gate cannot say which packages a unit compiles — nothing ran%s", goTree, said)
	}
	return string(out), nil
}

// The graph, keyed and valued by repository-relative directory: the package, everything it imports
// transitively, and each package its test files import together with everything THOSE reach.
//
// Filtered to this module by the listing itself, so no import path prefix is written down here.
func moduleImports(listing, root string) (map[string][]string, error) {
	home := map[string]string{}
	deps := map[string][]string{}
	fromTests := map[string][]string{}
	for _, line := range strings.Split(listing, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		// A record this cannot read is refused, never skipped. Skipping one drops the packages it named
		// out of the unit table with nothing said: the run then measures less than it reports, and the
		// only tell is a count nobody has a reason to compare. The empty listing below is already
		// refused; a truncated one is the same failure arriving one line at a time.
		if len(fields) < 5 || fields[0] == "" {
			return nil, fmt.Errorf("go list produced a line the gate cannot read (%d field(s), wanted 5): %q — every package it named would go unkeyed, so nothing ran", len(fields), shell.Oneline(line))
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
		return nil, fmt.Errorf("go list named no package under %s, so the gate cannot say what any unit compiles — nothing ran", goTree)
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
