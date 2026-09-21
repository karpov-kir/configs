// Package repotest is the in-memory repository the suites drive instead of forking git.
//
// It answers from a table and does NOT model git: the revision grammar, the index and merging are all
// absent. A case says what git would answer, and the code under test is driven against that. A fake built
// any other way agrees with itself (`testing.md` rule 5). repo.Exec is what could disagree with real git,
// and `repo/exec_test.go` drives it against one. A path is repository-relative, and content is held per
// revision, the empty one meaning the working tree. The `dir` every method takes is the directory git
// would have run in, and it is NOT decoration. It is safe for concurrent use: a case may drive two
// invocations against one table under t.Parallel(), so every method here takes the lock.
package repotest

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// WorkTree is the revision name for what is on disk. It is the empty string, because that is what
// repo.Git's callers pass for it.
const WorkTree = ""

// Fake answers repo.Git from what a case put in it. The zero value is an empty repository with an
// empty root, and New gives it a root and the usual answers.
type Fake struct {
	// Every answer and every builder below takes this, so a case may drive two invocations against one
	// table. It guards the fields too: a builder called while another goroutine reads is the same race.
	mu sync.Mutex

	// Root is what TopLevel answers, and the directory every other answer is anchored to.
	Root string
	// Git is what GitDir answers, and CommonDir too unless Common is set.
	Git string
	// Common is the store linked worktrees share, and empty means Git.
	Common string
	// PrefixByDir is what a directory's path below Root is, for a case that spells its directories some
	// way prefixOf, a method of Fake, cannot read. Every directory under Root places itself while this is
	// empty.
	PrefixByDir map[string]string
	// Trees is what git answers PER DIRECTORY, for a suite driving a sibling worktree, a subdirectory that
	// answers its worktree's root, or a forged `.git/worktrees/` entry naming another clone. NIL means
	// every directory is the single repository Root and Common describe, and New leaves it NIL.
	// PerDirectory turns it on, and a directory ABSENT from it then gets `rev-parse`'s own failure.
	Trees map[string]Tree
	// Config answers ConfigValue. A key PRESENT here is set whatever its value, including the empty
	// string, and a key absent is unset. The two are opposite answers.
	Config map[string]string

	// Revs maps a revision name to the files it holds. Revs[WorkTree] is the working tree.
	Revs map[string]map[string]string
	// Refs resolves a revision name to an object id. A name absent here resolves to the empty string,
	// which is what `rev-parse --verify --quiet` answers for an unborn HEAD.
	Refs map[string]string
	// Bases answers MergeBase, keyed "left\x00right" and consulted in both orders.
	Bases map[string]string

	// UntrackedPaths are present in the working tree, with the index holding none of them. One that
	// IgnoredPaths covers leaves the Untracked listing, because `--exclude-standard` is on it.
	UntrackedPaths []string
	// IgnoredPaths are what git would ignore, and Sources names the rule behind each.
	IgnoredPaths []string
	Sources      map[string]string
	// Changes answers ChangedWithStatus for one spelling of the revisions, verbatim, for a case whose
	// subject the derived comparison cannot know: a mode, or the blob a side held. The key is the
	// revisions joined by a space.
	Changes map[string][]repo.Change
	// PatchText is what Patch answers, verbatim — the diff a case says git would print. NOT derived
	// from Revs: a derivation would put a diff implementation in this fake, and a suite driven by that
	// agrees with the fake.
	PatchText string
	// PatchByRevisions answers Patch for one spelling of the revisions, where a case drives more than
	// one change set. The key is the revisions joined by a space, and PatchText answers anything absent.
	PatchByRevisions map[string]string
	// StatusLines is what `status --porcelain -uall` prints, verbatim.
	StatusLines []string
	// WorktreeList is what `worktree list` prints.
	WorktreeList []repo.Worktree

	// Fail makes the named method return this error, so a case can drive the refusal path without
	// arranging a broken repository. The key is the method name, e.g. "TopLevel".
	Fail map[string]error

	// onDisk says the working tree is real, and OnDisk sets it. Three answers then come off the
	// filesystem rather than out of a table, because no table can hold them: what is on disk and
	// outside the index, what an `add` swept up, and what a `.gitignore` written MID-RUN says.
	onDisk bool

	// Added records every path Add staged, in call order, so a case can assert on a write without a
	// repository to inspect.
	Added []string
	// Staged is every file an Add really put in the index, in call order. Added holds the pathspecs a
	// caller NAMED; this holds what they swept up, which is what `diff --cached --name-only` answers
	// and the only one of the two that tells a directory of ignored files from a directory of work.
	// Filled in OnDisk mode, where there is a tree to sweep.
	Staged []string
	// Asked records every method called, in order, for a case whose subject is whether something was
	// asked at all — a memo, or a listing taken once per run.
	Asked []string
}

