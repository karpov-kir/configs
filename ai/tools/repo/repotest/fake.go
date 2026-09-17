// Package repotest is the in-memory repository the suites drive instead of forking git.
//
// It answers from a table. It does NOT model git: there is no revision grammar here, no index, no
// merge. A case says what git would answer and the code under test is driven against that, which is
// the only way a fake can disagree with the production code rather than agree with itself
// (`ai/kk-flavor/standards/testing.md` rule 5). What could disagree with real git is the one adapter,
// `repo.Exec`, and `repo/exec_test.go` drives that against a real repository.
//
// A path is repository-relative, exactly as git prints one. Content is held per revision, with the
// empty revision meaning the working tree.
//
// The `dir` every method takes is the directory git would have run in, and it is NOT decoration: a
// pathspec is relative to it, and `ls-files` asked from a subdirectory lists only what sits under that
// directory. prefixOf below is where dir turns into a repository-relative prefix.
//
// Safe for concurrent use. Suites run cases with t.Parallel() and a case may drive two invocations
// against one table, so every method here takes the lock.
package repotest

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// WorkTree is the revision name for what is on disk rather than in a commit. Spelt as the empty
// string because that is what repo.Git's callers pass for it.
const WorkTree = ""

// Fake answers repo.Git from what a case put in it. The zero value is an empty repository at "/repo"
// with no commit; New gives it a root and the usual answers.
type Fake struct {
	// Every answer and every builder below takes this, so a case may drive two invocations against one
	// table. It guards the fields too: a builder called while another goroutine reads is the same race.
	mu sync.Mutex

	// Root is what TopLevel answers, and the directory every other answer is anchored to.
	Root string
	// Git is what GitDir answers, and CommonDir too unless Common is set.
	Git string
	// Common is the store linked worktrees share; empty means Git.
	Common string
	// PrefixByDir is what a directory's path below Root is, for a case that spells its directories some
	// way prefixOf cannot read. Left empty, every directory under Root places itself.
	PrefixByDir map[string]string
	// Trees is what git answers PER DIRECTORY, for a suite driving more than one of them: a sibling
	// worktree, a subdirectory that answers its worktree's root, a forged `.git/worktrees/` entry
	// naming an unrelated clone. NIL, which is how New leaves it, means every directory is the one
	// repository Root and Common describe — what all but one suite drives. PerDirectory below turns it
	// on, and a directory then ABSENT from it is no repository at all, which is the failure
	// `rev-parse` gives and the only way a case says so about one directory and not another.
	Trees map[string]Tree
	// Config answers ConfigValue. A key PRESENT here is set whatever its value, including the empty
	// string; a key absent is unset. The two are opposite answers rather than degrees of one.
	Config map[string]string

	// Revs maps a revision name to the files it holds. Revs[WorkTree] is the working tree.
	Revs map[string]map[string]string
	// Refs resolves a revision name to an object id. A name absent here resolves to nothing, which is
	// what `rev-parse --verify --quiet` answers for an unborn HEAD.
	Refs map[string]string
	// Bases answers MergeBase, keyed "left\x00right" and consulted in both orders.
	Bases map[string]string

	// UntrackedPaths are present in the working tree and not in the index. One that IgnoredPaths covers
	// leaves the Untracked listing, because `--exclude-standard` is on it.
	UntrackedPaths []string
	// IgnoredPaths are what git would ignore, and Sources says which rule said so.
	IgnoredPaths []string
	Sources      map[string]string
	// Changes answers ChangedWithStatus for one spelling of the revisions, verbatim, where a case's
	// subject is something the derived comparison below cannot know: a mode, or which blob a side held.
	// The key is the revisions joined by a space.
	Changes map[string][]repo.Change
	// PatchText is what Patch answers, verbatim — the diff a case says git would print. NOT derived
	// from Revs: deriving it would put a diff implementation in this fake, and a suite driven by that
	// would be agreeing with the fake rather than with git.
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

	// Added records every path Add staged, in call order, so a case can assert on a write without a
	// repository to inspect.
	Added []string
	// Asked records every method called, in order, for a case whose subject is whether something was
	// asked at all — a memo, or a listing taken once per run.
	Asked []string
}

// Tree is what git answers about one directory: the working tree root it sits in, and the store its
// clone shares. Both are spelled out — a tree that inherited either would make the one case this
// exists for, a directory belonging to a DIFFERENT clone, unable to say so.
type Tree struct {
	Root   string
	Common string
}

