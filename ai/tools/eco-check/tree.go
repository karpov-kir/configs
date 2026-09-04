package ecocheck

import (
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"kk-flavor/tools/shell"
)

type fsEntry struct {
	// The path as the walk built it, which is the root exactly as the caller spelled it plus the
	// way down. Every finding echoes this one.
	path string
	// The same file under the name rootName gives the root, which every spelling shares, plus the
	// way down.
	canonical string
	mode      fs.FileMode
}

func (e fsEntry) isRegular() bool { return e.mode.IsRegular() }

// tree is one `find <start>` answered once, plus the index that answers "does this cited token name
// a file in here". That question is asked once per cited token, and the shell version paid a process
// and a whole tree walk for each of them.
type tree struct {
	// The start as the caller spelled it, and rootName's name for it. Together they turn a walked
	// path into the canonical name the index is keyed on.
	start         string
	canonicalRoot string

	entries []fsEntry
	// Each regular file's canonical name, plus every tail of it that begins after a `/` — the
	// shapes prose writes a path in, from the root's own name down to a bare basename. Keyed on the
	// canonical name and never on the spelled one, so what the check reports never depends on which
	// spelling was typed. Insertion order is walk order, so a caller reading the first match reads
	// the one find would have printed first.
	suffixes map[string][]string
}

// The one place the reviewed tree becomes a set of files, so the one place --gate narrows it: every
// scan reaches its files through filesNamed below, every citation resolves through matchPath, and the
// skill-directory scan reads these entries straight. Filtering here is all three at once, and a scan
// added later inherits it — gate.go holds why a checkout's local files may not decide a commit's
// verdict.
func (c *checker) walkTree(start string) *tree {
	if cached, ok := c.trees[start]; ok {
		return cached
	}
	walked := newWalk(start)
	if c.gate != nil {
		walked = c.gate.keepCommittable(walked)
	}
	c.trees[start] = walked
	return walked
}

// newWalk mirrors `find <start>` with no -L flag: the starting point is lstat'ed like every entry
// under it, so a symlinked start yields the link itself and nothing beneath it. That is the whole
// of the direction scan's did-not-run guard — a branch that commits `kk-flavor` as a symlink to
// somewhere else gets an empty walk and a finding saying so, never a silent pass.
//
// Unfiltered and uncached, because enableGate walks the root through it to build the very set
// walkTree filters by.
func newWalk(start string) *tree {
	walked := &tree{start: start, canonicalRoot: rootName(start), suffixes: map[string][]string{}}
	if info, err := os.Lstat(start); err == nil {
		walked.add(walked.newEntry(start, info.Mode()))
		if info.IsDir() {
			walked.descend(start)
		}
	}
	return walked
}

// The directory name every spelling of start shares: `ai` for `ai`, `ai/`, `./ai`, the absolute path,
// and `.` run from inside that directory. filepath.Abs cleans the path and answers from the process
// working directory, which is how a relative root is read everywhere else in this tool.
//
// The root's own directory name, and nothing above it. Anchor this any higher — on the cleaned
// absolute path, say — and every ancestor of the checkout becomes a key in the index below. A
// citation naming one would resolve, and the report's silence would tell the branch's author what the
// reviewing machine's path is called. That is underRoot's probe again, arriving through the index
// instead of through a stat. TestACitationNamingADirectoryAboveTheRootDoesNotResolve holds it shut.
//
// The root's own name does leak. A citation whose first component guesses it right resolves and a
// wrong guess is reported, so one guess confirms the name. That is the whole of the residue: one
// low-value directory name, never an ancestor of it. Reading it back a character class at a time went
// with the pattern arm — citations.go → globInCitation refuses a glob before the index is asked.
//
// Symlinks are not resolved here. walkTree lstats the start, so a symlinked root is never descended
// into and its index stays empty whatever name this returns — resolving would only put this function
// at odds with the walk beside it.
func rootName(start string) string {
	absolute, err := filepath.Abs(start)
	if err != nil {
		return shell.BaseName(start)
	}
	return shell.BaseName(absolute)
}

// One walked entry, under both names. The relative half is a prefix cut rather than a path
// operation: every path the walk builds is start plus concatenated components, so start is a literal
// prefix of it. A spelling that leaves a doubled separator behind (`ai/` yields `ai//gate.sh`) loses
// it here with the rest of the leading slashes.
func (t *tree) newEntry(path string, mode fs.FileMode) fsEntry {
	rel := strings.TrimLeft(strings.TrimPrefix(path, t.start), "/")
	canonical := t.canonicalRoot
	if rel != "" {
		canonical = shell.Join(t.canonicalRoot, rel)
	}
	return fsEntry{path: path, canonical: canonical, mode: mode}
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
		path := shell.Join(dir, entry.Name())
		t.add(t.newEntry(path, info.Mode()))
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
	// The canonical name is a key in its own right, not only its tails. Every tail below begins
	// after a `/`, so none of them starts at the root's own directory name — and a citation written
	// from the repo root, such as `ai/gate.sh` cited in a file under `ai/`, names exactly that.
	t.suffixes[entry.canonical] = append(t.suffixes[entry.canonical], entry.path)
	for i := 0; i < len(entry.canonical); i++ {
		if entry.canonical[i] == '/' {
			tail := entry.canonical[i+1:]
			t.suffixes[tail] = append(t.suffixes[tail], entry.path)
		}
	}
}