// Tree is what git answers about one directory: the working tree root it sits in, and the store its
// clone shares. Both are spelled out, because a tree that inherited either could not name a directory
// belonging to a DIFFERENT clone.
type Tree struct {
	Root   string
	Common string
}

var _ repo.Git = (*Fake)(nil)

// New is an empty repository rooted at root with one commit. A tool that refuses an empty history then
// needs no setup to get past. Commit("HEAD", …) fills it.
func New(root string) *Fake {
	return &Fake{
		Root:        root,
		Git:         path.Join(root, ".git"),
		PrefixByDir: map[string]string{},
		Config:      map[string]string{},
		Revs:        map[string]map[string]string{WorkTree: {}},
		Refs:        map[string]string{},
		Bases:       map[string]string{},
		Sources:     map[string]string{},
		Fail:        map[string]error{},
	}
}

// OnDisk makes the repository real to everything that looks at the filesystem rather than at this
// fake: the git dir exists and holds a HEAD. Those two are what a tool pointed at a directory checks
// for itself — `repo-key` refuses a git dir with no HEAD in it, and a record is written INTO the git
// dir — and the answers around them stay this fake's. Three suites built the same two by hand.
//
// The root comes back resolved, and Root and Git are rewritten to the resolved spelling, because
// os.MkdirTemp hands back a symlinked path on macOS: a case quoting the path it created would not
// match the one the code under test resolved.
func (f *Fake) OnDisk() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(f.Git, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.Git, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(f.Root)
	if err != nil {
		return err
	}
	f.Root, f.Git = resolved, filepath.Join(resolved, ".git")
	f.onDisk = true
	return nil
}

// PerDirectory makes every answer about a directory a declared one. A directory none of the builders
// has named is then NO repository, which is what a project someone is merely trying a tool out in
// looks like. One Root answering for every directory leaves a suite driving several trees unable to
// say that.
func (f *Fake) PerDirectory() *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Trees == nil {
		f.Trees = map[string]Tree{}
	}
	return f
}

// WorktreeAt declares one worktree of this clone: git answers for the directory, it shares this
// fake's store, and `worktree list` names it. A case calls this after Common is set, since that is the
// store it records.
func (f *Fake) WorktreeAt(dir string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.treeAt(dir, dir, f.sharedDir())
	f.WorktreeList = append(f.WorktreeList, repo.Worktree{Path: dir})
	return f
}

// ForeignWorktreeAt is a directory git answers for that belongs to a DIFFERENT clone, listed among
// this one's worktrees. A forged `.git/worktrees/` entry naming an unrelated repository looks like
// this from here. The store it names is the whole tell, so the caller spells it.
func (f *Fake) ForeignWorktreeAt(dir, common string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.treeAt(dir, dir, common)
	f.WorktreeList = append(f.WorktreeList, repo.Worktree{Path: dir})
	return f
}

// TreeAt is a directory INSIDE a worktree, which `rev-parse` answers that worktree's root for. It
// stays out of the listing, because git lists worktrees and leaves the directories under them out.
func (f *Fake) TreeAt(dir, root string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.treeAt(dir, root, f.sharedDir())
	return f
}

func (f *Fake) treeAt(dir, root, common string) {
	if f.Trees == nil {
		f.Trees = map[string]Tree{}
	}
	f.Trees[dir] = Tree{Root: root, Common: common}
}

