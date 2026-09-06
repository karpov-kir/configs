package ecostats

import (
	"errors"
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
	// Whether the mounted-outside scan ran at all, which is not the same question as whether it
	// counted anything — the report needs both. Don't ask IsInstalled again there instead: that is a
	// second copy of budget.go's gate, answering before the first is consulted, so deleting the first
	// changes no output at all.
	outsideMeasured bool

	// The refused file's name is attacker-chosen, so it is bounded in length and in count. The count
	// stays exact, and it alone decides whether a row is withheld.
	budgetRefusals int
	// Which paths were already refused for being over the read bound, so a file two readers reach is
	// one refusal rather than two.
	refusedOversize map[string]bool

	// Paths under the root this run could not read at all — a directory it could not list, a file it
	// could not open. Distinct from a budget refusal, which says a file is not in the always-loaded
	// tier: this says the counts below cover whatever part of the tree was reachable. Every swallowed
	// error below feeds it, because a figure short by an unknown amount is what this must never report.
	unreadable int
	// Which paths were already counted unread, so a directory both the prose and the scripts walk reach
	// is one path rather than two.
	refusedUnreadable map[string]bool
	// How many of those paths were not under the root. A skill mounted at `~/.claude/skills` lives in
	// the user's home, so a shortfall message calling it a path "under <root>" sends a reader hunting
	// through the wrong tree. Not a second gate — every unread path already withholds the row — it only
	// decides how that one message describes where they were.
	unreadableOutside int
}

// Where the unread paths were, for the two messages that report the shortfall.
func (s *stats) unreadableWhere() string {
	switch {
	case s.unreadableOutside == 0:
		return fmt.Sprintf("under %s", s.root.Named())
	case s.unreadableOutside == s.unreadable:
		return fmt.Sprintf("mounted from outside %s", s.root.Named())
	default:
		return fmt.Sprintf("%d under %s and %d mounted from outside it",
			s.unreadable-s.unreadableOutside, s.root.Named(), s.unreadableOutside)
	}
}

func (s *stats) measure(errOut io.Writer) {
	prose := s.findFiles(s.root.Named(), "*.md", noDepthLimit, errOut)
	s.prose = s.wordsAcross(prose, errOut)
	s.proseFiles = wcLines(prose)

	// The ledger is a record, not an instruction, and gets its own line rather than sitting inside
	// prose: counted together, the number deciding whether a reduction is owed rises every time a
	// reduction records that it ran. Guarded like a budget file, not a bare `[ -f ]`: prose uses `find
	// -type f`, which does not walk a symlink, while `-f` follows one and subtracts words never held.
	ledger := shell.Join(s.root.Skills(), "kk-reduce/stats.md")
	if s.root.Contains(ledger) {
		s.ledgerWords = s.wordsInFile(ledger, errOut)
		s.prose -= s.ledgerWords
		s.proseFiles--
	}
	s.scripts = s.wordsAcross(s.findFiles(s.root.Named(), "*.sh", noDepthLimit, errOut), errOut)
	s.skills = wcLines(s.findFiles(s.root.Skills(), "SKILL.md", 2, errOut))

	s.resolveImports(s.budgetFiles(errOut), errOut)
	s.census(errOut)
	s.mountedOutside(errOut)
}

const noDepthLimit = -1

// `find <start> [-maxdepth n] -name <glob> -type f`, in find's own order rather than a sorted one,
// because the concatenation in wordsAcross is sensitive to it. No -L: the starting point is lstat'ed
// like every entry under it, so a symlinked start yields the link and nothing beneath it, and a symlink
// to a regular file is not `-type f` — which keeps a committed link from walking out of the tree.
func (s *stats) findFiles(start, glob string, maxDepth int, errOut io.Writer) []string {
	var found []string
	var walk func(path string, depth int)
	walk = func(path string, depth int) {
		info, err := os.Lstat(path)
		if err != nil {
			// A directory nobody can stat takes its whole subtree out of every figure below, so
			// returning in silence here would leave each of them short with nothing said.
			s.refuseUnreadable(errOut, path, err)
			return
		}
		if info.Mode().IsRegular() && shell.Fnmatch(glob, shell.BaseName(path)) {
			found = append(found, path)
		}
		if !info.IsDir() || (maxDepth != noDepthLimit && depth >= maxDepth) {
			return
		}
		for _, name := range s.readdirNames(path, errOut) {
			walk(shell.Join(path, name), depth+1)
		}
	}
	walk(start, 0)
	return found
}

// os.ReadDir sorts its result and find does not, so this does not either. A directory that cannot be
// listed is the whole of what a partial scan is made of: it yields no names, the walk finds nothing
// under it, and nothing distinguishes that from a directory holding no markdown. What one shut
// directory costs a run is on the guard that acts on it, in eco-stats.go.
func (s *stats) readdirNames(dir string, errOut io.Writer) []string {
	file, err := os.Open(dir)
	if err != nil {
		s.refuseUnreadable(errOut, dir, err)
		return nil
	}
	defer file.Close()
	names, err := file.Readdirnames(-1)
	if err != nil {
		s.refuseUnreadable(errOut, dir, err)
		return nil
	}
	return names
}

