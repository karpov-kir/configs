// Asking git what the checkout holds. Apart from the scans that consume it because both of them do:
// discovery lists the suites through listFiles, and the copied-file scan resolves a path against the
// listing readRepoListing builds from the same call. A scan reads a script's text; this reads the
// repository, and the two answer to different things when either is wrong.
package gate

import (
	"strings"
)

// The repository's own files. `all` keeps the listing order the suffix scan walks, and `topLevel`
// separates a path this repository could hold from one that exists only inside a fixture.
type repoListing struct {
	all      []string
	byPath   map[string]bool
	topLevel map[string]bool
}

func (g *gate) readRepoListing() (repoListing, int) {
	all, err := g.listFiles(".")
	if err != nil || len(all) == 0 {
		return repoListing{}, g.fail("discovery could not list the repository's files, so it cannot " +
			"say which of them a suite copies into its fixture — nothing ran")
	}
	repo := repoListing{all: all, byPath: map[string]bool{}, topLevel: map[string]bool{}}
	for _, file := range all {
		repo.byPath[file] = true
		repo.topLevel[firstSegment(file)] = true
	}
	return repo, 0
}

func (r repoListing) holdsDirectory(candidate string) bool {
	prefix := candidate + "/"
	for _, file := range r.all {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

// `-z` and `core.quotePath=false`, the rule ai/run-tests.sh lives by. Drop either and a name reaches
// the split below newline-separated or C-quoted, leaving a token safeToken refuses — the run exits 2
// blaming a name nothing is wrong with.
func (g *gate) listFiles(pathspec string) ([]string, error) {
	out, err := g.capture("git", "-c", "core.quotePath=false", "ls-files", "-z",
		"--cached", "--others", "--exclude-standard", "--", pathspec)
	if err != nil {
		return nil, err
	}
	var listed []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			listed = append(listed, name)
		}
	}
	return listed, nil
}