// LinkedWorktreeAt is a SECOND PORT, answering for another checkout of this clone: `git worktree
// add`'s result, from a tool that is handed one repo.Git per directory it stands in. WorktreeAt is
// the other shape, and the two are not interchangeable — that one declares a worktree inside THIS
// fake's per-directory table, for a tool that asks one port about several directories.
//
// The worktree's git dir is its own, because that is where its identity is minted. Everything else
// is the clone's: the common dir, the history, the ignore sources a tracked `.gitignore` gives every
// checkout, and the refusal switch — a case turning one question off means it off wherever that
// repository is asked. Those four are shared LIVE, so a commit or a refusal arranged after this call
// is answered from both sides, which is what makes this a worktree rather than a second clone.
//
// IgnoredPaths and WorktreeList are slices and cannot be shared that way: what a case stated before
// this call comes across, and a rule or a listing it states afterwards is stated on the tree that
// reads it.
func (f *Fake) LinkedWorktreeAt(dir, gitDir string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	linked := New(dir)
	linked.Git, linked.Common = gitDir, f.sharedDir()
	linked.Revs, linked.Refs, linked.Sources, linked.Fail, linked.Config = f.Revs, f.Refs, f.Sources, f.Fail, f.Config
	linked.IgnoredPaths = append([]string(nil), f.IgnoredPaths...)
	linked.onDisk = f.onDisk
	return linked
}

// Diff sets what Patch answers, for every revision spelling a case does not name on its own.
func (f *Fake) Diff(text string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PatchText = text
	return f
}

// Commit puts files at a revision and gives that revision an object id, so Resolve answers for it.
// The working tree is not touched: a case that wants the same content on disk calls Write too.
func (f *Fake) Commit(rev string, files map[string]string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Revs == nil {
		f.Revs = map[string]map[string]string{}
	}
	held := map[string]string{}
	for name, body := range files {
		held[name] = body
	}
	f.Revs[rev] = held
	if f.Refs == nil {
		f.Refs = map[string]string{}
	}
	if f.Refs[rev] == "" {
		f.Refs[rev] = objectID(rev)
	}
	// git answers about a commit by its object id as readily as by the name that reached it, and a
	// tool that resolves a base-ref and then diffs against the ID asks the second question that way.
	// The same map, so the two spellings cannot drift into two commits.
	if id := f.Refs[rev]; id != rev {
		f.Revs[id] = held
		f.Refs[id] = id
	}
	return f
}

// Write puts one file in the working tree and the index.
func (f *Fake) Write(name, body string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Revs == nil {
		f.Revs = map[string]map[string]string{}
	}
	if f.Revs[WorkTree] == nil {
		f.Revs[WorkTree] = map[string]string{}
	}
	f.Revs[WorkTree][name] = body
	return f
}

func (f *Fake) AddUntracked(names ...string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UntrackedPaths = append(f.UntrackedPaths, names...)
	return f
}

// Ignore marks paths ignored, with source as what `check-ignore -v` would print for each. A path the
// working tree also TRACKS stays unignored however this is called, because git does not call a tracked
// file ignored. isIgnored, a method of Fake, is where that is decided.
func (f *Fake) Ignore(source string, names ...string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.IgnoredPaths = append(f.IgnoredPaths, names...)
	if f.Sources == nil {
		f.Sources = map[string]string{}
	}
	for _, name := range names {
		f.Sources[name] = source
	}
	return f
}

// A stand-in for a git object id: forty hex characters derived from the name, so two revisions differ
// and one revision is stable across calls. No caller here depends on it being git's own hash.
func objectID(of string) string {
	sum := sha1.Sum([]byte(of))
	return hex.EncodeToString(sum[:])
}

func (f *Fake) note(method string) error {
	f.Asked = append(f.Asked, method)
	return f.Fail[method]
}

func (f *Fake) TopLevel(dir string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("TopLevel"); err != nil {
		return "", err
	}
	tree, err := f.treeOf(dir)
	return tree.Root, err
}

func (f *Fake) CommonDir(dir string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("CommonDir"); err != nil {
		return "", err
	}
	tree, err := f.treeOf(dir)
	return tree.Common, err
}

func (f *Fake) ConfigValue(dir, key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// No error to give: "unset" is an answer here, and Fail has no say over it.
	_ = f.note("ConfigValue")
	value, isSet := f.Config[key]
	return value, isSet
}

// What git answers about dir, or the failure it gives for a directory that is no repository.
func (f *Fake) treeOf(dir string) (Tree, error) {
	if f.Trees == nil {
		return Tree{Root: f.Root, Common: f.sharedDir()}, nil
	}
	tree, known := f.Trees[dir]
	if !known {
		return Tree{}, fmt.Errorf("the fake holds no worktree at %s", dir)
	}
	return tree, nil
}

// The store this clone shares. It is the git dir itself unless a case gave the clone one.
func (f *Fake) sharedDir() string {
	if f.Common != "" {
		return f.Common
	}
	return f.Git
}

func (f *Fake) GitDir(dir string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("GitDir"); err != nil {
		return "", err
	}
	return f.Git, nil
}