var _ repo.Git = (*Fake)(nil)

// New is an empty repository rooted at root with one commit, so the common case — a tool that refuses
// where there is no commit — needs no setup to get past. Commit("HEAD", …) fills it.
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

// PerDirectory makes every answer about a directory a declared one. From here a directory none of the
// builders below has named is NO repository, which is what a project someone is merely trying a tool
// out in looks like — and a suite driving several trees has no way to say that while one Root answers
// for every directory it is asked about.
func (f *Fake) PerDirectory() *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Trees == nil {
		f.Trees = map[string]Tree{}
	}
	return f
}

// WorktreeAt declares one worktree of this clone: git answers for the directory, it shares this
// fake's store, and `worktree list` names it. Called after Common is set, since that is the store it
// records.
func (f *Fake) WorktreeAt(dir string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.treeAt(dir, dir, f.sharedDir())
	f.WorktreeList = append(f.WorktreeList, repo.Worktree{Path: dir})
	return f
}

// ForeignWorktreeAt is a directory git answers for that belongs to a DIFFERENT clone, listed among
// this one's worktrees — what a forged `.git/worktrees/` entry naming an unrelated repository looks
// like from here. The store it names is the whole tell, so it is the caller's to spell.
func (f *Fake) ForeignWorktreeAt(dir, common string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.treeAt(dir, dir, common)
	f.WorktreeList = append(f.WorktreeList, repo.Worktree{Path: dir})
	return f
}

// TreeAt is a directory INSIDE a worktree, which `rev-parse` answers that worktree's root for. Not
// added to the listing: git lists worktrees, never the directories under them.
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
// working tree also TRACKS is not ignored however this is called: git does not call a tracked file
// ignored, and isIgnored is where that is decided.
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
// and one revision is stable across calls. Nothing here depends on it being git's own hash.
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
	// No error to give: "unset" is an answer here, and Fail has nothing to say about it.
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

// The store this clone shares, which is the git dir itself unless a case gave the clone one.
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
	// `--exclude-standard` is on this listing, so an ignored path is not in it at all. A case stating a
	// path both untracked and ignored gets git's answer rather than its own, and a caller is never
	// handed a file to filter out that git would not have named.
	var names []string
	for _, name := range f.UntrackedPaths {
		if !f.isIgnored(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return underPathspec(names, f.listingPathspec(dir, pathspec)), nil
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
// fills them through Commit; the empty revision list compares the working tree against HEAD, which is
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

// What a derived change calls a file. A case whose subject is a mode — a `.md` that was executable at
// the base, a symlink that became a regular file — states the change itself through Changes rather
// than letting this derive one, because this fake holds content and knows nothing about modes.
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

// The revision's table, read in the order the paths were asked. A path the revision does not hold is
// passed over and so is one over the cap, which is how a case drives the difference between a file
// that is not there and one the revision holds empty.
//
// visit runs with the lock RELEASED, so a caller whose visit asks this fake another question is
// answered rather than deadlocked.
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
	// Empty where nothing ignores the path, which is `check-ignore`'s exit 1 — and a tracked path is
	// one of those however a rule reads, so a case marking a committed file ignored gets the empty
	// answer git gives.
	if !f.isIgnored(name) {
		return "", nil
	}
	return f.Sources[name], nil
}

func (f *Fake) Worktrees(dir string) ([]repo.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Worktrees"); err != nil {
		return nil, err
	}
	return append([]repo.Worktree(nil), f.WorktreeList...), nil
}

func (f *Fake) Add(dir string, paths []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.note("Add"); err != nil {
		return err
	}
	f.Added = append(f.Added, paths...)
	return nil
}

// dir as a repository-relative prefix, slash-terminated and empty at the root — git's own
// `rev-parse --show-prefix`. A case that spells its directories some other way states them in
// PrefixByDir, which wins here; a directory this cannot place under Root is read as the root.
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
// pkg/ name pkg/pkg and match nothing.
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

// What git would ignore, which is not the same as what a rule matches: a TRACKED path is never
// ignored whatever rule covers it, and `check-ignore` says so — exec_test.go holds that against a real
// git. A caller filtering on the other answer drops a file every commit carries.
func (f *Fake) isIgnored(name string) bool {
	if _, tracked := f.Revs[WorkTree][name]; tracked {
		return false
	}
	for _, rule := range f.IgnoredPaths {
		if shell.IsWithin(name, rule) {
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
