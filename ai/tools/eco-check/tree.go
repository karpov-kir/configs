package ecocheck

import (
	"io/fs"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

type fsEntry struct {
	path string
	mode fs.FileMode
}

func (e fsEntry) isRegular() bool { return e.mode.IsRegular() }

// tree is one `find <start>` answered once, plus the index that answers `-path "*/<ref>" -type f`.
// That question is asked once per cited token, and the shell version paid a process and a whole
// tree walk for each of them.
type tree struct {
	entries []fsEntry
	// Every tail of a regular file's path that begins after a `/`, which is exactly what a
	// `*/<ref>` pattern can match. Insertion order is walk order, so a caller reading the first
	// match reads the one find would have printed first.
	suffixes map[string][]string
}

// walkTree mirrors `find <start>` with no -L flag: the starting point is lstat'ed like every entry
// under it, so a symlinked start yields the link itself and nothing beneath it. That is the whole
// of the direction scan's did-not-run guard — a branch that commits `kk-flavor` as a symlink to
// somewhere else gets an empty walk and a finding saying so, never a silent pass.
func (c *checker) walkTree(start string) *tree {
	if cached, ok := c.trees[start]; ok {
		return cached
	}
	walked := &tree{suffixes: map[string][]string{}}
	if info, err := os.Lstat(start); err == nil {
		walked.add(fsEntry{path: start, mode: info.Mode()})
		if info.IsDir() {
			walked.descend(start)
		}
	}
	c.trees[start] = walked
	return walked
}

func (t *tree) descend(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// Concatenated, not joined, so the path a finding echoes is the one the caller named.
		path := shell.Join(dir, entry.Name())
		t.add(fsEntry{path: path, mode: info.Mode()})
		// entry.IsDir() reads the directory entry's own type, so a symlink to a directory is not
		// descended into and no committed link can walk this scan out of the tree under review.
		if entry.IsDir() {
			t.descend(path)
		}
	}
}

func (t *tree) add(entry fsEntry) {
	t.entries = append(t.entries, entry)
	if !entry.isRegular() {
		return
	}
	for i := 0; i < len(entry.path); i++ {
		if entry.path[i] == '/' {
			tail := entry.path[i+1:]
			t.suffixes[tail] = append(t.suffixes[tail], entry.path)
		}
	}
}

// The regular files whose path a `*/<ref>` pattern matches, in walk order.
func (t *tree) matchPath(ref string) []string {
	if !strings.ContainsAny(ref, `*?[\`) {
		return t.suffixes[ref]
	}
	var matches []string
	for _, entry := range t.entries {
		if entry.isRegular() && shell.Fnmatch("*/"+ref, entry.path) {
			matches = append(matches, entry.path)
		}
	}
	return matches
}

// The regular files under start whose basename matches one of the given `find -name` globs.
func (c *checker) filesNamed(start string, globs ...string) []string {
	var matched []string
	for _, entry := range c.walkTree(start).entries {
		if !entry.isRegular() {
			continue
		}
		base := shell.BaseName(entry.path)
		for _, glob := range globs {
			if shell.Fnmatch(glob, base) {
				matched = append(matched, entry.path)
				break
			}
		}
	}
	return matched
}

// A path as prose writes it, resolved to a file under the root; empty when it resolves nowhere or
// to more than one file.
func (c *checker) resolveRef(dir, ref string) string {
	if rest, ok := strings.CutPrefix(ref, "~/.kk-flavor/"); ok {
		return existingOrEmpty(shell.Join(c.flavor, rest))
	}
	if rest, ok := strings.CutPrefix(ref, "~/.claude/skills/"); ok {
		return existingOrEmpty(shell.Join(c.skills, rest))
	}
	if dir != "" && shell.PathExists(shell.Join(dir, ref)) {
		return shell.Join(dir, ref)
	}
	if shell.PathExists(shell.Join(c.root, ref)) {
		return shell.Join(c.root, ref)
	}
	// A bare name is accepted only when one file in the tree could be meant. Counted in lines
	// rather than in paths, as the shell version counted them: a committed directory name holding a
	// newline splits one match across two lines, and reading that as ambiguous is the safe half.
	joined := strings.Join(c.walkTree(c.root).matchPath(ref), "\n")
	if countNonEmptyLines(joined) == 1 {
		return joined
	}
	return ""
}

func existingOrEmpty(path string) string {
	if shell.PathExists(path) {
		return path
	}
	return ""
}

// True when a cited path names at least one real file, an ambiguous bare name included.
func (c *checker) refExists(dir, ref string) bool {
	if c.resolveRef(dir, ref) != "" {
		return true
	}
	return len(c.walkTree(c.root).matchPath(ref)) > 0
}

// The entries of skills/ that stat as directories, dotfiles excluded and byte-sorted the way the
// shell's `"$skills"/*/` glob produced them.
func (c *checker) skillDirNames() []string {
	entries, err := os.ReadDir(c.skills)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if shell.IsDir(shell.Join(c.skills, entry.Name())) {
			names = append(names, entry.Name())
		}
	}
	return names
}
