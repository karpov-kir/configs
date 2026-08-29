package ecostats

import (
	"fmt"
	"io"
	"os"
	"strings"

	ecoroot "kk-flavor/tools/eco-root"
	"kk-flavor/tools/shell"
)

// One run's figures. Every one of them is a count of something read, never an estimate, and the
// counters that decide whether a row is withheld live here beside them rather than in a package
// variable, so two runs in one process cannot see each other's.
type stats struct {
	// The checkout being measured, which holds the root exactly as it was named: the refusal
	// messages echo a path built from it.
	root ecoroot.Root

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
	// Which paths were already refused for being over the read bound, so a file two readers reach is
	// one refusal rather than two.
	refusedOversize map[string]bool
}

func (s *stats) measure(errOut io.Writer) {
	prose := findFiles(s.root.Named(), "*.md", noDepthLimit)
	s.prose = wordsAcross(prose)
	s.proseFiles = wcLines(prose)

	// The ledger is a record, not an instruction, and is reported on its own line rather than inside
	// prose: counted together, the number that decides whether a reduction is owed rises every time a
	// reduction records that it ran.
	//
	// Guarded like a budget file rather than with a bare `[ -f ]`: prose is measured with
	// `find -type f`, which does not walk a symlink, while `-f` follows one — so a symlink here would
	// subtract words the total never held.
	ledger := shell.Join(s.root.Skills(), "kk-reduce/stats.md")
	if s.root.Contains(ledger) {
		s.ledgerWords = wordsInFile(ledger)
		s.prose -= s.ledgerWords
		s.proseFiles--
	}
	s.scripts = wordsAcross(findFiles(s.root.Named(), "*.sh", noDepthLimit))
	s.skills = wcLines(findFiles(s.root.Skills(), "SKILL.md", 2))

	s.resolveImports(s.budgetFiles(errOut), errOut)
	s.census(errOut)
	s.mountedOutside(errOut)
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
// file's first, which is one word fewer than summing the files separately. The figure has always been
// the concatenation's, and summing instead would move every prose and scripts number by however many
// files end mid-word. A file that cannot be opened contributes nothing and does not stop the run,
// exactly as `cat`'s own failure did not.
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

// The bound on a whole-file read. awk streamed, and this does not: a committed 64 MiB of newlines
// packs to about 65 KB and took 2.46 GB of resident memory here, because every line becomes a slice
// header. Half a gigabyte of it is an OOM kill rather than a measurement.
//
// A file over the bound is refused, never truncated. refuseBudgetFile names it, keeps the count
// exact, and the run exits 2 with no row appended — so the always-loaded figure is withheld rather
// than left quietly short, which is the one thing this tool must never do.
//
// Same value as ecocheck's, because the two tools measure the same tier and must agree on which files
// are in it.
const maxFileBytes = 8 << 20

// One file as awk saw it, for a path this tool chose rather than the tree.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return shell.SplitLines(string(data)), nil
}

// The same read for a path the measured tree chose, which is every file below. Over the bound it is
// refused rather than read, so the run withholds its row and exits 2 instead of reporting a tier it
// could not measure.
func (s *stats) readTreeLines(path string, errOut io.Writer) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.Size() > maxFileBytes {
		s.refuseOversize(path, info.Size(), errOut)
		return nil
	}
	lines, _ := readLines(path)
	return lines
}

// The words in a budget file, under the same bound the line read applies. wordsAcross streams and
// never slurps, so it would happily count a file no reader here may read. refuseBudgetFile, reached
// over that same file by the import scan, would then print "not read, not counted" and mark an exact
// figure SHORT in one output.
//
// Bounded here instead, so the refusal's own words hold for the figure as well as for the read.
// ecocheck refuses the same file at the same bound and counts it as no words, so both tools still
// place it in the same tier.
func (s *stats) countTreeWords(path string, errOut io.Writer) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if info.Size() > maxFileBytes {
		s.refuseOversize(path, info.Size(), errOut)
		return 0
	}
	return wordsInFile(path)
}

// Once per file, however many readers reach it: a budget file is counted here, read again for its
// `@import` lines by the scan, and the count in the exit line is meant to be how many files were
// refused, not how many times one was looked at.
func (s *stats) refuseOversize(path string, size int64, errOut io.Writer) {
	if s.refusedOversize[path] {
		return
	}
	if s.refusedOversize == nil {
		s.refusedOversize = map[string]bool{}
	}
	s.refusedOversize[path] = true
	// The bound leads, because refuseBudgetFile cuts this to 80 bytes and the path is the part the
	// tree chose the length of.
	s.refuseBudgetFile(errOut, fmt.Sprintf("%d bytes, over the %d-byte bound — NOT read: %s",
		size, maxFileBytes, path))
}