func (f *Fake) GitPath(dir, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("GitPath"); err != nil {
		return "", err
	}
	return path.Join(f.Git, name), nil
}

func (f *Fake) Prefix(dir string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Prefix"); err != nil {
		return "", err
	}
	// The same reading the listings use, so a case cannot be told dir sits at the root here and under
	// pkg/ there.
	return f.prefixOf(dir), nil
}

func (f *Fake) Resolve(dir, rev string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Resolve"); err != nil {
		return "", err
	}
	// git peels `^{}` and `^{commit}` off a name it already knows, so a case naming HEAD gets an
	// answer whether or not it spelled the suffix.
	name := rev
	if at := strings.Index(name, "^{"); at >= 0 {
		name = name[:at]
	}
	return f.Refs[name], nil
}

func (f *Fake) MergeBase(dir, left, right string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("MergeBase"); err != nil {
		return "", err
	}
	if base, found := f.Bases[left+"\x00"+right]; found {
		return base, nil
	}
	if base, found := f.Bases[right+"\x00"+left]; found {
		return base, nil
	}
	return "", fmt.Errorf("the fake holds no merge base for %s and %s", left, right)
}

func (f *Fake) Tracked(dir string, pathspec ...string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Tracked"); err != nil {
		return nil, err
	}
	return underPathspec(sortedKeys(f.Revs[WorkTree]), f.listingPathspec(dir, pathspec)), nil
}

func (f *Fake) Untracked(dir string, pathspec ...string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Untracked"); err != nil {
		return nil, err
	}
	// `--exclude-standard` is on this listing, so an ignored path stays out of it. A case stating a path
	// both untracked and ignored gets git's answer, and a caller is never handed a file git would have
	// left out.
	names, err := f.untrackedNames()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return underPathspec(names, f.listingPathspec(dir, pathspec)), nil
}

// What `ls-files --others --exclude-standard` answers: on disk, outside the index, and unignored.
// In OnDisk mode that is read off the tree, and never off a list a case has to keep in step with it
// — a case cannot then write a file and forget to mention it, which reads as an empty scope. With no
// tree to read, the list is what the case stated.
func (f *Fake) untrackedNames() ([]string, error) {
	if !f.onDisk {
		var stated []string
		for _, name := range f.UntrackedPaths {
			if !f.isIgnored(name) {
				stated = append(stated, name)
			}
		}
		return stated, nil
	}
	var names []string
	err := filepath.WalkDir(f.Root, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, under := strings.CutPrefix(full, f.Root+string(filepath.Separator))
		if !under {
			return nil
		}
		name = filepath.ToSlash(name)
		if entry.IsDir() {
			// The git dir is not the working tree, and git never lists a path inside it.
			if name == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if _, tracked := f.Revs[WorkTree][name]; tracked {
			return nil
		}
		if f.isIgnored(name) {
			return nil
		}
		names = append(names, name)
		return nil
	})
	return names, err
}

func (f *Fake) NamesAt(dir, rev string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("NamesAt"); err != nil {
		return nil, err
	}
	held, known := f.Revs[rev]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s", rev)
	}
	return sortedKeys(held), nil
}

// The changed set is what the two sides hold, compared. A case names the revisions it drives with and
// fills them through Commit. The empty revision list compares the working tree against HEAD, which is
// what git does.
func (f *Fake) Changed(dir string, revisions, pathspec []string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Changed"); err != nil {
		return nil, err
	}
	changes, err := f.diff(revisions)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, one := range changes {
		if one.Status != "D" {
			names = append(names, one.Path)
		}
	}
	return underPathspec(names, f.diffPathspec(dir, pathspec)), nil
}

func (f *Fake) ChangedWithStatus(dir string, revisions, pathspec []string) ([]repo.Change, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("ChangedWithStatus"); err != nil {
		return nil, err
	}
	changes, found := f.Changes[strings.Join(revisions, " ")]
	if !found {
		var err error
		if changes, err = f.diff(revisions); err != nil {
			return nil, err
		}
	}
	narrowed := f.diffPathspec(dir, pathspec)
	var kept []repo.Change
	for _, one := range changes {
		if matchesPathspec(one.Path, narrowed) {
			kept = append(kept, one)
		}
	}
	return kept, nil
}

// What a derived change calls a file. This fake holds content and knows no modes. A case whose subject
// is a mode states the change itself through Changes: a `.md` that was executable at the base, or a
// symlink that became a regular file.
const ordinaryFileMode = "100644"