// A path this run was meant to measure and could not read. Counted as well as named, because the
// count is the only reach the exit code has: `stats.sh` and CI branch on the status, and a shortfall
// that only ever reached stderr arrived at both as a measurement of the whole tree. Capped like a
// budget refusal and for the same reason — the names are the tree's own. The count stays exact.
func (s *stats) refuseUnreadable(errOut io.Writer, path string, err error) {
	// Once per path, however many walks reach it: prose and scripts are two passes over one tree, so a
	// directory that will not list is met twice.
	if s.refusedUnreadable[path] {
		return
	}
	if s.refusedUnreadable == nil {
		s.refusedUnreadable = map[string]bool{}
	}
	s.refusedUnreadable[path] = true

	s.unreadable++
	switch {
	case s.unreadable <= budgetRefusalCap:
		// Capped after the error text is joined on, not before: the message from the OS quotes the path
		// back, so bounding the path alone lets an attacker-chosen name through at full length anyway.
		// Marked, because what is cut here is the reason: on a short root the bound lands inside it and
		// `permission denied` goes out as `permission de`, reading as a whole reason rather than a cut one.
		fmt.Fprintf(errOut, "stats.sh: could not read, NOT counted: %s\n",
			shell.CutBytesMarked(shell.Oneline(err.Error()), 160))
	case s.unreadable == budgetRefusalCap+1:
		fmt.Fprintln(errOut, "stats.sh: further unreadable paths suppressed; the count in the exit message is the total")
	}
}

// `cat <files> | wc -w`: one word run can span a file boundary, so the state carries across files
// rather than restarting at each. A file with no final newline glues its last word onto the next
// file's first — one word fewer than summing the files separately, and summing instead would move
// every prose and scripts figure. A file that cannot be opened is counted and named, never dropped.
func (s *stats) wordsAcross(paths []string, errOut io.Writer) int {
	run := wordRun{}
	buffer := make([]byte, 64<<10)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			s.refuseUnreadable(errOut, path, err)
			continue
		}
		for {
			read, err := file.Read(buffer)
			run.count(buffer[:read])
			if err != nil {
				// EOF is how a whole file ends; anything else stopped the read partway and left this
				// file counted up to wherever it failed, which looks exactly like a shorter file.
				if !errors.Is(err, io.EOF) {
					s.refuseUnreadable(errOut, path, err)
				}
				break
			}
		}
		file.Close()
	}
	return run.words
}

type wordRun struct {
	words    int
	isInWord bool
}

func (w *wordRun) count(chunk []byte) {
	for i := 0; i < len(chunk); i++ {
		if shell.IsSpaceByte(chunk[i]) {
			w.isInWord = false
			continue
		}
		if !w.isInWord {
			w.words++
			w.isInWord = true
		}
	}
}

// `wc -w <file>` — the same count over one file, where nothing can glue onto it.
func (s *stats) wordsInFile(path string, errOut io.Writer) int {
	return s.wordsAcross([]string{path}, errOut)
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

// The bound and its reason are shell.MaxFileBytes. Here a file over it is refused, never truncated:
// refuseBudgetFile names it, keeps the count exact, and the run exits 2 with no row appended, so the
// always-loaded figure is withheld rather than left quietly short.
const maxFileBytes = shell.MaxFileBytes

// One file's lines, for a path this tool chose rather than the tree.
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
		s.refuseUnreadable(errOut, path, err)
		return nil
	}
	if info.Size() > maxFileBytes {
		s.refuseOversize(path, info.Size(), errOut)
		return nil
	}
	// The stat passing says nothing about the read: a SKILL.md at mode 000 inside a readable directory
	// stats happily and opens for nobody. Dropped, its description words leave the always-loaded figure
	// short while the skill still counts as routed.
	lines, err := readLines(path)
	if err != nil {
		s.refuseUnreadable(errOut, path, err)
		return nil
	}
	return lines
}

// The same read for a path that is, by construction, not under the root — mountedOutside has already
// asked HoldsSkillFile and been told no. Counted apart so the shortfall message can say where the path
// was; the read itself is identical. Measured by what the read added, since refuseUnreadable's dedupe
// means a path already counted adds nothing here either.
func (s *stats) readOutsideLines(path string, errOut io.Writer) []string {
	before := s.unreadable
	lines := s.readTreeLines(path, errOut)
	s.unreadableOutside += s.unreadable - before
	return lines
}

// The words in a budget file, under the same bound the line read applies. wordsAcross streams and
// never slurps, so it would happily count a file no reader here may read — and refuseBudgetFile,
// reached over that same file by the import scan, would then print "not read, not counted" and mark
// an exact figure SHORT in one output. ecocheck refuses it at the same bound, so the tiers agree.
func (s *stats) countTreeWords(path string, errOut io.Writer) int {
	info, err := os.Stat(path)
	if err != nil {
		s.refuseUnreadable(errOut, path, err)
		return 0
	}
	if info.Size() > maxFileBytes {
		s.refuseOversize(path, info.Size(), errOut)
		return 0
	}
	return s.wordsInFile(path, errOut)
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
