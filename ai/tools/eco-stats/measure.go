package ecostats

import (
	"io"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// One run's figures. Every one of them is a count of something read, never an estimate, and the
// counters that decide whether a row is withheld live here beside them rather than in a package
// variable, so two runs in one process cannot see each other's.
type stats struct {
	// The root exactly as it was named, because the refusal messages echo a path built from it.
	root      string
	rootCanon string
	home      string
	mount     shell.ImportMount

	prose       int
	proseFiles  int
	scripts     int
	ledgerWords int
	skills      int

	alwaysLoadedWords   int
	descriptionWords    int
	routedSkills        int
	importResolvedWords int
	// The imports named in the note because nothing resolved them — what makes the figure a lower
	// bound rather than a reading.
	uncounted []string

	outsideWords  int
	outsideSkills int

	// The refused file's name is attacker-chosen, so it is bounded in length and in count. The count
	// stays exact, and it alone decides whether a row is withheld.
	budgetRefusals int
}

func (s *stats) measure(errOut io.Writer) {
	prose := findFiles(s.root, "*.md", noDepthLimit)
	s.prose = wordsAcross(prose)
	s.proseFiles = wcLines(prose)

	// The ledger is a record, not an instruction, and is reported on its own line rather than inside
	// prose: counted together, the number that decides whether a reduction is owed rises every time a
	// reduction records that it ran.
	// Guarded like a budget file rather than with a bare `[ -f ]`: prose is measured with
	// `find -type f`, which does not walk a symlink, while `-f` follows one — so a symlink here would
	// subtract words the total never held.
	ledger := shell.Join(s.root, "skills/kk-reduce/stats.md")
	if shell.ContainedInRoot(s.rootCanon, ledger) {
		s.ledgerWords = wordsInFile(ledger)
		s.prose -= s.ledgerWords
		s.proseFiles--
	}
	s.scripts = wordsAcross(findFiles(s.root, "*.sh", noDepthLimit))
	s.skills = wcLines(findFiles(shell.Join(s.root, "skills"), "SKILL.md", 2))

	budget := s.budgetFiles(errOut)
	s.mount = shell.NewImportMount(s.home, shell.Join(s.root, "kk-flavor"), shell.Join(s.root, "CLAUDE.md"), readLines)
	s.resolveImports(budget, errOut)
	s.census()
	s.mountedOutside()
}

const noDepthLimit = -1

// `find <start> [-maxdepth n] -name <glob> -type f`, in find's own order — the directory's own order
// rather than a sorted one, because the concatenation in wordsAcross is sensitive to it.
//
// No -L: the starting point is lstat'ed like every entry under it, so a symlinked start yields the
// link itself and nothing beneath it, and a symlink to a regular file is not `-type f`. That is what
// keeps a committed link from walking the measurement out of the tree it is measuring.
func findFiles(start, glob string, maxDepth int) []string {
	var found []string
	var walk func(path string, depth int)
	walk = func(path string, depth int) {
		info, err := os.Lstat(path)
		if err != nil {
			return
		}
		if info.Mode().IsRegular() && shell.Fnmatch(glob, shell.BaseName(path)) {
			found = append(found, path)
		}
		if !info.IsDir() || (maxDepth != noDepthLimit && depth >= maxDepth) {
			return
		}
		for _, name := range readdirNames(path) {
			walk(shell.Join(path, name), depth+1)
		}
	}
	walk(start, 0)
	return found
}

// os.ReadDir sorts its result and find does not, so this does not either.
func readdirNames(dir string) []string {
	file, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer file.Close()
	names, err := file.Readdirnames(-1)
	if err != nil {
		return nil
	}
	return names
}

// `cat <files> | wc -w`: one word run can span a file boundary, so the state carries across files
// rather than restarting at each. A file with no final newline glues its last word onto the next
// file's first, which is one word fewer than summing the files separately — the figure has always
// been the concatenation's, and summing instead would move every prose and scripts number by however
// many files end mid-word. A file that cannot be opened contributes nothing and does not stop the
// run, exactly as `cat`'s own failure did not.
func wordsAcross(paths []string) int {
	words := 0
	inWord := false
	buffer := make([]byte, 64<<10)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		for {
			read, err := file.Read(buffer)
			for i := 0; i < read; i++ {
				if shell.IsSpaceByte(buffer[i]) {
					inWord = false
					continue
				}
				if !inWord {
					words++
					inWord = true
				}
			}
			if err != nil {
				break
			}
		}
		file.Close()
	}
	return words
}

// `wc -w <file>` — the same count over one file, where nothing can glue onto it.
func wordsInFile(path string) int {
	return wordsAcross([]string{path})
}

// `find … | wc -l`, which counts newline bytes rather than paths: a committed newline inside a
// filename splits one path across two lines and is counted twice. Overstating a file total is the
// safe half — the figure it feeds is a census, not a gate.
func wcLines(paths []string) int {
	lines := 0
	for _, path := range paths {
		lines += 1 + strings.Count(path, "\n")
	}
	return lines
}

// One file as awk saw it. Unbounded, as awk's streaming read was: ecocheck bounds the same read
// because a file over the bound there is reported and skipped, and there is no such report to make
// here — a bound would leave the always-loaded figure short with nothing saying so, which is the one
// thing this tool must never do.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return shell.SplitLines(string(data)), nil
}