// Two revisions' file maps compared, which is all a diff is to the code under test: a status letter
// per path. `a..b` and `a b` name the same pair, and the empty list means HEAD against the tree.
func (f *Fake) diff(revisions []string) ([]repo.Change, error) {
	left, right := "HEAD", WorkTree
	switch len(revisions) {
	case 0:
	case 1:
		spelt := revisions[0]
		if before, after, closed := strings.Cut(spelt, ".."); closed {
			left, right = strings.TrimSuffix(before, "."), strings.TrimPrefix(after, ".")
			if left == "" {
				left = "HEAD"
			}
			if right == "" {
				right = "HEAD"
			}
		} else {
			left = spelt
		}
	default:
		left, right = revisions[0], revisions[1]
	}
	from, known := f.Revs[left]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s to diff from", left)
	}
	to, known := f.Revs[right]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s to diff to", right)
	}
	seen := map[string]bool{}
	var changes []repo.Change
	for _, name := range sortedKeys(to) {
		seen[name] = true
		switch was, held := from[name]; {
		case !held:
			changes = append(changes, repo.Change{Status: "A", Path: name,
				NewMode: ordinaryFileMode, Blob: objectID(to[name])})
		case was != to[name]:
			changes = append(changes, repo.Change{Status: "M", Path: name,
				OldMode: ordinaryFileMode, NewMode: ordinaryFileMode,
				OldBlob: objectID(was), Blob: objectID(to[name])})
		}
	}
	for _, name := range sortedKeys(from) {
		if !seen[name] {
			changes = append(changes, repo.Change{Status: "D", Path: name,
				OldMode: ordinaryFileMode, OldBlob: objectID(from[name])})
		}
	}
	return changes, nil
}

func (f *Fake) Patch(dir string, revisions, pathspec []string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Patch"); err != nil {
		return nil, err
	}
	if text, found := f.PatchByRevisions[strings.Join(revisions, " ")]; found {
		return []byte(text), nil
	}
	return []byte(f.PatchText), nil
}

func (f *Fake) Status(dir string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Status"); err != nil {
		return nil, err
	}
	return append([]string(nil), f.StatusLines...), nil
}

func (f *Fake) Show(dir, rev, name string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Show"); err != nil {
		return nil, err
	}
	held, known := f.Revs[rev]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s", rev)
	}
	body, held2 := held[name]
	if !held2 {
		return nil, fmt.Errorf("%s holds no %s", rev, name)
	}
	return []byte(body), nil
}

// ContentsAt reads the revision's table in the order the paths were asked. A path the revision does
// not hold is passed over, and so is one over the cap. That is how a case drives the difference
// between a missing file and one the revision holds empty. visit runs with the lock RELEASED, so a
// caller asking this fake another question from inside visit gets an answer.
func (f *Fake) ContentsAt(dir, rev string, paths []string, maxBytes int64, visit func(string, []byte)) error {
	found, err := f.contentsAt(rev, paths, maxBytes)
	if err != nil {
		return err
	}
	for _, one := range found {
		visit(one.path, []byte(one.body))
	}
	return nil
}

type heldFile struct{ path, body string }

func (f *Fake) contentsAt(rev string, paths []string, maxBytes int64) ([]heldFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("ContentsAt"); err != nil {
		return nil, err
	}
	held, known := f.Revs[rev]
	if !known {
		return nil, fmt.Errorf("the fake holds no revision %s", rev)
	}
	var found []heldFile
	for _, name := range paths {
		body, holds := held[name]
		if !holds || int64(len(body)) > maxBytes {
			continue
		}
		found = append(found, heldFile{name, body})
	}
	return found, nil
}

// Blobs are looked up by the id diff handed out, so a case reads back what it wrote without knowing
// how an id is made.
func (f *Fake) Blob(dir, id string) ([]byte, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Blob"); err != nil {
		return nil, 0, err
	}
	for _, held := range f.Revs {
		for _, body := range held {
			if objectID(body) == id {
				return []byte(body), int64(len(body)), nil
			}
		}
	}
	return nil, 0, fmt.Errorf("the fake holds no blob %s", id)
}

func (f *Fake) Ignored(dir string, paths []string) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Ignored"); err != nil {
		return nil, err
	}
	ignored := map[string]bool{}
	for _, name := range paths {
		if f.isIgnored(name) {
			ignored[name] = true
		}
	}
	return ignored, nil
}