// The regular files a cited token names, in walk order: the files whose canonical name the token is,
// and those whose canonical name ends with it after a `/`.
//
// The index answers this whole question, because no token that reaches here holds a pattern. Every
// scan's token charset excludes `*?[\`, and the one scan that did not exclude them — the citation
// scan — now refuses them outright (citations.go → globInCitation).
func (t *tree) matchPath(ref string) []string {
	return t.suffixes[ref]
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

// The same files, with each one's lines already read. Every scan walks a set of files and reads each
// one, and every one of them made the same call about a file it cannot read: skip it. The reviewed
// tree chooses what is unreadable, so a scan that stopped there would be one a branch could switch
// off. The call is made once here, and a scan added later inherits it.
//
// Skipped, never in silence. readLines names the file before it hands the error back, so the skip
// costs a finding and no run can exit 0 over it; what a quiet one cost is at couldNotRead in shell.go.
// A file over the read bound comes back with no lines and no error instead, so a scan sees an empty
// file rather than one it never heard about.
func (c *checker) filesWithLines(start string, globs ...string) iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		for _, file := range c.filesNamed(start, globs...) {
			lines, err := c.readLines(file)
			if err != nil {
				continue
			}
			if !yield(file, lines) {
				return
			}
		}
	}
}

// Whether a path the reviewed tree wrote stays under the root, answered lexically and never by asking
// the filesystem — because asking is the leak. Every ref below is written by the branch under review,
// and `..` is in the charset of all three patterns that produce one. So a committed
// `](../../../../Users/someone/.ssh/id_rsa)` used to be stat'ed. The scan flags a link only when its
// target is missing, which turned the report into an oracle: its silence told the branch's author
// which files the reviewing machine holds. An out-of-root ref now resolves nowhere whether or not it
// is there, and that one answer either way is the whole of the fix.
//
// Lexical, so a symlinked directory committed inside the root can still redirect the stat that
// follows; what this closes is the arbitrary-path probe, not every path question.
func (c *checker) underRoot(path string) bool {
	rel, err := filepath.Rel(c.root.Named(), path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// shell.PathExists, asked only where the answer is about this tree. The gate term is not a second
// filter but the same one: resolveRef spells a path out of prose instead of taking it from the walk,
// so without it a citation still resolves through a gitignored file the walk has already dropped.
func (c *checker) existsUnderRoot(path string) bool {
	return c.underRoot(path) && !c.isSkippedByGate(path) && shell.PathExists(path)
}

// A path as prose writes it, resolved to a file under the root; empty when it resolves nowhere or
// to more than one file.
func (c *checker) resolveRef(dir, ref string) string {
	if rest, ok := strings.CutPrefix(ref, "~/.kk-flavor/"); ok {
		return c.existingOrEmpty(shell.Join(c.root.Flavor(), rest))
	}
	if rest, ok := strings.CutPrefix(ref, "~/.claude/skills/"); ok {
		return c.existingOrEmpty(shell.Join(c.root.Skills(), rest))
	}
	if dir != "" && c.existsUnderRoot(shell.Join(dir, ref)) {
		return shell.Join(dir, ref)
	}
	if c.existsUnderRoot(shell.Join(c.root.Named(), ref)) {
		return shell.Join(c.root.Named(), ref)
	}
	// A bare name is accepted only when one file in the tree could be meant. Counted in lines
	// rather than in paths, as the shell version counted them: a committed directory name holding a
	// newline splits one match across two lines, and reading that as ambiguous is the safe half.
	joined := strings.Join(c.walkTree(c.root.Named()).matchPath(ref), "\n")
	if countNonEmptyLines(joined) == 1 {
		return joined
	}
	return ""
}

func (c *checker) existingOrEmpty(path string) string {
	if c.existsUnderRoot(path) {
		return path
	}
	return ""
}

// True when a cited path names at least one real file, an ambiguous bare name included.
func (c *checker) refExists(dir, ref string) bool {
	if c.resolveRef(dir, ref) != "" {
		return true
	}
	return len(c.walkTree(c.root.Named()).matchPath(ref)) > 0
}

// The entries of skills/ that stat as directories, dotfiles excluded and byte-sorted the way the
// shell's `"$skills"/*/` glob produced them.
//
// Read straight off the directory rather than out of the walk, so the gate term is asked here too.
// The mount scan lists skills from this, and it runs only in the install: a gitignored skill
// directory would be `skill not mounted` there and nothing at all in a worktree of the same commit,
// which is the exact split the flag exists to close.
// One skill's SKILL.md. Named rather than joined at each of the four call sites: the layout is one
// fact, and four copies of it are four places a rename has to reach.
func (c *checker) skillFilePath(name string) string {
	return shell.Join(shell.Join(c.root.Skills(), name), "SKILL.md")
}

func (c *checker) skillDirNames() []string {
	entries, err := os.ReadDir(c.root.Skills())
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := shell.Join(c.root.Skills(), entry.Name())
		if c.isSkippedByGate(dir) {
			continue
		}
		if shell.IsDir(dir) {
			names = append(names, entry.Name())
		}
	}
	return names
}
