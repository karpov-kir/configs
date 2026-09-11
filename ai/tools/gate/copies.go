// Resolving what a suite copies into its fixture. A copied path is the one input read out of a
// script's text rather than named by the unit table, so it is the only one that has to be resolved
// against the checkout at all — and the only one that can name a file the gate must refuse rather
// than key on.
package gate

import (
	"path"
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

// Copying is the other way a suite reaches a repository file, and it has to be keyed on too: edit the
// copied file and the unit still answers out of its cache. Only the operand right after `cp` counts,
// which leaves a destination inside the fixture unkeyed. A file is named by the literal tail after the
// variable, so a copy whose basename is itself a variable names nothing.
var copiedFileLine = regexp.MustCompile(`\bcp[ \t]+(?:-[-A-Za-z]+(?:=[^ \t\n]*)?[ \t]+)*"\$\{?[A-Za-z_][A-Za-z0-9_]*\}?/([^"$*?\n]+)"`)

// The repository files a suite copies into its fixture, plus the first copy naming a file the gate
// cannot resolve. Keyed whether the copy is run or only read: a regexp cannot tell those apart, and
// either way the file moves what the suite measures. ai/mcp-sync-test.sh copies ai/mcp.jsonc in and
// asserts every command it names is executable.
func (g *gate) copiedRepoFiles(repo repoListing, suiteDir string, bodies ...string) ([]string, string) {
	var copied []string
	for _, body := range bodies {
		for _, line := range strings.Split(body, "\n") {
			if commentedLine.MatchString(line) {
				continue
			}
			for _, match := range copiedFileLine.FindAllStringSubmatch(line, -1) {
				tail := match[1]
				files, unresolved := resolveCopiedPath(repo, suiteDir, tail)
				if unresolved == "" {
					copied = append(copied, files...)
					continue
				}
				return nil, tail + ", which " + unresolved
			}
		}
	}
	return shell.SortUnique(copied), ""
}

// Three tries, tightest first: the tail as a repository path, then under the directory the suite lives
// in, and only then the tracked files it is a suffix of. Each try returns every path it hit, not the
// first. Two paths matching means the gate cannot say which one the suite copies. Keying on both
// costs one file too many, and picking one could key the unit on the wrong file altogether.
func resolveCopiedPath(repo repoListing, suiteDir, tail string) ([]string, string) {
	candidates := copyCandidates(suiteDir, tail)
	if resolved := allMatching(candidates, func(c string) bool { return repo.byPath[c] }); len(resolved) > 0 {
		return resolved, ""
	}
	// A directory only where the tail spells one out, never through the suffix scan below: a tail
	// landing on some deep directory by accident would key the unit on everything under it.
	if resolved := allMatching(candidates, repo.holdsDirectory); len(resolved) > 0 {
		return resolved, ""
	}
	var matches []string
	for _, file := range repo.all {
		if strings.HasSuffix(file, "/"+tail) {
			matches = append(matches, file)
		}
	}
	if len(matches) > 0 {
		return matches, ""
	}
	// Nothing resolved, and the tail does not start at a top-level entry this repository has. So it points
	// inside the fixture, which the suite built itself and no edit here can move. Keyed on nothing rather
	// than refused, because refusing would let an ordinary line like
	// `cp "$fixture/config.json" "$other/config.json"` stop the whole gate for everyone.
	if !repo.topLevel[firstSegment(tail)] {
		return nil, ""
	}
	return nil, "names no file in this repository"
}

// Both spellings the suites use: `$checkout/ai/bootstrap-test.sh` is rooted at the repository, while
// `$script_dir/mcp-env.sh` names a sibling of the suite.
func copyCandidates(suiteDir, tail string) []string {
	if suiteDir == "" || suiteDir == "." {
		return []string{tail}
	}
	return []string{tail, path.Join(suiteDir, tail)}
}

func allMatching(candidates []string, holds func(string) bool) []string {
	var resolved []string
	for _, candidate := range candidates {
		if holds(candidate) {
			resolved = append(resolved, candidate)
		}
	}
	return resolved
}

func firstSegment(tail string) string {
	first, _, _ := strings.Cut(tail, "/")
	return first
}