func (f *Fake) IgnoreSource(dir, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("IgnoreSource"); err != nil {
		return "", err
	}
	// `check-ignore` exits 1 for a path no rule ignores, and this returns empty there. A tracked path
	// counts as one of those however a rule reads, so a case marking a committed file ignored gets
	// git's empty answer.
	if !f.isIgnored(name) {
		return "", nil
	}
	if source, matched := f.gitignoreSource(name); matched {
		return source, nil
	}
	return f.Sources[name], nil
}

// What the tree's own `.gitignore` says about a path, read at the moment of the question. A tool
// that writes a rule and then asks whether it took effect is the case this exists for: an answer
// arranged before the run matches whatever the tool wrote, and can never fail.
//
// Only literal glob matching with git's last-match-wins negation is modelled. A gitignore
// implementation here would be the fake agreeing with itself about a question `repo/exec_test.go`
// holds real git to; a case wanting any other rule — `.git/info/exclude`, core.excludesFile — states
// the answer with Ignore, and the file is consulted first because that is git's precedence.
func (f *Fake) gitignoreSource(name string) (string, bool) {
	if !f.onDisk {
		return "", false
	}
	body, err := os.ReadFile(filepath.Join(f.Root, ".gitignore"))
	if err != nil {
		return "", false
	}
	source, matched := "", false
	for at, line := range strings.Split(string(body), "\n") {
		rule := strings.TrimSpace(line)
		if rule == "" || strings.HasPrefix(rule, "#") {
			continue
		}
		if !ruleCovers(strings.TrimPrefix(rule, "!"), name) {
			continue
		}
		matched = true
		// `check-ignore -v` names the file, the line and the rule that decided. A negated rule leaves
		// the path unignored, which is the empty answer and not a source.
		source = fmt.Sprintf(".gitignore:%d:%s\t%s", at+1, rule, name)
		if strings.HasPrefix(rule, "!") {
			source = ""
		}
	}
	return source, matched
}

// What one rule covers: the glob, or a directory every path under it. One reading, so a rule a case
// stated and a rule the tree's own `.gitignore` holds cannot be answered two ways.
func ruleCovers(rule, name string) bool {
	rule = strings.TrimSuffix(rule, "/")
	if hit, _ := path.Match(rule, name); hit {
		return true
	}
	// A rule carrying no slash is held against the BASENAME at any depth, which is git's own reading:
	// `*.out` covers records/build.out, and a rule matched against the whole path alone would not.
	if !strings.Contains(rule, "/") {
		if hit, _ := path.Match(rule, path.Base(name)); hit {
			return true
		}
	}
	return shell.IsWithin(name, rule)
}

func (f *Fake) Worktrees(dir string) ([]repo.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Worktrees"); err != nil {
		return nil, err
	}
	return append([]repo.Worktree(nil), f.WorktreeList...), nil
}

// Add records the pathspecs it was asked for, and in OnDisk mode stages what they swept up. The
// difference is the whole of what a caller reads back afterwards: `git add` over a directory whose
// every file is ignored stages nothing and still exits 0, so a fake recording only the request
// answers that the work is still outside the index, and every promotion built on that refuses.
func (f *Fake) Add(dir string, paths []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Add"); err != nil {
		return err
	}
	f.Added = append(f.Added, paths...)
	if !f.onDisk {
		return nil
	}
	// A path NAMED to `git add` and covered by a rule is refused outright, where the same file swept
	// up by a directory pathspec is passed over in silence. A caller naming its files one by one is
	// counting on that refusal, and one that hands over a directory is counting on the silence.
	for _, spec := range paths {
		name := f.namedInTree(dir, spec)
		if f.isIgnored(name) && isFile(filepath.Join(f.Root, name)) {
			return fmt.Errorf("The following paths are ignored by one of your .gitignore files:\n%s\n"+
				"hint: Use -f if you really want to add them.", spec)
		}
	}
	for _, spec := range paths {
		if err := f.stageUnder(f.namedInTree(dir, spec)); err != nil {
			return err
		}
	}
	return nil
}

// One pathspec as a repository-relative name. A caller spells it absolute or relative to the
// directory git was run in, and git reads both.
func (f *Fake) namedInTree(dir, spec string) string {
	if below, under := strings.CutPrefix(spec, f.Root+"/"); under {
		return below
	}
	return path.Clean(f.prefixOf(dir) + spec)
}

// Everything under one pathspec put in the index, with what is on disk at each. A rule covering a
// file swept up this way takes it out, which is git's silence above.
func (f *Fake) stageUnder(name string) error {
	return filepath.WalkDir(filepath.Join(f.Root, filepath.FromSlash(name)), func(full string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			// A pathspec naming nothing is not an error to `git add`, and neither is a directory.
			return nil
		}
		under, inside := strings.CutPrefix(full, f.Root+string(filepath.Separator))
		if !inside {
			return nil
		}
		held := filepath.ToSlash(under)
		if held == ".git" || strings.HasPrefix(held, ".git/") || f.isIgnored(held) {
			return nil
		}
		body, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		if f.Revs[WorkTree] == nil {
			f.Revs[WorkTree] = map[string]string{}
		}
		f.Revs[WorkTree][held] = string(body)
		f.Staged = append(f.Staged, held)
		return nil
	})
}

func isFile(full string) bool {
	info, err := os.Lstat(full)
	return err == nil && info.Mode().IsRegular()
}

// dir as a repository-relative prefix, slash-terminated and empty at the root — git's own
// `rev-parse --show-prefix`. A case that spells its directories some other way states them in
// PrefixByDir, which wins here. A directory this cannot place under Root is read as the root.
func (f *Fake) prefixOf(dir string) string {
	if prefix, stated := f.PrefixByDir[dir]; stated {
		return prefix
	}
	if f.Root == "" || dir == "" || dir == f.Root {
		return ""
	}
	if below, under := strings.CutPrefix(dir, f.Root+"/"); under {
		return below + "/"
	}
	return ""
}

// `ls-files`' reading of dir: the listing covers only what sits under dir, and each spec is relative
// to dir. Both come out as one repository-relative pathspec.
func (f *Fake) listingPathspec(dir string, pathspec []string) []string {
	prefix := f.prefixOf(dir)
	if len(pathspec) == 0 {
		return []string{strings.TrimSuffix(prefix, "/")}
	}
	return rootedPathspec(prefix, pathspec)
}

// `diff`'s reading of the same two, which differs in the half that matters: the comparison is the
// whole tree wherever git ran, and only a pathspec is relative to dir.
func (f *Fake) diffPathspec(dir string, pathspec []string) []string {
	if len(pathspec) == 0 {
		return nil
	}
	return rootedPathspec(f.prefixOf(dir), pathspec)
}

// Each spec resolved against the directory git ran in, which is what makes `pkg` asked from inside
// pkg/ name pkg/pkg and match no file.
func rootedPathspec(prefix string, pathspec []string) []string {
	rooted := make([]string, 0, len(pathspec))
	for _, spec := range pathspec {
		switch {
		case spec == "":
			rooted = append(rooted, strings.TrimSuffix(prefix, "/"))
		case strings.HasPrefix(spec, "/"):
			rooted = append(rooted, path.Clean(spec))
		default:
			rooted = append(rooted, path.Clean(prefix+spec))
		}
	}
	return rooted
}

// What git would ignore, which differs from what a rule matches. A TRACKED path is never ignored
// whatever rule covers it, and `check-ignore` says so. exec_test.go holds that against a real git. A
// caller filtering on the other answer drops a file every commit carries.
func (f *Fake) isIgnored(name string) bool {
	if _, tracked := f.Revs[WorkTree][name]; tracked {
		return false
	}
	if source, matched := f.gitignoreSource(name); matched {
		return source != ""
	}
	for _, rule := range f.IgnoredPaths {
		if ruleCovers(rule, name) {
			return true
		}
	}
	return false
}

// git's pathspec, as much of it as these tools use: a path, or a directory every path under it. An
// empty pathspec, and the empty spec a listing at the root produces, keep everything.
func underPathspec(names, pathspec []string) []string {
	var kept []string
	for _, name := range names {
		if matchesPathspec(name, pathspec) {
			kept = append(kept, name)
		}
	}
	return kept
}

func matchesPathspec(name string, pathspec []string) bool {
	if len(pathspec) == 0 {
		return true
	}
	for _, spec := range pathspec {
		spec = strings.TrimSuffix(spec, "/")
		if spec == "" || spec == "." || shell.IsWithin(name, spec) {
			return true
		}
	}
	return false
}

func sortedKeys(held map[string]string) []string {
	names := make([]string, 0, len(held))
	for name := range held {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
